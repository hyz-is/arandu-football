package unit_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	football "github.com/hyz-is/arandu-football"
)

// recorder is a local stand-in for the service: it records every request and
// answers each path with the JSON registered for it.
type recorder struct {
	mu       sync.Mutex
	requests []*http.Request
	answers  map[string]string
	headers  http.Header
	status   int
}

func newService(t *testing.T, answers map[string]string) (*football.APIFootball, *recorder) {
	t.Helper()
	rec := &recorder{answers: answers, headers: http.Header{}, status: http.StatusOK}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.requests = append(rec.requests, r)
		body, status, headers := rec.answers[r.URL.Path], rec.status, rec.headers.Clone()
		rec.mu.Unlock()
		for name, values := range headers {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)

	client, err := football.New(football.Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client, rec
}

func (r *recorder) last() *http.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[len(r.requests)-1]
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

const leaguesJSON = `{
  "get": "leagues", "parameters": {"search": "premier"}, "errors": [], "results": 1,
  "paging": {"current": 1, "total": 1},
  "response": [{
    "league": {"id": 39, "name": "Premier League", "type": "League", "logo": "https://media.api-sports.io/football/leagues/39.png"},
    "country": {"name": "England", "code": "GB", "flag": "https://media.api-sports.io/flags/gb.svg"},
    "seasons": [{"year": 2023, "current": false}, {"year": 2024, "current": true}]
  }]
}`

const teamsJSON = `{
  "get": "teams", "errors": [], "results": 2, "paging": {"current": 1, "total": 1},
  "response": [
    {"team": {"id": 33, "name": "Manchester United", "code": "MUN", "country": "England", "founded": 1878, "national": false, "logo": "https://media.api-sports.io/football/teams/33.png"},
     "venue": {"id": 556, "name": "Old Trafford"}},
    {"team": {"id": 34, "name": "Newcastle", "code": "NEW", "country": "England", "founded": 1892, "national": false, "logo": "https://media.api-sports.io/football/teams/34.png"},
     "venue": {"id": 562, "name": "St. James' Park"}}
  ]
}`

func fixtureJSON(id int, short string, elapsed string, goals, fulltime, extratime, penalty string, league int, date string, ts int64) string {
	return fmt.Sprintf(`{
  "fixture": {"id": %d, "referee": null, "timezone": "UTC", "date": %q, "timestamp": %d,
    "status": {"long": "x", "short": %q, "elapsed": %s, "extra": null}},
  "league": {"id": %d, "name": "Premier League", "country": "England", "season": 2024, "round": "Regular Season - 14"},
  "teams": {"home": {"id": 33, "name": "Manchester United", "logo": "h.png", "winner": true},
            "away": {"id": 34, "name": "Newcastle", "logo": "a.png", "winner": false}},
  "goals": %s,
  "score": {"halftime": {"home": 0, "away": 0}, "fulltime": %s, "extratime": %s, "penalty": %s}
}`, id, date, ts, short, elapsed, league, goals, fulltime, extratime, penalty)
}

func envelopeOf(rows ...string) string {
	return fmt.Sprintf(`{"get":"fixtures","errors":[],"results":%d,"paging":{"current":1,"total":1},"response":[%s]}`,
		len(rows), strings.Join(rows, ","))
}

const none = `{"home": null, "away": null}`

func TestNewRefusesAnEmptyKeyAndABadAddress(t *testing.T) {
	t.Parallel()
	if _, err := football.New(football.Config{}); err == nil {
		t.Error("New accepted an empty API key")
	}
	if _, err := football.New(football.Config{APIKey: "k", BaseURL: "ftp://example.com"}); err == nil {
		t.Error("New accepted a BaseURL that is not http or https")
	}
	if _, err := football.New(football.Config{APIKey: "k", Timeout: -time.Second}); err == nil {
		t.Error("New accepted a negative Timeout")
	}
	if _, err := football.New(football.Config{APIKey: "k"}); err != nil {
		t.Errorf("New refused the defaults: %v", err)
	}
}

func TestConfigNeverPrintsTheKey(t *testing.T) {
	t.Parallel()
	config := football.Config{APIKey: "super-secret-key", BaseURL: "https://example.test"}
	for _, text := range []string{fmt.Sprint(config), fmt.Sprintf("%v", config), fmt.Sprintf("%+v", config), fmt.Sprintf("%#v", config)} {
		if strings.Contains(text, "super-secret-key") {
			t.Errorf("a formatted Config carries the key: %s", text)
		}
	}
}

func TestLeaguesSendsTheKeyAndReadsTheAnswer(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, map[string]string{"/leagues": leaguesJSON})

	leagues, err := client.Leagues(context.Background(), "premier")
	if err != nil {
		t.Fatalf("Leagues: %v", err)
	}
	request := rec.last()
	if got := request.Header.Get("x-apisports-key"); got != "test-key" {
		t.Errorf("x-apisports-key = %q", got)
	}
	if got := request.URL.Query().Get("search"); got != "premier" {
		t.Errorf("search = %q", got)
	}
	if len(leagues) != 1 {
		t.Fatalf("got %d leagues, want 1", len(leagues))
	}
	league := leagues[0]
	if league.ID != 39 || league.Name != "Premier League" || league.Country != "England" ||
		!strings.HasSuffix(league.Logo, "/39.png") || len(league.Seasons) != 2 || league.Seasons[1] != 2024 {
		t.Errorf("unexpected league: %+v", league)
	}

	if _, err := client.Leagues(context.Background(), ""); err != nil {
		t.Fatalf("Leagues without search: %v", err)
	}
	if rec.last().URL.Query().Has("search") {
		t.Error("an empty search was sent")
	}
}

func TestTeamsAsksForTheLeagueAndSeason(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, map[string]string{"/teams": teamsJSON})

	teams, err := client.Teams(context.Background(), 39, 2024)
	if err != nil {
		t.Fatalf("Teams: %v", err)
	}
	query := rec.last().URL.Query()
	if query.Get("league") != "39" || query.Get("season") != "2024" {
		t.Errorf("query = %v", query)
	}
	if len(teams) != 2 || teams[0].ID != 33 || teams[0].Code != "MUN" || teams[0].Country != "England" ||
		!strings.HasSuffix(teams[1].Logo, "/34.png") {
		t.Errorf("unexpected teams: %+v", teams)
	}
	if _, err := client.Teams(context.Background(), 0, 2024); err == nil {
		t.Error("Teams accepted a missing league")
	}
}

func TestFixturesByLeagueSeasonAndDates(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, map[string]string{"/fixtures": envelopeOf(
		fixtureJSON(1001, "NS", "null", none, none, none, none, 39, "2024-12-01T15:00:00+00:00", 1733065200),
	)})

	// A date late in the evening at -03:00 is already the next day in UTC.
	from := time.Date(2024, 12, 1, 22, 30, 0, 0, time.FixedZone("BRT", -3*3600))
	to := time.Date(2024, 12, 8, 0, 0, 0, 0, time.UTC)
	fixtures, err := client.Fixtures(context.Background(), football.FixtureQuery{LeagueID: 39, Season: 2024, From: from, To: to})
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	query := rec.last().URL.Query()
	for key, want := range map[string]string{"league": "39", "season": "2024", "from": "2024-12-02", "to": "2024-12-08", "timezone": "UTC"} {
		if got := query.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if len(fixtures) != 1 {
		t.Fatalf("got %d fixtures", len(fixtures))
	}
	fixture := fixtures[0]
	if fixture.ID != 1001 || fixture.LeagueID != 39 || fixture.Season != 2024 || fixture.Round != "Regular Season - 14" ||
		fixture.Status != football.StatusScheduled || fixture.Home.ID != 33 || fixture.Home.Name != "Manchester United" ||
		fixture.Away.Logo != "a.png" || fixture.Elapsed != 0 {
		t.Errorf("unexpected fixture: %+v", fixture)
	}
	if want := time.Date(2024, 12, 1, 15, 0, 0, 0, time.UTC); !fixture.Kickoff.Equal(want) || fixture.Kickoff.Location() != time.UTC {
		t.Errorf("Kickoff = %v, want %v in UTC", fixture.Kickoff, want)
	}
	if fixture.HomeGoals != nil || fixture.AwayGoals != nil {
		t.Errorf("a fixture that has not started has a score: %v-%v", fixture.HomeGoals, fixture.AwayGoals)
	}
}

func TestAFixtureQueryThatSelectsNothingIsRefused(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, nil)
	if _, err := client.Fixtures(context.Background(), football.FixtureQuery{}); err == nil {
		t.Error("an empty query was accepted")
	}
	if rec.count() != 0 {
		t.Error("an empty query reached the service")
	}
}

func TestFixturesByIDGoInBatchesOfTwenty(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, map[string]string{"/fixtures": envelopeOf(
		fixtureJSON(1, "FT", "90", `{"home":1,"away":0}`, `{"home":1,"away":0}`, none, none, 39, "2024-12-01T15:00:00+00:00", 1733065200),
	)})

	ids := make([]int64, 0, 46)
	for i := int64(1); i <= 45; i++ {
		ids = append(ids, i)
	}
	ids = append(ids, 3) // a duplicate is asked for once

	fixtures, err := client.Fixtures(context.Background(), football.FixtureQuery{IDs: ids, LeagueID: 39, LiveOnly: true})
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	if rec.count() != 3 {
		t.Fatalf("made %d requests, want 3 (20+20+5)", rec.count())
	}
	if len(fixtures) != 3 {
		t.Errorf("got %d fixtures, want one per answer", len(fixtures))
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	sizes := []int{}
	for _, request := range rec.requests {
		query := request.URL.Query()
		sizes = append(sizes, len(strings.Split(query.Get("ids"), "-")))
		if query.Get("timezone") != "UTC" || query.Has("live") || query.Has("league") {
			t.Errorf("unexpected query %v", query)
		}
	}
	if sizes[0] != 20 || sizes[1] != 20 || sizes[2] != 5 {
		t.Errorf("batch sizes = %v", sizes)
	}
}

func TestLiveFixturesAreFilteredByLeague(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, map[string]string{"/fixtures": envelopeOf(
		fixtureJSON(1, "1H", "23", `{"home":1,"away":0}`, none, none, none, 39, "2024-12-01T15:00:00+00:00", 1733065200),
		fixtureJSON(2, "2H", "61", `{"home":0,"away":2}`, none, none, none, 140, "2024-12-01T15:00:00+00:00", 1733065200),
	)})

	all, err := client.Fixtures(context.Background(), football.FixtureQuery{LiveOnly: true})
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	query := rec.last().URL.Query()
	if query.Get("live") != "all" || query.Has("league") || query.Has("ids") {
		t.Errorf("query = %v", query)
	}
	if len(all) != 2 || all[0].Elapsed != 23 || all[0].Status != football.StatusLive {
		t.Errorf("unexpected live fixtures: %+v", all)
	}
	if *all[0].HomeGoals != 1 || *all[1].AwayGoals != 2 {
		t.Error("a live fixture lost its running score")
	}

	one, err := client.Fixtures(context.Background(), football.FixtureQuery{LiveOnly: true, LeagueID: 140})
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	if len(one) != 1 || one[0].ID != 2 {
		t.Errorf("league filter kept %+v", one)
	}
}

func TestEveryShortStatusMapsToOneOfTheFive(t *testing.T) {
	t.Parallel()
	want := map[string]football.Status{
		"NS": football.StatusScheduled, "TBD": football.StatusScheduled,
		"1H": football.StatusLive, "HT": football.StatusLive, "2H": football.StatusLive, "ET": football.StatusLive,
		"BT": football.StatusLive, "P": football.StatusLive, "LIVE": football.StatusLive, "INT": football.StatusLive,
		"FT": football.StatusFinished, "AET": football.StatusFinished, "PEN": football.StatusFinished,
		"AWD": football.StatusFinished, "WO": football.StatusFinished,
		"PST": football.StatusPostponed, "SUSP": football.StatusPostponed,
		"CANC": football.StatusCancelled, "ABD": football.StatusCancelled,
		"NEW-CODE": football.StatusScheduled,
	}
	rows := []string{}
	order := []string{}
	for short := range want {
		rows = append(rows, fixtureJSON(len(rows)+1, short, "null", none, none, none, none, 39, "2024-12-01T15:00:00+00:00", 1733065200))
		order = append(order, short)
	}
	client, _ := newService(t, map[string]string{"/fixtures": envelopeOf(rows...)})
	fixtures, err := client.Fixtures(context.Background(), football.FixtureQuery{LeagueID: 39})
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	for i, fixture := range fixtures {
		if fixture.Status != want[order[i]] {
			t.Errorf("%s mapped to %q, want %q", order[i], fixture.Status, want[order[i]])
		}
	}
}

func TestTheScoreIsFulltimePlusExtratimeAndNeverThePenalties(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                   string
		short, goals, fulltime, extra, penalty string
		home, away                             int
	}{
		{"normal time", "FT", `{"home":2,"away":1}`, `{"home":2,"away":1}`, none, none, 2, 1},
		{"extra time", "AET", `{"home":3,"away":2}`, `{"home":1,"away":1}`, `{"home":2,"away":1}`, none, 3, 2},
		{"penalties", "PEN", `{"home":2,"away":2}`, `{"home":1,"away":1}`, `{"home":1,"away":1}`, `{"home":4,"away":3}`, 2, 2},
		{"penalties after a goalless extra time", "PEN", `{"home":1,"away":1}`, `{"home":1,"away":1}`, `{"home":0,"away":0}`, `{"home":5,"away":4}`, 1, 1},
		{"goalless", "FT", `{"home":0,"away":0}`, `{"home":0,"away":0}`, none, none, 0, 0},
	}
	rows := []string{}
	for i, c := range cases {
		rows = append(rows, fixtureJSON(i+1, c.short, "120", c.goals, c.fulltime, c.extra, orNone(c.penalty), 39, "2024-12-01T15:00:00+00:00", 1733065200))
	}
	client, _ := newService(t, map[string]string{"/fixtures": envelopeOf(rows...)})
	fixtures, err := client.Fixtures(context.Background(), football.FixtureQuery{LeagueID: 39})
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	for i, c := range cases {
		f := fixtures[i]
		if f.HomeGoals == nil || f.AwayGoals == nil || *f.HomeGoals != c.home || *f.AwayGoals != c.away {
			t.Errorf("%s: score = %v-%v, want %d-%d", c.name, deref(f.HomeGoals), deref(f.AwayGoals), c.home, c.away)
		}
		if f.Status != football.StatusFinished {
			t.Errorf("%s: status = %q", c.name, f.Status)
		}
		if f.Elapsed != 0 {
			t.Errorf("%s: Elapsed = %d for a finished fixture", c.name, f.Elapsed)
		}
	}
}

func orNone(v string) string {
	if v == "" {
		return none
	}
	return v
}

func deref(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func TestAKickoffWithoutATimestampIsReadFromTheDate(t *testing.T) {
	t.Parallel()
	client, _ := newService(t, map[string]string{"/fixtures": envelopeOf(
		fixtureJSON(1, "NS", "null", none, none, none, none, 39, "2024-12-01T16:00:00+01:00", 0),
	)})
	fixtures, err := client.Fixtures(context.Background(), football.FixtureQuery{LeagueID: 39})
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	if want := time.Date(2024, 12, 1, 15, 0, 0, 0, time.UTC); !fixtures[0].Kickoff.Equal(want) || fixtures[0].Kickoff.Location() != time.UTC {
		t.Errorf("Kickoff = %v, want %v", fixtures[0].Kickoff, want)
	}
}

func TestQuotaIsReadFromTheHeaders(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, map[string]string{"/leagues": leaguesJSON})
	if q := client.Quota(); !q.At.IsZero() || q.Remaining != 0 || q.Limit != 0 {
		t.Errorf("Quota before any answer = %+v", q)
	}
	rec.headers.Set("x-ratelimit-requests-remaining", "7412")
	rec.headers.Set("x-ratelimit-requests-limit", "7500")

	before := time.Now()
	if _, err := client.Leagues(context.Background(), ""); err != nil {
		t.Fatalf("Leagues: %v", err)
	}
	q := client.Quota()
	if q.Remaining != 7412 || q.Limit != 7500 || q.At.Before(before) {
		t.Errorf("Quota = %+v", q)
	}

	// An answer without the headers leaves the last figures.
	rec.headers = http.Header{}
	if _, err := client.Leagues(context.Background(), ""); err != nil {
		t.Fatalf("Leagues: %v", err)
	}
	if again := client.Quota(); again != q {
		t.Errorf("Quota moved without headers: %+v -> %+v", q, again)
	}
}

func TestErrorsInsideAnHTTP200AreTreated(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want error
	}{
		{"invalid token", `{"get":"leagues","errors":{"token":"Error/Missing application key. Go to https://www.api-football.com/documentation-v3 to learn how to get your API application key."},"results":0,"response":[]}`, football.ErrUnauthorized},
		{"suspended account", `{"errors":{"access":"Your account is suspended."},"results":0,"response":[]}`, football.ErrUnauthorized},
		{"minute limit", `{"errors":{"rateLimit":"Too many requests. Your rate limit is 10 requests per minute."},"results":0,"response":[]}`, football.ErrRateLimited},
		{"day limit", `{"errors":{"requests":"You have reached the request limit for the day, Go to https://dashboard.api-football.com to upgrade your plan."},"results":0,"response":[]}`, football.ErrRateLimited},
	}
	for _, c := range cases {
		client, _ := newService(t, map[string]string{"/leagues": c.body})
		_, err := client.Leagues(context.Background(), "")
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}

	client, _ := newService(t, map[string]string{"/leagues": `{"errors":{"plan":"Free plans do not have access to this season."},"results":0,"response":[]}`})
	_, err := client.Leagues(context.Background(), "")
	if err == nil || errors.Is(err, football.ErrUnauthorized) || errors.Is(err, football.ErrRateLimited) || !strings.Contains(err.Error(), "Free plans") {
		t.Errorf("another API error = %v", err)
	}
}

func TestHTTPStatusesMapToTheTwoErrors(t *testing.T) {
	t.Parallel()
	for status, want := range map[int]error{
		http.StatusUnauthorized:    football.ErrUnauthorized,
		http.StatusForbidden:       football.ErrUnauthorized,
		http.StatusTooManyRequests: football.ErrRateLimited,
	} {
		client, rec := newService(t, map[string]string{"/leagues": `{}`})
		rec.status = status
		if _, err := client.Leagues(context.Background(), ""); !errors.Is(err, want) {
			t.Errorf("HTTP %d: err = %v, want %v", status, err, want)
		}
	}

	client, rec := newService(t, map[string]string{"/leagues": `oops`})
	rec.status = http.StatusBadGateway
	if _, err := client.Leagues(context.Background(), ""); err == nil || strings.Contains(err.Error(), "oops") {
		t.Errorf("HTTP 502: err = %v", err)
	}
}

func TestAnErrorNeverCarriesTheKey(t *testing.T) {
	t.Parallel()
	client, err := football.New(football.Config{APIKey: "super-secret-key", BaseURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Leagues(context.Background(), "")
	if err == nil {
		t.Fatal("a refused connection did not fail")
	}
	if strings.Contains(err.Error(), "super-secret-key") {
		t.Errorf("the error carries the key: %v", err)
	}
}

func TestACancelledContextStopsTheRequest(t *testing.T) {
	t.Parallel()
	client, rec := newService(t, map[string]string{"/leagues": leaguesJSON})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Leagues(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if rec.count() != 0 {
		t.Error("a cancelled context still reached the service")
	}
}

func TestTheTimeoutBoundsASlowService(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); server.Close() })

	client, err := football.New(football.Config{APIKey: "k", BaseURL: server.URL, Timeout: 50 * time.Millisecond, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := client.Leagues(context.Background(), ""); err == nil {
		t.Fatal("a service that never answers did not fail")
	}
	if time.Since(start) > 5*time.Second {
		t.Error("the Timeout did not bound the request")
	}
}
