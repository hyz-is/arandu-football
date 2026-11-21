package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	football "github.com/hyz-is/arandu-football"
)

func intp(v int) *int { return &v }

func at(day, hour int) time.Time {
	return time.Date(2024, 12, day, hour, 0, 0, 0, time.UTC)
}

func seeded() *football.Fake {
	fake := football.NewFake()
	fake.AddLeague(football.League{ID: 140, Name: "La Liga", Country: "Spain", Seasons: []int{2024}})
	fake.AddLeague(football.League{ID: 39, Name: "Premier League", Country: "England", Seasons: []int{2023, 2024}})
	fake.AddTeam(39, 2024, football.Team{ID: 34, Name: "Newcastle"})
	fake.AddTeam(39, 2024, football.Team{ID: 33, Name: "Manchester United"})
	fake.AddTeam(39, 2023, football.Team{ID: 40, Name: "Liverpool"})
	fake.AddFixture(football.Fixture{ID: 3, LeagueID: 39, Season: 2024, Kickoff: at(2, 20), Status: football.StatusScheduled})
	fake.AddFixture(football.Fixture{ID: 2, LeagueID: 39, Season: 2024, Kickoff: at(1, 15), Status: football.StatusLive, Elapsed: 30, HomeGoals: intp(1), AwayGoals: intp(0)})
	fake.AddFixture(football.Fixture{ID: 1, LeagueID: 140, Season: 2024, Kickoff: at(1, 15), Status: football.StatusLive})
	fake.AddFixture(football.Fixture{ID: 4, LeagueID: 39, Season: 2023, Kickoff: at(9, 15), Status: football.StatusFinished})
	return fake
}

func ids(fixtures []football.Fixture) []int64 {
	out := []int64{}
	for _, f := range fixtures {
		out = append(out, f.ID)
	}
	return out
}

func equal(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTheFakeIsASource(t *testing.T) {
	t.Parallel()
	var source football.Source = football.NewFake()
	if _, err := source.Leagues(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}

func TestTheFakeSearchesLeaguesByNameOrCountry(t *testing.T) {
	t.Parallel()
	fake := seeded()
	all, _ := fake.Leagues(context.Background(), "")
	if len(all) != 2 || all[0].ID != 39 || all[1].ID != 140 {
		t.Errorf("all = %+v", all)
	}
	byName, _ := fake.Leagues(context.Background(), "  PREMIER ")
	if len(byName) != 1 || byName[0].ID != 39 {
		t.Errorf("by name = %+v", byName)
	}
	byCountry, _ := fake.Leagues(context.Background(), "spain")
	if len(byCountry) != 1 || byCountry[0].ID != 140 {
		t.Errorf("by country = %+v", byCountry)
	}
}

func TestTheFakeListsTeamsPerLeagueAndSeason(t *testing.T) {
	t.Parallel()
	fake := seeded()
	teams, _ := fake.Teams(context.Background(), 39, 2024)
	if len(teams) != 2 || teams[0].ID != 33 || teams[1].ID != 34 {
		t.Errorf("teams = %+v", teams)
	}
	none, _ := fake.Teams(context.Background(), 39, 1999)
	if len(none) != 0 {
		t.Errorf("teams of an unknown season = %+v", none)
	}
}

func TestTheFakeAnswersAFixtureQueryLikeTheService(t *testing.T) {
	t.Parallel()
	fake := seeded()
	ctx := context.Background()

	byLeague, _ := fake.Fixtures(ctx, football.FixtureQuery{LeagueID: 39, Season: 2024})
	if want := []int64{2, 3}; !equal(ids(byLeague), want) {
		t.Errorf("league+season = %v, want %v", ids(byLeague), want)
	}

	// Only the date counts, and both ends are inclusive.
	byDate, _ := fake.Fixtures(ctx, football.FixtureQuery{LeagueID: 39, Season: 2024, From: at(1, 23), To: at(1, 0)})
	if want := []int64{2}; !equal(ids(byDate), want) {
		t.Errorf("dates = %v, want %v", ids(byDate), want)
	}

	live, _ := fake.Fixtures(ctx, football.FixtureQuery{LiveOnly: true})
	if want := []int64{1, 2}; !equal(ids(live), want) {
		t.Errorf("live = %v, want %v", ids(live), want)
	}
	liveInLeague, _ := fake.Fixtures(ctx, football.FixtureQuery{LiveOnly: true, LeagueID: 140})
	if want := []int64{1}; !equal(ids(liveInLeague), want) {
		t.Errorf("live in league = %v, want %v", ids(liveInLeague), want)
	}

	byID, _ := fake.Fixtures(ctx, football.FixtureQuery{IDs: []int64{4, 3, 99}, LeagueID: 140, LiveOnly: true})
	if want := []int64{3, 4}; !equal(ids(byID), want) {
		t.Errorf("ids = %v, want %v", ids(byID), want)
	}
}

func TestSetFixtureMovesAMatchThroughItsLife(t *testing.T) {
	t.Parallel()
	fake := seeded()
	ctx := context.Background()

	fake.SetFixture(football.Fixture{ID: 3, LeagueID: 39, Season: 2024, Kickoff: at(2, 20), Status: football.StatusFinished, HomeGoals: intp(2), AwayGoals: intp(2)})
	got, _ := fake.Fixtures(ctx, football.FixtureQuery{IDs: []int64{3}})
	if len(got) != 1 || got[0].Status != football.StatusFinished || *got[0].HomeGoals != 2 {
		t.Fatalf("after SetFixture: %+v", got)
	}
	live, _ := fake.Fixtures(ctx, football.FixtureQuery{LiveOnly: true, LeagueID: 39})
	if want := []int64{2}; !equal(ids(live), want) {
		t.Errorf("live = %v, want %v", ids(live), want)
	}

	fake.SetFixture(football.Fixture{ID: 77, LeagueID: 39, Season: 2024, Kickoff: at(5, 12)})
	if all, _ := fake.Fixtures(ctx, football.FixtureQuery{LeagueID: 39, Season: 2024}); len(all) != 3 {
		t.Errorf("SetFixture of a new ID did not add it: %v", ids(all))
	}
}

func TestAResultHandedOutCannotChangeTheFake(t *testing.T) {
	t.Parallel()
	fake := seeded()
	ctx := context.Background()

	got, _ := fake.Fixtures(ctx, football.FixtureQuery{IDs: []int64{2}})
	*got[0].HomeGoals = 99

	again, _ := fake.Fixtures(ctx, football.FixtureQuery{IDs: []int64{2}})
	if *again[0].HomeGoals != 1 {
		t.Error("changing a returned score changed the Fake")
	}

	score := 5
	fake.SetFixture(football.Fixture{ID: 50, LeagueID: 39, Season: 2024, HomeGoals: &score, AwayGoals: &score})
	score = 6
	stored, _ := fake.Fixtures(ctx, football.FixtureQuery{IDs: []int64{50}})
	if *stored[0].HomeGoals != 5 {
		t.Error("changing the score passed to SetFixture changed the Fake")
	}
}

func TestTheFakeFailsOnRequestAndHonoursTheContext(t *testing.T) {
	t.Parallel()
	fake := seeded()
	fake.SetQuota(football.Quota{Remaining: 10, Limit: 100, At: at(1, 0)})
	if q := fake.Quota(); q.Remaining != 10 || q.Limit != 100 {
		t.Errorf("Quota = %+v", q)
	}

	fake.FailWith(football.ErrRateLimited)
	if _, err := fake.Leagues(context.Background(), ""); !errors.Is(err, football.ErrRateLimited) {
		t.Errorf("Leagues err = %v", err)
	}
	if _, err := fake.Fixtures(context.Background(), football.FixtureQuery{LiveOnly: true}); !errors.Is(err, football.ErrRateLimited) {
		t.Errorf("Fixtures err = %v", err)
	}
	fake.FailWith(nil)
	if _, err := fake.Teams(context.Background(), 39, 2024); err != nil {
		t.Errorf("Teams after FailWith(nil): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fake.Leagues(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled context: err = %v", err)
	}
}
