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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/transport"
)

// pointTransportAt swaps the shared transport for the duration of a test.
func pointTransportAt(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	previous := internal.Transport()
	internal.SetTransport(transport.Config{BaseURL: srv.URL + "/api"})
	t.Cleanup(func() {
		srv.Close()
		internal.SetTransport(previous)
	})
}

func TestResolveInitModelReturnsAnExplicitModelVerbatim(t *testing.T) {
	// Pointed at a server that would fail the lookup: an explicit model must not
	// depend on the server being reachable at all.
	pointTransportAt(t, http.StatusInternalServerError, `{"message":"boom"}`)

	got, err := resolveInitModel(context.Background(), "anthropic/some-model")
	if err != nil {
		t.Fatalf("resolveInitModel: %v", err)
	}
	if got != "anthropic/some-model" {
		t.Errorf("model = %q, want it passed through unchanged", got)
	}
}

func TestResolveInitModelNamesConfiguredProviders(t *testing.T) {
	pointTransportAt(t, http.StatusOK, `{"managedByHost":false,"providers":[
		{"name":"openai","configured":false},
		{"name":"anthropic","configured":true},
		{"name":"perplexity","configured":true}]}`)

	_, err := resolveInitModel(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error when no model is given")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--model") {
		t.Errorf("error should name the flag to pass: %q", msg)
	}
	if !strings.Contains(msg, "anthropic") || !strings.Contains(msg, "perplexity") {
		t.Errorf("error should name the configured providers: %q", msg)
	}
	if strings.Contains(msg, "openai") {
		t.Errorf("error should not offer an unconfigured provider: %q", msg)
	}
}

// init scaffolds a local file. It must stay usable with no server reachable, so the
// provider list is a courtesy rather than a precondition.
func TestResolveInitModelStillFailsUsefullyWithoutAServer(t *testing.T) {
	pointTransportAt(t, http.StatusNotFound, `{"status":404,"message":"Not Found"}`)

	_, err := resolveInitModel(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error when no model is given")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--model") {
		t.Errorf("error should name the flag to pass: %q", msg)
	}
	if !strings.Contains(msg, "doctor") {
		t.Errorf("error should point at a way to find the answer: %q", msg)
	}
}

func TestResolveInitModelReportsHostManagedConfiguration(t *testing.T) {
	pointTransportAt(t, http.StatusOK, `{"managedByHost":true,"providers":[]}`)

	_, err := resolveInitModel(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error when no model is given")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("error should explain why no providers are listed: %q", err.Error())
	}
}
