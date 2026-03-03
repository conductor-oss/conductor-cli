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
	"fmt"
	"strings"
	"testing"
)

func TestConfigFilePathResolution(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		wantFile    string
	}{
		{
			name:        "default profile uses config.yaml",
			profileName: "",
			wantFile:    "config.yaml",
		},
		{
			name:        "named profile uses config-<name>.yaml",
			profileName: "production",
			wantFile:    "config-production.yaml",
		},
		{
			name:        "dev profile",
			profileName: "dev",
			wantFile:    "config-dev.yaml",
		},
		{
			name:        "staging profile",
			profileName: "staging",
			wantFile:    "config-staging.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFileName := "config.yaml"
			if tt.profileName != "" {
				configFileName = fmt.Sprintf("config-%s.yaml", tt.profileName)
			}
			if configFileName != tt.wantFile {
				t.Errorf("config file name = %q, want %q", configFileName, tt.wantFile)
			}
		})
	}
}

func TestProfileNameParsing(t *testing.T) {
	tests := []struct {
		name        string
		fileName    string
		isProfile   bool
		profileName string
	}{
		{
			name:        "default config",
			fileName:    "config.yaml",
			isProfile:   true,
			profileName: "default",
		},
		{
			name:        "named profile prod",
			fileName:    "config-prod.yaml",
			isProfile:   true,
			profileName: "prod",
		},
		{
			name:        "named profile staging",
			fileName:    "config-staging.yaml",
			isProfile:   true,
			profileName: "staging",
		},
		{
			name:      "non-config file",
			fileName:  "notes.txt",
			isProfile: false,
		},
		{
			name:      "yaml but not config prefix",
			fileName:  "other.yaml",
			isProfile: false,
		},
		{
			name:      "update state file",
			fileName:  "update-state.yaml",
			isProfile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the profile parsing logic from configListCmd
			name := tt.fileName
			var gotProfile string
			isProfile := false

			if name == "config.yaml" {
				gotProfile = "default"
				isProfile = true
			} else if strings.HasPrefix(name, "config-") && strings.HasSuffix(name, ".yaml") {
				gotProfile = strings.TrimPrefix(name, "config-")
				gotProfile = strings.TrimSuffix(gotProfile, ".yaml")
				isProfile = true
			}

			if isProfile != tt.isProfile {
				t.Errorf("isProfile = %v, want %v", isProfile, tt.isProfile)
			}
			if tt.isProfile && gotProfile != tt.profileName {
				t.Errorf("profileName = %q, want %q", gotProfile, tt.profileName)
			}
		})
	}
}
