# Security policy

## Supported versions

The latest minor release of Arandu Football receives security fixes. Older minor
releases do not: a published Go module version is immutable, so a fix is a new
release rather than a moved tag.

## Reporting a vulnerability

Report it privately, through GitHub's advisory form:

<https://github.com/hyz-is/arandu-football/security/advisories/new>

Do not open a public issue and do not describe the problem in a pull request.
A report that arrives in public is a report every reader of this repository can
act on before there is a release to upgrade to.

Include what you need to reproduce it: the version, the wiring, and the request
or call that triggers it.

Expect an acknowledgement within a few days. If the report is confirmed, the
fix, the release and the advisory are published together, and you are credited
unless you ask not to be.

## What is in scope

Anything that exposes what the package keeps, or sends a request it should not.
In particular:

- the API key reaching a log line, an error, a formatted `Config` or any request
  header other than `x-apisports-key`;
- a request to an address other than `Config.BaseURL`, or to an endpoint of the
  service other than leagues, teams and fixtures;
- a response the package reads in a way that lets a malformed answer run code,
  exhaust memory or loop;
- a `Fake` that hands out a value a caller can use to change what it holds.

## What is not

- A vulnerability in API-Football itself. Report that to the service.
- An API key that its owner published. Rotate it in the service's dashboard.
