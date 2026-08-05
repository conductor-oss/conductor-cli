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
conductor server start --version 3.32.0-rc.23   # from a scratch dir, see note below
export CONDUCTOR_SERVER_URL=http://localhost:8080/api
export CONDUCTOR_SERVER_TYPE=OSS
bats --filter-tags 'tier:pr,!orkes-only' test/e2e/
```

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

A single suite, or a count without running anything:

```bash
bats test/e2e/agent.bats
bats --count --filter-tags 'tier:pr,!orkes-only' test/e2e/
```

> Start the local server from a scratch directory (e.g. `/tmp/conductor-e2e`).
> `conductor server start` writes its SQLite database relative to the working
> directory with no flag to override it, so starting it from the repo drops a
> multi-hundred-MB `c123.db` here. See issue #104.

## Tags

| Tag | Meaning |
|-----|---------|
| `tier:pr` | Runs on every pull request. Free and deterministic. |
| `tier:nightly` | Runs on the nightly schedule. Costs money, or depends on a third party, or is non-deterministic. |
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
