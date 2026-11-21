# Upgrade Guide

## Unreleased

## v0.1.2

Nothing has to change. A plan refusal (`errors.plan`, such as the current season
on a free plan) used to be a generic error and is now `ErrPlan`; check it with
`errors.Is` to tell the operator to upgrade the plan instead of retrying.
Install it with `go get github.com/hyz-is/arandu-football@v0.1.2`.

## v0.1.1

No code has to change, but one behaviour does: a fixture whose code is `SUSP`
(suspended) now reads as `StatusLive` instead of `StatusPostponed`, as the
service's own table classes it ("In Play"). If your code stops polling a fixture
that is not live, a suspended match now keeps being polled until it resumes or
changes code. `Leagues` now returns an error for a search of one or two
characters, which the service refused anyway. `Quota` has two new fields.
Install it with `go get github.com/hyz-is/arandu-football@v0.1.1`.

## v0.1.0

First release: nothing to upgrade. Install it with
`go get github.com/hyz-is/arandu-football@v0.1.0`.
