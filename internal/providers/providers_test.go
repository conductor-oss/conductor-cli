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

package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal/transport"
)

// stub serves one canned response and records the paths it was asked for.
func stub(t *testing.T, status int, body string) (transport.Config, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return transport.Config{BaseURL: srv.URL + "/api"}, &paths
}

func TestFetchReadsPerProviderStatus(t *testing.T) {
	cfg, paths := stub(t, http.StatusOK, `{"managedByHost":false,"providers":[
		{"name":"openai","configured":true},
		{"name":"anthropic","configured":false},
		{"name":"ollama","configured":true,"baseUrl":"http://localhost:11434","reachable":false}]}`)

	st, err := Fetch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if st.ManagedByHost {
		t.Error("ManagedByHost should be false")
	}
	if len(st.Providers) != 3 {
		t.Fatalf("got %d providers, want 3", len(st.Providers))
	}
	if !st.Providers[0].Configured || st.Providers[0].Name != "openai" {
		t.Errorf("first provider = %+v", st.Providers[0])
	}
	if st.Providers[1].Configured {
		t.Error("anthropic should be unconfigured")
	}

	ollama := st.Providers[2]
	if ollama.BaseURL != "http://localhost:11434" {
		t.Errorf("ollama BaseURL = %q", ollama.BaseURL)
	}
	if ollama.Reachable == nil || *ollama.Reachable {
		t.Errorf("ollama Reachable = %v, want a non-nil false", ollama.Reachable)
	}

	// Reachability is only reported for providers the server probes; the absence of
	// the field must stay distinguishable from a probe that failed.
	if st.Providers[0].Reachable != nil {
		t.Error("openai should carry no reachability")
	}

	if len(*paths) != 1 {
		t.Fatalf("made %d requests, want exactly 1: %v", len(*paths), *paths)
	}
	if (*paths)[0] != "/api/providers/status" {
		t.Errorf("requested %q", (*paths)[0])
	}
}

func TestFetchReportsHostManagedConfiguration(t *testing.T) {
	cfg, _ := stub(t, http.StatusOK, `{"managedByHost":true,"providers":[]}`)

	st, err := Fetch(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !st.ManagedByHost {
		t.Error("ManagedByHost should be true")
	}
	if len(st.Providers) != 0 {
		t.Errorf("host-managed response should carry no per-provider detail, got %d", len(st.Providers))
	}
}

// A server with AI integrations disabled has no such route. That is a fact about the
// deployment, not a failure, so callers need to tell it apart from a real error.
func TestFetchTreatsMissingEndpointAsUnsupported(t *testing.T) {
	cfg, _ := stub(t, http.StatusNotFound, `{"status":404,"message":"Not Found"}`)

	if _, err := Fetch(context.Background(), cfg); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestFetchSurfacesOtherErrors(t *testing.T) {
	cfg, _ := stub(t, http.StatusInternalServerError, `{"status":500,"message":"boom"}`)

	_, err := Fetch(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrUnsupported) {
		t.Error("a 500 is a real failure, not an unsupported endpoint")
	}
}
