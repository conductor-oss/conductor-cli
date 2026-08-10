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

# The default config lives at a fixed path inside HOME, so these tests use a
# throwaway one. Without it they would overwrite the developer's real
# ~/.conductor-cli/config.yaml.
isolated_home() {
    local home
    home="$(mktemp -d)"
    mkdir -p "$home/.conductor-cli"
    echo "$home"
}

# Any config variable switches the CLI onto the environment. CI exports
# CONDUCTOR_SERVER_URL and CONDUCTOR_SERVER_TYPE for the whole run, so tests that
# want the file to win must clear all of them, not just the one they care about.
NO_CONFIG_ENV="env -u CONDUCTOR_SERVER_URL -u CONDUCTOR_SERVER_TYPE -u CONDUCTOR_AUTH_KEY -u CONDUCTOR_AUTH_SECRET -u CONDUCTOR_AUTH_TOKEN"

@test "8. Save with an empty profile name writes the default config" {
    local home
    home="$(isolated_home)"
    run bash -c "printf '\n\n\n\n\n' | HOME='$home' ./conductor config save 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"config.yaml"* ]]
    [ -f "$home/.conductor-cli/config.yaml" ]
    rm -rf "$home"
}

# Regression guard for #98: config.yaml was read but not writable.
@test "9. Default profile can be managed through the CLI" {
    local home
    home="$(isolated_home)"

    run bash -c "printf '\n\n\n\n\n' | HOME='$home' ./conductor config save 2>&1"
    [ "$status" -eq 0 ]

    run bash -c "HOME='$home' ./conductor config list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"default"* ]]

    run bash -c "HOME='$home' ./conductor config delete default -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [ ! -f "$home/.conductor-cli/config.yaml" ]
    rm -rf "$home"
}

@test "10. --profile default writes the same file as an empty name" {
    local home
    home="$(isolated_home)"

    run bash -c "printf '\n\n\n\n\n' | HOME='$home' ./conductor config save --profile default 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"config.yaml"* ]]

    # The alias must not create a second, unreachable file.
    [ -f "$home/.conductor-cli/config.yaml" ]
    [ ! -f "$home/.conductor-cli/config-default.yaml" ]
    rm -rf "$home"
}

@test "11. A stray config-default.yaml is reported, not listed" {
    local home
    home="$(isolated_home)"
    printf 'server: http://stray:8080/api\n' > "$home/.conductor-cli/config-default.yaml"
    printf 'server: http://real:8080/api\n' > "$home/.conductor-cli/config.yaml"

    run bash -c "HOME='$home' ./conductor config list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"config-default.yaml is not used"* ]]
    # "default" must appear once, for config.yaml only.
    [ "$(echo "$output" | grep -cx 'default')" -eq 1 ]
    rm -rf "$home"
}

@test "12. config show reports the source of each value" {
    local home
    home="$(isolated_home)"
    printf 'server: http://from-file:8080/api\nserver-type: OSS\n' > "$home/.conductor-cli/config.yaml"

    # File supplies the value when the environment does not.
    run bash -c "$NO_CONFIG_ENV HOME='$home' ./conductor config show 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"from-file"* ]]
    [[ "$output" == *"config.yaml"* ]]

    run bash -c "HOME='$home' CONDUCTOR_SERVER_URL=http://from-env:9999/api ./conductor config show 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"env CONDUCTOR_SERVER_URL"* ]]
    [[ "$output" == *"from-env"* ]]
    [[ "$output" == *"config file not read"* ]]
    rm -rf "$home"
}

@test "14. Setting an env var takes the config file out of play entirely" {
    local home
    home="$(isolated_home)"
    # The file carries a token the environment does not; it must not be used.
    printf 'server: http://from-file:8080/api\nauth-token: tok-from-file\nserver-type: OSS\n' \
        > "$home/.conductor-cli/config.yaml"

    # Clear the auth variables: the Enterprise runner exports real ones, and a
    # "****" sourced from the environment says nothing about the file.
    run bash -c "env -u CONDUCTOR_AUTH_KEY -u CONDUCTOR_AUTH_SECRET -u CONDUCTOR_AUTH_TOKEN \
        HOME='$home' CONDUCTOR_SERVER_URL=http://from-env:9999/api ./conductor config show 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"from-env"* ]]
    [[ "$output" != *"from-file"* ]]
    [[ "$output" == *"config file not read"* ]]

    # The file's auth-token must not be picked up.
    token_row="$(echo "$output" | grep '^auth-token')"
    echo "auth-token row: $token_row"
    [[ "$token_row" != *"config.yaml"* ]]
    [[ "$token_row" != *"****"* ]]
    rm -rf "$home"
}

@test "15. Naming a file beats the environment" {
    local home
    home="$(isolated_home)"
    printf 'server: http://from-profile:7777/api\nserver-type: OSS\n' > "$home/.conductor-cli/config-e2eprof.yaml"
    printf 'server: http://from-explicit:6666/api\nserver-type: OSS\n' > "$home/explicit.yaml"

    # --profile names one of the CLI's files.
    run bash -c "HOME='$home' CONDUCTOR_SERVER_URL=http://from-env:9999/api ./conductor --profile e2eprof config show 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"from-profile"* ]]
    [[ "$output" != *"from-env"* ]]

    # --config names an arbitrary path.
    run bash -c "HOME='$home' CONDUCTOR_SERVER_URL=http://from-env:9999/api ./conductor --config '$home/explicit.yaml' config show 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"from-explicit"* ]]
    [[ "$output" != *"from-env"* ]]
    rm -rf "$home"
}

@test "16. A flag overrides one key without discarding the rest" {
    local home
    home="$(isolated_home)"
    printf 'server: http://from-file:8080/api\nauth-token: tok-from-file\nserver-type: OSS\n' \
        > "$home/.conductor-cli/config.yaml"

    run bash -c "$NO_CONFIG_ENV HOME='$home' ./conductor --server http://from-flag:1111/api config show 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"from-flag"* ]]
    [[ "$output" == *"flag --server"* ]]
    # Token still from the file: flags are per-key, not a source switch.
    [[ "$output" == *"****"* ]]
    rm -rf "$home"
}

@test "13. config show masks secrets unless asked" {
    local home
    home="$(isolated_home)"
    printf 'server: http://x:8080/api\nauth-token: supersecrettoken\n' > "$home/.conductor-cli/config.yaml"

    run bash -c "$NO_CONFIG_ENV HOME='$home' ./conductor config show 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" != *"supersecrettoken"* ]]
    [[ "$output" == *"****"* ]]

    run bash -c "$NO_CONFIG_ENV HOME='$home' ./conductor config show --show-secrets 2>&1"
    [ "$status" -eq 0 ]
    [[ "$output" == *"supersecrettoken"* ]]
    rm -rf "$home"
}
