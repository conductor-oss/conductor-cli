---
status: accepted
---

# E2E pins a published Server Version rather than building the server from source

The server-backed E2E jobs no longer check out `conductor-oss/conductor` and build it
with Gradle. They start a Local server through the CLI's own
`conductor server start --version <pinned>`, and the OSS-safe job runs that suite
against the current release, which gates a merge.

The job is shaped as a matrix over a declared list of pins, but the list holds one
entry. Testing older lines was considered and deliberately not adopted — see
[why the older lines are not pinned](#why-the-older-lines-are-not-pinned).

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

## What pinning buys, and why the shape is still a matrix

ADR-0002 tested one moving target, and that target was not a version anyone had
released. Pinning the current release means CI tests the version users actually have,
byte-identically between runs.

The job is nonetheless written as a matrix over a declared list of `{version, blocking}`
objects, with `continue-on-error` on any entry that is not blocking. That machinery
costs nothing at one entry and makes testing an additional line a one-line edit rather
than a redesign — which matters, because the reason no older line is pinned today is a
server-side one that may well be resolved (below). Exactly one entry must be blocking,
and the preflight refuses to start otherwise.

The nightly tier stays on the blocking version alone. Its suites depend on
capabilities older lines lack, and an extra leg would spend model tokens per run to
discover that.

The Orkes-facing job is untouched. It addresses a remote server whose Server Version is
neither known nor selectable, so a pin cannot reach it.

## Why the older lines are not pinned

`3.31.0` and `3.30.2` were pinned as non-blocking legs in an earlier revision of this
change and run in CI. What follows is the measurement, and why the legs were then
dropped rather than kept.

All three Server Versions were run before this landed, each downloaded through the CLI
and started against its own fresh database, so the older ones are a measurement rather
than an expectation. These counts are from a **local macOS run**; one skip on every line
is the macOS-only GNU `timeout(1)` guard, which does not skip on a Linux runner, so
expect one fewer of each in CI:

| Server Version | tests | skips | failures |
|---|---|---|---|
| `3.32.0` (pinned, blocking) | 116 | 2 | 0 |
| `3.31.0` (not pinned) | 116 | 11 | 3 |
| `3.30.2` (not pinned) | 116 | 11 | 3 |

The six recovered `server.bats` tests pass on **all three**, so the launch path itself is
not version-sensitive — the result that actually mattered for this change. The older
lines are otherwise *not* clean.

The mechanism worked as designed: the preflight passed, the matrix expanded to the three
expected legs, the `3.32.0` leg and the Orkes-facing job passed, both older legs failed,
and the run's overall conclusion was still **success**.

**It was not the mechanism that failed, it was the reading cost.** Two permanently-red
checks on every pull request oblige every reviewer to learn which reds are load-bearing,
and standing red is indistinguishable at a glance from a CI system that is merely broken.
The failures are known, understood, upstream, and not the CLI's; that is a fact worth
recording once, here, rather than re-asserting on every run through a check that readers
must be told to ignore. Nobody asked for the older lines when the choice was put to
review, so nothing is being given up that anyone had claimed.

**Three failures on both older lines, one root cause — and it is already fixed upstream.**
`GET /api/scheduler/schedules/<name>` returns `HTTP 200` with an empty body for a
schedule that does not exist. That is
[conductor-oss/conductor#1357](https://github.com/conductor-oss/conductor/pull/1357),
merged 2026-07-19 as `c08d60c8a`, which makes the endpoint throw `NotFoundException`
instead. By ancestry the fix is in `v3.32.0` and its RCs from `rc.14` on, and in neither
`v3.31.0` nor `v3.30.2` — so the older lines were not reporting a new defect, they were
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
is nothing to file, and making these lines green would mean asking for a backport of
#1357 to the 3.31 and 3.30 lines. That is a support-policy decision rather than a bug
report, and it is the decision that would have to be taken before pinning them is worth
doing.

**The nine extra skips are the Agents API.** Neither older line serves it, so nine
`agent.bats` tests skip on their runtime guard rather than fail. That guard exists
because not every deployment serves those endpoints, so this is intended behaviour, not
a gap.

Neither finding is a defect in this change. Together they are the case for not pinning
these lines: the suite cannot be green against either one for reasons that live outside
this repository, and a check that can only be red reports nothing a reader can act on.

**What would change this.** If #1357 is backported to the 3.31 or 3.30 line, or the
project decides it wants a supported-line signal enough to tag around the three
`schedule.bats` failures, add the entry back with `"blocking": false`. The machinery is
still there; the argument above is what would need to have changed, not the workflow.

One caveat for anyone reproducing this locally: run each version against a *fresh*
working directory. Pointing two Server Versions at one `c123.db` produced an extra,
spurious `workflow create --force` failure on `3.30.2` that disappears on a clean
database. CI is unaffected — each matrix leg is its own runner, and the jar cache holds
only the jar, not the database or the server state.

## Published availability is the constraint, not the tag list

The S3 bucket the CLI downloads from carries a *subset* of the server repo's tags.
Verified by `HEAD` at the time of writing: `3.32.0`, `3.31.0`, `3.30.2` and `latest`
return 200, while `3.31.2` and `3.21.23` return 403. A pin must therefore be chosen
against bucket contents rather than against `git tag` — the 3.31 line, had it been
pinned, would have had to sit at `3.31.0` rather than at its newest patch.

Because "someone pins a tag the bucket does not carry" is a live failure mode rather
than a hypothetical, a preflight job `HEAD`s every pin before any job does real work,
naming the version, the URL and the status code. Without it a bad pin surfaces minutes
into a job as an opaque download error.

The preflight is asymmetric, and deliberately so. An unavailable **blocking** pin fails
it outright — there is nothing meaningful left to run. An unavailable **non-blocking**
pin only drops its own leg, with a warning naming what was dropped, because failing the
whole preflight would skip the blocking leg along with it and let an older-version
problem gate a merge through the back door. The preflight therefore emits the matrix it
verified rather than the matrix it was given. With a single blocking pin the second
branch is currently unexercised; it is kept because it is what makes adding a line safe.

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

**The pin needs bumping.** A release nobody pins is a release nobody tests, and with a
single pin that is the whole of the version signal. The pin lives in one declaration in
the workflow, and the preflight refuses a version the bucket does not carry, so a bump
either works or fails immediately and legibly.

**Nothing tests the older supported lines.** This is the cost of the section above, and
it is real: a break against 3.31 or 3.30 will now first be reported by a user. It was
accepted because the alternative on offer was not a working signal but a permanently
red one.

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

Testing older lines at all — let alone gating on them — is deliberately not attempted
here. Doing it properly requires the suites to know which capabilities exist in which
Server Version, so that a line which simply lacks an endpoint skips rather than fails.
That is the cross-version capabilities matrix, and it is separate work.

The seam that work builds on is the pin declaration in the `server-versions` job: it
emits a JSON array of `{version, blocking}` objects that the OSS-safe job consumes as
its matrix, and each leg exports its version to the suite through
`conductor server start --version`. Adding a per-version capability dimension means
extending those objects and having the suites select on them, not inventing a
version-parameterised run from scratch.

## Alternatives considered

**Keep 3.31.0 and 3.30.2 as non-blocking legs.** Implemented, run in CI, then rejected —
see [why the older lines are not pinned](#why-the-older-lines-are-not-pinned). The signal
they carried was one standing fact, already recorded here; the cost was two permanently
red checks on every pull request and a reviewer convention for ignoring them. Put to
reviewers in the pull request that introduced them, with no reply either way.

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
