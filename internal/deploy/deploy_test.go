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

package deploy

import (
	"context"
	"strings"
	"testing"
)

type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

func envValue(env []string, key string) (string, bool) {
	for _, e := range env {
		if strings.HasPrefix(e, key+"=") {
			return strings.TrimPrefix(e, key+"="), true
		}
	}
	return "", false
}

func TestEnvBuilderStripsStaleAndInjectsConnection(t *testing.T) {
	b := EnvBuilder{
		BaseURL: "http://localhost:8080/api",
		Tokens:  staticToken("jwt-x"),
		BaseEnv: []string{"PATH=/usr/bin", "AGENTSPAN_SERVER_URL=stale", "AGENTSPAN_API_KEY=old"},
	}
	env, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if _, ok := envValue(env, "PATH"); !ok {
		t.Error("inherited PATH should be preserved")
	}
	if v, _ := envValue(env, EnvServerURL); v != "http://localhost:8080/api" {
		t.Errorf("%s = %q", EnvServerURL, v)
	}
	if v, _ := envValue(env, EnvAutoStart); v != autoStartOff {
		t.Errorf("%s = %q, want %q", EnvAutoStart, v, autoStartOff)
	}
	if v, _ := envValue(env, EnvAuthToken); v != "jwt-x" {
		t.Errorf("%s = %q, want jwt-x", EnvAuthToken, v)
	}
	// The stale AGENTSPAN_API_KEY must not survive.
	if _, ok := envValue(env, "AGENTSPAN_API_KEY"); ok {
		t.Error("stale AGENTSPAN_API_KEY should have been stripped")
	}
}

func TestEnvBuilderAnonymousOmitsToken(t *testing.T) {
	b := EnvBuilder{BaseURL: "http://localhost:8080/api", BaseEnv: []string{"PATH=/usr/bin"}}
	env, err := b.Build(context.Background())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := envValue(env, EnvAuthToken); ok {
		t.Error("anonymous build must not set an auth token")
	}
}
