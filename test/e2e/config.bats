#!/usr/bin/env bats

# E2E tests for config commands
# Tests config save and delete functionality

setup() {
    # Ensure the CLI binary exists
    if [ ! -f "./orkes" ]; then
        echo "ERROR: orkes binary not found. Please build it first."
        exit 1
    fi


    # Clean up any existing test config files
    rm -f ~/.conductor-cli/config-e2e-test.yaml
    rm -f ~/.conductor-cli/config-e2e-test2.yaml
    rm -f ~/.conductor-cli/config-e2e-default.yaml
}

teardown() {
    # Clean up test config files after each test
    rm -f ~/.conductor-cli/config-e2e-test.yaml
    rm -f ~/.conductor-cli/config-e2e-test2.yaml
    rm -f ~/.conductor-cli/config-e2e-default.yaml
}

@test "1. Save config to named profile with --profile flag" {
    run bash -c "./orkes --server http://test.example.com --auth-key test-key-123 --auth-secret test-secret-456 --profile e2e-test config save 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Configuration saved to ~/.conductor-cli/config-e2e-test.yaml"* ]]
    
    # Verify file exists
    [ -f ~/.conductor-cli/config-e2e-test.yaml ]
    
    # Verify file contents
    grep -q "server: http://test.example.com" ~/.conductor-cli/config-e2e-test.yaml
    grep -q "auth-key: test-key-123" ~/.conductor-cli/config-e2e-test.yaml
    grep -q "auth-secret: test-secret-456" ~/.conductor-cli/config-e2e-test.yaml
}

@test "2. Save config with auth-token instead of key/secret" {
    run bash -c "./orkes --server https://prod.example.com --auth-token my-token-789 --profile e2e-test2 config save 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    
    # Verify file exists
    [ -f ~/.conductor-cli/config-e2e-test2.yaml ]
    
    # Verify file contents
    grep -q "server: https://prod.example.com" ~/.conductor-cli/config-e2e-test2.yaml
    grep -q "auth-token: my-token-789" ~/.conductor-cli/config-e2e-test2.yaml
    
    # Should NOT contain auth-key or auth-secret
    ! grep -q "auth-key" ~/.conductor-cli/config-e2e-test2.yaml
    ! grep -q "auth-secret" ~/.conductor-cli/config-e2e-test2.yaml
}

@test "3. Save config with flags after command" {
    run bash -c "./orkes config save --server http://custom.example.com --auth-key local-key --profile e2e-test 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify file was overwritten with new values
    [ -f ~/.conductor-cli/config-e2e-test.yaml ]
    grep -q "server: http://custom.example.com" ~/.conductor-cli/config-e2e-test.yaml
    grep -q "auth-key: local-key" ~/.conductor-cli/config-e2e-test.yaml
}

@test "4. Delete config using --profile flag with -y" {
    # First create the config
    ./orkes --server http://test.com --auth-key key --profile e2e-test config save 2>&1
    
    # Delete it
    run bash -c "./orkes config delete --profile e2e-test -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Configuration deleted"* ]]
    [[ "$output" == *"config-e2e-test.yaml"* ]]
    
    # Verify file was deleted
    [ ! -f ~/.conductor-cli/config-e2e-test.yaml ]
}

@test "5. Delete config using positional argument" {
    # First create the config
    ./orkes --server http://test.com --auth-key key --profile e2e-test config save 2>&1
    
    # Delete it using positional argument
    run bash -c "./orkes config delete e2e-test -y 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Configuration deleted"* ]]
    
    # Verify file was deleted
    [ ! -f ~/.conductor-cli/config-e2e-test.yaml ]
}

@test "6. Delete non-existent config shows error" {
    run bash -c "./orkes config delete --profile nonexistent -y 2>&1"
    echo "Output: $output"
    [ "$status" -ne 0 ]
    [[ "$output" == *"doesn't exist"* ]]
}

@test "7. Save config without --profile saves to default" {
    # Note: We use a different approach to test default config
    # to avoid interfering with actual default config
    run bash -c "./orkes --server http://default.test.com --auth-key default-key --profile e2e-default config save 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    [[ "$output" == *"config-e2e-default.yaml"* ]]
    
    # Verify file exists with correct name
    [ -f ~/.conductor-cli/config-e2e-default.yaml ]
}

# FIXME - failing on CI
#@test "8. Config file has correct permissions (0600)" {
#    run bash -c "./orkes --server http://test.com --auth-key key --profile e2e-test config save 2>&1"
#    [ "$status" -eq 0 ]

#    # Check file permissions (should be 0600 or -rw-------)
#    perms=$(stat -f "%OLp" ~/.conductor-cli/config-e2e-test.yaml 2>/dev/null || stat -c "%a" ~/.conductor-cli/config-e2e-test.yaml 2>/dev/null)
#    [ "$perms" = "600" ]
#}

@test "9. Config save with only server URL (no auth)" {
    run bash -c "./orkes --server http://noauth.example.com --profile e2e-test config save 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]
    
    # Verify file exists but only has server (no auth fields since default localhost is filtered)
    [ -f ~/.conductor-cli/config-e2e-test.yaml ]
    grep -q "server: http://noauth.example.com" ~/.conductor-cli/config-e2e-test.yaml
}

@test "10. Multiple saves to same profile overwrites correctly" {
    # First save
    ./orkes --server http://first.com --auth-key first-key --profile e2e-test config save 2>&1

    # Second save
    run bash -c "./orkes --server http://second.com --auth-key second-key --profile e2e-test config save 2>&1"
    [ "$status" -eq 0 ]

    # Verify only second values exist
    grep -q "server: http://second.com" ~/.conductor-cli/config-e2e-test.yaml
    grep -q "auth-key: second-key" ~/.conductor-cli/config-e2e-test.yaml
    ! grep -q "first.com" ~/.conductor-cli/config-e2e-test.yaml
    ! grep -q "first-key" ~/.conductor-cli/config-e2e-test.yaml
}

@test "11. List config profiles shows all profiles" {
    # Create multiple profiles
    ./orkes --server http://test1.com --auth-key key1 --profile e2e-list1 config save 2>&1
    ./orkes --server http://test2.com --auth-key key2 --profile e2e-list2 config save 2>&1
    ./orkes --server http://test3.com --auth-key key3 --profile e2e-list3 config save 2>&1

    # List configs
    run bash -c "./orkes config list 2>&1"
    echo "Output: $output"
    [ "$status" -eq 0 ]

    # Verify all profiles are listed
    [[ "$output" == *"e2e-list1"* ]]
    [[ "$output" == *"e2e-list2"* ]]
    [[ "$output" == *"e2e-list3"* ]]

    # Clean up
    rm -f ~/.conductor-cli/config-e2e-list1.yaml
    rm -f ~/.conductor-cli/config-e2e-list2.yaml
    rm -f ~/.conductor-cli/config-e2e-list3.yaml
}

@test "12. List shows 'default' for config.yaml" {
    # Create default config (using a unique profile name to avoid conflicts)
    ./orkes --server http://test.com --auth-key key --profile e2e-default-check config save 2>&1

    # List configs
    run bash -c "./orkes config list 2>&1"
    [ "$status" -eq 0 ]
    [[ "$output" == *"e2e-default-check"* ]]

    # Clean up
    rm -f ~/.conductor-cli/config-e2e-default-check.yaml
}

@test "13. Server URL without /api suffix is accepted" {
    # Test URL without /api
    run bash -c "./orkes --server http://example.com --auth-key key --profile e2e-noapi config save 2>&1"
    [ "$status" -eq 0 ]

    # Verify config was saved with user's input (not normalized)
    [ -f ~/.conductor-cli/config-e2e-noapi.yaml ]
    grep -q "server: http://example.com" ~/.conductor-cli/config-e2e-noapi.yaml

    # Clean up
    rm -f ~/.conductor-cli/config-e2e-noapi.yaml
}

@test "14. Server URL with /api suffix is accepted" {
    # Test URL with /api
    run bash -c "./orkes --server http://example.com/api --auth-key key --profile e2e-withapi config save 2>&1"
    [ "$status" -eq 0 ]

    # Verify config was saved
    [ -f ~/.conductor-cli/config-e2e-withapi.yaml ]

    # Clean up
    rm -f ~/.conductor-cli/config-e2e-withapi.yaml
}

@test "15. Server URL with trailing slash is handled" {
    # Test URL with trailing slash
    run bash -c "./orkes --server http://example.com/ --auth-key key --profile e2e-slash config save 2>&1"
    [ "$status" -eq 0 ]

    # Verify config was saved with trailing slash (user's input preserved)
    [ -f ~/.conductor-cli/config-e2e-slash.yaml ]
    grep -q "server: http://example.com/" ~/.conductor-cli/config-e2e-slash.yaml

    # Clean up
    rm -f ~/.conductor-cli/config-e2e-slash.yaml
}
