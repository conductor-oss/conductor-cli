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

package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

func TestDoJSONSetsAuthHeaderAndRoundTrips(t *testing.T) {
	var gotAuth, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get(authHeader)
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	c := Config{BaseURL: srv.URL, Tokens: staticToken("jwt-123")}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.DoJSON(context.Background(), http.MethodGet, "/x", nil, &out); err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if gotAuth != "jwt-123" {
		t.Errorf("X-Authorization = %q, want jwt-123", gotAuth)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if out.ID != "abc" {
		t.Errorf("id = %q, want abc", out.ID)
	}
}

func TestDoOmitsAuthHeaderWhenAnonymous(t *testing.T) {
	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header[authHeader]
	}))
	defer srv.Close()

	c := Config{BaseURL: srv.URL} // no Tokens => anonymous
	resp, err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
	if hadAuth {
		t.Error("expected no X-Authorization header for anonymous transport")
	}
}

func TestDoNormalizesServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":404,"message":"agent not found"}`))
	}))
	defer srv.Close()

	c := Config{BaseURL: srv.URL}
	_, err := c.Do(context.Background(), http.MethodGet, "/api/agent/x", nil, nil)
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusNotFound || apiErr.Message != "agent not found" {
		t.Errorf("got %+v, want status 404 / message 'agent not found'", apiErr)
	}
}
