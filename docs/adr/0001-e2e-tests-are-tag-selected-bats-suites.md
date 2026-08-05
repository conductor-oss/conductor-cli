---
status: accepted
---

# E2E tests are bats suites selected by tier and venue tags

The CLI's end-to-end tests are bats suites under `test/e2e/`, and CI chooses which
of them to run using bats' native tag filtering (`# bats file_tags=` /
`--filter-tags`) rather than per-job lists of filenames. Tests are tagged by
**tier** (when they run) and, where it matters, by **venue** (which Conductor
distribution they are valid against). We chose this because the E2E surface has to
span two server distributions with genuinely different capabilities, and because a
hand-maintained file list had already silently orphaned an entire suite.

## Considered options

**k6.** Explicitly proposed, and rejected on capability rather than preference:
k6's JavaScript runtime has no subprocess API — no `child_process`, no `os.exec` —
so it cannot invoke a CLI binary at all. Testing `conductor` with it would require
building a custom k6 with the `xk6-exec` extension and then driving a shell from
inside a load-testing VM. k6 remains the right tool if we ever want to load-test the
Conductor *server's* HTTP API, which is a different project.

**Go integration tests driving the binary via `os/exec`.** Genuinely attractive:
one toolchain, structured JSON assertions instead of `grep`, and idiomatic gating
via build tags. Rejected because 129 bats tests already existed and worked; adding
a second E2E idiom means two things to maintain and teach, for a benefit that is
real but not decisive.

**Positive tagging (tag every test with its venue).** Rejected after discovering
that bats `file_tags` can be *added to* by `test_tags` but never *subtracted from*.
Expressing "this file is valid everywhere except these two tests" positively would
mean tagging all ~150 remaining tests explicitly. We tag only the exceptions
(`orkes-only`, `oss-only`, `unauthenticated`) and select with negation, so the
default is "runs everywhere".

**Separate workflow files per tier.** Clear separation and independent scheduling,
but it duplicates checkout/build/bats-setup across files and drifts.

## Consequences

CI runs three jobs. Two run per pull request — one against a remote Orkes server for
the Orkes-only surface, one against a pinned local OSS server — and a third holds the
tests that spend model tokens or depend on PyPI. A test's venue is a property of the
test, so adding a suite requires no CI edit; this is what recovered
`api_gateway.bats`, whose 18 tests existed but appeared in no job's file list and so
ran nowhere.

The third job is **manual-only for now**: no `schedule:` trigger is configured, so it
runs only on an explicit `workflow_dispatch` with `run_nightly=true`. The tier keeps
the name `nightly` because that is the intended cadence, but scheduling was
deliberately deferred until someone owns watching the results — an unattended cron
that spends model tokens and reports to nobody is worse than no coverage. The cron
line is present but commented, with the one other change needed to enable it.

The local-server job exists because the remote-only arrangement could not, even in
principle, catch certain classes of defect: it pinned `CONDUCTOR_SERVER_TYPE` to
`Enterprise`, so OSS code paths were never exercised, and it tested against a server
of unknown version rather than the code being released. The first run of the new job
found `schedule pause`/`resume` completely broken on OSS (#101).

That job **builds the server from `conductor-oss/conductor` at `main`** rather than
downloading a published jar, because releases are cut from `main` and nothing is
published from it: Maven Central carries only tagged RCs, and the S3 `latest` jar has
not moved since June. Building is the only way to test what will actually ship.

The costs are real and tracked in #105: a Gradle build on every PR run, and this
repo's CI becoming sensitive to the server repo's build health. That choice, and the
reasons pinning a published version was preferred but not possible, are recorded
separately in [ADR-0002](./0002-e2e-builds-the-conductor-server-from-source.md).

One consequence is worth knowing: `conductor server start` can only download
published versions, so a source-built jar must be launched with `java -jar` directly.
No CLI-managed pid file exists, and the six server-dependent tests in `server.bats`
skip in CI. They skip with a stated reason rather than failing, and #105 proposes a
`--jar` flag that would close the gap.

The scheme depends on bats >= 1.8.0. Both server-backed jobs assert that
`--filter-tags` is supported before running, because a bats that ignored the flag
would silently run the wrong set of tests — a worse failure than an error.

Known-broken behaviour is recorded as a test of the correct behaviour plus a `skip`
naming the issue. This keeps CI green while making each gap executable
documentation, at the cost of requiring someone to remove the skip when the fix
lands; a stale skip is a lie, so removing it belongs in the fix.
