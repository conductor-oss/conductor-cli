#!/usr/bin/env bats

# E2E tests for authentication error handling
# These tests verify that the CLI provides helpful error messages when authentication fails
# This test assumes no credentials are set.

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi
}

@test "1. workflow start without credentials shows helpful error" {
    # Try to start a workflow without credentials
    run bash -c "./conductor workflow start --workflow test_workflow 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"conductor config save"* ]]
}

@test "2. workflow start sync without credentials shows helpful error" {
    # Try to start a workflow without credentials
    run bash -c "./conductor workflow start --workflow test_workflow --sync --version 1 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"conductor config save"* ]]
}

@test "3. workflow list without credentials shows helpful error" {
    # Try to list workflows without credentials
    run bash -c "./conductor workflow list 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"conductor config save"* ]]
}

@test "4. task list without credentials shows helpful error" {
    # Try to list tasks without credentials
    run bash -c "./conductor task list 2>&1"
    echo "Output: $output"
    echo "Status: $status"

    # Command should fail
    [ "$status" -ne 0 ]

    # Should contain the helpful error message (check for authentication error guidance)
    [[ "$output" == *"Please check your authentication settings"* ]]
    [[ "$output" == *"conductor config save"* ]]
}

@test "5. config commands work without valid credentials (local-only operations)" {
    # Test that config commands don't require API access or valid tokens
    # These are local file operations and should always work

    # config list should work (lists local config files)
    run ./conductor config list
    echo "config list output: $output"
    echo "config list status: $status"
    [ "$status" -eq 0 ]

    # config save should work (though we'll skip interactive part)
    # Just verify the command itself doesn't fail on token validation
    run bash -c "echo '' | timeout 2 ./conductor config save 2>&1 || true"
    echo "config save output: $output"
    # Should not contain token expiration error
    [[ "$output" != *"your token has expired"* ]]
}
