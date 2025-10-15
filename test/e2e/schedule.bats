#!/usr/bin/env bats

# E2E tests for schedule commands
# Tests schedule create, list, get, and delete functionality

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi

    # Clean up any existing test schedules
    ./orkes schedule delete e2e_test_schedule -y 2>/dev/null || true
    ./orkes schedule delete e2e_test_schedule_2 -y 2>/dev/null || true
    ./orkes schedule delete e2e_test_paused -y 2>/dev/null || true
}

teardown() {
    # Clean up test schedules after each test
    ./orkes schedule delete e2e_test_schedule -y 2>/dev/null || true
    ./orkes schedule delete e2e_test_schedule_2 -y 2>/dev/null || true
    ./orkes schedule delete e2e_test_paused -y 2>/dev/null || true
}

@test "1. Create schedule with flags" {
    run bash -c "./orkes schedule create -n e2e_test_schedule -c '0 0 * ? * *' -w hello_world 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "2. List schedules shows created schedule" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # List and verify it appears
    run bash -c "./orkes schedule list 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"e2e_test_schedule"* ]]
}

@test "3. List with cron flag shows schedule name and cron expression" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # List with cron flag
    run bash -c "./orkes schedule list --cron 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"e2e_test_schedule"* ]]
    [[ "$output" == *"0 0 * ? * *"* ]]
}

@test "4. Get schedule returns full details" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # Get schedule details
    run bash -c "./orkes schedule get e2e_test_schedule 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify JSON contains expected fields
    [[ "$output" == *'"name": "e2e_test_schedule"'* ]]
    [[ "$output" == *'"cronExpression": "0 0 * ? * *"'* ]]
    [[ "$output" == *'"name": "hello_world"'* ]]
}

@test "5. Create schedule with input JSON" {
    run bash -c "./orkes schedule create -n e2e_test_schedule_2 -c '0 0 * ? * *' -w hello_world -i '{\"key\":\"value\"}' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify input was set
    run bash -c "./orkes schedule get e2e_test_schedule_2 2>/dev/null"
    [[ "$output" == *'"key"'* ]]
    [[ "$output" == *'"value"'* ]]
}

@test "6. Create paused schedule" {
    run bash -c "./orkes schedule create -n e2e_test_paused -c '0 0 * ? * *' -w hello_world -p 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify schedule is paused
    run bash -c "./orkes schedule get e2e_test_paused 2>/dev/null"
    [[ "$output" == *'"paused": true'* ]]
}

@test "7. Delete schedule removes it" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # Verify it exists
    run bash -c "./orkes schedule get e2e_test_schedule 2>/dev/null"
    [ "$status" -eq 0 ]

    # Delete it with -y flag
    run bash -c "./orkes schedule delete e2e_test_schedule -y 2>&1"
    echo "Delete output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"deleted successfully"* ]]

    # Verify it's gone
    run bash -c "./orkes schedule get e2e_test_schedule 2>&1"
    [ "$status" -ne 0 ]
}

@test "8. Create schedule without required flags shows help" {
    # Missing all flags - should show usage/help
    run bash -c "./orkes schedule create </dev/null 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage:"* ]]
}

@test "9. Create schedule with missing name flag shows error" {
    run bash -c "./orkes schedule create -c '0 0 * ? * *' -w hello_world 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"--name is required"* ]]
}

@test "10. Create schedule with missing cron flag shows error" {
    run bash -c "./orkes schedule create -n e2e_test -w hello_world 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"--cron is required"* ]]
}

@test "11. Create schedule with missing workflow flag shows error" {
    run bash -c "./orkes schedule create -n e2e_test -c '0 0 * ? * *' 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"--workflow is required"* ]]
}

@test "12. Create duplicate schedule shows error" {
    # Create first schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # Try to create duplicate
    run bash -c "./orkes schedule create -n e2e_test_schedule -c '0 0 * ? * *' -w hello_world 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"already exists"* ]]
}

@test "13. Update existing schedule" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # Update with new cron expression
    run bash -c "./orkes schedule update -n e2e_test_schedule -c '0 0 12 ? * *' -w hello_world 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify cron was updated
    run bash -c "./orkes schedule get e2e_test_schedule 2>/dev/null"
    [[ "$output" == *'"cronExpression": "0 0 12 ? * *"'* ]]
}

@test "14. Update non-existent schedule shows error" {
    run bash -c "./orkes schedule update -n nonexistent -c '0 0 * ? * *' -w hello_world 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"no such schedule"* ]]
}

@test "15. Pause schedule" {
    # Create active schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # Pause it
    run bash -c "./orkes schedule pause e2e_test_schedule 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify it's paused
    run bash -c "./orkes schedule get e2e_test_schedule 2>/dev/null"
    [[ "$output" == *'"paused": true'* ]]
}

@test "16. Resume schedule" {
    # Create paused schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world -p 2>/dev/null

    # Resume it
    run bash -c "./orkes schedule resume e2e_test_schedule 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify it's active (paused should not be true, or absent entirely)
    run bash -c "./orkes schedule get e2e_test_schedule 2>/dev/null"
    [[ "$output" != *'"paused": true'* ]]
}

@test "17. Create schedule with workflow version" {
    run bash -c "./orkes schedule create -n e2e_test_schedule -c '0 0 * ? * *' -w hello_world --version 1 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify version was set
    run bash -c "./orkes schedule get e2e_test_schedule 2>/dev/null"
    [[ "$output" == *'"version": 1'* ]]
}

@test "18. Create schedule with invalid JSON input shows error" {
    run bash -c "./orkes schedule create -n e2e_test_schedule -c '0 0 * ? * *' -w hello_world -i 'not-json' 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"JSON"* ]]
}

@test "19. List schedules with JSON flag shows full details" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # List with JSON flag
    run bash -c "./orkes schedule list --json 2>/dev/null | grep e2e_test_schedule"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"name"'* ]]
    [[ "$output" == *'"cronExpression"'* ]]
}

@test "20. Get multiple schedules at once" {
    # Create two schedules
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null
    ./orkes schedule create -n e2e_test_schedule_2 -c "0 0 12 ? * *" -w hello_world 2>/dev/null

    # Get both at once
    run bash -c "./orkes schedule get e2e_test_schedule e2e_test_schedule_2 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"e2e_test_schedule"* ]]
    [[ "$output" == *"e2e_test_schedule_2"* ]]
}

@test "21. Delete without -y flag prompts for confirmation" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # Try to delete without -y flag (should prompt and timeout in test)
    run bash -c "echo 'n' | timeout 2 ./orkes schedule delete e2e_test_schedule 2>&1"
    echo "Output: $output"
    # Should contain confirmation prompt
    [[ "$output" == *"Are you sure"* ]]
}

@test "22. Delete with -y flag skips confirmation" {
    # Create schedule
    ./orkes schedule create -n e2e_test_schedule -c "0 0 * ? * *" -w hello_world 2>/dev/null

    # Delete with -y flag (no prompt)
    run bash -c "./orkes schedule delete e2e_test_schedule -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Should NOT contain confirmation prompt
    [[ "$output" != *"Are you sure"* ]]
    [[ "$output" == *"deleted successfully"* ]]
}
