# Working on Arandu Football

This is an Arandu package that reads football data: a client for the
API-Football v3 service (`New`, which returns an `*APIFootball`) and an
in-memory stand-in for it (`NewFake`). Both implement `Source`, the interface an
application depends on. It is a Go module somebody `go get`s and imports.

It began as the package skeleton, configured as `hyz-is/football`, and the HTTP
module the template ships was removed on purpose: there is no entity, no table,
no migration, no policy, no route and no `Module` to register. Reading a fixture
is a call the application makes with values it already has, and nothing here
should grow back into a module.

Read `.agents/skills/` before writing code. Each skill is a procedure, and the
one you need is named by the situation you are in.

## The gates

Nothing is finished until all four exit zero.

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

`GOWORK=off` is not a preference. This checkout may sit beside a Go workspace
that lists the framework repositories and does not list this one; when it does,
every command above fails before it compiles anything:

```
pattern ./...: directory prefix . does not contain modules listed in go.work
or their selected dependencies
```

With the workspace off, the module resolves the versions in `go.mod`, which is
what CI compiles against and what somebody's `go get` will get.

`aru doctor` is not one of the gates. It reads applications, and exits 1 here.
It does not read this package after it is installed either, which is why
`tests/Unit/audit_test.go` compares `arandu.mod.toml` with what the code calls.

## What this repository holds

| | measured with |
| --- | --- |
| 5 Go files in the root package: types, config, status, client, fake (and `doc.go`) | `ls *.go` |
| 0 routes, 0 tables, 1 capability (`network`) | `arandu.mod.toml`, `TestThePackageIsALibrary` |
| 0 dependencies: the standard library only | `go list -m all` |

```
doc.go     the package comment
types.go   Status, League, Team, TeamRef, Fixture, FixtureQuery, Quota and Source
config.go  Config, its defaults, and a String that hides the key
status.go  the short status codes of the service reduced to the five Status
client.go  APIFootball: the requests, the score, the errors, the quota
fake.go    Fake: the same Source, in memory
```

## What does not exist here, on purpose

| A model reaches for | What is here instead |
| --- | --- |
| odds, predictions, a bookmakers or bets endpoint | nothing. The package follows matches and scores; `TestThePackageHasNoBettingSurface` holds it |
| a `Module`, a route, a line in `WithProviders` | nothing to register. `New` builds a client the application keeps |
| a model, a table or a cache on disk | nothing. The application stores what it keeps; the package holds the last quota in memory |
| a retry loop that waits out a rate limit | `ErrRateLimited`, returned at once. Waiting spends the caller's time and the account's allowance, and only the application knows how long it can wait |
| a third dependency, or an import of the framework | the standard library. `TestThePackageIsALibrary` and `TestTheModuleRequiresNothing` hold it |
| the API key in a log line, an error or `%v` | `Config.String` and `GoString` hide it, and no error carries a header or the request |
| a fallback that invents a score when the service sends none | `nil` goals. A fixture that has not started has no score |
| a status the service sends that the package does not know read as live or finished | scheduled. A fixture is never reported started or over on a guess |

## The four properties

A change that breaks one of them is not merged, whatever else it improves.

1. **No betting surface.** Leagues, teams and fixtures only.
2. **The score is normal time plus extra time, never the shoot-out.** While live
   it is the running score; before the start it is `nil`.
3. **The key stays on the server.** It reaches `x-apisports-key` and nothing
   else.
4. **`arandu.mod.toml` matches the code.** `network = true` for the GET
   requests, the other three `false`, and `tests/Unit/audit_test.go` compares
   that with what the code calls.

## Writing code

Everything in the source is in English: identifiers, doc comments, internal
comments, error messages, and the names and messages of tests.

Every exported symbol carries a doc comment, and the comment documents the
symbol and nothing else. Why a signature is what it is belongs there when it is
a fact about the code. A date, an issue number, a version in progress or the
name of another repository does not.

The exported names are a contract other repositories are written against. A
change to one is a break, and `UPGRADE.md` names it.

Tests go under `tests/Unit`, declaring `package unit_test`, and import the
package by its module path, which is what makes them see exactly what a caller
sees. No test reaches the network: the service is a local `httptest` server,
and the JSON it answers with is written by hand from the public documentation,
never captured with a real key.
