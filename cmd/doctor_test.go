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
	"errors"
	"strings"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal/providers"
)

func boolPtr(b bool) *bool { return &b }

func TestServerProvidersReportConfiguredState(t *testing.T) {
	var sb strings.Builder
	printServerProviders(&sb, providers.Status{Providers: []providers.Provider{
		{Name: "openai", Configured: true},
		{Name: "anthropic", Configured: false},
	}}, nil)
	out := sb.String()

	if !strings.Contains(out, "OpenAI") || !strings.Contains(out, "Anthropic") {
		t.Errorf("both providers should be listed:\n%s", out)
	}
	// The whole point of the section: the reader must not mistake it for the local
	// environment check that follows.
	if !strings.Contains(strings.ToLower(out), "server") {
		t.Errorf("section should attribute the report to the server:\n%s", out)
	}
}

func TestServerProvidersShowReachabilityWhenProbed(t *testing.T) {
	var sb strings.Builder
	printServerProviders(&sb, providers.Status{Providers: []providers.Provider{
		{Name: "ollama", Configured: true, BaseURL: "http://localhost:11434", Reachable: boolPtr(false)},
	}}, nil)
	out := sb.String()

	if !strings.Contains(out, "http://localhost:11434") {
		t.Errorf("base url should be shown:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "unreachable") {
		t.Errorf("a failed probe should be called out:\n%s", out)
	}
}

func TestServerProvidersReportHostManagedConfiguration(t *testing.T) {
	var sb strings.Builder
	printServerProviders(&sb, providers.Status{ManagedByHost: true}, nil)
	out := strings.ToLower(sb.String())

	if !strings.Contains(out, "host") {
		t.Errorf("host-managed configuration should be named as such:\n%s", out)
	}
	if strings.Contains(out, "error") || strings.Contains(out, "failed") {
		t.Errorf("host-managed configuration is not a failure:\n%s", out)
	}
}

// A server with AI integrations disabled is a normal deployment, not a broken one.
func TestServerProvidersReportMissingEndpointPlainly(t *testing.T) {
	var sb strings.Builder
	printServerProviders(&sb, providers.Status{}, providers.ErrUnsupported)
	out := sb.String()

	if strings.TrimSpace(out) == "" {
		t.Fatal("an unsupported endpoint should still say something")
	}
	if strings.Contains(strings.ToLower(out), "error") {
		t.Errorf("should read as information, not an error:\n%s", out)
	}
}

func TestServerProvidersReportUnreachableServer(t *testing.T) {
	var sb strings.Builder
	printServerProviders(&sb, providers.Status{}, errors.New("dial tcp: connection refused"))

	if strings.TrimSpace(sb.String()) == "" {
		t.Fatal("a failed lookup should still say something")
	}
}

// Server and CLI spell the same provider differently. One provider, one name.
func TestServerProviderNamesMapToDisplayNames(t *testing.T) {
	for serverName, want := range map[string]string{
		"gemini":      "Google Gemini",
		"azureopenai": "Azure OpenAI",
		"huggingface": "Hugging Face",
		"aws_bedrock": "AWS Bedrock",
		"openai":      "OpenAI",
		"ollama":      "Ollama", // server-only: not in the local registry at all
	} {
		if got := displayProviderName(serverName); got != want {
			t.Errorf("displayProviderName(%q) = %q, want %q", serverName, got, want)
		}
	}
}

// Regression guard for #103: doctor reported model ids it could not keep current, and
// two of them had already been withdrawn by the provider. Model identifiers carry a
// "provider/model" shape; nothing else in this section contains a slash.
func TestLocalProvidersAdvertiseNoModelIdentifiers(t *testing.T) {
	var sb strings.Builder
	printLocalProviders(&sb)
	out := sb.String()

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "/") {
			t.Errorf("doctor should name no model identifiers, got: %q", line)
		}
	}
	if !strings.Contains(out, "Anthropic") {
		t.Errorf("providers should still be listed:\n%s", out)
	}
}

func TestProviderConfiguredRequiresAllEnvVars(t *testing.T) {
	p := aiProvider{name: "Test", envVars: []string{"DOC_TEST_KEY", "DOC_TEST_ENDPOINT"}}

	if isProviderConfigured(p) {
		t.Fatal("provider should not be configured with no env vars set")
	}

	t.Setenv("DOC_TEST_KEY", "x")
	if isProviderConfigured(p) {
		t.Error("provider should not be configured with only one of two env vars set")
	}
	if !providerOptedIn(p) {
		t.Error("provider should be opted in once any env var is set")
	}

	t.Setenv("DOC_TEST_ENDPOINT", "y")
	if !isProviderConfigured(p) {
		t.Error("provider should be configured once all env vars are set")
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine("openjdk version \"21\"\nmore\n"); got != `openjdk version "21"` {
		t.Errorf("firstLine = %q", got)
	}
}
