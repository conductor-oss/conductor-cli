#!/usr/bin/env bats

# E2E tests for secret management functionality

SECRET_KEY="e2e_test_secret"
SECRET_KEY_2="e2e_test_secret_2"
SECRET_VALUE="test_secret_value_12345"
SECRET_VALUE_UPDATED="updated_secret_value_67890"

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi

    # Clean up any existing test secrets
    ./orkes secret delete "$SECRET_KEY" -y 2>/dev/null || true
    ./orkes secret delete "$SECRET_KEY_2" -y 2>/dev/null || true
}

teardown() {
    # Clean up test secrets after each test
    ./orkes secret delete "$SECRET_KEY" -y 2>/dev/null || true
    ./orkes secret delete "$SECRET_KEY_2" -y 2>/dev/null || true
}

@test "1. Create secret using command arguments" {
    run bash -c "./orkes secret put '$SECRET_KEY' '$SECRET_VALUE' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "saved successfully"
}

@test "2. Check if secret exists" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # Check if it exists
    run bash -c "./orkes secret exists '$SECRET_KEY' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "exists"
}

@test "3. Get secret value (without --show-value flag)" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # Get without showing value
    run bash -c "./orkes secret get '$SECRET_KEY' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "Secret exists"
    # Should NOT contain the actual value
    ! echo "$output" | grep -q "$SECRET_VALUE"
}

@test "4. Get secret value (with --show-value flag)" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # Get with showing value
    run bash -c "./orkes secret get '$SECRET_KEY' --show-value 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    # Should contain the actual value
    echo "$output" | grep -q "$SECRET_VALUE"
}

@test "5. Update secret with new value" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # Update with new value
    run bash -c "./orkes secret put '$SECRET_KEY' '$SECRET_VALUE_UPDATED' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "saved successfully"

    # Verify new value
    run bash -c "./orkes secret get '$SECRET_KEY' --show-value 2>&1"
    echo "$output" | grep -q "$SECRET_VALUE_UPDATED"
    # Old value should not be present
    ! echo "$output" | grep -q "$SECRET_VALUE"
}

@test "6. Create secret using --value flag" {
    run bash -c "./orkes secret put '$SECRET_KEY_2' --value '$SECRET_VALUE' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "saved successfully"

    # Verify value
    run bash -c "./orkes secret get '$SECRET_KEY_2' --show-value 2>&1"
    echo "$output" | grep -q "$SECRET_VALUE"
}

@test "7. Create secret from stdin" {
    run bash -c "echo '$SECRET_VALUE' | ./orkes secret put '$SECRET_KEY' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "saved successfully"

    # Verify value
    run bash -c "./orkes secret get '$SECRET_KEY' --show-value 2>&1"
    echo "$output" | grep -q "$SECRET_VALUE"
}

@test "8. List secrets (should include created secret)" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # List secrets
    run bash -c "./orkes secret list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "$SECRET_KEY"
}

@test "9. List secrets with JSON output" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # List with JSON
    run bash -c "./orkes secret list --json 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "$SECRET_KEY"
}

@test "10. Add tags to secret" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # Add tags
    run bash -c "./orkes secret tag-add '$SECRET_KEY' --tag env:test --tag team:e2e 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "Tags added"
}

@test "11. List tags for secret" {
    # Create secret and add tags
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null
    ./orkes secret tag-add "$SECRET_KEY" --tag env:test --tag team:e2e 2>/dev/null

    # List tags
    run bash -c "./orkes secret tag-list '$SECRET_KEY' 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "env"
    echo "$output" | grep -q "test"
    echo "$output" | grep -q "team"
    echo "$output" | grep -q "e2e"
}

@test "12. List secrets with tags" {
    # Create secret and add tags
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null
    ./orkes secret tag-add "$SECRET_KEY" --tag env:test 2>/dev/null

    # List with tags
    run bash -c "./orkes secret list --with-tags 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "$SECRET_KEY"
    echo "$output" | grep -q "env:test"
}

@test "13. Delete tags from secret" {
    # Create secret and add tags
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null
    ./orkes secret tag-add "$SECRET_KEY" --tag env:test --tag team:e2e 2>/dev/null

    # Delete one tag
    run bash -c "./orkes secret tag-delete '$SECRET_KEY' --tag env:test 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "Tags deleted"

    # Verify tag was deleted
    run bash -c "./orkes secret tag-list '$SECRET_KEY' 2>&1"
    echo "Output: $output"
    # Should still have team tag
    echo "$output" | grep -q "team"
    # Should not have env tag
    ! echo "$output" | grep -q "env.*test"
}

@test "14. Clear local cache" {
    run bash -c "./orkes secret cache-clear --local 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "Local cache cleared successfully"
}

@test "15. Clear Redis cache" {
    run bash -c "./orkes secret cache-clear --redis 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "Redis cache cleared successfully"
}

@test "16. Clear both caches (no flags)" {
    run bash -c "./orkes secret cache-clear 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "Local cache cleared successfully"
    echo "$output" | grep -q "Redis cache cleared successfully"
}

@test "17. Delete secret" {
    # Create secret first
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null

    # Delete with -y flag to skip confirmation
    run bash -c "./orkes secret delete '$SECRET_KEY' -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "deleted successfully"
}

@test "18. Verify deleted secret no longer exists" {
    # Create and then delete secret
    ./orkes secret put "$SECRET_KEY" "$SECRET_VALUE" 2>/dev/null
    ./orkes secret delete "$SECRET_KEY" -y 2>/dev/null

    # Try to get deleted secret (should fail)
    run bash -c "./orkes secret get '$SECRET_KEY' 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

@test "19. Error handling - get non-existent secret" {
    # Try to get a secret that doesn't exist
    run bash -c "./orkes secret get 'non_existent_secret_xyz' 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}

@test "20. Error handling - delete non-existent secret" {
    # Try to delete a secret that doesn't exist
    run bash -c "./orkes secret delete 'non_existent_secret_xyz' -y 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
}
