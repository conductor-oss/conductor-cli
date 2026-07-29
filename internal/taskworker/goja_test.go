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

// Tests for the JavaScript worker runtime and the two on-the-wire result shapes. These
// moved here with injectUtilities and httpRequest when the worker flavours converged onto
// one poll loop; they previously lived in cmd/worker_test.go.
package taskworker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dop251/goja"
)

func TestStdioResultJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		status string
		hasOut bool
	}{
		{
			name:   "completed with output",
			input:  `{"status":"COMPLETED","output":{"key":"value"},"logs":["done"]}`,
			status: "COMPLETED",
			hasOut: true,
		},
		{
			name:   "failed with reason",
			input:  `{"status":"FAILED","reason":"timeout"}`,
			status: "FAILED",
			hasOut: false,
		},
		{
			name:   "in progress",
			input:  `{"status":"IN_PROGRESS"}`,
			status: "IN_PROGRESS",
			hasOut: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result stdioResult
			err := json.Unmarshal([]byte(tt.input), &result)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if result.Status != tt.status {
				t.Errorf("Status: got %q, want %q", result.Status, tt.status)
			}
			if tt.hasOut && result.Output == nil {
				t.Error("expected non-nil output")
			}
		})
	}
}

func TestGojaResultJSON(t *testing.T) {
	result := gojaResult{
		Status: "COMPLETED",
		Body: map[string]interface{}{
			"message": "hello",
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded gojaResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Status != "COMPLETED" {
		t.Errorf("Status: got %q, want %q", decoded.Status, "COMPLETED")
	}
	if decoded.Body["message"] != "hello" {
		t.Errorf("Body.message: got %v, want %q", decoded.Body["message"], "hello")
	}
}

func TestHttpRequest(t *testing.T) {
	t.Run("GET request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.Header.Get("X-Custom") != "test" {
				t.Errorf("missing custom header")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"result":"ok"}`))
		}))
		defer server.Close()

		result := httpRequest("GET", server.URL, map[string]interface{}{"X-Custom": "test"}, "")
		if result["status"] != http.StatusOK {
			t.Errorf("status: got %v, want %d", result["status"], http.StatusOK)
		}
		body, ok := result["body"].(map[string]interface{})
		if !ok {
			t.Fatal("expected body to be a map")
		}
		if body["result"] != "ok" {
			t.Errorf("body.result: got %v, want %q", body["result"], "ok")
		}
	})

	t.Run("POST request with body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.Write([]byte(`{"created":true}`))
		}))
		defer server.Close()

		result := httpRequest("POST", server.URL, nil, `{"name":"test"}`)
		if result["status"] != http.StatusOK {
			t.Errorf("status: got %v, want %d", result["status"], http.StatusOK)
		}
	})

	t.Run("non-JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("plain text response"))
		}))
		defer server.Close()

		result := httpRequest("GET", server.URL, nil, "")
		if result["text"] != "plain text response" {
			t.Errorf("text: got %v, want %q", result["text"], "plain text response")
		}
		if result["body"] != nil {
			t.Errorf("expected nil body for non-JSON, got %v", result["body"])
		}
	})

	t.Run("connection error", func(t *testing.T) {
		result := httpRequest("GET", "http://localhost:1", nil, "")
		if result["error"] == nil {
			t.Error("expected error for connection failure")
		}
		if result["status"] != 0 {
			t.Errorf("status: got %v, want 0", result["status"])
		}
	})
}

func TestInjectUtilitiesCrypto(t *testing.T) {
	vm := goja.New()
	injectUtilities(vm)

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"md5", `crypto.md5("hello")`, "5d41402abc4b2a76b9719d911017c592"},
		{"sha1", `crypto.sha1("hello")`, "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
		{"sha256", `crypto.sha256("hello")`, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{"base64Encode", `crypto.base64Encode("hello world")`, "aGVsbG8gd29ybGQ="},
		{"base64Decode", `crypto.base64Decode("aGVsbG8gd29ybGQ=")`, "hello world"},
		{"base64Decode invalid", `crypto.base64Decode("!!!invalid!!!")`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("script error: %v", err)
			}
			if val.String() != tt.want {
				t.Errorf("got %q, want %q", val.String(), tt.want)
			}
		})
	}
}

func TestInjectUtilitiesString(t *testing.T) {
	vm := goja.New()
	injectUtilities(vm)

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"toUpper", `str.toUpper("hello")`, "HELLO"},
		{"toLower", `str.toLower("WORLD")`, "world"},
		{"trim", `str.trim("  spaces  ")`, "spaces"},
		{"contains true", `str.contains("hello world", "world")`, "true"},
		{"contains false", `str.contains("hello", "xyz")`, "false"},
		{"hasPrefix", `str.hasPrefix("hello", "hel")`, "true"},
		{"hasSuffix", `str.hasSuffix("hello", "llo")`, "true"},
		{"replace", `str.replace("foo bar foo", "foo", "baz")`, "baz bar baz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("script error: %v", err)
			}
			if val.String() != tt.want {
				t.Errorf("got %q, want %q", val.String(), tt.want)
			}
		})
	}
}

func TestInjectUtilitiesSplit(t *testing.T) {
	vm := goja.New()
	injectUtilities(vm)

	val, err := vm.RunString(`JSON.stringify(str.split("a,b,c", ","))`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.String() != `["a","b","c"]` {
		t.Errorf("got %q, want %q", val.String(), `["a","b","c"]`)
	}
}

func TestInjectUtilitiesJoin(t *testing.T) {
	vm := goja.New()
	injectUtilities(vm)

	val, err := vm.RunString(`str.join(["a","b","c"], "-")`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.String() != "a-b-c" {
		t.Errorf("got %q, want %q", val.String(), "a-b-c")
	}
}

func TestInjectUtilitiesEnv(t *testing.T) {
	vm := goja.New()
	injectUtilities(vm)

	os.Setenv("TEST_CONDUCTOR_VAR", "test_value")
	defer os.Unsetenv("TEST_CONDUCTOR_VAR")

	val, err := vm.RunString(`util.env("TEST_CONDUCTOR_VAR")`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.String() != "test_value" {
		t.Errorf("got %q, want %q", val.String(), "test_value")
	}

	// Non-existent env var
	val, err = vm.RunString(`util.env("NONEXISTENT_VAR_12345")`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.String() != "" {
		t.Errorf("got %q, want empty string", val.String())
	}
}

func TestInjectUtilitiesUUID(t *testing.T) {
	vm := goja.New()
	injectUtilities(vm)

	val, err := vm.RunString(`util.uuid()`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	if val.String() == "" {
		t.Error("expected non-empty UUID")
	}
}

func TestInjectUtilitiesHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"method":"` + r.Method + `"}`))
	}))
	defer server.Close()

	vm := goja.New()
	injectUtilities(vm)

	// Test http.get
	val, err := vm.RunString(`JSON.stringify(http.get("` + server.URL + `", {}))`)
	if err != nil {
		t.Fatalf("script error: %v", err)
	}
	result := val.String()
	if result == "" {
		t.Error("expected non-empty result")
	}
}
