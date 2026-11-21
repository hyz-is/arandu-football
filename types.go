package football

import (
	"context"
	"time"
)

// Status is where a fixture stands, reduced from the many short codes the
// service uses to the five an application acts on.
type Status string

const (
	// StatusScheduled is a fixture that has not started: NS and TBD, and any
	// code this package does not know.
	StatusScheduled Status = "scheduled"
	// StatusLive is a fixture in play, including the half-time break, extra
	// time, the penalty shoot-out and a temporary interruption: 1H, HT, 2H, ET,
	// BT, P, LIVE and INT.
	StatusLive Status = "live"
	// StatusFinished is a fixture with a result: FT, AET and PEN, and AWD and
	// WO, where the result was awarded without the match being played.
	StatusFinished Status = "finished"
	// StatusPostponed is a fixture that will be played at another time: PST
	// and SUSP.
	StatusPostponed Status = "postponed"
	// StatusCancelled is a fixture that will not be played: CANC and ABD.
	StatusCancelled Status = "cancelled"
)

// League is a competition.
type League struct {
	ID      int64
	Name    string
	Country string
	// Logo is the address of the competition's logo. Logos are marks of the
	// competitions: whether to show one is the application's decision.
	Logo string
	// Seasons lists the years the service holds for the competition.
	Seasons []int
}

// Team is a club or a national side.
type Team struct {
	ID      int64
	Name    string
	Code    string
	Country string
	// Logo is the address of the club's crest. Crests are marks of the clubs:
	// whether to show one is the application's decision.
	Logo string
}

// TeamRef is a team as a fixture names it.
type TeamRef struct {
	ID   int64
	Name string
	Logo string
}

// Fixture is one match.
type Fixture struct {
	ID       int64
	LeagueID int64
	Season   int
	// Round is the supplier's text, for example "Regular Season - 14".
	Round string
	// Kickoff is the scheduled start, in UTC.
	Kickoff time.Time
	Status  Status
	Home    TeamRef
	Away    TeamRef
	// HomeGoals and AwayGoals are the score of normal time plus extra time,
	// never the penalty shoot-out. They are nil while the fixture has not
	// started.
	HomeGoals *int
	AwayGoals *int
	// Elapsed is the minute of play while the fixture is live.
	Elapsed int
}

// FixtureQuery selects fixtures. IDs wins over everything else; LiveOnly comes
// next, optionally narrowed to one league; otherwise the league, the season
// and the date range apply, and a zero field is left out.
type FixtureQuery struct {
	LeagueID int64
	Season   int
	// From and To bound the kickoff date, inclusive and in UTC. Only the date
	// counts.
	From, To time.Time
	IDs      []int64
	LiveOnly bool
}

// Quota is the last request allowance the service reported.
type Quota struct {
	Remaining int
	Limit     int
	// At is when the figures were read. It is the zero time before the first
	// answer.
	At time.Time
}

// Source is what an application reads football data through. [APIFootball]
// reads the real service and [Fake] holds data in memory.
type Source interface {
	// Leagues lists competitions whose name or country contains search; an
	// empty search lists them all.
	Leagues(ctx context.Context, search string) ([]League, error)
	// Teams lists the teams of a league in a season.
	Teams(ctx context.Context, leagueID int64, season int) ([]Team, error)
	// Fixtures lists the fixtures a query selects.
	Fixtures(ctx context.Context, q FixtureQuery) ([]Fixture, error)
	// Quota is the last allowance known, read from the headers of the most
	// recent answer. It makes no request.
	Quota() Quota
}
