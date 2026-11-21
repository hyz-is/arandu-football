# Arandu Football

A Go client for the [API-Football v3](https://www.api-football.com/documentation-v3)
service, and an in-memory stand-in for it.

It reads **leagues**, **teams** and **fixtures** (kickoff, status, score), keeps
the request allowance the service reports, and turns the service's many status
codes into five. It imports the standard library only, registers nothing and
owns no table: there is no module to wire, no route and no migration.

## What it is not

- **It has no betting surface.** There is no endpoint, type or parameter for
  odds, predictions or bets, and a test reads the source to keep it that way.
- **It does not come with a key.** The API key belongs to whoever holds the
  subscription with the service, and every request spends that account's
  allowance. Keep it in the environment, never in the repository.
- **It does not license the logos.** `League.Logo`, `Team.Logo` and
  `TeamRef.Logo` are addresses the service gives; the crests and competition
  logos are trademarks of the clubs and the competitions. Whether to show them
  is the application's decision.

## Install

```sh
go get github.com/hyz-is/arandu-football@v0.1.1
```

Go 1.26.10 or later.

## Use

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	football "github.com/hyz-is/arandu-football"
)

func main() {
	client, err := football.New(football.Config{APIKey: os.Getenv("API_FOOTBALL_KEY")})
	if err != nil {
		panic(err)
	}
	var source football.Source = client // depend on the interface

	ctx := context.Background()
	fixtures, err := source.Fixtures(ctx, football.FixtureQuery{
		LeagueID: 39,
		Season:   2024,
		From:     time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
		To:       time.Date(2024, 12, 8, 0, 0, 0, 0, time.UTC),
	})
	switch {
	case errors.Is(err, football.ErrRateLimited):
		fmt.Println("out of requests; try again later")
	case errors.Is(err, football.ErrUnauthorized):
		fmt.Println("the API key was refused")
	case errors.Is(err, football.ErrPlan):
		fmt.Println("the plan does not cover this season; upgrade the plan")
	case err != nil:
		panic(err)
	}
	for _, f := range fixtures {
		fmt.Println(f.Kickoff, f.Home.Name, f.Status, f.Away.Name)
	}
	fmt.Println("requests left:", source.Quota().Remaining)
}
```

`Config` takes the key, and optionally `BaseURL` (default
`https://v3.football.api-sports.io`), `HTTPClient` (default a client with
`Timeout`) and `Timeout` (default 15 seconds, applied to every request).
`Config` prints without its key, even with `%+v` or `%#v`.

### The source

```go
type Source interface {
	Leagues(ctx context.Context, search string) ([]League, error)
	Teams(ctx context.Context, leagueID int64, season int) ([]Team, error)
	Fixtures(ctx context.Context, q FixtureQuery) ([]Fixture, error)
	Quota() Quota
}
```

`Quota` makes no request: it is the last allowance read from the headers of the
most recent answer, and its `At` is the zero time before the first answer. The
day's allowance (`Remaining`, `Limit`) comes from `x-ratelimit-requests-remaining`
and `x-ratelimit-requests-limit`; the minute's (`MinuteRemaining`, `MinuteLimit`)
from `X-RateLimit-Remaining` and `X-RateLimit-Limit`.

When an answer says it has more than one page (`paging.total`), the next pages
are read with the `page` parameter and joined, so a list is never cut short.
`Leagues` refuses a search of one or two characters, as the service does, before
it spends a request.

### Fixtures

`FixtureQuery` is read in this order:

1. `IDs`: asked with the service's `ids` parameter, twenty to a request, so a
   long list is several requests. Duplicates are asked once.
2. `LiveOnly`: the live fixtures, `live=all` or `live=<LeagueID>` when a league
   is given.
3. Otherwise `LeagueID`, `Season` and the `From` and `To` dates, sent as
   `YYYY-MM-DD` in UTC. A zero field is left out; a query with every field zero
   is refused.

Every request carries `timezone=UTC`, and `Kickoff` is in UTC.

`HomeGoals` and `AwayGoals` are the score of normal time plus extra time, and
never the penalty shoot-out: `score.fulltime` plus `score.extratime` when there
is one. While a fixture is live they are the running score the service reports.
They are `nil` until the fixture has started.

| Service code | Status |
| --- | --- |
| `NS`, `TBD` | `StatusScheduled` |
| `1H`, `HT`, `2H`, `ET`, `BT`, `P`, `SUSP`, `INT`, `LIVE` | `StatusLive` |
| `FT`, `AET`, `PEN`, `AWD`, `WO` | `StatusFinished` |
| `PST` | `StatusPostponed` |
| `CANC`, `ABD` | `StatusCancelled` |

A code the package does not know is read as scheduled: a fixture is never
reported live or finished on a guess.

### Errors

The service reports most failures inside an HTTP 200, in an `errors` field. They
are treated: a refused key or account is `ErrUnauthorized`, and a minute or day
limit is `ErrRateLimited`, as are HTTP 401, 403 and 429 for the first and the
last, and a request the account's plan does not cover (`errors.plan`, such as
the current season on a free plan) is `ErrPlan`. Test them with `errors.Is`. Any other refusal is an error carrying the
service's message. No error carries the API key.

## Test your own code

`NewFake` returns a `Source` that holds data in memory and never touches the
network:

```go
fake := football.NewFake()
fake.AddLeague(football.League{ID: 39, Name: "Premier League", Country: "England", Seasons: []int{2024}})
fake.AddTeam(39, 2024, football.Team{ID: 33, Name: "Manchester United"})
fake.AddFixture(football.Fixture{
	ID: 1001, LeagueID: 39, Season: 2024,
	Kickoff: time.Date(2024, 12, 1, 15, 0, 0, 0, time.UTC),
	Status:  football.StatusScheduled,
})

// The match is played: set the same ID again.
two, one := 2, 1
fake.SetFixture(football.Fixture{
	ID: 1001, LeagueID: 39, Season: 2024,
	Kickoff: time.Date(2024, 12, 1, 15, 0, 0, 0, time.UTC),
	Status:  football.StatusFinished, HomeGoals: &two, AwayGoals: &one,
})

fake.SetQuota(football.Quota{Remaining: 10, Limit: 100})
fake.FailWith(football.ErrRateLimited) // every read now fails; FailWith(nil) undoes it
```

The Fake answers a `FixtureQuery` the way the service does (IDs, then live, then
league, season and dates, by UTC date), orders results by kickoff and then ID,
and hands out copies, so a test that changes a result does not change the Fake.

## The manifest

`arandu.mod.toml` declares `network = true` and nothing else: the package makes
GET requests to the service's address. `tests/Unit/audit_test.go` compares the
declaration with what the code calls.

## Development

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

No test reaches the network: the service is a local `httptest` server with JSON
written by hand.

See [CONTRIBUTING.md](CONTRIBUTING.md), [CHANGELOG.md](CHANGELOG.md) and
[UPGRADE.md](UPGRADE.md). Vulnerabilities follow [SECURITY.md](SECURITY.md).

## Licence

MIT, see [LICENSE.md](LICENSE.md).
