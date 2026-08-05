#!/usr/bin/env bats

# E2E tests for the deploy command
# Covers agent discovery, deployment, name filtering, JSON output and error paths
# against the fixture project in test/e2e/fixtures/python_agents.
#
# deploy shells out to `python -m agentspan.cli.discover`, so this suite depends
# on the external `agentspan` package (pinned in the fixture's requirements.txt).
# That external dependency is why it sits in the nightly tier behind
# needs:agentspan rather than running on every PR — a PyPI outage or an SDK API
# change must not block merges.
#
# Only the Python path is covered; TypeScript deploy is deliberately out of scope.

# bats file_tags=tier:nightly,needs:agentspan

FIXTURE="test/e2e/fixtures/python_agents"
PKG="fixture_agents"
AGENT_1="e2e_fixture_greeter"
AGENT_2="e2e_fixture_echo"

setup_file() {
    # Ensure the CLI binary exists
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi

    if [ ! -d "$FIXTURE" ]; then
        echo "ERROR: fixture project not found at $FIXTURE"
        exit 1
    fi

    # Provision the fixture venv once per file. conductor deploy auto-detects
    # <project>/.venv/bin/python.
    if [ ! -x "$FIXTURE/.venv/bin/python" ]; then
        python3 -m venv "$FIXTURE/.venv" >/dev/null 2>&1 || return 0
        "$FIXTURE/.venv/bin/pip" install -q -r "$FIXTURE/requirements.txt" >/dev/null 2>&1 || return 0
    fi

    ./conductor agent delete "$AGENT_1" -y 2>/dev/null || true
    ./conductor agent delete "$AGENT_2" -y 2>/dev/null || true
}

teardown_file() {
    ./conductor agent delete "$AGENT_1" -y 2>/dev/null || true
    ./conductor agent delete "$AGENT_2" -y 2>/dev/null || true
}

# Helper: skip when the fixture venv could not be provisioned (offline CI, PyPI
# outage). Better to skip loudly than to fail for an unrelated reason.
require_agentspan() {
    if [ ! -x "$FIXTURE/.venv/bin/python" ]; then
        skip "fixture venv not provisioned (agentspan unavailable)"
    fi
    if ! "$FIXTURE/.venv/bin/python" -c 'import agentspan.cli.discover' 2>/dev/null; then
        skip "agentspan.cli.discover not importable in fixture venv"
    fi
}

@test "1. Deploy discovers both fixture agents" {
    require_agentspan

    run bash -c "cd '$FIXTURE' && '$PWD/conductor' deploy --language python --package $PKG -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"$AGENT_1"* ]]
    [[ "$output" == *"$AGENT_2"* ]]
}

@test "2. Deploy reports the discovered count" {
    require_agentspan

    run bash -c "cd '$FIXTURE' && '$PWD/conductor' deploy --language python --package $PKG -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Discovered 2 agent(s)"* ]]
}

@test "3. Deployed agents appear in agent list" {
    require_agentspan
    bash -c "cd '$FIXTURE' && '$PWD/conductor' deploy --language python --package $PKG -y" >/dev/null 2>&1 || true

    run bash -c "./conductor agent list 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"$AGENT_1"* ]]
    [[ "$output" == *"$AGENT_2"* ]]
}

@test "4. Deploy --agents filters to a single agent" {
    require_agentspan

    run bash -c "cd '$FIXTURE' && '$PWD/conductor' deploy --language python --package $PKG --agents $AGENT_2 -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"All 1 agent(s) deployed successfully"* ]]
}

@test "5. Deploy --json emits valid JSON with a summary" {
    require_agentspan

    run bash -c "cd '$FIXTURE' && '$PWD/conductor' deploy --language python --package $PKG --json -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | python3 -c '
import json,sys
d = json.load(sys.stdin)
assert "summary" in d, "missing summary"
assert d["summary"]["total"] == 2, d["summary"]
assert d["summary"]["failed"] == 0, d["summary"]
'
}

@test "6. Deploy with an unknown agent name fails and lists the known ones" {
    require_agentspan

    run bash -c "cd '$FIXTURE' && '$PWD/conductor' deploy --language python --package $PKG --agents e2e_not_a_real_agent -y 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"not found"* ]]
    [[ "$output" == *"$AGENT_1"* ]]
}

@test "7. Deploy with an unknown package fails" {
    require_agentspan

    run bash -c "cd '$FIXTURE' && '$PWD/conductor' deploy --language python --package definitely_not_a_package -y 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}
