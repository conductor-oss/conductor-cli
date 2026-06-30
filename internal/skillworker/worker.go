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

// Package skillworker is the local tool-worker runtime for skill run/serve. When a
// skill agent runs on the server, it dispatches tool tasks (read_skill_file, each
// script, workspace tools) back to the CLI; this package polls for those tasks,
// runs them locally, and returns the result. It is layered: the poll→handle→update
// loop and its two interfaces live here, the Conductor SDK is confined to the
// runner bridge, and the concrete tool logic lives in the handlers (later stage).
package skillworker

import (
	"context"
	"encoding/json"
	"time"
)

// Worker protocol constants — fixed by the skill agent/server contract, so they are
// named constants, never inline literals.
const (
	taskTypeSep     = "__"            // task type is "{skillName}__{tool}"
	outputKeyResult = "result"        // handler output is wrapped as {result: <output>}
	workerID        = "conductor-cli" // identifies this worker in task results
)

// pollBackoff is the idle wait between polls that return no task or an error. The
// production runner also long-polls the server, so this is a hot-loop backstop.
const pollBackoff = 100 * time.Millisecond

// TaskType builds the "{skillName}__{tool}" task type dispatched for a skill tool.
func TaskType(skillName, tool string) string {
	return skillName + taskTypeSep + tool
}

// ToolHandler handles one dispatched tool task. IO is json.RawMessage, so no map
// crosses this boundary; the concrete tool logic lives in the handlers.
type ToolHandler interface {
	Handle(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// Task is one polled tool task, decoupled from the SDK's model.Task.
type Task struct {
	ID         string
	WorkflowID string
	Input      json.RawMessage
}

// TaskRunner is the poll/complete/fail seam. The production impl (runner.go) wraps
// Conductor's TaskResourceApiService; tests inject a fake. It keeps model.Task and
// *client.TaskResourceApiService out of the worker loop and the handlers.
type TaskRunner interface {
	// Poll returns the next task for taskType. ok=false means no task was available
	// (poll again); a non-nil err is a real polling failure.
	Poll(ctx context.Context, taskType string) (task Task, ok bool, err error)
	Complete(ctx context.Context, t Task, output json.RawMessage) error
	Fail(ctx context.Context, t Task, reason string) error
}

// Worker runs the poll→handle→update loop for a single task type over a TaskRunner.
type Worker struct {
	runner TaskRunner
}

// NewWorker returns a Worker backed by the given TaskRunner.
func NewWorker(runner TaskRunner) *Worker {
	return &Worker{runner: runner}
}

// Run polls taskType and dispatches each task to h until ctx is cancelled. Transient
// poll failures back off and retry rather than stop the loop; a handler error fails
// only that task. Run returns when ctx is done.
func (w *Worker) Run(ctx context.Context, taskType string, h ToolHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, ok, err := w.runner.Poll(ctx, taskType)
		if err != nil {
			if !sleep(ctx, pollBackoff) {
				return
			}
			continue
		}
		if !ok {
			if !sleep(ctx, pollBackoff) {
				return
			}
			continue
		}

		output, handleErr := h.Handle(ctx, task.Input)
		if handleErr != nil {
			_ = w.runner.Fail(ctx, task, handleErr.Error())
			continue
		}
		_ = w.runner.Complete(ctx, task, output)
	}
}

// sleep waits d or until ctx is cancelled; it returns false if ctx was cancelled,
// which keeps the poll loop responsive to Ctrl-C during idle waits.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
