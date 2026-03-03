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
	"strings"
	"testing"
)

func TestIsEnterpriseServer(t *testing.T) {
	tests := []struct {
		name       string
		serverVal  string
		want       bool
	}{
		{
			name:      "Enterprise lowercase",
			serverVal: "Enterprise",
			want:      true,
		},
		{
			name:      "ENTERPRISE uppercase",
			serverVal: "ENTERPRISE",
			want:      true,
		},
		{
			name:      "enterprise all lower",
			serverVal: "enterprise",
			want:      true,
		},
		{
			name:      "OSS returns false",
			serverVal: "OSS",
			want:      false,
		},
		{
			name:      "oss lowercase returns false",
			serverVal: "oss",
			want:      false,
		},
		{
			name:      "empty string returns false",
			serverVal: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global state
			oldServerType := serverType
			defer func() { serverType = oldServerType }()

			serverType = tt.serverVal
			got := isEnterpriseServer()
			if got != tt.want {
				t.Errorf("isEnterpriseServer() with serverType=%q = %v, want %v", tt.serverVal, got, tt.want)
			}
		})
	}
}

func TestConfirmDeletion_YesFlag(t *testing.T) {
	// Save and restore global state
	oldYes := yes
	defer func() { yes = oldYes }()

	yes = true
	if !confirmDeletion("workflow", "test-workflow") {
		t.Errorf("confirmDeletion() with yes=true should return true")
	}
}

func TestURLNormalization(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "adds /api suffix",
			url:  "http://localhost:8080",
			want: "http://localhost:8080/api",
		},
		{
			name: "already has /api",
			url:  "http://localhost:8080/api",
			want: "http://localhost:8080/api",
		},
		{
			name: "removes trailing slash then adds /api",
			url:  "http://localhost:8080/",
			want: "http://localhost:8080/api",
		},
		{
			name: "trailing slash after /api is removed",
			url:  "http://localhost:8080/api/",
			want: "http://localhost:8080/api",
		},
		{
			name: "https with path",
			url:  "https://conductor.example.com",
			want: "https://conductor.example.com/api",
		},
		{
			name: "https already with /api",
			url:  "https://conductor.example.com/api",
			want: "https://conductor.example.com/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the URL normalization logic from PersistentPreRunE
			result := strings.TrimSuffix(tt.url, "/")
			if !strings.HasSuffix(result, "/api") {
				result = result + "/api"
			}
			if result != tt.want {
				t.Errorf("URL normalization(%q) = %q, want %q", tt.url, result, tt.want)
			}
		})
	}
}
