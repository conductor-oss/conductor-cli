#!/usr/bin/env bats

# E2E tests for conductor-cli
# Tests must be run in order as they depend on each other

WORKFLOW_NAME="e2e_test_workflow"
WORKFLOW_FILE="test/e2e/test-workflow.json"
WORKFLOW_ID=""

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi

    # Verify required environment variables are set
    if [ -z "$CONDUCTOR_SERVER_URL" ]; then
        echo "ERROR: CONDUCTOR_SERVER_URL environment variable is not set"
        exit 1
    fi
}

# Helper function to extract workflow ID from execution start output
get_workflow_id() {
    echo "$1" | grep -oE '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}'
}

@test "1. Create workflow definition" {
    run ./orkes workflow create "$WORKFLOW_FILE" --force
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "2. Get workflow definition" {
    run ./orkes workflow get "$WORKFLOW_NAME"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"$WORKFLOW_NAME"* ]]
    [[ "$output" == *"wait_task"* ]]
}

@test "3. Start workflow execution" {
    run ./orkes execution start --workflow "$WORKFLOW_NAME"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Extract and save workflow ID
    WORKFLOW_ID=$(get_workflow_id "$output")
    [ -n "$WORKFLOW_ID" ]
    echo "$WORKFLOW_ID" > /tmp/workflow_id.txt
    echo "Workflow ID: $WORKFLOW_ID"
}

@test "4. Check workflow status is RUNNING" {
    # Read the workflow ID from previous test
    WORKFLOW_ID=$(cat /tmp/workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run ./orkes execution status "$WORKFLOW_ID"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == "RUNNING" ]]
}

@test "5. Terminate workflow execution" {
    # Read the workflow ID from previous test
    WORKFLOW_ID=$(cat /tmp/workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run ./orkes execution terminate "$WORKFLOW_ID"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "6. Check workflow status is TERMINATED" {
    # Read the workflow ID from previous test
    WORKFLOW_ID=$(cat /tmp/workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    # Wait a moment for termination to process
    sleep 2

    run ./orkes execution status "$WORKFLOW_ID"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == "TERMINATED" ]]
}

@test "7. Delete workflow execution" {
    # Read the workflow ID from previous test
    WORKFLOW_ID=$(cat /tmp/workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run ./orkes execution delete "$WORKFLOW_ID"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "8. Verify execution no longer exists" {
    # Read the workflow ID from previous test
    WORKFLOW_ID=$(cat /tmp/workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    # Wait a moment for deletion to process
    sleep 2

    run ./orkes execution status "$WORKFLOW_ID"
    echo "Output: $output"
    # Should fail since the execution was deleted
    [ "$status" -ne 0 ]
}

@test "9. Cleanup - delete workflow definition" {
    run ./orkes workflow delete "$WORKFLOW_NAME" 1
    echo "Output: $output"
    # Clean up the temp file
    rm -f /tmp/workflow_id.txt
}
