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
	"encoding/json"
	"testing"

	"github.com/conductor-sdk/conductor-go/sdk/model"
)

// TestToTaskResultPreservesFlavourShapes pins the three result shapes that existed before
// the worker flavours were converged onto one loop. Workflows can observe all three, so a
// change here is a breaking change rather than a refactor.
func TestToTaskResultPreservesFlavourShapes(t *testing.T) {
	polled := Task{ID: "t1", WorkflowID: "wf1", Type: "greet", WorkerID: "worker-from-task"}

	tests := []struct {
		name   string
		result Result
		opts   RunnerOptions
		want   model.TaskResult
	}{
		{
			// worker js: reports the polled task's own worker id, and a failure carries
			// the message under an "error" output key with no ReasonForIncompletion.
			name:   "js failure shape",
			result: Result{Status: StatusFailed, Output: map[string]interface{}{"error": "script blew up"}},
			opts:   RunnerOptions{UseTaskWorkerID: true},
			want: model.TaskResult{
				TaskId:             "t1",
				WorkflowInstanceId: "wf1",
				WorkerId:           "worker-from-task",
				Status:             model.FailedTask,
				OutputData:         map[string]interface{}{"error": "script blew up"},
			},
		},
		{
			// worker stdio: uses the configured --worker-id, and a failure carries
			// ReasonForIncompletion plus logs and no "error" key.
			name:   "stdio failure shape",
			result: Result{Status: StatusFailed, Reason: "exit 1", Logs: []string{"stderr line"}},
			opts:   RunnerOptions{WorkerID: "my-worker"},
			want: model.TaskResult{
				TaskId:                "t1",
				WorkflowInstanceId:    "wf1",
				WorkerId:              "my-worker",
				Status:                model.FailedTask,
				ReasonForIncompletion: "exit 1",
				Logs:                  []model.TaskExecLog{{Log: "stderr line"}},
			},
		},
		{
			// skill tools: constant worker id, output wrapped under "result".
			name:   "skill success shape",
			result: Result{Status: StatusCompleted, Output: map[string]interface{}{"result": "Hello\n"}},
			opts:   RunnerOptions{WorkerID: "conductor-cli"},
			want: model.TaskResult{
				TaskId:             "t1",
				WorkflowInstanceId: "wf1",
				WorkerId:           "conductor-cli",
				Status:             model.CompletedTask,
				OutputData:         map[string]interface{}{"result": "Hello\n"},
			},
		},
		{
			// An unset --worker-id must leave WorkerId empty rather than sending "".
			name:   "no worker id configured",
			result: Result{Status: StatusCompleted},
			opts:   RunnerOptions{},
			want: model.TaskResult{
				TaskId:             "t1",
				WorkflowInstanceId: "wf1",
				Status:             model.CompletedTask,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToTaskResult(polled, tt.result, tt.opts)
			assertTaskResultEqual(t, got, tt.want)
		})
	}
}

// TestToTaskResultPassesStatusThrough covers the open Status type: worker js forwards
// whatever status its script returned, including values the CLI does not model.
func TestToTaskResultPassesStatusThrough(t *testing.T) {
	tests := []Status{
		StatusCompleted,
		StatusFailed,
		StatusInProgress,
		StatusFailedWithTerminalError,
		Status("SOMETHING_THE_CLI_DOES_NOT_KNOW"),
	}

	for _, status := range tests {
		t.Run(string(status), func(t *testing.T) {
			got := ToTaskResult(Task{ID: "t1"}, Result{Status: status}, RunnerOptions{})
			if string(got.Status) != string(status) {
				t.Errorf("Status = %q, want %q — status must pass through unchanged", got.Status, status)
			}
		})
	}
}

func TestToTaskResultOmitsEmptyLogsAndReason(t *testing.T) {
	got := ToTaskResult(Task{ID: "t1"}, Result{Status: StatusCompleted}, RunnerOptions{})
	if got.Logs != nil {
		t.Errorf("Logs = %v, want nil", got.Logs)
	}
	if got.ReasonForIncompletion != "" {
		t.Errorf("ReasonForIncompletion = %q, want empty", got.ReasonForIncompletion)
	}
}

func TestTaskFromModelCarriesFullTaskAsRaw(t *testing.T) {
	polled := taskFromModel(model.Task{
		TaskId:             "t1",
		WorkflowInstanceId: "wf1",
		TaskType:           "greet",
		WorkerId:           "w1",
		InputData:          map[string]interface{}{"name": "Miguel"},
	})

	if polled.Err != nil {
		t.Fatalf("conversion error = %v", polled.Err)
	}
	if polled.Task.ID != "t1" || polled.Task.WorkflowID != "wf1" || polled.Task.Type != "greet" {
		t.Errorf("identity fields wrong: %+v", polled.Task)
	}
	if polled.Task.WorkerID != "w1" {
		t.Errorf("WorkerID = %q, want w1 — worker js reports this back", polled.Task.WorkerID)
	}

	// Raw must be the whole task, since goja exposes it as $.task and stdio writes it to
	// the child's stdin. Both would break if it were only inputData.
	var raw map[string]interface{}
	if err := json.Unmarshal(polled.Task.Raw, &raw); err != nil {
		t.Fatalf("Raw is not valid JSON: %v", err)
	}
	for _, key := range []string{"taskId", "workflowInstanceId", "taskType", "inputData"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("Raw is missing %q — it must be the full task, not just inputData", key)
		}
	}

	input, err := polled.Task.InputData()
	if err != nil {
		t.Fatalf("InputData() error = %v", err)
	}
	if string(input) != `{"name":"Miguel"}` {
		t.Errorf("InputData() = %s", input)
	}
}

func assertTaskResultEqual(t *testing.T, got *model.TaskResult, want model.TaskResult) {
	t.Helper()
	if got.TaskId != want.TaskId {
		t.Errorf("TaskId = %q, want %q", got.TaskId, want.TaskId)
	}
	if got.WorkflowInstanceId != want.WorkflowInstanceId {
		t.Errorf("WorkflowInstanceId = %q, want %q", got.WorkflowInstanceId, want.WorkflowInstanceId)
	}
	if got.WorkerId != want.WorkerId {
		t.Errorf("WorkerId = %q, want %q", got.WorkerId, want.WorkerId)
	}
	if got.Status != want.Status {
		t.Errorf("Status = %q, want %q", got.Status, want.Status)
	}
	if got.ReasonForIncompletion != want.ReasonForIncompletion {
		t.Errorf("ReasonForIncompletion = %q, want %q", got.ReasonForIncompletion, want.ReasonForIncompletion)
	}
	if len(got.Logs) != len(want.Logs) {
		t.Fatalf("len(Logs) = %d, want %d", len(got.Logs), len(want.Logs))
	}
	for i := range want.Logs {
		if got.Logs[i].Log != want.Logs[i].Log {
			t.Errorf("Logs[%d] = %q, want %q", i, got.Logs[i].Log, want.Logs[i].Log)
		}
	}
	if len(got.OutputData) != len(want.OutputData) {
		t.Fatalf("len(OutputData) = %d, want %d", len(got.OutputData), len(want.OutputData))
	}
	for k, v := range want.OutputData {
		if got.OutputData[k] != v {
			t.Errorf("OutputData[%q] = %v, want %v", k, got.OutputData[k], v)
		}
	}
}
