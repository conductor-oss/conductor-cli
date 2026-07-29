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
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func stdioTask() Task {
	return Task{
		ID:         "task-1",
		WorkflowID: "wf-1",
		Type:       "greet",
		Raw:        json.RawMessage(`{"taskId":"task-1","taskType":"greet","inputData":{"name":"Miguel"}}`),
	}
}

// shWorker builds options that run a shell snippet as the worker program.
func shWorker(script string) StdioOptions {
	return StdioOptions{Command: "sh", Args: []string{"-c", script}}
}

func TestStdioHandlerCompletedResult(t *testing.T) {
	h := NewStdioHandler(shWorker(`echo '{"status":"COMPLETED","output":{"message":"hi"},"logs":["did it"]}'`))

	got := h.Handle(context.Background(), stdioTask())

	if got.Status != StatusCompleted {
		t.Errorf("Status = %q, want COMPLETED", got.Status)
	}
	if got.Output["message"] != "hi" {
		t.Errorf("Output = %v, want message=hi", got.Output)
	}
	if len(got.Logs) != 1 || got.Logs[0] != "did it" {
		t.Errorf("Logs = %v, want [did it]", got.Logs)
	}
}

func TestStdioHandlerReceivesFullTaskOnStdin(t *testing.T) {
	// The worker echoes back what it read, so the test can assert the whole task —
	// not just inputData — arrived on stdin.
	h := NewStdioHandler(shWorker(`payload=$(cat); echo "{\"status\":\"COMPLETED\",\"output\":{\"echoed\":$payload}}"`))

	got := h.Handle(context.Background(), stdioTask())

	if got.Status != StatusCompleted {
		t.Fatalf("Status = %q, want COMPLETED", got.Status)
	}
	echoed, ok := got.Output["echoed"].(map[string]interface{})
	if !ok {
		t.Fatalf("Output[echoed] = %#v, want an object", got.Output["echoed"])
	}
	for _, key := range []string{"taskId", "taskType", "inputData"} {
		if _, present := echoed[key]; !present {
			t.Errorf("stdin payload missing %q — the whole task must be written, not just inputData", key)
		}
	}
}

func TestStdioHandlerExportsTaskMetadataToChild(t *testing.T) {
	h := NewStdioHandler(shWorker(
		`echo "{\"status\":\"COMPLETED\",\"output\":{\"type\":\"$TASK_TYPE\",\"id\":\"$TASK_ID\",\"wf\":\"$WORKFLOW_ID\",\"exec\":\"$EXECUTION_ID\"}}"`))

	got := h.Handle(context.Background(), stdioTask())

	want := map[string]string{"type": "greet", "id": "task-1", "wf": "wf-1", "exec": "wf-1"}
	for k, v := range want {
		if got.Output[k] != v {
			t.Errorf("child env produced %s=%v, want %q", k, got.Output[k], v)
		}
	}
}

func TestStdioHandlerExportsDomainAndInjectedEnv(t *testing.T) {
	opts := shWorker(`echo "{\"status\":\"COMPLETED\",\"output\":{\"domain\":\"$POLL_DOMAIN\",\"injected\":\"$CONDUCTOR_AUTH_TOKEN\"}}"`)
	opts.Domain = "prod"
	opts.Env = []string{"CONDUCTOR_AUTH_TOKEN=tok-123"}
	h := NewStdioHandler(opts)

	got := h.Handle(context.Background(), stdioTask())

	if got.Output["domain"] != "prod" {
		t.Errorf("POLL_DOMAIN = %v, want prod", got.Output["domain"])
	}
	if got.Output["injected"] != "tok-123" {
		t.Errorf("injected env did not reach the child: %v", got.Output["injected"])
	}
}

func TestStdioHandlerNonZeroExitFails(t *testing.T) {
	h := NewStdioHandler(shWorker(`echo "something broke" >&2; exit 3`))

	got := h.Handle(context.Background(), stdioTask())

	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	if !strings.Contains(got.Reason, "worker execution failed") {
		t.Errorf("Reason = %q, want it to mention worker execution failed", got.Reason)
	}
	if len(got.Logs) != 1 || !strings.Contains(got.Logs[0], "something broke") {
		t.Errorf("Logs = %v, want stderr captured", got.Logs)
	}
}

func TestStdioHandlerMalformedJSONFails(t *testing.T) {
	h := NewStdioHandler(shWorker(`echo 'this is not json'`))

	got := h.Handle(context.Background(), stdioTask())

	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	if !strings.Contains(got.Reason, "invalid worker stdout JSON") {
		t.Errorf("Reason = %q, want it to mention invalid worker stdout JSON", got.Reason)
	}
	if len(got.Logs) != 1 || !strings.Contains(got.Logs[0], "this is not json") {
		t.Errorf("Logs = %v, want the raw stdout captured for debugging", got.Logs)
	}
}

func TestStdioHandlerExecTimeoutKillsChild(t *testing.T) {
	opts := StdioOptions{Command: "sleep", Args: []string{"30"}}
	opts.ExecTimeout = 100 * time.Millisecond
	h := NewStdioHandler(opts)

	start := time.Now()
	got := h.Handle(context.Background(), stdioTask())
	elapsed := time.Since(start)

	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %v — the exec timeout did not kill the child", elapsed)
	}
}

func TestStdioHandlerCancelledContextStopsChild(t *testing.T) {
	h := NewStdioHandler(StdioOptions{Command: "sleep", Args: []string{"30"}})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	got := h.Handle(ctx, stdioTask())

	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v — cancelling the context did not stop the child", elapsed)
	}
}

// TestStdioHandlerExecTimeoutDoesNotReapGrandchildren documents a limitation carried over
// from the pre-convergence implementation: the timeout signals only the direct child, and
// Run then waits for the captured stdout/stderr pipes to close. A worker that forks a
// long-running subprocess keeps those pipes open, so the effective timeout is the
// grandchild's lifetime, not ExecTimeout.
//
// Bounding a whole process tree needs process groups (setpgid plus kill on the negative
// pgid) and is platform-specific, so it is deliberately not part of the convergence work.
// The test is skipped by default because asserting it means waiting out the grandchild.
func TestStdioHandlerExecTimeoutDoesNotReapGrandchildren(t *testing.T) {
	t.Skip("documents a known pre-existing limitation; unskip to observe it (takes ~3s)")

	// A compound script forks a real grandchild rather than exec-ing into it.
	opts := shWorker(`sleep 3; echo '{"status":"COMPLETED"}'`)
	opts.ExecTimeout = 100 * time.Millisecond
	h := NewStdioHandler(opts)

	start := time.Now()
	h.Handle(context.Background(), stdioTask())

	if elapsed := time.Since(start); elapsed < time.Second {
		t.Errorf("returned in %v — grandchildren are now reaped, so this limitation is fixed "+
			"and both this test and the comment on it should be removed", elapsed)
	}
}

// TestStdioHandlerUnknownStatusIsNormalized documents that stdio workers, unlike
// JavaScript workers, do not get to invent statuses.
func TestStdioHandlerUnknownStatusIsNormalized(t *testing.T) {
	h := NewStdioHandler(shWorker(`echo '{"status":"BANANA"}'`))

	got := h.Handle(context.Background(), stdioTask())

	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	if !strings.Contains(got.Reason, "invalid status from worker") {
		t.Errorf("Reason = %q, want it to mention an invalid status", got.Reason)
	}
}

// TestNormalizeStdioResultPreservesStatuses covers the three statuses that pass through
// untouched, plus the pre-existing quirk in the rejection message.
func TestNormalizeStdioResultPreservesStatuses(t *testing.T) {
	for _, status := range []Status{StatusCompleted, StatusFailed, StatusInProgress} {
		t.Run(string(status), func(t *testing.T) {
			got := normalizeStdioResult(stdioResult{Status: string(status), Reason: "as-is"})
			if got.Status != status {
				t.Errorf("Status = %q, want %q", got.Status, status)
			}
			if got.Reason != "as-is" {
				t.Errorf("Reason = %q, want it left alone", got.Reason)
			}
		})
	}

	t.Run("unknown status message quirk is preserved", func(t *testing.T) {
		got := normalizeStdioResult(stdioResult{Status: "WEIRD"})
		// Faithful to the pre-convergence behaviour: the message names FAILED rather
		// than the offending status, because Status is overwritten first.
		if got.Reason != "invalid status from worker: FAILED" {
			t.Errorf("Reason = %q, want the pre-existing wording preserved", got.Reason)
		}
	})

	t.Run("empty status is rejected", func(t *testing.T) {
		got := normalizeStdioResult(stdioResult{})
		if got.Status != StatusFailed {
			t.Errorf("Status = %q, want FAILED for an empty status", got.Status)
		}
	})
}

// TestStdioHandlerConcurrentUse guards the Handler concurrency contract: one handler is
// shared across all goroutines of a batch poll.
func TestStdioHandlerConcurrentUse(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	h := NewStdioHandler(shWorker(`echo "{\"status\":\"COMPLETED\",\"output\":{\"id\":\"$TASK_ID\"}}"`))

	const n = 8
	var wg sync.WaitGroup
	results := make([]Result, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task := stdioTask()
			task.ID = string(rune('a' + i))
			results[i] = h.Handle(context.Background(), task)
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		if got.Status != StatusCompleted {
			t.Errorf("result %d status = %q, want COMPLETED", i, got.Status)
		}
		if want := string(rune('a' + i)); got.Output["id"] != want {
			t.Errorf("result %d id = %v, want %q — concurrent Handle calls crossed state", i, got.Output["id"], want)
		}
	}
}
