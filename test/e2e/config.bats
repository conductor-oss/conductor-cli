#!/usr/bin/env bats

# E2E tests for config profile management
# Tests save / list / delete and profile selection precedence.
#
# config operates purely on ~/.conductor-cli/ and needs no server, so it is
# venue-agnostic. Tests use a dedicated profile name and clean up after
# themselves so they never disturb a developer's real profiles.

# bats file_tags=tier:pr

PROFILE="e2e_cfg_probe"
PROFILE_2="e2e_cfg_probe_2"

setup_file() {
    # Ensure the CLI binary exists
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi

    # Remove leftovers from previous runs
    ./conductor config delete "$PROFILE" -y 2>/dev/null || true
    ./conductor config delete "$PROFILE_2" -y 2>/dev/null || true
}

teardown_file() {
    ./conductor config delete "$PROFILE" -y 2>/dev/null || true
    ./conductor config delete "$PROFILE_2" -y 2>/dev/null || true
}

# Helper: create a profile non-interactively by accepting every prompt default.
save_profile() {
    local name="$1"
    printf '\n\n\n\n\n' | ./conductor config save --profile "$name" >/dev/null 2>&1
}

@test "1. Save a named profile" {
    run bash -c "printf '\n\n\n\n\n' | ./conductor config save --profile $PROFILE 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"config-$PROFILE.yaml"* ]]
}

@test "2. List shows the saved profile" {
    save_profile "$PROFILE"

    run bash -c "./conductor config list 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"$PROFILE"* ]]
}

@test "3. Saved profile is usable via --profile" {
    save_profile "$PROFILE"

    run bash -c "./conductor --profile $PROFILE config list 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "4. Selecting a nonexistent profile fails with a helpful message" {
    run bash -c "./conductor --profile e2e_definitely_absent workflow list 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"e2e_definitely_absent"* ]]
}

@test "5. Multiple profiles coexist" {
    save_profile "$PROFILE"
    save_profile "$PROFILE_2"

    run bash -c "./conductor config list 2>/dev/null"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"$PROFILE"* ]]
    [[ "$output" == *"$PROFILE_2"* ]]
}

@test "6. Delete a profile with -y" {
    save_profile "$PROFILE_2"

    run bash -c "./conductor config delete $PROFILE_2 -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    run bash -c "./conductor config list 2>/dev/null"
    echo "After delete: $output"
    [[ "$output" != *"$PROFILE_2"* ]]
}

@test "7. Delete accepts the profile via --profile flag" {
    save_profile "$PROFILE_2"

    run bash -c "./conductor config delete --profile $PROFILE_2 -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
}

@test "8. Save without a profile name is rejected" {
    # config save always targets config-<name>.yaml; with no --profile it prompts,
    # and empty input is an error rather than a default-profile write.
    run bash -c "printf '\n' | ./conductor config save 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"profile name is required"* ]]
}

# Regression guard for #98: the default profile (~/.conductor-cli/config.yaml) is
# still read but cannot be created or updated through the CLI. Asserts the
# desired end state.
@test "9. Default profile can be managed through the CLI" {
    skip "known broken: #98 — config save cannot write the default config.yaml"
    run bash -c "printf '\n\n\n\n\n' | ./conductor config save 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"config.yaml"* ]]
}
