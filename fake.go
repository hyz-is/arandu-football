package football

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
)

// Fake is a [Source] that holds its data in memory, for the tests of whatever
// reads football data. It makes no request. It is safe for concurrent use.
type Fake struct {
	mu       sync.Mutex
	leagues  map[int64]League
	teams    map[teamKey]map[int64]Team
	fixtures map[int64]Fixture
	quota    Quota
	failure  error
}

type teamKey struct {
	league int64
	season int
}

var _ Source = (*Fake)(nil)

// NewFake returns an empty Fake.
func NewFake() *Fake {
	return &Fake{
		leagues:  map[int64]League{},
		teams:    map[teamKey]map[int64]Team{},
		fixtures: map[int64]Fixture{},
	}
}

// AddLeague adds a league, or replaces the one with the same ID.
func (f *Fake) AddLeague(l League) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l.Seasons = slices.Clone(l.Seasons)
	f.leagues[l.ID] = l
}

// AddTeam adds a team to a league in a season, or replaces the one with the
// same ID there.
func (f *Fake) AddTeam(leagueID int64, season int, t Team) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := teamKey{leagueID, season}
	if f.teams[key] == nil {
		f.teams[key] = map[int64]Team{}
	}
	f.teams[key][t.ID] = t
}

// AddFixture adds a fixture, or replaces the one with the same ID.
func (f *Fake) AddFixture(x Fixture) {
	f.SetFixture(x)
}

// SetFixture replaces the fixture with the same ID, or adds it when there is
// none. A test moves a match through its life by setting it again with a new
// Status and score.
func (f *Fake) SetFixture(x Fixture) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fixtures[x.ID] = cloneFixture(x)
}

// SetQuota sets what Quota reports.
func (f *Fake) SetQuota(q Quota) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quota = q
}

// FailWith makes every read answer err until it is called again with nil, so a
// test can see how its code treats [ErrRateLimited] or [ErrUnauthorized].
func (f *Fake) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failure = err
}

// Quota returns what SetQuota set.
func (f *Fake) Quota() Quota {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.quota
}

// Leagues lists the leagues whose name or country contains search, ignoring
// case, ordered by ID.
func (f *Fake) Leagues(ctx context.Context, search string) ([]League, error) {
	if err := f.ready(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	needle := strings.ToLower(strings.TrimSpace(search))
	out := []League{}
	for _, league := range f.leagues {
		if needle == "" ||
			strings.Contains(strings.ToLower(league.Name), needle) ||
			strings.Contains(strings.ToLower(league.Country), needle) {
			league.Seasons = slices.Clone(league.Seasons)
			out = append(out, league)
		}
	}
	slices.SortFunc(out, func(a, b League) int { return compareID(a.ID, b.ID) })
	return out, nil
}

// Teams lists the teams added for the league and season, ordered by ID.
func (f *Fake) Teams(ctx context.Context, leagueID int64, season int) ([]Team, error) {
	if err := f.ready(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Team{}
	for _, team := range f.teams[teamKey{leagueID, season}] {
		out = append(out, team)
	}
	slices.SortFunc(out, func(a, b Team) int { return compareID(a.ID, b.ID) })
	return out, nil
}

// Fixtures answers a query the way the real service does: IDs wins, then
// LiveOnly, then the league, the season and the dates. The result is ordered by
// kickoff and then by ID.
func (f *Fake) Fixtures(ctx context.Context, q FixtureQuery) ([]Fixture, error) {
	if err := f.ready(ctx); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	var from, to time.Time
	if !q.From.IsZero() {
		from = dayOf(q.From)
	}
	if !q.To.IsZero() {
		to = dayOf(q.To)
	}

	out := []Fixture{}
	for _, fixture := range f.fixtures {
		switch {
		case len(q.IDs) > 0:
			if !slices.Contains(q.IDs, fixture.ID) {
				continue
			}
		case q.LiveOnly:
			if fixture.Status != StatusLive || (q.LeagueID != 0 && fixture.LeagueID != q.LeagueID) {
				continue
			}
		default:
			if q.LeagueID != 0 && fixture.LeagueID != q.LeagueID {
				continue
			}
			if q.Season != 0 && fixture.Season != q.Season {
				continue
			}
			day := dayOf(fixture.Kickoff)
			if (!from.IsZero() && day.Before(from)) || (!to.IsZero() && day.After(to)) {
				continue
			}
		}
		out = append(out, cloneFixture(fixture))
	}
	slices.SortFunc(out, func(a, b Fixture) int {
		if c := a.Kickoff.Compare(b.Kickoff); c != 0 {
			return c
		}
		return compareID(a.ID, b.ID)
	})
	return out, nil
}

func (f *Fake) ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failure != nil {
		return f.failure
	}
	return nil
}

func dayOf(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func compareID(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// cloneFixture copies the score pointers, so a caller that changes a result it
// was handed does not change the Fake.
func cloneFixture(x Fixture) Fixture {
	if x.HomeGoals != nil {
		x.HomeGoals = copyInt(x.HomeGoals)
	}
	if x.AwayGoals != nil {
		x.AwayGoals = copyInt(x.AwayGoals)
	}
	x.Kickoff = x.Kickoff.UTC()
	return x
}
