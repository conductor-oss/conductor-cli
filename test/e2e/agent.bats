#!/usr/bin/env bats

# E2E tests for agent commands
# Covers config scaffolding, definition CRUD, execution search, and (in the
# nightly tier) live agent runs that call a real LLM.
#
# Tier is declared per test rather than at file level: the offline/CRUD tests are
# free and deterministic (tier:pr), while runs that invoke a provider cost tokens
# and are non-deterministic (tier:nightly,needs:llm). Assertions on LLM tests are
# structural only — execution ids and terminal status, never generated text.

# Tier is set per test; no file-level tier, because bats file_tags can only be
# added to by test_tags, never subtracted.

AGENT_NAME="e2e_agent_probe"

# A model verified to exist; the `agent init` default (openai/gpt-4o) is not
# usable unless an OpenAI key happens to be configured — see #103.
LLM_MODEL="anthropic/claude-haiku-4-5-20251001"

setup_file() {
    # Ensure the CLI binary exists
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi

    ./conductor agent delete "$AGENT_NAME" -y 2>/dev/null || true
}

teardown_file() {
    ./conductor agent delete "$AGENT_NAME" -y 2>/dev/null || true
}

# Helper: skip when no LLM provider credential is available.
require_llm() {
    if [ -z "$ANTHROPIC_API_KEY" ]; then
        skip "no ANTHROPIC_API_KEY configured"
    fi
}

# Helper: skip when the server has no Agents API. Not every Conductor deployment
# enables it — including the Enterprise server CI targets, which answers
# "Agents API is not available on this Conductor server". That is a deployment fact,
# not a CLI defect, so skip rather than fail. Applied per test rather than in
# setup() so the offline `agent init` tests still run everywhere.
require_agents_api() {
    if ./conductor agent list 2>&1 | grep -q 'Agents API is not available'; then
        skip "server has no Agents API enabled"
    fi
}

# Helper: write an agent config that uses a known-good model.
write_agent_config() {
    local path="$1"
    cat > "$path" <<EOF
name: $AGENT_NAME
description: e2e probe agent
model: $LLM_MODEL
instructions: You are terse. Answer with a single short word.
maxTurns: 2
tools: []
EOF
}

# Helper: run a command with a portable time bound (macOS has no GNU timeout).
# Returns 124 on timeout, mirroring timeout(1).
run_bounded() {
    local secs="$1"; shift
    "$@" >"$BATS_TEST_TMPDIR/bounded.out" 2>&1 &
    local pid=$!
    local i=0
    while [ "$i" -lt "$secs" ]; do
        kill -0 "$pid" 2>/dev/null || { wait "$pid"; return $?; }
        sleep 1
        i=$((i + 1))
    done
    kill "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
    return 124
}

# ---- definition scaffolding and CRUD (tier:pr) ----

# bats test_tags=tier:pr
@test "1. Agent init creates a YAML config" {
    run bash -c "cd '$BATS_TEST_TMPDIR' && '$PWD/conductor' agent init inittest 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [ -f "$BATS_TEST_TMPDIR/inittest.yaml" ]
}

# bats test_tags=tier:pr
@test "2. Agent init --format json creates a JSON config" {
    run bash -c "cd '$BATS_TEST_TMPDIR' && '$PWD/conductor' agent init jsontest --format json 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [ -f "$BATS_TEST_TMPDIR/jsontest.json" ]
}

# bats test_tags=tier:pr
@test "3. Agent init --strategy records the strategy" {
    run bash -c "cd '$BATS_TEST_TMPDIR' && '$PWD/conductor' agent init strattest --strategy handoff 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    grep -q 'handoff' "$BATS_TEST_TMPDIR/strattest.yaml"
}

# bats test_tags=tier:pr
@test "4. Agent list succeeds" {
    require_agents_api
    run bash -c "./conductor agent list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

# bats test_tags=tier:pr
@test "5. Agent list --json produces valid JSON" {
    require_agents_api
    run bash -c "./conductor agent list --json 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | python3 -c 'import json,sys; json.load(sys.stdin)'
}

# bats test_tags=tier:pr
@test "6. Agent get for an unknown name fails" {
    require_agents_api
    run bash -c "./conductor agent get e2e_agent_definitely_absent 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

# bats test_tags=tier:pr
@test "7. Agent delete for an unknown name fails" {
    require_agents_api
    run bash -c "./conductor agent delete e2e_agent_definitely_absent -y 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

# ---- execution search (tier:pr) ----

# bats test_tags=tier:pr
@test "8. Agent execution search succeeds" {
    require_agents_api
    run bash -c "./conductor agent execution 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

# bats test_tags=tier:pr
@test "9. Agent execution --name filter succeeds" {
    require_agents_api
    run bash -c "./conductor agent execution --name e2e_agent_definitely_absent 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"No executions found"* ]]
}

# bats test_tags=tier:pr
@test "10. Agent execution --status filter succeeds" {
    require_agents_api
    run bash -c "./conductor agent execution --status FAILED 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

# bats test_tags=tier:pr
@test "11. Agent prune --dry-run does not delete" {
    require_agents_api
    run bash -c "./conductor agent prune --older-than 3650 --dry-run 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Dry run"* ]]
}

# ---- known-broken guards ----

# Regression guard for #96: the CLI posts a bare config instead of
# {"agentConfig": ...}, so compile fails for every input.
# bats test_tags=tier:pr
@test "12. Agent compile returns an execution plan" {
    skip "known broken: #96 — CLI sends a bare config, server rejects it"
    write_agent_config "$BATS_TEST_TMPDIR/compile.yaml"
    run bash -c "./conductor agent compile '$BATS_TEST_TMPDIR/compile.yaml' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

# ---- live LLM runs (tier:nightly) ----

# bats test_tags=tier:nightly,needs:llm
@test "13. Agent run --config completes and returns an execution id" {
    require_agents_api
    require_llm
    write_agent_config "$BATS_TEST_TMPDIR/run.yaml"

    run bash -c "./conductor agent run --config '$BATS_TEST_TMPDIR/run.yaml' 'Reply with one word' --no-stream 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Structural assertion only: a UUID execution id must be present.
    [[ "$output" =~ [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12} ]]
}

# bats test_tags=tier:nightly,needs:llm
@test "14. Agent run registers the agent, then run --name works" {
    require_agents_api
    require_llm
    write_agent_config "$BATS_TEST_TMPDIR/run2.yaml"
    ./conductor agent run --config "$BATS_TEST_TMPDIR/run2.yaml" "Reply with one word" --no-stream >/dev/null 2>&1

    run bash -c "./conductor agent list 2>/dev/null"
    echo "List: $output"
    [[ "$output" == *"$AGENT_NAME"* ]]

    run bash -c "./conductor agent run --name $AGENT_NAME 'Reply with one word' --no-stream 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

# bats test_tags=tier:nightly,needs:llm
@test "15. Agent status reports a terminal state for a finished run" {
    require_agents_api
    require_llm
    write_agent_config "$BATS_TEST_TMPDIR/run3.yaml"
    out=$(./conductor agent run --config "$BATS_TEST_TMPDIR/run3.yaml" "Reply with one word" --no-stream 2>&1)
    eid=$(echo "$out" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1)
    echo "Execution: $eid"
    [ -n "$eid" ]

    # Poll until terminal, bounded.
    status_val=""
    for _ in $(seq 1 30); do
        status_val=$(./conductor agent status "$eid" 2>/dev/null | grep '"status"' | cut -d'"' -f4)
        [ "$status_val" != "RUNNING" ] && break
        sleep 2
    done
    echo "Final status: $status_val"
    [ -n "$status_val" ]
    [ "$status_val" != "RUNNING" ]
}

# Regression guard for #97: --since and --window return nothing even when
# matching executions exist.
# bats test_tags=tier:nightly,needs:llm
@test "16. Agent execution --since finds a just-created execution" {
    skip "known broken: #97 — --since/--window always return no results"
    require_llm
    write_agent_config "$BATS_TEST_TMPDIR/run4.yaml"
    ./conductor agent run --config "$BATS_TEST_TMPDIR/run4.yaml" "Reply with one word" --no-stream >/dev/null 2>&1

    run bash -c "./conductor agent execution --since 1d 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" != *"No executions found"* ]]
}

# Regression guard for #102: agent stream never exits once the execution is
# terminal. run_bounded keeps this from hanging CI if the skip is removed before
# the fix lands.
# bats test_tags=tier:nightly,needs:llm
@test "17. Agent stream exits after the terminal event" {
    skip "known broken: #102 — agent stream hangs on a terminal execution"
    require_llm
    write_agent_config "$BATS_TEST_TMPDIR/run5.yaml"
    out=$(./conductor agent run --config "$BATS_TEST_TMPDIR/run5.yaml" "Reply with one word" 2>&1)
    eid=$(echo "$out" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1)
    [ -n "$eid" ]

    run_bounded 30 ./conductor agent stream "$eid"
    rc=$?
    echo "stream exit: $rc"
    cat "$BATS_TEST_TMPDIR/bounded.out" || true
    [ "$rc" -ne 124 ]
}
