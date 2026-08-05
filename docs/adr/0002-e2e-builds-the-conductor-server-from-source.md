---
status: accepted
---

# E2E builds the Conductor server from source rather than pinning a published version

The E2E jobs check out `conductor-oss/conductor` at `main` and run
`:conductor-server:bootJar`, instead of pinning a published version via
`conductor server start --version <x>`. We chose this because releases are cut from
`main` and **no artifact is published from `main`**, so a pinned version cannot
validate the code that will actually ship. This is a deliberate trade of speed,
reproducibility and CI isolation for fidelity, and it is expected to be revisited —
see [#105](https://github.com/conductor-oss/conductor-cli/issues/105).

## Why pinning was the preferred option, and why we did not take it

Pinning is the better engineering default and was the original plan. `@mp-orkes`
raised it in review of [#106](https://github.com/conductor-oss/conductor-cli/pull/106):

> I really don't think we should be doing the gradle build from `main`. We should do
> something like `conductor server start --version 3.32.0-rc.23`

It is faster (a cached jar versus a Gradle build), byte-for-byte reproducible between
runs, and it keeps this repo's CI independent of the server repo's build health. It
also uses the CLI's own `server start`, which means the `server` command is exercised
rather than bypassed.

What blocked it is narrow and factual: **there is no published artifact built from
`main`.**

- Maven Central's `org.conductoross:conductor-server` publishes tagged releases only.
  At the time of writing its newest was `3.32.0-rc.23`.
- The S3 jar that `conductor server start` downloads as `latest` had
  `Last-Modified: 3 June 2026` — months stale, and apparently no longer published.

Release validation for this cycle was explicitly scoped to `main`, because that is
what the release is cut from. Pinning `3.32.0-rc.23` would have tested a snapshot 8
commits behind `main`. In this instance those 8 commits were UI fixes, a CI fix,
provider API-key trimming and agent token-usage aggregation — nothing CLI-facing — so
pinning would have been *adequate*. But that is a property of this particular gap, not
a guarantee, and it is only knowable after the fact.

## Consequences

**A Gradle build runs on every PR.** Measured at 3m56s for the whole job on a cold
runner, which is acceptable but not free, and it will grow with the server.

**This repo's CI becomes sensitive to the server repo.** A broken `main` in
`conductor-oss/conductor` turns conductor-cli PRs red for reasons unrelated to the
CLI. This is the most significant cost and the most likely trigger for reverting to a
pin.

**Runs are not reproducible.** Two runs of the same CLI commit can test different
server code. A failure may not reproduce later.

**`server start` is bypassed, so six tests skip.** It can only download published
versions, so a source-built jar must be launched with `java -jar`. With no CLI-managed
pid file, the six server-dependent tests in `server.bats` skip with a stated reason.
#105 proposes a `--jar` flag to close that.

## When to revisit

Any of these should prompt switching back to a pin:

- A red CI run traced to the server build rather than the CLI.
- A `main`-tracking snapshot becoming available — either a fixed S3 `latest` or
  published snapshots. That removes the reason for this decision entirely.
- Release validation no longer being scoped to unreleased `main`.

The workflow keeps a single knob for this: `CONDUCTOR_SERVER_REF`. Reverting means
replacing the checkout-and-build steps with `conductor server start --version <x>`,
which the earlier revision of #106 already implemented, so the change is recoverable
from git history rather than needing redesign.

## Alternatives considered

**Pin the newest published RC.** Covered above. Rejected for this cycle only because
it cannot test `main`; preferred on every other axis.

**Ask for `main` snapshots to be published.** The best outcome — it would make this
ADR obsolete and give CI a cheap, current artifact. Depends on another team, so it was
not available for this release. Raised in #105, together with the apparently stalled
S3 `latest` publish.

**Test only against the remote Enterprise server, as CI did before.** Rejected: it
pins `CONDUCTOR_SERVER_TYPE=Enterprise`, so OSS code paths were never exercised, and it
targets a server of unknown version. That arrangement is what allowed
[#101](https://github.com/conductor-oss/conductor-cli/issues/101) —
`schedule pause`/`resume` completely broken on OSS — to go unnoticed.
