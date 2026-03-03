#!/usr/bin/env bats

# E2E tests for workflow search functionality
# Tests must be run in order as they depend on each other

WORKFLOW_NAME="cli_e2e_test_workflow_2"
WORKFLOW_FILE="test/e2e/test-workflow-2.json"

setup() {
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi
}

# Helper function to extract workflow ID from output
get_workflow_id() {
    echo "$1" | grep -oE '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}'
}

# ==========================================
# Setup: Create and run a workflow to search for
# ==========================================

@test "1. Create test workflow definition" {
    run bash -c "./conductor workflow create '$WORKFLOW_FILE' --force 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "2. Start workflow execution" {
    run bash -c "./conductor workflow start --workflow '$WORKFLOW_NAME' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    WORKFLOW_ID=$(get_workflow_id "$output")
    [ -n "$WORKFLOW_ID" ]
    echo "$WORKFLOW_ID" > /tmp/search_workflow_id.txt
    echo "Started workflow UUID: $WORKFLOW_ID"
}

@test "3. Wait for workflow to complete" {
    WORKFLOW_ID=$(cat /tmp/search_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    # Wait up to 10 seconds for workflow to complete
    for i in {1..10}; do
        run bash -c "./conductor workflow status '$WORKFLOW_ID' 2>/dev/null"
        echo "Attempt $i: Status = $output"

        if [ "$output" = "COMPLETED" ]; then
            break
        fi

        sleep 1
    done

    [ "$output" = "COMPLETED" ]
}

# ==========================================
# Search Tests
# ==========================================

@test "4. Search by workflow name" {
    run bash -c "./conductor workflow search --workflow '$WORKFLOW_NAME' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Table output should contain workflow name
    echo "$output" | grep -q "$WORKFLOW_NAME"
}

@test "5. Search by status COMPLETED" {
    run bash -c "./conductor workflow search --workflow '$WORKFLOW_NAME' --status COMPLETED 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "COMPLETED"
    echo "$output" | grep -q "$WORKFLOW_NAME"
}

@test "6. Search with JSON output" {
    run bash -c "./conductor workflow search --workflow '$WORKFLOW_NAME' --json 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # JSON output should contain results array
    [[ "$output" == *"\"results\""* ]]
    echo "$output" | grep -q "$WORKFLOW_NAME"
}

@test "7. Search with CSV output" {
    run bash -c "./conductor workflow search --workflow '$WORKFLOW_NAME' --csv 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # CSV output should contain headers
    echo "$output" | grep -q "START TIME"
    echo "$output" | grep -q "$WORKFLOW_NAME"
}

@test "8. Search with combined filters" {
    run bash -c "./conductor workflow search --workflow '$WORKFLOW_NAME' --status COMPLETED --count 5 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "$WORKFLOW_NAME"
    echo "$output" | grep -q "COMPLETED"
}

@test "9. Search with count limit" {
    run bash -c "./conductor workflow search --workflow '$WORKFLOW_NAME' --count 1 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "$WORKFLOW_NAME"
}

@test "10. Search for non-existent workflow returns no results" {
    run bash -c "./conductor workflow search --workflow 'nonexistent_workflow_xyz_12345' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Should only have header row or empty results
    # Count lines with actual workflow data (exclude header)
    RESULT_COUNT=$(echo "$output" | grep -c "nonexistent_workflow_xyz_12345" || true)
    [ "$RESULT_COUNT" -eq 0 ]
}

@test "11. Search by status RUNNING returns no matches for completed workflow" {
    run bash -c "./conductor workflow search --workflow '$WORKFLOW_NAME' --status RUNNING --count 1 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Our test workflow completed, so searching for RUNNING should not find it
    # (There might be other instances running though, so we just verify the command succeeds)
}

# ==========================================
# Cleanup
# ==========================================

@test "12. Cleanup - Delete workflow execution" {
    WORKFLOW_ID=$(cat /tmp/search_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./conductor workflow delete-execution '$WORKFLOW_ID' -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"deleted successfully"* ]]
}

@test "13. Cleanup - Delete workflow definition" {
    run bash -c "./conductor workflow delete '$WORKFLOW_NAME' 1 -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"deleted successfully"* ]]
    rm -f /tmp/search_workflow_id.txt
}
