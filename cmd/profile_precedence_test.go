/*
 * Copyright 2026 Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// setupTestConfig creates a temp config directory with the given profile config,
// resets viper, and sets the cfgFile global to point at it. Returns a cleanup function.
func setupTestConfig(t *testing.T, profileName string, configContent string) (configDir string, cleanup func()) {
	t.Helper()

	configDir = t.TempDir()

	fileName := "config.yaml"
	if profileName != "" {
		fileName = "config-" + profileName + ".yaml"
	}

	configPath := filepath.Join(configDir, fileName)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Save globals
	oldProfile := profile
	oldCfgFile := cfgFile

	cleanup = func() {
		profile = oldProfile
		cfgFile = oldCfgFile
		viper.Reset()
		// Clean up env vars that tests may have set
		os.Unsetenv("CONDUCTOR_SERVER_URL")
		os.Unsetenv("CONDUCTOR_AUTH_TOKEN")
		os.Unsetenv("CONDUCTOR_PROFILE")
	}

	return configDir, cleanup
}

// initConfigForTest replicates the config loading logic from initConfig()
// but uses a provided configDir instead of ~/.conductor-cli, avoiding os.Exit
// and os.Args dependencies.
func initConfigForTest(configDir string, activeProfile string) {
	viper.Reset()
	viper.AddConfigPath(configDir)
	viper.SetConfigType("yaml")

	if activeProfile != "" {
		viper.SetConfigName("config-" + activeProfile)
	} else {
		viper.SetConfigName("config")
	}

	// Replicate the env var binding logic from initConfig:
	// Only bind env vars when no profile is active.
	if activeProfile == "" {
		viper.SetEnvPrefix("CONDUCTOR")
		viper.AutomaticEnv()
		viper.BindEnv("server", "CONDUCTOR_SERVER_URL")
		viper.BindEnv("auth-key", "CONDUCTOR_AUTH_KEY")
		viper.BindEnv("auth-secret", "CONDUCTOR_AUTH_SECRET")
		viper.BindEnv("auth-token", "CONDUCTOR_AUTH_TOKEN")
		viper.BindEnv("server-type", "CONDUCTOR_SERVER_TYPE")
	}

	viper.ReadInConfig()
}

func TestProfileOverridesEnvVar(t *testing.T) {
	profileConfig := `server: https://profile-server.example.com/api
auth-token: profile-token-123
`
	configDir, cleanup := setupTestConfig(t, "prod", profileConfig)
	defer cleanup()

	// Set env vars that should be overridden by profile
	os.Setenv("CONDUCTOR_SERVER_URL", "https://env-server.example.com/api")
	os.Setenv("CONDUCTOR_AUTH_TOKEN", "env-token-456")

	initConfigForTest(configDir, "prod")

	got := viper.GetString("server")
	if got != "https://profile-server.example.com/api" {
		t.Errorf("with --profile, server = %q, want profile value %q", got, "https://profile-server.example.com/api")
	}

	gotToken := viper.GetString("auth-token")
	if gotToken != "profile-token-123" {
		t.Errorf("with --profile, auth-token = %q, want profile value %q", gotToken, "profile-token-123")
	}
}

func TestEnvVarUsedWithoutProfile(t *testing.T) {
	// Default config with a different server
	defaultConfig := `server: https://default-server.example.com/api
`
	configDir, cleanup := setupTestConfig(t, "", defaultConfig)
	defer cleanup()

	// Set env var — should override default config when no profile is active
	os.Setenv("CONDUCTOR_SERVER_URL", "https://env-server.example.com/api")

	initConfigForTest(configDir, "")

	got := viper.GetString("server")
	if got != "https://env-server.example.com/api" {
		t.Errorf("without profile, server = %q, want env value %q", got, "https://env-server.example.com/api")
	}
}

func TestEnvVarUsedWhenNoConfigFile(t *testing.T) {
	configDir := t.TempDir() // empty dir, no config file
	defer func() {
		viper.Reset()
		os.Unsetenv("CONDUCTOR_SERVER_URL")
	}()

	os.Setenv("CONDUCTOR_SERVER_URL", "https://env-server.example.com/api")

	initConfigForTest(configDir, "")

	got := viper.GetString("server")
	if got != "https://env-server.example.com/api" {
		t.Errorf("with no config file, server = %q, want env value %q", got, "https://env-server.example.com/api")
	}
}

func TestProfileConfigUsedWithoutEnvVar(t *testing.T) {
	profileConfig := `server: https://profile-server.example.com/api
auth-token: profile-token-123
`
	configDir, cleanup := setupTestConfig(t, "staging", profileConfig)
	defer cleanup()

	// No env vars set — profile config should be used
	os.Unsetenv("CONDUCTOR_SERVER_URL")
	os.Unsetenv("CONDUCTOR_AUTH_TOKEN")

	initConfigForTest(configDir, "staging")

	got := viper.GetString("server")
	if got != "https://profile-server.example.com/api" {
		t.Errorf("server = %q, want profile value %q", got, "https://profile-server.example.com/api")
	}

	gotToken := viper.GetString("auth-token")
	if gotToken != "profile-token-123" {
		t.Errorf("auth-token = %q, want profile value %q", gotToken, "profile-token-123")
	}
}

func TestAllConfigKeysOverriddenByProfile(t *testing.T) {
	profileConfig := `server: https://profile-server.example.com/api
auth-key: profile-key
auth-secret: profile-secret
auth-token: profile-token
server-type: Enterprise
`
	configDir, cleanup := setupTestConfig(t, "prod", profileConfig)
	defer cleanup()

	// Set ALL env vars
	os.Setenv("CONDUCTOR_SERVER_URL", "https://env-server.example.com/api")
	os.Setenv("CONDUCTOR_AUTH_TOKEN", "env-token")

	initConfigForTest(configDir, "prod")

	tests := []struct {
		key  string
		want string
	}{
		{"server", "https://profile-server.example.com/api"},
		{"auth-key", "profile-key"},
		{"auth-secret", "profile-secret"},
		{"auth-token", "profile-token"},
		{"server-type", "Enterprise"},
	}

	for _, tt := range tests {
		got := viper.GetString(tt.key)
		if got != tt.want {
			t.Errorf("with profile, %s = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestConductorProfileEnvVarActivatesProfile(t *testing.T) {
	profileConfig := `server: https://profile-server.example.com/api
`
	configDir, cleanup := setupTestConfig(t, "dev", profileConfig)
	defer cleanup()

	// CONDUCTOR_PROFILE env var should activate the profile
	os.Setenv("CONDUCTOR_PROFILE", "dev")
	os.Setenv("CONDUCTOR_SERVER_URL", "https://env-server.example.com/api")

	// Simulate: no --profile flag, but CONDUCTOR_PROFILE is set
	activeProfile := os.Getenv("CONDUCTOR_PROFILE")
	initConfigForTest(configDir, activeProfile)

	got := viper.GetString("server")
	if got != "https://profile-server.example.com/api" {
		t.Errorf("with CONDUCTOR_PROFILE env, server = %q, want profile value %q", got, "https://profile-server.example.com/api")
	}
}

func TestDefaultConfigUsedWhenNoProfileNoEnv(t *testing.T) {
	defaultConfig := `server: https://default-server.example.com/api
auth-token: default-token
`
	configDir, cleanup := setupTestConfig(t, "", defaultConfig)
	defer cleanup()

	os.Unsetenv("CONDUCTOR_SERVER_URL")
	os.Unsetenv("CONDUCTOR_AUTH_TOKEN")

	initConfigForTest(configDir, "")

	got := viper.GetString("server")
	if got != "https://default-server.example.com/api" {
		t.Errorf("server = %q, want default config value %q", got, "https://default-server.example.com/api")
	}

	gotToken := viper.GetString("auth-token")
	if gotToken != "default-token" {
		t.Errorf("auth-token = %q, want default config value %q", gotToken, "default-token")
	}
}
