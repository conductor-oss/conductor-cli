#!/usr/bin/env bats

# E2E tests for whoami command

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi
}

@test "whoami prints output" {
    run bash -c "./orkes whoami 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Count lines - should have at least server URL
    line_count=$(echo "$output" | wc -l | tr -d ' ')
    [ "$line_count" -ge 1 ]
}