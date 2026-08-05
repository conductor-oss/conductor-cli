#!/usr/bin/env bats

# E2E tests for the doctor command
# Tests runtime detection, server reporting, and AI provider detection.
#
# doctor is read-only and venue-agnostic: it reports configuration rather than
# calling server APIs, so it runs against OSS and Enterprise alike.

# bats file_tags=tier:pr

setup_file() {
    # Ensure the CLI binary exists
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi
}

@test "1. Doctor runs and reports the three sections" {
    run bash -c "./conductor doctor 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Runtime"* ]]
    [[ "$output" == *"Conductor server"* ]]
    [[ "$output" == *"AI Providers"* ]]
}

@test "2. Doctor reports the configured server URL" {
    run bash -c "./conductor doctor 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Server:"* ]]
}

@test "3. Doctor reports Java presence or absence explicitly" {
    run bash -c "./conductor doctor 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Either "ok  Java <version>" or the "--  Java not found" advisory.
    [[ "$output" == *"Java"* ]]
}

@test "4. Doctor summarises the AI provider count" {
    run bash -c "./conductor doctor 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"AI provider(s) configured"* ]]
}

@test "5. Doctor honours an explicit --server flag" {
    run bash -c "./conductor --server http://example.invalid:9999/api doctor 2>&1"
    echo "Output: $output"
    # doctor reports configuration; it must not fail merely because the URL is unreachable.
    [ "$status" -eq 0 ]
    [[ "$output" == *"example.invalid:9999"* ]]
}

@test "6. Doctor lists a known provider env var name" {
    run bash -c "./conductor doctor 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Provider rows name their env vars whether configured or not.
    [[ "$output" == *"ANTHROPIC_API_KEY"* ]]
    [[ "$output" == *"OPENAI_API_KEY"* ]]
}

# Regression guard for #103: doctor advertises specific model strings that have
# drifted out of date (two Anthropic models return 404 from the provider API).
# Asserts the desired end state — that doctor does not print known-dead models.
@test "7. Doctor does not advertise retired model identifiers" {
    skip "known broken: #103 — doctor hardcodes stale model strings"
    run bash -c "./conductor doctor 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" != *"claude-sonnet-4-20250514"* ]]
    [[ "$output" != *"claude-3-5-sonnet-20241022"* ]]
}
