package football

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	// ErrUnauthorized means the service refused the API key, or the account.
	ErrUnauthorized = errors.New("football: the service refused the API key")
	// ErrRateLimited means the account is out of requests, for the minute or
	// for the day.
	ErrRateLimited = errors.New("football: the request limit was reached")
)

// maxIDsPerRequest is how many fixture ids the service takes in one call.
const maxIDsPerRequest = 20

// maxBody caps what is read from one answer.
const maxBody = 16 << 20

// APIFootball reads the API-Football v3 service. It implements [Source] and is
// safe for concurrent use.
type APIFootball struct {
	apiKey  string
	baseURL string
	client  *http.Client
	timeout time.Duration
	now     func() time.Time

	mu    sync.Mutex
	quota Quota
}

var _ Source = (*APIFootball)(nil)

// New builds the client. It makes no request.
func New(c Config) (*APIFootball, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, errors.New("football: Config.APIKey is empty")
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("football: Config.BaseURL is not an http or https address")
	}
	if c.Timeout < 0 {
		return nil, errors.New("football: Config.Timeout is negative")
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &APIFootball{
		apiKey:  strings.TrimSpace(c.APIKey),
		baseURL: base,
		client:  client,
		timeout: timeout,
		now:     time.Now,
	}, nil
}

// Quota returns the allowance reported by the most recent answer.
func (a *APIFootball) Quota() Quota {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.quota
}

// Leagues lists competitions whose name or country contains search.
func (a *APIFootball) Leagues(ctx context.Context, search string) ([]League, error) {
	params := url.Values{}
	if s := strings.TrimSpace(search); s != "" {
		if err := checkSearch(s); err != nil {
			return nil, err
		}
		params.Set("search", s)
	}
	var rows []struct {
		League struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"league"`
		Country struct {
			Name string `json:"name"`
		} `json:"country"`
		Seasons []struct {
			Year int `json:"year"`
		} `json:"seasons"`
	}
	if err := a.get(ctx, "/leagues", params, &rows); err != nil {
		return nil, err
	}
	out := make([]League, 0, len(rows))
	for _, row := range rows {
		league := League{ID: row.League.ID, Name: row.League.Name, Country: row.Country.Name, Logo: row.League.Logo}
		for _, season := range row.Seasons {
			league.Seasons = append(league.Seasons, season.Year)
		}
		out = append(out, league)
	}
	return out, nil
}

// minSearch is the shortest search the service takes for /leagues.
const minSearch = 3

// checkSearch refuses a search the service would refuse, before it costs a
// request from the allowance.
func checkSearch(search string) error {
	if utf8.RuneCountInString(search) < minSearch {
		return fmt.Errorf("football: the search %q is shorter than %d characters", search, minSearch)
	}
	return nil
}

// Teams lists the teams of a league in a season.
func (a *APIFootball) Teams(ctx context.Context, leagueID int64, season int) ([]Team, error) {
	if leagueID <= 0 || season <= 0 {
		return nil, errors.New("football: Teams needs a league and a season")
	}
	params := url.Values{}
	params.Set("league", strconv.FormatInt(leagueID, 10))
	params.Set("season", strconv.Itoa(season))
	var rows []struct {
		Team struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Code    string `json:"code"`
			Country string `json:"country"`
			Logo    string `json:"logo"`
		} `json:"team"`
	}
	if err := a.get(ctx, "/teams", params, &rows); err != nil {
		return nil, err
	}
	out := make([]Team, 0, len(rows))
	for _, row := range rows {
		out = append(out, Team{ID: row.Team.ID, Name: row.Team.Name, Code: row.Team.Code, Country: row.Team.Country, Logo: row.Team.Logo})
	}
	return out, nil
}

// Fixtures lists the fixtures a query selects.
//
// With IDs it asks for them in batches of twenty. With LiveOnly it asks for
// the live fixtures, of LeagueID when one is given (live=<league>), otherwise
// of every league (live=all). Otherwise it sends the league, the season and the
// dates, with the clock in UTC.
func (a *APIFootball) Fixtures(ctx context.Context, q FixtureQuery) ([]Fixture, error) {
	switch {
	case len(q.IDs) > 0:
		return a.fixturesByID(ctx, q.IDs)
	case q.LiveOnly:
		params := url.Values{}
		params.Set("live", "all")
		if q.LeagueID != 0 {
			params.Set("live", strconv.FormatInt(q.LeagueID, 10))
		}
		params.Set("timezone", "UTC")
		fixtures, err := a.fixtureRequest(ctx, params)
		if err != nil || q.LeagueID == 0 {
			return fixtures, err
		}
		// The service already narrowed the answer; the filter is a guard.
		kept := fixtures[:0]
		for _, fixture := range fixtures {
			if fixture.LeagueID == q.LeagueID {
				kept = append(kept, fixture)
			}
		}
		return kept, nil
	}

	if q.LeagueID == 0 && q.Season == 0 && q.From.IsZero() && q.To.IsZero() {
		return nil, errors.New("football: the FixtureQuery selects nothing")
	}
	params := url.Values{}
	if q.LeagueID != 0 {
		params.Set("league", strconv.FormatInt(q.LeagueID, 10))
	}
	if q.Season != 0 {
		params.Set("season", strconv.Itoa(q.Season))
	}
	if !q.From.IsZero() {
		params.Set("from", q.From.UTC().Format(time.DateOnly))
	}
	if !q.To.IsZero() {
		params.Set("to", q.To.UTC().Format(time.DateOnly))
	}
	params.Set("timezone", "UTC")
	return a.fixtureRequest(ctx, params)
}

func (a *APIFootball) fixturesByID(ctx context.Context, ids []int64) ([]Fixture, error) {
	seen := map[int64]bool{}
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	out := []Fixture{}
	for start := 0; start < len(unique); start += maxIDsPerRequest {
		end := min(start+maxIDsPerRequest, len(unique))
		parts := make([]string, 0, end-start)
		for _, id := range unique[start:end] {
			parts = append(parts, strconv.FormatInt(id, 10))
		}
		params := url.Values{}
		params.Set("ids", strings.Join(parts, "-"))
		params.Set("timezone", "UTC")
		batch, err := a.fixtureRequest(ctx, params)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

// fixtureRow is one element of the fixtures answer.
type fixtureRow struct {
	Fixture struct {
		ID        int64  `json:"id"`
		Date      string `json:"date"`
		Timestamp int64  `json:"timestamp"`
		Status    struct {
			Short   string `json:"short"`
			Elapsed *int   `json:"elapsed"`
		} `json:"status"`
	} `json:"fixture"`
	League struct {
		ID     int64  `json:"id"`
		Season int    `json:"season"`
		Round  string `json:"round"`
	} `json:"league"`
	Teams struct {
		Home teamRow `json:"home"`
		Away teamRow `json:"away"`
	} `json:"teams"`
	Goals goalsRow `json:"goals"`
	Score struct {
		Fulltime  goalsRow `json:"fulltime"`
		Extratime goalsRow `json:"extratime"`
	} `json:"score"`
}

type teamRow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Logo string `json:"logo"`
}

type goalsRow struct {
	Home *int `json:"home"`
	Away *int `json:"away"`
}

func (a *APIFootball) fixtureRequest(ctx context.Context, params url.Values) ([]Fixture, error) {
	var rows []fixtureRow
	if err := a.get(ctx, "/fixtures", params, &rows); err != nil {
		return nil, err
	}
	out := make([]Fixture, 0, len(rows))
	for _, row := range rows {
		fixture, err := row.fixture()
		if err != nil {
			return nil, err
		}
		out = append(out, fixture)
	}
	return out, nil
}

func (r fixtureRow) fixture() (Fixture, error) {
	kickoff, err := parseKickoff(r.Fixture.Timestamp, r.Fixture.Date)
	if err != nil {
		return Fixture{}, fmt.Errorf("football: fixture %d: %w", r.Fixture.ID, err)
	}
	status := statusFromShort(r.Fixture.Status.Short)
	home, away := score(status, r.Goals, r.Score.Fulltime, r.Score.Extratime)
	fixture := Fixture{
		ID:        r.Fixture.ID,
		LeagueID:  r.League.ID,
		Season:    r.League.Season,
		Round:     r.League.Round,
		Kickoff:   kickoff,
		Status:    status,
		Home:      TeamRef(r.Teams.Home),
		Away:      TeamRef(r.Teams.Away),
		HomeGoals: home,
		AwayGoals: away,
	}
	if r.Fixture.Status.Elapsed != nil && status == StatusLive {
		fixture.Elapsed = *r.Fixture.Status.Elapsed
	}
	return fixture, nil
}

func parseKickoff(timestamp int64, date string) (time.Time, error) {
	if timestamp > 0 {
		return time.Unix(timestamp, 0).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return time.Time{}, fmt.Errorf("unreadable kickoff %q", date)
	}
	return parsed.UTC(), nil
}

// score is the result of normal time plus extra time, never the shoot-out.
//
// While a fixture is live the service's running score is the truth, because
// score.fulltime is not filled until the whole of normal time has been played.
// Afterwards it is score.fulltime plus score.extratime; the running score is
// the fallback for a fixture whose period scores were not filled.
func score(status Status, running, fulltime, extratime goalsRow) (home, away *int) {
	if status == StatusLive && running.Home != nil && running.Away != nil {
		return copyInt(running.Home), copyInt(running.Away)
	}
	if fulltime.Home != nil && fulltime.Away != nil {
		return sum(fulltime.Home, extratime.Home), sum(fulltime.Away, extratime.Away)
	}
	if running.Home != nil && running.Away != nil {
		return copyInt(running.Home), copyInt(running.Away)
	}
	return nil, nil
}

func sum(a, b *int) *int {
	total := *a
	if b != nil {
		total += *b
	}
	return &total
}

func copyInt(v *int) *int {
	c := *v
	return &c
}

// envelope is the frame every answer comes in: get, parameters, errors,
// results, paging and response.
type envelope struct {
	Errors json.RawMessage `json:"errors"`
	Paging struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"paging"`
	Response json.RawMessage `json:"response"`
}

// maxPages stops a runaway paging field from draining the allowance.
const maxPages = 50

// get makes the request and decodes its response array into out. An answer
// whose paging.total is more than one is followed page by page, with the page
// parameter, and the pages are joined into one array.
func (a *APIFootball) get(ctx context.Context, path string, params url.Values, out any) error {
	first, err := a.page(ctx, path, params)
	if err != nil {
		return err
	}
	total := first.Paging.Total
	if total <= 1 {
		return decodeResponse(path, first.Response, out)
	}
	if total > maxPages {
		return fmt.Errorf("football: %s has %d pages, more than the %d this package reads", path, total, maxPages)
	}

	var rows []json.RawMessage
	if err := appendRows(&rows, path, first.Response); err != nil {
		return err
	}
	for number := 2; number <= total; number++ {
		next := url.Values{}
		for key, values := range params {
			next[key] = values
		}
		next.Set("page", strconv.Itoa(number))
		frame, err := a.page(ctx, path, next)
		if err != nil {
			return err
		}
		if err := appendRows(&rows, path, frame.Response); err != nil {
			return err
		}
	}
	joined, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("football: joining the pages of %s: %w", path, err)
	}
	return decodeResponse(path, joined, out)
}

func appendRows(rows *[]json.RawMessage, path string, response json.RawMessage) error {
	if len(response) == 0 || string(response) == "null" {
		return nil
	}
	var page []json.RawMessage
	if err := json.Unmarshal(response, &page); err != nil {
		return fmt.Errorf("football: %s sent an unexpected response: %w", path, err)
	}
	*rows = append(*rows, page...)
	return nil
}

func decodeResponse(path string, response json.RawMessage, out any) error {
	if len(response) == 0 || string(response) == "null" {
		return nil
	}
	if err := json.Unmarshal(response, out); err != nil {
		return fmt.Errorf("football: %s sent an unexpected response: %w", path, err)
	}
	return nil
}

// page makes one request and returns its frame, with the errors field already
// treated.
func (a *APIFootball) page(ctx context.Context, path string, params url.Values) (envelope, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	endpoint := a.baseURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return envelope{}, fmt.Errorf("football: building the request: %w", err)
	}
	request.Header.Set("x-apisports-key", a.apiKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "arandu-football")

	response, err := a.client.Do(request)
	if err != nil {
		return envelope{}, fmt.Errorf("football: %s: %w", path, err)
	}
	defer response.Body.Close()
	a.readQuota(response.Header)

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody))
	if err != nil {
		return envelope{}, fmt.Errorf("football: reading %s: %w", path, err)
	}

	switch response.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return envelope{}, fmt.Errorf("%w (HTTP %d)", ErrUnauthorized, response.StatusCode)
	case http.StatusTooManyRequests:
		return envelope{}, fmt.Errorf("%w (HTTP 429)", ErrRateLimited)
	}
	if response.StatusCode != http.StatusOK {
		return envelope{}, fmt.Errorf("football: %s answered HTTP %d", path, response.StatusCode)
	}

	var frame envelope
	if err := json.Unmarshal(body, &frame); err != nil {
		return envelope{}, fmt.Errorf("football: %s sent unreadable JSON: %w", path, err)
	}
	if err := apiError(frame.Errors); err != nil {
		return envelope{}, err
	}
	return frame, nil
}

// apiError reads the errors field, which is an empty array when all is well and
// an object of field to message otherwise, and the service answers it with HTTP
// 200.
func apiError(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	messages := map[string]string{}

	var object map[string]any
	if err := json.Unmarshal(raw, &object); err == nil {
		for key, value := range object {
			messages[key] = fmt.Sprint(value)
		}
	} else {
		var list []any
		if err := json.Unmarshal(raw, &list); err != nil {
			return fmt.Errorf("football: unreadable errors field %s", raw)
		}
		for i, value := range list {
			messages[strconv.Itoa(i)] = fmt.Sprint(value)
		}
	}
	if len(messages) == 0 {
		return nil
	}

	keys := make([]string, 0, len(messages))
	for key := range messages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+messages[key])
	}
	detail := strings.Join(parts, "; ")

	for _, key := range keys {
		switch strings.ToLower(key) {
		case "token", "access":
			return fmt.Errorf("%w: %s", ErrUnauthorized, detail)
		case "requests", "ratelimit":
			return fmt.Errorf("%w: %s", ErrRateLimited, detail)
		}
	}
	lowered := strings.ToLower(detail)
	switch {
	case strings.Contains(lowered, "rate limit"), strings.Contains(lowered, "request limit"), strings.Contains(lowered, "too many requests"):
		return fmt.Errorf("%w: %s", ErrRateLimited, detail)
	case strings.Contains(lowered, "application key"), strings.Contains(lowered, "invalid key"):
		return fmt.Errorf("%w: %s", ErrUnauthorized, detail)
	}
	return fmt.Errorf("football: the service refused the request: %s", detail)
}

// readQuota keeps the allowance the headers report: the day's, in
// x-ratelimit-requests-limit and x-ratelimit-requests-remaining, and the
// minute's, in X-RateLimit-Limit and X-RateLimit-Remaining. A header that is
// missing leaves the figure it had.
func (a *APIFootball) readQuota(header http.Header) {
	remaining, okRemaining := headerInt(header, "x-ratelimit-requests-remaining")
	limit, okLimit := headerInt(header, "x-ratelimit-requests-limit")
	minuteRemaining, okMinuteRemaining := headerInt(header, "X-RateLimit-Remaining")
	minuteLimit, okMinuteLimit := headerInt(header, "X-RateLimit-Limit")
	if !okRemaining && !okLimit && !okMinuteRemaining && !okMinuteLimit {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if okRemaining {
		a.quota.Remaining = remaining
	}
	if okLimit {
		a.quota.Limit = limit
	}
	if okMinuteRemaining {
		a.quota.MinuteRemaining = minuteRemaining
	}
	if okMinuteLimit {
		a.quota.MinuteLimit = minuteLimit
	}
	a.quota.At = a.now()
}

func headerInt(header http.Header, name string) (int, bool) {
	value := strings.TrimSpace(header.Get(name))
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	return n, err == nil
}
