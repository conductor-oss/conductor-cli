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

package taskworker

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func gojaTask() Task {
	return Task{
		ID:         "task-1",
		WorkflowID: "wf-1",
		Type:       "greet",
		WorkerID:   "w-1",
		Raw:        json.RawMessage(`{"taskId":"task-1","taskType":"greet","inputData":{"name":"Miguel"},"pollCount":1}`),
	}
}

func handleScript(t *testing.T, script string) Result {
	t.Helper()
	h, err := NewGojaHandler(script, "test.js")
	if err != nil {
		t.Fatalf("NewGojaHandler() error = %v", err)
	}
	return h.Handle(context.Background(), gojaTask())
}

func TestGojaHandlerStatusAndBodyMapping(t *testing.T) {
	got := handleScript(t, `(function () {
		return { status: "COMPLETED", body: { message: "hi " + $.task.inputData.name } };
	})();`)

	if got.Status != StatusCompleted {
		t.Errorf("Status = %q, want COMPLETED", got.Status)
	}
	if got.Output["message"] != "hi Miguel" {
		t.Errorf("Output = %v, want message from $.task.inputData", got.Output)
	}
}

// TestGojaHandlerExposesWholeTask pins that $.task is the entire task, not just its
// input: WORKER_JS.md documents fields like pollCount and taskType on it.
func TestGojaHandlerExposesWholeTask(t *testing.T) {
	got := handleScript(t, `(function () {
		return { status: "COMPLETED", body: { type: $.task.taskType, polls: $.task.pollCount } };
	})();`)

	if got.Output["type"] != "greet" {
		t.Errorf("$.task.taskType = %v, want greet", got.Output["type"])
	}
	if got.Output["polls"] == nil {
		t.Error("$.task.pollCount missing — $.task must be the whole task")
	}
}

// TestGojaHandlerPassesUnknownStatusThrough is the behaviour that forced Status to be an
// open string type: FAILED_WITH_TERMINAL_ERROR is documented, and the CLI must not
// second-guess statuses the server understands.
func TestGojaHandlerPassesUnknownStatusThrough(t *testing.T) {
	for _, status := range []string{"FAILED_WITH_TERMINAL_ERROR", "IN_PROGRESS", "SOMETHING_NEW"} {
		t.Run(status, func(t *testing.T) {
			got := handleScript(t, `(function () { return { status: "`+status+`", body: {} }; })();`)
			if string(got.Status) != status {
				t.Errorf("Status = %q, want %q passed through unchanged", got.Status, status)
			}
		})
	}
}

// TestGojaHandlerScriptErrorUsesErrorOutputKey pins the JavaScript failure shape. It
// differs from the stdio one, and a workflow reading ${task.output.error} depends on it.
func TestGojaHandlerScriptErrorUsesErrorOutputKey(t *testing.T) {
	got := handleScript(t, `(function () { throw new Error("boom"); })();`)

	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	msg, ok := got.Output["error"].(string)
	if !ok {
		t.Fatalf(`Output["error"] = %#v, want a string — js failures report under "error"`, got.Output["error"])
	}
	if !strings.Contains(msg, "boom") {
		t.Errorf(`Output["error"] = %q, want it to contain the script error`, msg)
	}
	if got.Reason != "" {
		t.Errorf("Reason = %q, want empty — js workers do not set ReasonForIncompletion", got.Reason)
	}
}

func TestGojaHandlerCompileErrorIsReportedAtConstruction(t *testing.T) {
	if _, err := NewGojaHandler(`function ( {{{ bad syntax`, "bad.js"); err == nil {
		t.Error("NewGojaHandler() on invalid JavaScript returned nil error")
	}
}

func TestGojaHandlerResultShapes(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		wantStatus Status
		check      func(*testing.T, Result)
	}{
		{
			name:       "no return value completes with empty output",
			script:     `(function () { var x = 1; })();`,
			wantStatus: StatusCompleted,
			check: func(t *testing.T, got Result) {
				if len(got.Output) != 0 {
					t.Errorf("Output = %v, want empty", got.Output)
				}
			},
		},
		{
			name:       "null completes with empty output",
			script:     `null;`,
			wantStatus: StatusCompleted,
			check: func(t *testing.T, got Result) {
				if len(got.Output) != 0 {
					t.Errorf("Output = %v, want empty", got.Output)
				}
			},
		},
		{
			name:       "object without a status completes and carries the value",
			script:     `({ greeting: "hello" });`,
			wantStatus: StatusCompleted,
			check: func(t *testing.T, got Result) {
				if got.Output["result"] == nil {
					t.Errorf(`Output = %v, want the value under "result"`, got.Output)
				}
			},
		},
		{
			name:       "bare string completes and carries the value",
			script:     `"just a string";`,
			wantStatus: StatusCompleted,
			check: func(t *testing.T, got Result) {
				if got.Output["result"] != "just a string" {
					t.Errorf(`Output["result"] = %v, want the string`, got.Output["result"])
				}
			},
		},
		{
			name:       "status with no body yields an empty output map",
			script:     `({ status: "COMPLETED" });`,
			wantStatus: StatusCompleted,
			check: func(t *testing.T, got Result) {
				if got.Output == nil {
					t.Error("Output = nil, want an empty map")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handleScript(t, tt.script)
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			tt.check(t, got)
		})
	}
}

// TestGojaHandlerConcurrentUse guards the Handler concurrency contract. The compiled
// program is shared, so each task must get its own Runtime — goja Runtimes are not safe
// for concurrent use, and a batch poll runs tasks in parallel.
func TestGojaHandlerConcurrentUse(t *testing.T) {
	h, err := NewGojaHandler(`(function () {
		return { status: "COMPLETED", body: { id: $.task.taskId } };
	})();`, "test.js")
	if err != nil {
		t.Fatalf("NewGojaHandler() error = %v", err)
	}

	const n = 16
	var wg sync.WaitGroup
	results := make([]Result, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i))
			results[i] = h.Handle(context.Background(), Task{
				ID:  id,
				Raw: json.RawMessage(`{"taskId":"` + id + `"}`),
			})
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		want := string(rune('a' + i))
		if got.Status != StatusCompleted {
			t.Errorf("result %d status = %q, want COMPLETED", i, got.Status)
		}
		if got.Output["id"] != want {
			t.Errorf("result %d id = %v, want %q — concurrent Handle calls shared a Runtime", i, got.Output["id"], want)
		}
	}
}

func TestGojaHandlerMalformedRawFails(t *testing.T) {
	h, err := NewGojaHandler(`({ status: "COMPLETED" });`, "test.js")
	if err != nil {
		t.Fatalf("NewGojaHandler() error = %v", err)
	}

	got := h.Handle(context.Background(), Task{ID: "t", Raw: json.RawMessage(`not json`)})
	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	if got.Output["error"] == nil {
		t.Error(`Output["error"] missing for an unparseable task`)
	}
}
