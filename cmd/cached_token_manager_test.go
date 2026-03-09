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
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewCachedTokenManager(t *testing.T) {
	mgr := NewCachedTokenManager("key1", "secret1", "cached-token", 1234567890, "/tmp/config.yaml", nil)

	if mgr.Key != "key1" {
		t.Errorf("Key = %q, want %q", mgr.Key, "key1")
	}
	if mgr.Secret != "secret1" {
		t.Errorf("Secret = %q, want %q", mgr.Secret, "secret1")
	}
	if mgr.cachedToken != "cached-token" {
		t.Errorf("cachedToken = %q, want %q", mgr.cachedToken, "cached-token")
	}
	if mgr.tokenExpiry != 1234567890 {
		t.Errorf("tokenExpiry = %d, want %d", mgr.tokenExpiry, 1234567890)
	}
	if mgr.configPath != "/tmp/config.yaml" {
		t.Errorf("configPath = %q, want %q", mgr.configPath, "/tmp/config.yaml")
	}
}

func TestGetConfigPath(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		wantSuffix  string
	}{
		{
			name:        "empty profile returns error",
			profileName: "",
			wantSuffix:  "",
		},
		{
			name:        "named profile",
			profileName: "production",
			wantSuffix:  filepath.Join(".conductor-cli", "config-production.yaml"),
		},
		{
			name:        "another named profile",
			profileName: "dev",
			wantSuffix:  filepath.Join(".conductor-cli", "config-dev.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getConfigPath(tt.profileName)
			if tt.profileName == "" {
				if err == nil {
					t.Fatalf("getConfigPath(%q) expected error, got path %q", tt.profileName, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("getConfigPath(%q) error: %v", tt.profileName, err)
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("getConfigPath(%q) = %q, want suffix %q", tt.profileName, got, tt.wantSuffix)
			}
		})
	}
}

func TestReadConfigFile(t *testing.T) {
	t.Run("file does not exist returns empty map", func(t *testing.T) {
		mgr := &CachedTokenManager{
			configPath: filepath.Join(t.TempDir(), "nonexistent.yaml"),
		}
		result, err := mgr.readConfigFile()
		if err != nil {
			t.Fatalf("readConfigFile() error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("readConfigFile() returned %d entries, want 0", len(result))
		}
	})

	t.Run("valid YAML file returns parsed map", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		data := map[string]interface{}{
			"server":    "http://localhost:8080/api",
			"auth-key":  "my-key",
		}
		yamlData, _ := yaml.Marshal(data)
		os.WriteFile(configPath, yamlData, 0600)

		mgr := &CachedTokenManager{configPath: configPath}
		result, err := mgr.readConfigFile()
		if err != nil {
			t.Fatalf("readConfigFile() error: %v", err)
		}
		if result["server"] != "http://localhost:8080/api" {
			t.Errorf("server = %v, want %q", result["server"], "http://localhost:8080/api")
		}
		if result["auth-key"] != "my-key" {
			t.Errorf("auth-key = %v, want %q", result["auth-key"], "my-key")
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bad.yaml")
		// Use content that can't be unmarshalled into map[string]interface{}
		os.WriteFile(configPath, []byte("- item1\n- item2\n"), 0600)

		mgr := &CachedTokenManager{configPath: configPath}
		_, err := mgr.readConfigFile()
		if err == nil {
			t.Errorf("readConfigFile() expected error for non-map YAML, got nil")
		}
	})
}

func TestSaveCachedToken(t *testing.T) {
	t.Run("empty config path returns error", func(t *testing.T) {
		mgr := &CachedTokenManager{configPath: ""}
		err := mgr.saveCachedToken()
		if err == nil {
			t.Errorf("saveCachedToken() expected error for empty config path, got nil")
		}
	})

	t.Run("saves token and expiry to new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		mgr := &CachedTokenManager{
			configPath:  configPath,
			cachedToken: "my-new-token",
			tokenExpiry: 9999999999,
		}

		err := mgr.saveCachedToken()
		if err != nil {
			t.Fatalf("saveCachedToken() error: %v", err)
		}

		// Read back and verify
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}

		var config map[string]interface{}
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if config["cached-token"] != "my-new-token" {
			t.Errorf("cached-token = %v, want %q", config["cached-token"], "my-new-token")
		}
	})

	t.Run("preserves existing config fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		// Write existing config
		existing := map[string]interface{}{
			"server":   "http://prod:8080/api",
			"auth-key": "existing-key",
		}
		yamlData, _ := yaml.Marshal(existing)
		os.WriteFile(configPath, yamlData, 0600)

		mgr := &CachedTokenManager{
			configPath:  configPath,
			cachedToken: "new-token",
			tokenExpiry: 1234567890,
		}

		err := mgr.saveCachedToken()
		if err != nil {
			t.Fatalf("saveCachedToken() error: %v", err)
		}

		// Read back and verify existing fields preserved
		data, _ := os.ReadFile(configPath)
		var config map[string]interface{}
		yaml.Unmarshal(data, &config)

		if config["server"] != "http://prod:8080/api" {
			t.Errorf("server = %v, want preserved value", config["server"])
		}
		if config["auth-key"] != "existing-key" {
			t.Errorf("auth-key = %v, want preserved value", config["auth-key"])
		}
		if config["cached-token"] != "new-token" {
			t.Errorf("cached-token = %v, want %q", config["cached-token"], "new-token")
		}
	})

	t.Run("file has correct permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		mgr := &CachedTokenManager{
			configPath:  configPath,
			cachedToken: "token",
			tokenExpiry: 123,
		}

		mgr.saveCachedToken()

		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatalf("Stat error: %v", err)
		}
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("file permissions = %o, want 0600", perm)
		}
	})
}
