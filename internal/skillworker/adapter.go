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

	"github.com/conductor-oss/conductor-cli/internal/taskworker"
)

// WorkerID identifies the CLI in the task results it reports for skill tools.
const WorkerID = "conductor-cli"

// PollTimeoutMs is the server-side long-poll wait used for skill tool tasks.
const PollTimeoutMs = 100

// RunnerOptions returns the poll settings for skill tool workers. Skill tools take one
// task at a time and always identify as the CLI, unlike the worker commands where the
// user chooses a worker id.
func RunnerOptions() taskworker.RunnerOptions {
	return taskworker.RunnerOptions{
		WorkerID:      WorkerID,
		Count:         1,
		PollTimeoutMs: PollTimeoutMs,
	}
}

// AsTaskHandler adapts a ToolHandler onto the shared worker loop.
//
// ToolHandler stays the vocabulary of this package — tool logic in, raw JSON out — and
// this adapter supplies what the loop needs around it: the task's inputData rather than
// the whole task, the {"result": …} envelope the skill agent expects, and the mapping
// from a handler error to a failed task.
func AsTaskHandler(h ToolHandler) taskworker.Handler {
	return taskworker.HandlerFunc(func(ctx context.Context, t taskworker.Task) taskworker.Result {
		input, err := t.InputData()
		if err != nil {
			return taskworker.Failure(err.Error())
		}

		output, err := h.Handle(ctx, input)
		if err != nil {
			return taskworker.Failure(err.Error())
		}

		return taskworker.Result{
			Status: taskworker.StatusCompleted,
			Output: wrapResult(output),
		}
	})
}

// wrapResult wraps a handler's raw output under the "result" key the skill agent expects.
// Decoding to a generic value happens only here, at the seam. Output that is not valid
// JSON is carried through as a string rather than failing the task.
func wrapResult(output json.RawMessage) map[string]interface{} {
	var v interface{}
	if len(output) > 0 {
		if err := json.Unmarshal(output, &v); err != nil {
			v = string(output)
		}
	}
	return map[string]interface{}{outputKeyResult: v}
}
