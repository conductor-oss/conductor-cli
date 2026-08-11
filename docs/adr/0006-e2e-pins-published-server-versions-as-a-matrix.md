---
status: accepted
---

# E2E pins published Server Versions and runs the OSS-safe suite as a matrix

The server-backed E2E jobs no longer check out `conductor-oss/conductor` and build it
with Gradle. They start a Local server through the CLI's own
`conductor server start --version <pinned>`, and the OSS-safe job runs that suite
against three pinned Server Versions as a matrix: the current release, which gates a
merge, plus one older version per supported line, which report but do not gate.

This reverses [ADR-0002](./0002-e2e-builds-the-conductor-server-from-source.md), and it
is the arrangement `@mp-orkes` asked for in review of
[#106](https://github.com/conductor-oss/conductor-cli/pull/106). Pinning was always the
better default on speed, reproducibility and CI isolation; the single thing blocking it
was that no artifact was published from `main`, so a pin could not validate the code
about to ship.

## Why now

That blocker expired. `3.32.0` GA is published, and the floating `latest` jar — which
ADR-0002 recorded as months stale and apparently abandoned — was republished to match
it: both URLs return the same 455,946,354-byte artifact. Release validation is
therefore no longer scoped to an unreleased `main`, which was the second of ADR-0002's
three stated revisit triggers.

## What the matrix buys

ADR-0002 tested one moving target, and that target was not a version anyone had
released. A maintainer could not tell whether the CLI still worked against the versions
the project commits to supporting; the first report of a break against an older line
would come from a user.

So the OSS-safe job runs one leg per supported line. Only the current-release leg is
blocking, because that is the version users have. The older legs use
`continue-on-error`, which keeps two properties that matter more than they sound:
version skew predating a change cannot block that change, and a failing older leg is
visually distinct in the checks list from a failing blocking one. They run on every
pull request rather than behind a manual trigger — the repo is public, so runner
minutes are free, and matrix legs run in parallel, so once the jar cache is warm the
extra legs cost no wall clock. Promoting one to blocking is a one-line change.

The nightly tier stays on the blocking version alone. Its suites depend on
capabilities older lines lack, and a matrix there would spend model tokens per leg to
discover that.

The Orkes-facing job is untouched. It addresses a remote server whose Server Version is
neither known nor selectable, so a matrix cannot reach it.

## What the older legs actually report

All three Server Versions were run before this landed, each downloaded through the CLI
and started against its own fresh database, so the older ones are a measurement rather
than an expectation. These counts are from a **local macOS run**; one skip on every line
is the macOS-only GNU `timeout(1)` guard, which does not skip on a Linux runner, so
expect one fewer of each in CI:

| Server Version | tests | skips | failures |
|---|---|---|---|
| `3.32.0` (blocking) | 116 | 2 | 0 |
| `3.31.0` | 116 | 11 | 3 |
| `3.30.2` | 116 | 11 | 3 |

The six recovered `server.bats` tests pass on **all three**, so the launch path itself is
not version-sensitive. Both older legs are otherwise *not* clean, which is why they are
non-blocking rather than aspirationally blocking.

The first CI run reproduced this and confirmed the gating behaves as intended: the
preflight passed, the matrix expanded to the three expected legs, the `3.32.0` leg and
the Orkes-facing job passed, both older legs failed — and the run's overall conclusion
was still **success**. Two red legs that do not gate a merge is the whole point.

**Three failures on both older legs, one root cause — and it is already fixed upstream.**
`GET /api/scheduler/schedules/<name>` returns `HTTP 200` with an empty body for a
schedule that does not exist. That is
[conductor-oss/conductor#1357](https://github.com/conductor-oss/conductor/pull/1357),
merged 2026-07-19 as `c08d60c8a`, which makes the endpoint throw `NotFoundException`
instead. By ancestry the fix is in `v3.32.0` and its RCs from `rc.14` on, and in neither
`v3.31.0` nor `v3.30.2` — so the older legs are not reporting a new defect, they are
reporting a fix they predate. Confirmed against `3.32.0`, which returns `404` with an
error body both for a name that never existed and after a delete.

Note that `schedule delete` works on the older lines: the schedule leaves `schedule list`
and the underlying list API. It is only the single-schedule read that disagrees.

That one behaviour produces all three failures, once directly and twice through a
helper. `schedule.bats` test 7 asserts a deleted schedule can no longer be fetched, and
fails outright. The suite's `ensure_schedule` helper decides whether to create a schedule
by calling `schedule get`, so it concludes the schedule already exists, skips creating
it, and leaves tests 12 and 19 asserting against a schedule that is not there.
Pre-existing server-side skew, unrelated to the CLI and unrelated to pinning — so there
is nothing to file, and making these legs green would mean asking for a backport to the
3.31 and 3.30 lines, which is a support-policy decision rather than a bug report.

**The nine extra skips are the Agents API.** Neither older line serves it, so nine
`agent.bats` tests skip on their runtime guard rather than fail. That guard exists
because not every deployment serves those endpoints, so this is intended behaviour, not
a gap.

Neither finding is a defect in this change, and neither can block a merge. That is the
argument for `continue-on-error` on these legs: the signal is visible without being a
veto.

One caveat for anyone reproducing this locally: run each version against a *fresh*
working directory. Pointing two Server Versions at one `c123.db` produced an extra,
spurious `workflow create --force` failure on `3.30.2` that disappears on a clean
database. CI is unaffected — each matrix leg is its own runner, and the jar cache holds
only the jar, not the database or the server state.

## Published availability is the constraint, not the tag list

The S3 bucket the CLI downloads from carries a *subset* of the server repo's tags.
Verified by `HEAD` at the time of writing: `3.32.0`, `3.31.0`, `3.30.2` and `latest`
return 200, while `3.31.2` and `3.21.23` return 403. The 3.31 line is consequently
pinned at `3.31.0` rather than at its newest patch, and a pin must be chosen against
bucket contents rather than against `git tag`.

Because "someone pins a tag the bucket does not carry" is a live failure mode rather
than a hypothetical, a preflight job `HEAD`s every pin before any job does real work,
naming the version, the URL and the status code. Without it a bad pin surfaces minutes
into a job as an opaque download error.

The preflight is asymmetric, and deliberately so. An unavailable **blocking** pin fails
it outright — there is nothing meaningful left to run. An unavailable **older** pin only
drops its own leg, with a warning naming what was dropped. Failing the whole preflight
would skip the blocking leg along with it, so an older-version problem would gate a
merge through the back door — the exact property `continue-on-error` exists to prevent.
The preflight therefore emits the matrix it verified rather than the matrix it was
given.

`latest` is deliberately not pinned, even though it currently resolves to the GA
release. It reintroduces the irreproducibility this change removes, and it is mutable
content under a stable cache key — a jar cache keyed on `latest` would serve stale
bytes indefinitely.

## Consequences

**Six tests stopped skipping.** `server start` can only launch a published version, so
under ADR-0002 the source-built jar had to be launched with `java -jar`, leaving no
CLI-managed Local server and six `server.bats` tests skipping on a runtime condition —
including the `server start` and `server update` mutual-exclusion guards. A downloaded
version is started by the CLI, so those tests run. Un-skipping them is the
fix-verification step, in the Known-broken guard sense.

Measured against `3.32.0` by launching the same jar both ways, changing nothing but the
launch path: the OSS-safe `tier:pr` selection is 116 tests, and the skip count goes from
**8 to 2**, with all six recovered tests passing, including both mutual-exclusion guards.
The two that remain are unrelated and expected — the `#103` Known-broken guard, and one
macOS-only skip for absent GNU `timeout(1)` which does not skip on a Linux runner, so CI
should report 1.

To keep that from silently regressing, the job fails the leg if any test skips for want
of a CLI-managed Local server, and reports the skip count as a run annotation either
way. A number nobody looks at is not a check.

**Runs are reproducible.** A pinned Server Version is byte-identical between runs, so a
failure seen yesterday reproduces today.

**CI no longer depends on the server repo's build health.** A broken `main` in
`conductor-oss/conductor` can no longer redden an unrelated conductor-cli pull request.
This was ADR-0002's most significant cost.

**The Gradle build is gone from the pull-request path**, along with a build time that
would have grown with the server.

**Fidelity against unreleased `main` is given up.** This is the real cost, and it is
the trade ADR-0002 made in the other direction. A CLI-facing server change now shows up
in E2E only once it is published. The mitigation is that the pin is one line, so
tracking a new release or an RC during a release cycle is a one-line change.

**Pins need bumping.** A release nobody pins is a release nobody tests. The pins live
in one declaration in the workflow, and the preflight refuses a version the bucket does
not carry, so a bump either works or fails immediately and legibly.

## Recovering the source build

If fidelity against unreleased `main` becomes the priority again — a release cycle
scoped to `main`, or a CLI-facing server change that must be validated before it ships
— the source build is recoverable from commit
[`c7eb744`](https://github.com/conductor-oss/conductor-cli/commit/c7eb744), which
carries `CONDUCTOR_SERVER_REF`, the `conductor-oss/conductor` checkout, the Gradle setup
and the `:conductor-server:bootJar` build, for both server-backed jobs:

```
git show c7eb744:.github/workflows/e2e.yml
```

That commit is deliberately one that is **reachable from `main`**, which is the mistake
ADR-0002 made: it named `cf9d6b8c` from a squash-merged pull request, so the commit was
unreachable and `git log -S` could not find it. If this change is itself squash-merged,
the pointer still holds, and it stays discoverable without reading this file, because
the string it searches for is one this change removes:

```
git log -S'CONDUCTOR_SERVER_REF' -- .github/workflows/e2e.yml
```

Note that recovering it re-skips the six `server.bats` tests, for the reason above.
Restoring both fidelity *and* that coverage needs `conductor server start --jar <path>`,
option A of [#105](https://github.com/conductor-oss/conductor-cli/issues/105), filed
separately as [#118](https://github.com/conductor-oss/conductor-cli/issues/118).

## The seam this leaves for the capabilities matrix

Making the older legs blocking is deliberately not attempted here. It requires the
suites to know which capabilities exist in which Server Version — the cross-version
capabilities matrix, which is separate work.

The seam that work builds on is the pin declaration in the `server-versions` job: it
emits a JSON array of `{version, blocking}` objects that the OSS-safe job consumes as
its matrix, and each leg exports its version to the suite through
`conductor server start --version`. Adding a per-version capability dimension means
extending those objects and having the suites select on them, not inventing a
version-parameterised run from scratch.

## Alternatives considered

**Keep both launch paths behind a flag.** Rejected. It doubles the workflow's surface
to hedge a fidelity gap that is currently small, and an untaken path in CI rots
unnoticed. Documented recovery is cheaper than live redundancy.

**Pin an RC instead of the GA release.** Rejected. RCs are cut from `main`, so an RC is
a closer proxy for unreleased code, but it is not what users run, and the bucket's RC
coverage is patchy — several RCs return 403. Pinning an RC during a release cycle
remains a reasonable one-line change when fidelity matters more than fidelity to users.

**Ask for `main`-tracking snapshots to be published.** ADR-0002 called this the best
outcome, and [#105](https://github.com/conductor-oss/conductor-cli/issues/105) raised it
partly because `latest` looked abandoned. `latest` is demonstrably alive, so the request
is no longer needed to unblock pinning. It would still make the fidelity trade above
disappear, and remains worth having.
