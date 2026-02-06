#!/usr/bin/env bats

# E2E tests for webhook functionality

WEBHOOK_FILE="test/e2e/webhook.json"
WEBHOOK_NAME="custom_webhook_1"
WEBHOOK_ID=""

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./conductor" ]; then
        echo "ERROR: conductor binary not found. Please build it first."
        exit 1
    fi
}

# Helper function to extract webhook ID from JSON output
get_webhook_id() {
    echo "$1" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -o '"[a-zA-Z0-9_-]*"$' | tr -d '"'
}

@test "1. Create webhook using command flags" {
    run bash -c "./conductor webhook create \
        --name custom_webhook_1 \
        --source-platform Custom \
        --verifier HEADER_BASED \
        --headers 'Authorization:BB12346789' \
        --receiver-workflows hello_world:1 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Extract and save webhook ID
    WEBHOOK_ID=$(get_webhook_id "$output")
    [ -n "$WEBHOOK_ID" ]
    echo "$WEBHOOK_ID" > /tmp/webhook_test_id.txt
    echo "Created webhook ID: $WEBHOOK_ID"

    # Verify the webhook name is in the output
    echo "$output" | grep -q "$WEBHOOK_NAME"
}

@test "2. Get webhook by ID" {
    WEBHOOK_ID=$(cat /tmp/webhook_test_id.txt)
    [ -n "$WEBHOOK_ID" ]

    run bash -c "./conductor webhook get '$WEBHOOK_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify webhook details
    echo "$output" | grep -q "$WEBHOOK_NAME"
    echo "$output" | grep -q "BB12346789"
    echo "$output" | grep -q "hello_world"
}

@test "3. List webhooks (should include created webhook)" {
    WEBHOOK_ID=$(cat /tmp/webhook_test_id.txt)
    [ -n "$WEBHOOK_ID" ]

    run bash -c "./conductor webhook list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify the webhook is in the list
    echo "$output" | grep -q "$WEBHOOK_NAME"
    echo "$output" | grep -q "$WEBHOOK_ID"
}

@test "4. List webhooks with JSON output" {
    WEBHOOK_ID=$(cat /tmp/webhook_test_id.txt)
    [ -n "$WEBHOOK_ID" ]

    run bash -c "./conductor webhook list --json 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify JSON format and webhook presence
    echo "$output" | grep -q "$WEBHOOK_NAME"
    echo "$output" | grep -q "$WEBHOOK_ID"
}

@test "5. Update webhook - change Authorization header" {
    WEBHOOK_ID=$(cat /tmp/webhook_test_id.txt)
    [ -n "$WEBHOOK_ID" ]

    # Create updated webhook JSON with new Authorization value
    cat > /tmp/webhook_update.json <<EOF
{
  "sourcePlatform": "Custom",
  "headers": {
    "Authorization": "UPDATED_AUTH_TOKEN_123"
  },
  "receiverWorkflowNamesToVersions": {
    "hello_world": 1
  },
  "verifier": "HEADER_BASED",
  "name": "custom_webhook_1"
}
EOF

    run bash -c "./conductor webhook update '$WEBHOOK_ID' --file /tmp/webhook_update.json 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify updated Authorization in output
    echo "$output" | grep -q "UPDATED_AUTH_TOKEN_123"
}

@test "6. Verify webhook was updated" {
    WEBHOOK_ID=$(cat /tmp/webhook_test_id.txt)
    [ -n "$WEBHOOK_ID" ]

    run bash -c "./conductor webhook get '$WEBHOOK_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify new Authorization value
    echo "$output" | grep -q "UPDATED_AUTH_TOKEN_123"
    # Old value should not be present
    ! echo "$output" | grep -q "BB12346789"
}

@test "7. Delete webhook" {
    WEBHOOK_ID=$(cat /tmp/webhook_test_id.txt)
    [ -n "$WEBHOOK_ID" ]

    run bash -c "./conductor webhook delete '$WEBHOOK_ID' -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    echo "$output" | grep -q "deleted successfully"
    echo "Deleted webhook ID: $WEBHOOK_ID"
}

@test "8. Verify webhook no longer exists" {
    WEBHOOK_ID=$(cat /tmp/webhook_test_id.txt)
    [ -n "$WEBHOOK_ID" ]

    # Getting deleted webhook should fail
    run bash -c "./conductor webhook get '$WEBHOOK_ID' 2>/dev/null"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

@test "9. Cleanup temp files" {
    rm -f /tmp/webhook_test_id.txt /tmp/webhook_update.json
}
