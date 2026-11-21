# Changelog

Everything worth knowing about a release of Arandu Football is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

## [0.1.0] - 2026-11-21

### Added

- `New(Config)`, a client for the API-Football v3 service that implements
  `Source`: `Leagues`, `Teams`, `Fixtures` and `Quota`. The key travels in
  `x-apisports-key`, the address and the HTTP client can be replaced, and every
  request is bounded by `Config.Timeout` (15 seconds by default). `Config`
  prints without its key.
- `Fixtures` takes a `FixtureQuery`: `IDs` in batches of twenty, `LiveOnly` with
  an optional league, or league, season and `From`/`To` dates in UTC. Every
  request asks for `timezone=UTC`.
- The service's short status codes reduced to the five `Status` values, and a
  score that is `score.fulltime` plus `score.extratime`, never the penalty
  shoot-out; while a fixture is live it is the running score.
- Errors the service reports inside an HTTP 200 are treated: `ErrUnauthorized`
  for a refused key or account and `ErrRateLimited` for a minute or day limit,
  and the same two for HTTP 401, 403 and 429.
- `Quota`, the last allowance read from the `x-ratelimit-requests-*` headers.
- `NewFake`, a `Source` in memory with `AddLeague`, `AddTeam`, `AddFixture`,
  `SetFixture`, `SetQuota` and `FailWith`, for the tests of whatever reads
  football data.
- The package has no betting surface: no odds, predictions or bets, held by a
  test. It imports the standard library only.
