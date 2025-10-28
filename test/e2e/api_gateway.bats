#!/usr/bin/env bats

# E2E tests for API Gateway functionality
# Tests service, auth config, and route management

SERVICE_ID="e2e_test_service"
AUTH_CONFIG_ID="e2e_test_auth"
WORKFLOW_NAME="cli_e2e_test_workflow_2"

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi

    # Ensure test workflow exists
    if [ ! -f "test/e2e/test-workflow-2.json" ]; then
        echo "ERROR: test-workflow-2.json not found"
        exit 1
    fi

    # Create the test workflow if it doesn't exist
    ./orkes workflow create test/e2e/test-workflow-2.json --force 2>/dev/null || true
}

teardown() {
    # Clean up in reverse order: routes -> services -> auth configs
    # Routes are deleted when service is deleted, so just clean up services and auth configs
    ./orkes api-gateway service delete "$SERVICE_ID" -y 2>/dev/null || true
    ./orkes api-gateway auth delete "$AUTH_CONFIG_ID" -y 2>/dev/null || true
}

# ==================== Auth Config Tests ====================

@test "1. Create auth config using flags" {
    run bash -c "./orkes api-gateway auth create \
        --auth-config-id '$AUTH_CONFIG_ID' \
        --auth-type 'API_KEY' \
        --application-id 'e2e-test-app' \
        --api-keys 'test-key-1,test-key-2' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "created successfully"
}

@test "2. Get auth config by ID" {
    # Ensure auth config exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" \
        --application-id "e2e-test-app" \
        --api-keys "test-key-1,test-key-2" 2>/dev/null || true

    run bash -c "./orkes api-gateway auth get '$AUTH_CONFIG_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify auth config details
    echo "$output" | grep -q "$AUTH_CONFIG_ID"
    echo "$output" | grep -q "API_KEY"
    echo "$output" | grep -q "e2e-test-app"
}

@test "3. List auth configs includes created config" {
    # Ensure auth config exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" \
        --application-id "e2e-test-app" \
        --api-keys "test-key-1" 2>/dev/null || true

    run bash -c "./orkes api-gateway auth list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify the auth config is in the list
    echo "$output" | grep -q "$AUTH_CONFIG_ID"
}

# ==================== Service Tests ====================

@test "4. Create service using flags" {
    # Ensure auth config exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" \
        --application-id "e2e-test-app" 2>/dev/null || true

    run bash -c "./orkes api-gateway service create \
        --service-id '$SERVICE_ID' \
        --name 'E2E Test Service' \
        --path '/api/e2e' \
        --description 'Service for E2E testing' \
        --enabled \
        --auth-config-id '$AUTH_CONFIG_ID' \
        --cors-allowed-origins '*' \
        --cors-allowed-methods 'GET,POST,PUT,DELETE' \
        --cors-allowed-headers 'Content-Type,Authorization' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "created successfully"
}

@test "5. Get service by ID" {
    # Ensure auth config and service exist
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" \
        --application-id "e2e-test-app" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled \
        --auth-config-id "$AUTH_CONFIG_ID" 2>/dev/null || true

    run bash -c "./orkes api-gateway service get '$SERVICE_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify service details
    echo "$output" | grep -q "$SERVICE_ID"
    echo "$output" | grep -q "E2E Test Service"
    echo "$output" | grep -q "/api/e2e"
    echo "$output" | grep -q "$AUTH_CONFIG_ID"
}

@test "6. List services includes created service" {
    # Ensure service exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true

    run bash -c "./orkes api-gateway service list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify the service is in the list
    echo "$output" | grep -q "$SERVICE_ID"
}

@test "7. Update service using JSON file" {
    # Ensure service exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true

    # Create update JSON
    cat > /tmp/service_update.json <<EOF
{
  "id": "$SERVICE_ID",
  "name": "Updated E2E Test Service",
  "path": "/api/e2e",
  "description": "Updated description for testing",
  "enabled": true,
  "authConfigId": "$AUTH_CONFIG_ID",
  "corsConfig": {
    "accessControlAllowOrigin": ["https://example.com"],
    "accessControlAllowMethods": ["GET", "POST"],
    "accessControlAllowHeaders": ["*"]
  }
}
EOF

    run bash -c "./orkes api-gateway service update '$SERVICE_ID' /tmp/service_update.json 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "updated successfully"
}

@test "8. Verify service was updated" {
    # Ensure service is updated
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true

    cat > /tmp/service_update.json <<EOF
{
  "id": "$SERVICE_ID",
  "name": "Updated E2E Test Service",
  "path": "/api/e2e",
  "description": "Updated description for testing",
  "enabled": true
}
EOF
    ./orkes api-gateway service update "$SERVICE_ID" /tmp/service_update.json 2>/dev/null

    run bash -c "./orkes api-gateway service get '$SERVICE_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify updated fields
    echo "$output" | grep -q "Updated E2E Test Service"
    echo "$output" | grep -q "Updated description for testing"
}

# ==================== Route Tests ====================

@test "9. Create route using flags" {
    # Ensure service exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true

    run bash -c "./orkes api-gateway route create '$SERVICE_ID' \
        --http-method 'GET' \
        --path '/test' \
        --description 'E2E test route' \
        --workflow-name '$WORKFLOW_NAME' \
        --workflow-version 1 \
        --execution-mode 'SYNC' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "created successfully"
}

@test "10. List routes for service" {
    # Ensure service and route exist
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true
    ./orkes api-gateway route create "$SERVICE_ID" \
        --http-method "GET" \
        --path "/test" \
        --workflow-name "$WORKFLOW_NAME" \
        --workflow-version 1 \
        --execution-mode "SYNC" 2>/dev/null || true

    run bash -c "./orkes api-gateway route list '$SERVICE_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify route is in the list
    echo "$output" | grep -q "GET"
    echo "$output" | grep -q "/test"
    echo "$output" | grep -q "$WORKFLOW_NAME"
}

@test "11. Create route with advanced options using flags" {
    # Ensure service exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true

    run bash -c "./orkes api-gateway route create '$SERVICE_ID' \
        --http-method 'POST' \
        --path '/advanced' \
        --description 'Advanced route with metadata' \
        --workflow-name '$WORKFLOW_NAME' \
        --workflow-version 1 \
        --execution-mode 'ASYNC' \
        --request-metadata-as-input \
        --workflow-metadata-in-output 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "created successfully"
}

@test "12. Update route using JSON file" {
    # Ensure service and route exist
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true
    ./orkes api-gateway route create "$SERVICE_ID" \
        --http-method "PUT" \
        --path "/update-test" \
        --workflow-name "$WORKFLOW_NAME" \
        --workflow-version 1 \
        --execution-mode "SYNC" 2>/dev/null || true

    # Create route update JSON
    cat > /tmp/route_update.json <<EOF
{
  "httpMethod": "PUT",
  "path": "/update-test",
  "description": "Updated route description",
  "workflowExecutionMode": "SYNC",
  "mappedWorkflow": {
    "name": "$WORKFLOW_NAME",
    "version": 1
  }
}
EOF

    run bash -c "./orkes api-gateway route update '$SERVICE_ID' '/update-test' /tmp/route_update.json 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "updated successfully"
}

@test "13. Delete route" {
    # Ensure service and route exist
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true
    ./orkes api-gateway route create "$SERVICE_ID" \
        --http-method "DELETE" \
        --path "/delete-test" \
        --workflow-name "$WORKFLOW_NAME" \
        --workflow-version 1 \
        --execution-mode "SYNC" 2>/dev/null || true

    run bash -c "./orkes api-gateway route delete '$SERVICE_ID' 'DELETE' '/delete-test' -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "deleted successfully"
}

# ==================== Cleanup Tests ====================

@test "14. Delete service" {
    # Ensure service exists
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" 2>/dev/null || true
    ./orkes api-gateway service create \
        --service-id "$SERVICE_ID" \
        --name "E2E Test Service" \
        --path "/api/e2e" \
        --enabled 2>/dev/null || true

    run bash -c "./orkes api-gateway service delete '$SERVICE_ID' -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "deleted successfully"
}

@test "15. Verify service no longer exists" {
    # Ensure service was deleted
    ./orkes api-gateway service delete "$SERVICE_ID" -y 2>/dev/null || true

    # Getting deleted service should fail or return empty
    run bash -c "./orkes api-gateway service get '$SERVICE_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

@test "16. Delete auth config" {
    # Ensure auth config exists and service is deleted (to avoid FK constraint)
    ./orkes api-gateway service delete "$SERVICE_ID" -y 2>/dev/null || true
    ./orkes api-gateway auth create \
        --auth-config-id "$AUTH_CONFIG_ID" \
        --auth-type "API_KEY" \
        --application-id "e2e-test-app" 2>/dev/null || true

    run bash -c "./orkes api-gateway auth delete '$AUTH_CONFIG_ID' -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "deleted successfully"
}

@test "17. Verify auth config no longer exists" {
    # Ensure auth config was deleted
    ./orkes api-gateway auth delete "$AUTH_CONFIG_ID" -y 2>/dev/null || true

    # Getting deleted auth config should fail or return empty
    run bash -c "./orkes api-gateway auth get '$AUTH_CONFIG_ID' 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

@test "18. Cleanup temp files" {
    rm -f /tmp/service_update.json /tmp/route_update.json
}
