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

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal/transport"
)

func newTestClient(t *testing.T, h http.HandlerFunc) Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(transport.Config{BaseURL: srv.URL})
}

func TestListAgents(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathList {
			t.Errorf("path = %q, want %q", r.URL.Path, pathList)
		}
		_, _ = w.Write([]byte(`[{"name":"greeter","version":2,"type":"native"}]`))
	})
	out, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 || out[0].Name != "greeter" || out[0].Version != 2 {
		t.Errorf("got %+v", out)
	}
}

func TestRunSendsAgentConfigEnvelope(t *testing.T) {
	var body startRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != pathStart {
			t.Errorf("got %s %s, want POST %s", r.Method, r.URL.Path, pathStart)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"executionId":"e1","agentName":"greeter"}`))
	})
	exec, err := c.Run(context.Background(), RunRequest{
		Definition: json.RawMessage(`{"name":"greeter"}`),
		Prompt:     "hi",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if exec.ID != "e1" || exec.AgentName != "greeter" {
		t.Errorf("exec = %+v", exec)
	}
	if body.Prompt != "hi" || string(body.AgentConfig) != `{"name":"greeter"}` {
		t.Errorf("request body = %+v", body)
	}
}

func TestRunSendsFrameworkEnvelope(t *testing.T) {
	var raw map[string]json.RawMessage
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = w.Write([]byte(`{"executionId":"e2","agentName":"x"}`))
	})
	_, err := c.Run(context.Background(), RunRequest{
		Framework:  "openai",
		Definition: json.RawMessage(`{"name":"x"}`),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := raw["framework"]; !ok {
		t.Errorf("expected framework envelope, got keys %v", keysOf(raw))
	}
	if _, ok := raw["agentConfig"]; ok {
		t.Error("framework envelope must not carry agentConfig")
	}
}

// staticToken is a TokenProvider that always yields the same JWT.
type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

func TestDeploySendsFrameworkEnvelopeAndDecodes(t *testing.T) {
	var req deployRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != pathDeploy {
			t.Errorf("got %s %s, want POST %s", r.Method, r.URL.Path, pathDeploy)
		}
		if got := r.Header.Get("X-Authorization"); got != "jwt-123" {
			t.Errorf("X-Authorization = %q, want %q", got, "jwt-123")
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, _ = w.Write([]byte(`{"agentName":"my-skill","requiredWorkers":["my-skill__run"]}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(transport.Config{BaseURL: srv.URL, Tokens: staticToken("jwt-123")})

	res, err := c.Deploy(context.Background(), "skill", json.RawMessage(`{"skillMd":"hi"}`))
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if req.Framework != "skill" || string(req.RawConfig) != `{"skillMd":"hi"}` {
		t.Errorf("request = %+v", req)
	}
	if res.AgentName != "my-skill" || len(res.RequiredWorkers) != 1 || res.RequiredWorkers[0] != "my-skill__run" {
		t.Errorf("result = %+v", res)
	}
}

func TestDeployNormalizesErrorResponse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad skill"}`))
	})
	_, err := c.Deploy(context.Background(), "skill", json.RawMessage(`{}`))
	apiErr, ok := err.(*transport.APIError)
	if !ok {
		t.Fatalf("error = %T, want *transport.APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest || apiErr.Message != "bad skill" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestSearchExecutionsSetsQuery(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get(queryAgentName) != "greeter" || q.Get(queryStatus) != "FAILED" {
			t.Errorf("query = %v", q)
		}
		if q.Get(querySort) != sortExecutions {
			t.Errorf("sort = %q, want %q", q.Get(querySort), sortExecutions)
		}
		_, _ = w.Write([]byte(`{"totalHits":1,"results":[{"executionId":"e1","status":"FAILED"}]}`))
	})
	page, err := c.SearchExecutions(context.Background(), ExecutionFilter{
		AgentName: "greeter", Status: "FAILED", Size: 10,
	})
	if err != nil {
		t.Fatalf("SearchExecutions: %v", err)
	}
	if page.TotalHits != 1 || len(page.Results) != 1 || page.Results[0].ExecutionID != "e1" {
		t.Errorf("page = %+v", page)
	}
}

func TestPruneReturnsDeletedCount(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathPrune {
			t.Errorf("path = %q, want %q", r.URL.Path, pathPrune)
		}
		if got := r.URL.Query().Get(queryOlderThanDays); got != "7" {
			t.Errorf("olderThanDays = %q, want 7", got)
		}
		_, _ = w.Write([]byte(`{"deleted":5}`))
	})
	res, err := c.Prune(context.Background(), PruneRequest{OlderThanDays: 7})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Removed != 5 {
		t.Errorf("removed = %d, want 5", res.Removed)
	}
}

func TestGetReturnsRawPassthrough(t *testing.T) {
	const payload = `{"name":"greeter","_framework":"openai"}`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathAgents+"/greeter" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(payload))
	})
	out, err := c.Get(context.Background(), "greeter", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(out) != payload {
		t.Errorf("got %s, want %s", out, payload)
	}
}

func TestListReportsUnsupportedAgentsAPI(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	_, err := c.List(context.Background())
	if err == nil || err.Error() != unsupportedAPIMessage {
		t.Fatalf("error = %v, want %q", err, unsupportedAPIMessage)
	}
}

func TestGetReportsUnsupportedAgentsAPIWhenProbeIsAlsoMissing(t *testing.T) {
	var paths []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		http.NotFound(w, r)
	})

	_, err := c.Get(context.Background(), "greeter", nil)
	if err == nil || err.Error() != unsupportedAPIMessage {
		t.Fatalf("error = %v, want %q", err, unsupportedAPIMessage)
	}
	if len(paths) != 2 || paths[0] != pathAgents+"/greeter" || paths[1] != pathList {
		t.Fatalf("request paths = %v, want resource request followed by capability probe", paths)
	}
}

func TestGetPreservesResourceNotFoundWhenAgentsAPIExists(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathList {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"agent not found"}`))
	})

	_, err := c.Get(context.Background(), "missing", nil)
	var apiErr *transport.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *transport.APIError", err)
	}
	if apiErr.Message != "agent not found" {
		t.Fatalf("message = %q, want agent not found", apiErr.Message)
	}
}

func TestStreamReadsSSE(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(headerAccept); got != mimeEventStream {
			t.Errorf("Accept = %q, want %q", got, mimeEventStream)
		}
		w.Header().Set("Content-Type", mimeEventStream)
		_, _ = w.Write([]byte("event: message\ndata: {\"content\":\"hi\"}\n\nevent: done\ndata: {}\n\n"))
	})

	events, errc := c.Stream(context.Background(), "exec-1", "")
	var got []SSEEvent
	for e := range events {
		got = append(got, e)
	}
	if err := <-errc; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if len(got) != 2 || got[0].Type != EventMessage || got[1].Type != EventDone {
		t.Fatalf("got %+v", got)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
