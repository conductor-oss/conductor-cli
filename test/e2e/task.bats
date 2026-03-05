#!/usr/bin/env bats

# E2E tests for task definition CRUD and task execution operations
# Tests must be run in order as they depend on each other

TASK_NAME="cli_e2e_test_task"
TASK_FILE="test/e2e/test-task.json"
WORKFLOW_NAME="cli_e2e_test_workflow"
WORKFLOW_FILE="test/e2e/test-workflow.json"

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
# Task Definition CRUD Tests
# ==========================================

@test "1. Create task definition from JSON file" {
    run bash -c "./conductor task create '$TASK_FILE' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "2. List task definitions (table output)" {
    run bash -c "./conductor task list 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Should contain table headers
    echo "$output" | grep -q "NAME"
    # Should contain our test task
    echo "$output" | grep -q "$TASK_NAME"
}

@test "3. List task definitions (JSON output)" {
    run bash -c "./conductor task list --json 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Should be valid JSON containing the task name
    echo "$output" | grep -q "\"name\""
    echo "$output" | grep -q "$TASK_NAME"
}

@test "4. Get task definition by name" {
    run bash -c "./conductor task get '$TASK_NAME' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "$TASK_NAME"
    echo "$output" | grep -q "\"retryCount\""
    echo "$output" | grep -q "\"timeoutSeconds\""
}

@test "5. Update task definition" {
    # Create updated task definition with new description
    cat > /tmp/e2e_task_update.json <<EOF
{
  "name": "cli_e2e_test_task",
  "description": "Updated E2E test task",
  "retryCount": 5,
  "timeoutSeconds": 600,
  "responseTimeoutSeconds": 180,
  "timeoutPolicy": "TIME_OUT_WF",
  "ownerEmail": "test@example.com"
}
EOF

    run bash -c "./conductor task update /tmp/e2e_task_update.json 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify update was applied
    run bash -c "./conductor task get '$TASK_NAME' 2>/dev/null"
    echo "Output: $output"
    echo "$output" | grep -q "Updated E2E test task"

    rm -f /tmp/e2e_task_update.json
}

# ==========================================
# Task Signal (Async) Tests
# ==========================================

@test "6. Create workflow for signal tests" {
    run bash -c "./conductor workflow create '$WORKFLOW_FILE' --force 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "7. Start workflow for async signal test" {
    run bash -c "./conductor workflow start --workflow '$WORKFLOW_NAME' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    WORKFLOW_ID=$(get_workflow_id "$output")
    [ -n "$WORKFLOW_ID" ]
    echo "$WORKFLOW_ID" > /tmp/task_signal_workflow_id.txt
    echo "Started workflow UUID: $WORKFLOW_ID"
}

@test "8. Verify workflow is RUNNING before signal" {
    WORKFLOW_ID=$(cat /tmp/task_signal_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./conductor workflow status '$WORKFLOW_ID' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == "RUNNING" ]]
}

@test "9. Signal WAIT task asynchronously" {
    WORKFLOW_ID=$(cat /tmp/task_signal_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./conductor task signal --workflow-id '$WORKFLOW_ID' --status COMPLETED 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "Task signal sent asynchronously"
}

@test "10. Verify workflow completes after async signal" {
    WORKFLOW_ID=$(cat /tmp/task_signal_workflow_id.txt)
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

@test "11. Cleanup async signal workflow execution" {
    WORKFLOW_ID=$(cat /tmp/task_signal_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./conductor workflow delete-execution '$WORKFLOW_ID' -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    rm -f /tmp/task_signal_workflow_id.txt
}

# ==========================================
# Task Signal-Sync Tests
# ==========================================

@test "12. Start workflow for sync signal test" {
    run bash -c "./conductor workflow start --workflow '$WORKFLOW_NAME' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    WORKFLOW_ID=$(get_workflow_id "$output")
    [ -n "$WORKFLOW_ID" ]
    echo "$WORKFLOW_ID" > /tmp/task_signal_sync_workflow_id.txt
    echo "Started workflow UUID: $WORKFLOW_ID"
}

@test "13. Signal WAIT task synchronously" {
    WORKFLOW_ID=$(cat /tmp/task_signal_sync_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./conductor task signal-sync --workflow-id '$WORKFLOW_ID' --status COMPLETED 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # signal-sync returns JSON response
    [[ "$output" == *"\"status\""* ]]
}

@test "14. Verify workflow completes after sync signal" {
    WORKFLOW_ID=$(cat /tmp/task_signal_sync_workflow_id.txt)
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

@test "15. Cleanup sync signal workflow execution" {
    WORKFLOW_ID=$(cat /tmp/task_signal_sync_workflow_id.txt)
    [ -n "$WORKFLOW_ID" ]

    run bash -c "./conductor workflow delete-execution '$WORKFLOW_ID' -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    rm -f /tmp/task_signal_sync_workflow_id.txt
}

# ==========================================
# Task Poll Tests
# ==========================================

@test "16. Poll for non-existent task type returns no tasks" {
    run bash -c "./conductor task poll 'nonexistent_task_type_xyz' --timeout 500 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"No tasks available"* ]]
}

# ==========================================
# Signal Validation Tests
# ==========================================

@test "17. Signal without required flags fails" {
    run bash -c "./conductor task signal 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"--workflow-id"* ]] || [[ "$output" == *"required"* ]]
}

@test "18. Signal-sync with invalid status fails" {
    run bash -c "./conductor task signal-sync --workflow-id 'fake-id' --status INVALID_STATUS 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

# ==========================================
# Cleanup
# ==========================================

@test "19. Delete task definition" {
    run bash -c "./conductor task delete '$TASK_NAME' -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"deleted successfully"* ]]
}

@test "20. Cleanup workflow definition" {
    run bash -c "./conductor workflow delete '$WORKFLOW_NAME' 1 -y 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    rm -f /tmp/task_signal_workflow_id.txt /tmp/task_signal_sync_workflow_id.txt
}
