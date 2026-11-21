---
name: football-package
description: Read leagues, teams and fixtures from API-Football in an Arandu application with Arandu Football. Use when the request is to "show the matches", "get the fixtures", "sync the championship", "live scores", "follow a league", "API-Football", "api-sports", "go get github.com/hyz-is/arandu-football", "ErrRateLimited", "ErrUnauthorized", "how many requests are left", "test the code that reads football data", or when a project's go.mod already requires github.com/hyz-is/arandu-football. Covers the key, the client built once and kept as a football.Source, the fixture queries, the status and score rules, the errors, the quota, and the Fake for tests.
license: MIT
metadata:
  audience: app
---

# Using Arandu Football

Three pieces, each a few lines the application writes. Nothing is registered:
there is no module, no route and no migration.

## 1. The key

An account at API-Football (or API-Sports) gives a key. It belongs to whoever
holds the subscription and every request spends that account's allowance. It
goes in `.env` and in the deployment's secrets, never in the repository:

```dotenv
API_FOOTBALL_KEY=...
```

## 2. The client, built once

```sh
go get github.com/hyz-is/arandu-football@v0.1.1
```

```go
client, err := football.New(football.Config{APIKey: os.Getenv("API_FOOTBALL_KEY")})
if err != nil {
	return err // an empty key, or a BaseURL that is not http or https
}
```

Keep it where the application keeps its services (explicit wiring, for example in
`bootstrap/app.go`), and give the code that reads football data a
`football.Source`, not an `*APIFootball`, so a test can hand it `football.NewFake()`.

## 3. The calls

```go
leagues, err := src.Leagues(ctx, "premier")           // empty search lists all
teams, err := src.Teams(ctx, 39, 2024)                 // league, season
fixtures, err := src.Fixtures(ctx, football.FixtureQuery{LeagueID: 39, Season: 2024, From: from, To: to})
live, err := src.Fixtures(ctx, football.FixtureQuery{LiveOnly: true, LeagueID: 39})
some, err := src.Fixtures(ctx, football.FixtureQuery{IDs: ids}) // twenty to a request
left := src.Quota().Remaining                          // makes no request
```

- `IDs` wins over everything else, then `LiveOnly`, then league, season and dates
  (UTC dates, inclusive). A query with every field zero is refused.
- `Kickoff` is UTC. `Status` is one of five. A code the package does not know is
  read as scheduled.
- `HomeGoals` and `AwayGoals` are normal time plus extra time, never the penalty
  shoot-out, and `nil` until the match starts. Do not read a shoot-out result
  from them.
- `Elapsed` is the minute of play, only while live.

## Errors

`errors.Is(err, football.ErrRateLimited)`: out of requests for the minute or the
day. Do not retry in a loop; schedule the next read, and use `Quota()` to decide
how often to poll. `errors.Is(err, football.ErrUnauthorized)`: the key or the
account was refused; retrying will not help. Anything else is the service's own
message.

## What the package does not do

- No odds, predictions or bets. Do not look for them; they are not there.
- It does not store anything. The application keeps what it needs; polling live
  fixtures costs one request per call.
- Logos are trademarks of the clubs and competitions. The addresses are given;
  using them is the application's decision.

## Tests

```go
fake := football.NewFake()
fake.AddLeague(football.League{ID: 39, Name: "Premier League"})
fake.AddFixture(football.Fixture{ID: 1, LeagueID: 39, Season: 2024, Kickoff: kickoff, Status: football.StatusScheduled})
fake.SetFixture(football.Fixture{ID: 1, LeagueID: 39, Season: 2024, Kickoff: kickoff, Status: football.StatusFinished, HomeGoals: &two, AwayGoals: &one})
fake.FailWith(football.ErrRateLimited) // FailWith(nil) undoes it
```

`SetFixture` with the same ID moves a match through its life. No test of the
application should reach the network.

## Symptoms

| symptom | piece |
| --- | --- |
| `football: Config.APIKey is empty` | 1: the variable is not set in this environment |
| `ErrUnauthorized` on every call | 1: wrong key, suspended account, or the key of another product |
| `ErrRateLimited` after a few calls | 3: polling too often for the plan; read `Quota()` |
| a finished match shows the wrong score | 3: the score is normal plus extra time, by design, and not the shoot-out |
| no fixtures for a day | 3: the dates are UTC; a local midnight is another UTC day |
