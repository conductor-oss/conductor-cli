#!/usr/bin/env bats

# E2E tests for local Conductor server management
#
# Scope note: this suite deliberately does NOT exercise the start/stop lifecycle.
# The CLI tracks one server instance in ~/.conductor-cli/server/server-state.json
# (single pid/port), so starting or stopping a server here would clobber the
# instance the rest of the E2E run depends on. Instead it asserts the read-only
# commands plus the mutual-exclusion guards, which are non-destructive and are the
# behaviours most likely to regress.
#
# Full lifecycle coverage needs an exclusive server and a way to isolate the
# datasource (see #104 — `server start` writes c123.db into the current working
# directory with no flag to override).
#
# venue:oss only — these commands manage a local OSS jar and are meaningless
# against a remote Enterprise server.

# bats file_tags=tier:pr,oss-only,needs:server

setup_file() {
    # Ensure the CLI binary exists
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi
}

# Helper: skip unless a CLI-managed local server is running. These tests assert
# behaviour relative to a running instance; without one they are meaningless.
require_running_server() {
    if ! ./conductor server status 2>/dev/null | grep -q 'is running'; then
        skip "no CLI-managed local server is running"
    fi
}

@test "1. Server status reports a running server with its PID" {
    require_running_server

    run bash -c "./conductor server status 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"is running"* ]]
    [[ "$output" == *"PID"* ]]
}

@test "2. Server status reports health and endpoints" {
    require_running_server

    run bash -c "./conductor server status 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"healthy"* ]]
    [[ "$output" == *"API:"* ]]
}

@test "3. Server logs returns output" {
    require_running_server

    run bash -c "./conductor server logs 2>&1"
    echo "Output length: ${#output}"
    [ "$status" -eq 0 ]
    [ "${#output}" -gt 0 ]
}

@test "4. Server logs -n bounds the number of lines" {
    require_running_server

    run bash -c "./conductor server logs -n 5 2>/dev/null | wc -l | tr -d ' '"
    echo "Line count: $output"
    [ "$status" -eq 0 ]
    [ "$output" -le 5 ]
}

@test "5. Server start refuses while an instance is already running" {
    require_running_server

    # Non-destructive: the guard returns before touching the running instance.
    run bash -c "./conductor server start 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"already running"* ]]
}

@test "6. Server update refuses while an instance is already running" {
    require_running_server

    # Non-destructive, and importantly avoids triggering a ~435 MB download.
    run bash -c "./conductor server update 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"currently running"* ]]
}

@test "7. Server help lists the lifecycle subcommands" {
    run bash -c "./conductor server --help 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"start"* ]]
    [[ "$output" == *"stop"* ]]
    [[ "$output" == *"status"* ]]
    [[ "$output" == *"logs"* ]]
    [[ "$output" == *"update"* ]]
}

@test "8. Server start --help documents the version and port flags" {
    run bash -c "./conductor server start --help 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"--port"* ]]
    [[ "$output" == *"--version"* ]]
    [[ "$output" == *"--foreground"* ]]
}
