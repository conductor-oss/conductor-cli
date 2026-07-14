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

import "testing"

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
