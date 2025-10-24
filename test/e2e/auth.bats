#!/usr/bin/env bats

# E2E tests for authentication error handling
# These tests verify that the CLI provides helpful error messages when authentication fails
# This test assumes no credentials are set.

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi
}

@test "1. workflow start without credentials shows helpful error" {
    # Try to start a workflow without credentials
    run bash -c "./orkes workflow start --workflow test_workflow 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"orkes config save"* ]]
}

@test "2. workflow start sync without credentials shows helpful error" {
    # Try to start a workflow without credentials
    run bash -c "./orkes workflow start --workflow test_workflow --sync --version 1 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"orkes config save"* ]]
}

@test "3. workflow list without credentials shows helpful error" {
    # Try to list workflows without credentials
    run bash -c "./orkes workflow list 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"orkes config save"* ]]
}

@test "4. task list without credentials shows helpful error" {
    # Try to list tasks without credentials
    run bash -c "./orkes task list 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"orkes config save"* ]]
}
