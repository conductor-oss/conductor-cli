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

package skillworker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal/taskworker"
)

// stubHandler returns a fixed output or error, standing in for a real tool.
type stubHandler struct {
	output json.RawMessage
	err    error
	gotIn  json.RawMessage
}

func (s *stubHandler) Handle(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	s.gotIn = input
	return s.output, s.err
}

func skillTask(raw string) taskworker.Task {
	return taskworker.Task{ID: "t1", WorkflowID: "wf1", Raw: json.RawMessage(raw)}
}

// TestAsTaskHandlerWrapsUnderResultKey pins the skill agent's output contract. It is the
// only contract of the three with no documentation outside the code, so this test is the
// specification.
func TestAsTaskHandlerWrapsUnderResultKey(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   interface{}
	}{
		{name: "json string", output: `"Hello\n"`, want: "Hello\n"},
		{name: "json number", output: `42`, want: float64(42)},
		{name: "non-json carried through as a string", output: `not json at all`, want: "not json at all"},
		{name: "empty output becomes nil", output: ``, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := AsTaskHandler(&stubHandler{output: json.RawMessage(tt.output)})
			got := h.Handle(context.Background(), skillTask(`{"inputData":{}}`))

			if got.Status != taskworker.StatusCompleted {
				t.Errorf("Status = %q, want COMPLETED", got.Status)
			}
			if len(got.Output) != 1 {
				t.Fatalf("Output = %v, want exactly one key", got.Output)
			}
			if got.Output["result"] != tt.want {
				t.Errorf("Output[result] = %#v, want %#v", got.Output["result"], tt.want)
			}
		})
	}
}

// TestAsTaskHandlerNilOutputYieldsNullResult pins that the key is always present, holding
// null, rather than the output map being empty.
func TestAsTaskHandlerNilOutputYieldsNullResult(t *testing.T) {
	h := AsTaskHandler(&stubHandler{output: nil})
	got := h.Handle(context.Background(), skillTask(`{"inputData":{}}`))

	value, present := got.Output["result"]
	if !present {
		t.Fatal(`Output has no "result" key; it must be present even when the output is empty`)
	}
	if value != nil {
		t.Errorf("Output[result] = %#v, want nil", value)
	}
}

// TestAsTaskHandlerObjectOutputIsDecoded checks that structured output survives as
// structure rather than being stringified.
func TestAsTaskHandlerObjectOutputIsDecoded(t *testing.T) {
	h := AsTaskHandler(&stubHandler{output: json.RawMessage(`{"files":["a.go"],"count":1}`)})
	got := h.Handle(context.Background(), skillTask(`{"inputData":{}}`))

	inner, ok := got.Output["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Output[result] = %#v, want a decoded object", got.Output["result"])
	}
	if inner["count"] != float64(1) {
		t.Errorf("result.count = %#v, want 1", inner["count"])
	}
}

func TestAsTaskHandlerErrorFailsTheTask(t *testing.T) {
	h := AsTaskHandler(&stubHandler{err: errors.New("tool exploded")})
	got := h.Handle(context.Background(), skillTask(`{"inputData":{}}`))

	if got.Status != taskworker.StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
	if got.Reason != "tool exploded" {
		t.Errorf("Reason = %q, want the handler error", got.Reason)
	}
	if got.Output != nil {
		t.Errorf("Output = %v, want nil for a failure", got.Output)
	}
}

// TestAsTaskHandlerPassesOnlyInputData pins that a tool receives inputData rather than the
// whole task, which is how the tool handlers have always been written.
func TestAsTaskHandlerPassesOnlyInputData(t *testing.T) {
	stub := &stubHandler{output: json.RawMessage(`"ok"`)}
	h := AsTaskHandler(stub)

	h.Handle(context.Background(), skillTask(`{"taskId":"t1","inputData":{"command":"greet"}}`))

	if string(stub.gotIn) != `{"command":"greet"}` {
		t.Errorf("handler received %s, want only inputData", stub.gotIn)
	}
}

func TestAsTaskHandlerMalformedTaskFails(t *testing.T) {
	h := AsTaskHandler(&stubHandler{output: json.RawMessage(`"ok"`)})
	got := h.Handle(context.Background(), skillTask(`not json`))

	if got.Status != taskworker.StatusFailed {
		t.Errorf("Status = %q, want FAILED", got.Status)
	}
}

// TestRunnerOptionsMatchSkillContract pins the poll settings the skill agent expects: one
// task at a time, identifying as the CLI.
func TestRunnerOptionsMatchSkillContract(t *testing.T) {
	opts := RunnerOptions()

	// Asserted against the literal too: the constant is the skill agent's wire contract,
	// so renaming its value is a breaking change rather than a rename.
	if opts.WorkerID != "conductor-cli" {
		t.Errorf("WorkerID = %q, want conductor-cli", opts.WorkerID)
	}
	if opts.Count != 1 {
		t.Errorf("Count = %d, want 1", opts.Count)
	}
	if opts.PollTimeoutMs != PollTimeoutMs {
		t.Errorf("PollTimeoutMs = %d, want %d", opts.PollTimeoutMs, PollTimeoutMs)
	}
	if opts.UseTaskWorkerID {
		t.Error("UseTaskWorkerID = true, want false — skill tools always report as conductor-cli")
	}
}

func TestTaskType(t *testing.T) {
	if got := TaskType("greetskill", "greet"); got != "greetskill__greet" {
		t.Errorf("TaskType() = %q, want greetskill__greet", got)
	}
}
