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
		name      string
		serverVal string
		want      bool
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

// TestCodeCommandRemoved guards the removal of `conductor code`. Its templates were
// fetched from an unversioned external origin and it embedded credentials in generated
// source; see issue #93. A stray re-registration would silently bring both back.
func TestCodeCommandRemoved(t *testing.T) {
	for _, args := range [][]string{{"code"}, {"code", "list"}} {
		cmd, _, err := rootCmd.Find(args)
		if err == nil && cmd != rootCmd && cmd.Name() == args[len(args)-1] {
			t.Errorf("rootCmd.Find(%v) resolved to a real command; `conductor code` was removed in #93", args)
		}
	}
}

func TestIsLocalOnlyCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "update", args: []string{"update"}, want: true},
		{name: "config save", args: []string{"config", "save"}, want: true},
		{name: "server start", args: []string{"server", "start"}, want: true},
		// Subcommands that merely share a name with a local-only command still
		// need an API client.
		{name: "schedule update", args: []string{"schedule", "update"}, want: false},
		{name: "webhook update", args: []string{"webhook", "update"}, want: false},
		{name: "task update", args: []string{"task", "update"}, want: false},
		{name: "workflow update", args: []string{"workflow", "update"}, want: false},
		{name: "workflow list", args: []string{"workflow", "list"}, want: false},
		{name: "api-gateway service list", args: []string{"api-gateway", "service", "list"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _, err := rootCmd.Find(tt.args)
			if err != nil {
				t.Fatalf("rootCmd.Find(%v) returned error: %v", tt.args, err)
			}
			// Guard against a typo in args silently resolving to a parent command.
			if cmd.Name() != tt.args[len(tt.args)-1] {
				t.Fatalf("rootCmd.Find(%v) resolved to %q, want %q", tt.args, cmd.Name(), tt.args[len(tt.args)-1])
			}
			if got := isLocalOnlyCommand(cmd); got != tt.want {
				t.Errorf("isLocalOnlyCommand(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
