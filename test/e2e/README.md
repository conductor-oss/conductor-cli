# E2E tests

Bats suites that drive the built `conductor` binary against a real Conductor
server. See [ADR-0001](../../docs/adr/0001-e2e-tests-are-tag-selected-bats-suites.md)
for why bats and why tags; see [CONTEXT.md](../../CONTEXT.md) for the meaning of
*tier*, *venue* and *Orkes-only*.

## Prerequisites

```bash
brew install bats-core            # >= 1.8.0, required for --filter-tags
go build -o conductor .           # suites invoke ./conductor from the repo root
```

Run from the **repository root**, not from this directory.

## Running

Against a local OSS server:

```bash
conductor server start --version 3.32.0   # from a scratch dir, see note below
export CONDUCTOR_SERVER_URL=http://localhost:8080/api
export CONDUCTOR_SERVER_TYPE=OSS
bats --filter-tags 'tier:pr,!orkes-only' test/e2e/
```

This is what CI does, and starting the server through the CLI rather than with
`java -jar` is what gives `server.bats` a CLI-managed Local server to assert against.

To reproduce a specific CI leg, pass that leg's Server Version. CI pins three, and the
checks list names the version in each job title. The pins are declared in one place —
the `server-versions` job in [`e2e.yml`](../../.github/workflows/e2e.yml) — and only the
current release gates a merge:

```bash
conductor server start --version 3.31.0    # a non-blocking leg
```

Pin a version the download bucket actually carries: it holds a subset of the server
repo's tags, so several tagged versions return 403. Check before pinning, which is also
what CI's preflight does:

```bash
curl -sI https://conductor-server.s3.us-east-2.amazonaws.com/conductor-server-3.31.0.jar | head -1
```

See [ADR-0006](../../docs/adr/0006-e2e-pins-published-server-versions-as-a-matrix.md) for
why E2E pins published versions instead of building the server from source, and how to
recover the source build if fidelity against unreleased `main` is needed.

Against an Orkes server:

```bash
export CONDUCTOR_SERVER_URL=https://your-server/api
export CONDUCTOR_AUTH_KEY=... CONDUCTOR_AUTH_SECRET=...
export CONDUCTOR_SERVER_TYPE=Enterprise
bats --filter-tags 'tier:pr,!oss-only,!unauthenticated' test/e2e/
bats --filter-tags 'unauthenticated' test/e2e/     # run with the key/secret UNSET
```

Nightly tier (spends model tokens, installs `agentspan` from PyPI):

```bash
bats --filter-tags 'tier:nightly' test/e2e/
```

> In CI the nightly tier is **manual-only**: there is no cron trigger, so it runs
> only via *Run workflow* on the Actions tab with `run_nightly` checked. The tag is
> named for its intended cadence, not its current wiring. Enabling the schedule means
> uncommenting the `schedule:` block in `.github/workflows/e2e.yml` and dropping the
> `workflow_dispatch` condition on the `e2e-nightly` job.

A single suite, or a count without running anything:

```bash
bats test/e2e/agent.bats
bats --count --filter-tags 'tier:pr,!orkes-only' test/e2e/
```

### What the OSS-safe run should report

Against the blocking Server Version: 116 tests and **1 skip** on Linux, the `#103`
Known-broken guard. On macOS expect a second, for absent GNU `timeout(1)`.

Against an older leg, expect **more** skips and some failures — neither older line
serves the Agents API, so nine `agent.bats` tests skip, and both currently fail three
`schedule.bats` tests. That is pre-existing version skew, which is why those legs do not
gate a merge;
[ADR-0006](../../docs/adr/0006-e2e-pins-published-server-versions-as-a-matrix.md) records
the root cause.

Anything skipping with *"no CLI-managed local server is running"* means the server was
not started through `conductor server start`, so the six `server.bats` tests did not
run. CI fails the leg on that rather than letting the coverage vanish quietly:

```bash
bats --filter-tags 'tier:pr,!orkes-only' test/e2e/ | grep '# skip'
```

> Start the local server from a scratch directory (e.g. `/tmp/conductor-e2e`).
> `conductor server start` writes its SQLite database relative to the working
> directory with no flag to override it, so starting it from the repo drops a
> multi-hundred-MB `c123.db` here. See issue #104.

## Changing which Server Versions CI tests

The OSS-safe suite runs once per pinned Server Version. All the pins live in one place —
the `server-versions` job in [`e2e.yml`](../../.github/workflows/e2e.yml):

```json
[
  { "version": "3.32.0", "blocking": true  },
  { "version": "3.31.0", "blocking": false },
  { "version": "3.30.2", "blocking": false }
]
```

`blocking: true` means that leg can fail a merge. Exactly one entry must be blocking, and
the job refuses to start otherwise. Every other leg runs on the same pull requests and
reports in the same checks list, but cannot gate a merge, so version skew that predates
your change can't block it.

### Before adding a version, check it exists

The jar bucket carries only a *subset* of the server repo's tags — some tagged versions
are simply absent. Check with a `HEAD` first:

```bash
V=3.31.0
curl -sI "https://conductor-server.s3.us-east-2.amazonaws.com/conductor-server-$V.jar" | head -1
```

`200` means you can pin it. `403` means you can't, whatever `git tag` says in the server
repo. This is why the 3.31 line is pinned at `3.31.0` rather than a later patch.

CI checks this too, and treats the two cases differently: an unavailable **blocking** pin
fails the preflight outright, while an unavailable **non-blocking** pin only drops its own
leg, with a warning saying so. Failing the whole preflight would skip the blocking leg as
well, which would let an older version gate a merge through the back door.

### The four changes you're likely to make

| Goal | Change |
|---|---|
| Bump the blocking pin for a new release | Edit the `version` of the `blocking: true` entry |
| Add a line to the matrix | Add an entry with `"blocking": false` |
| Stop testing a line | Delete its entry |
| Make an older line gate merges | Flip its `blocking` to `true` and the old one to `false` |

Nothing else needs touching: the job name, the jar cache key, and the nightly job's
version all derive from these entries. Adding a leg costs one jar download the first
time and is cache-hit afterwards, because pins are immutable.

Promoting an older line to blocking is a one-line edit but a real commitment — it means
the suite must pass against that line, and today the older lines fail tests for reasons
that are server-side, not CLI-side. Expect to fix or tag around those first.

### Check your change before pushing

```bash
# what each leg will run
bats --count --filter-tags 'tier:pr,!orkes-only' test/e2e/

# reproduce a leg locally, from a scratch dir, on a fresh database
conductor server start --version 3.31.0
```

Run each version against its own working directory. Pointing two Server Versions at one
`c123.db` produces spurious failures that have nothing to do with the version.

## Tags

| Tag | Meaning |
|-----|---------|
| `tier:pr` | Runs on every pull request. Free and deterministic. |
| `tier:nightly` | Costs money, or depends on a third party, or is non-deterministic. Intended for a nightly cadence; currently run manually only. |
| `orkes-only` | Requires Orkes Conductor. OSS refuses the operation. |
| `oss-only` | Requires a local OSS server; meaningless against a remote Orkes server. |
| `unauthenticated` | Needs a *secured* server reached *without* credentials. Must be excluded from authenticated runs. |
| `needs:llm` | Requires a model provider credential. |
| `needs:agentspan` | Requires the `agentspan` Python package. |
| `needs:timeout` | Requires GNU `timeout(1)` (absent on stock macOS). |

Selection uses **negation**, so untagged tests run everywhere. Tag only the
exceptions. `bats` `file_tags` can be added to by `test_tags` but never subtracted,
which is why the default has to be permissive.

## Conventions

- One suite per command, named `<command>.bats`.
- Numbered test names (`"1. Create workflow definition"`), so ordering is legible in
  output.
- `setup_file` checks for `./conductor` and cleans up leftovers from prior runs;
  `teardown_file` cleans up again. Tests must be re-runnable without manual reset.
- Assert on `$status` as well as `$output`. The CLI exits non-zero on failure, so
  status assertions are meaningful.
- `echo "Output: $output"` before assertions, so failures are diagnosable from CI
  logs.
- Suites are self-contained: helpers are duplicated per file rather than shared, in
  keeping with the existing suites.

### Tests for known defects

Assert the **correct** behaviour, then `skip` with the issue number:

```bash
@test "12. Agent compile returns an execution plan" {
    skip "known broken: #96 — CLI sends a bare config, server rejects it"
    ...
}
```

CI stays green, the gap is executable documentation, and verifying a fix is just
removing the line. A skip that outlives its defect is a lie — delete it as part of
the fix.

### Mixed-tier suites

Put the tier in `test_tags` per test rather than in `file_tags`, since `file_tags`
applies to every test in the file and cannot be overridden. `agent.bats` does this.

## Fixtures

`fixtures/python_agents/` is a minimal project used by `deploy.bats` to exercise
agent discovery. `deploy.bats` provisions its `.venv` on first run from the pinned
`requirements.txt`, and *skips* rather than fails when that is not possible, so a
PyPI outage cannot turn into a red build.
