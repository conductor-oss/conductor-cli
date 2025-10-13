#!/usr/bin/env bats

# E2E tests for workflow rerun functionality

WORKFLOW_NAME="cli_e2e_test_workflow_2"
WORKFLOW_FILE="test/e2e/test-workflow-2.json"
WORKFLOW_ID=""

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi
}

# Helper function to extract workflow ID from execution start output
get_workflow_id() {
    echo "$1" | grep -oE '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}'
}

@test "1. Create test workflow definition" {
    run bash -c "./orkes workflow create '$WORKFLOW_FILE' --force 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "2. Start workflow execution" {
    run bash -c "./orkes execution start --workflow '$WORKFLOW_NAME' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Extract and save workflow ID
    WORKFLOW_ID=$(get_workflow_id "$output")
    [ -n "$WORKFLOW_ID" ]
    echo "$WORKFLOW_ID" > /tmp/rerun_workflow_id.txt
    echo "Started workflow UUID: $WORKFLOW_ID"
}

@test "3. Wait for workflow to complete" {
    # Read the workflow ID from previous test
    WORKFLOW_ID=$(cat /tmp/rerun_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    # Wait up to 10 seconds for workflow to complete
    for i in {1..10}; do
        run bash -c "./orkes execution status '$WORKFLOW_ID' 2>/dev/null"
        echo "Attempt $i: Status = $output"

        if [ "$output" = "COMPLETED" ]; then
            break
        fi

        sleep 1
    done

    [ "$output" = "COMPLETED" ]
}

@test "4. Get workflow execution details" {
    WORKFLOW_ID=$(cat /tmp/rerun_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./orkes execution get '$WORKFLOW_ID' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify workflow completed successfully
    echo "$output" | grep -q "COMPLETED"
}

@test "5. Rerun the completed workflow" {
    WORKFLOW_ID=$(cat /tmp/rerun_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./orkes execution rerun '$WORKFLOW_ID' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Rerun returns the same workflow ID (it reruns the same instance)
    RERUN_WORKFLOW_ID=$(get_workflow_id "$output")
    [ -n "$RERUN_WORKFLOW_ID" ]
    echo "Rerun workflow UUID: $RERUN_WORKFLOW_ID"

    # Verify it's the same workflow ID (rerun uses same instance)
    [ "$RERUN_WORKFLOW_ID" = "$WORKFLOW_ID" ]
}

@test "6. Verify rerun workflow completes" {
    WORKFLOW_ID=$(cat /tmp/rerun_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    # Wait up to 10 seconds for rerun workflow to complete
    for i in {1..10}; do
        run bash -c "./orkes execution status '$WORKFLOW_ID' 2>/dev/null"
        echo "Attempt $i: Rerun status = $output"

        if [ "$output" = "COMPLETED" ]; then
            break
        fi

        sleep 1
    done

    [ "$output" = "COMPLETED" ]
}

@test "7. Cleanup - Delete workflow execution" {
    WORKFLOW_ID=$(cat /tmp/rerun_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./orkes execution delete '$WORKFLOW_ID' -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"deleted successfully"* ]]
    echo "Deleted workflow UUID: $WORKFLOW_ID"
}

@test "8. Cleanup - Delete workflow definition" {
    run bash -c "./orkes workflow delete '$WORKFLOW_NAME' 1 -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"deleted successfully"* ]]
}
