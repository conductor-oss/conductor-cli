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
	"fmt"

	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
)

// Poll tuning: one task per poll, with a short server-side long-poll wait. The
// worker loop adds its own backoff on top (see pollBackoff).
const (
	pollBatchSize = 1   // tasks requested per poll
	pollTimeoutMs = 100 // server-side long-poll wait, milliseconds
)

// conductorRunner adapts Conductor's TaskResourceApiService to TaskRunner. It is the
// ONLY place model.* and *client.TaskResourceApiService appear in this package —
// the worker loop and the handlers see only skillworker.Task and json.RawMessage.
type conductorRunner struct {
	client *client.TaskResourceApiService
}

// NewConductorRunner returns a TaskRunner backed by the Conductor task client
// (supplied by the cmd layer via internal.GetTaskClient()).
func NewConductorRunner(taskClient *client.TaskResourceApiService) TaskRunner {
	return &conductorRunner{client: taskClient}
}

func (r *conductorRunner) Poll(ctx context.Context, taskType string) (Task, bool, error) {
	opts := &client.TaskResourceApiBatchPollOpts{
		Workerid: optional.NewString(workerID),
		Count:    optional.NewInt32(pollBatchSize),
		Timeout:  optional.NewInt32(pollTimeoutMs),
	}
	tasks, _, err := r.client.BatchPoll(ctx, taskType, opts)
	if err != nil {
		return Task{}, false, err
	}
	if len(tasks) == 0 {
		return Task{}, false, nil
	}
	return taskFromModel(tasks[0])
}

func (r *conductorRunner) Complete(ctx context.Context, t Task, output json.RawMessage) error {
	return r.update(ctx, t, model.CompletedTask, wrapResult(output), "")
}

func (r *conductorRunner) Fail(ctx context.Context, t Task, reason string) error {
	return r.update(ctx, t, model.FailedTask, nil, reason)
}

func (r *conductorRunner) update(ctx context.Context, t Task, status model.TaskResultStatus, output map[string]interface{}, reason string) error {
	result := &model.TaskResult{
		TaskId:             t.ID,
		WorkflowInstanceId: t.WorkflowID,
		WorkerId:           workerID,
		Status:             status,
		OutputData:         output,
	}
	if reason != "" {
		result.ReasonForIncompletion = reason
	}
	_, _, err := r.client.UpdateTask(ctx, result)
	return err
}

// taskFromModel marshals the SDK task's input map to bytes at the seam, so the
// map[string]interface{} never crosses into the worker loop or the handlers.
func taskFromModel(t model.Task) (Task, bool, error) {
	input, err := json.Marshal(t.InputData)
	if err != nil {
		return Task{}, false, fmt.Errorf("marshal task input: %w", err)
	}
	return Task{ID: t.TaskId, WorkflowID: t.WorkflowInstanceId, Input: input}, true, nil
}

// wrapResult wraps a handler's raw output under the "result" key the skill agent
// expects. Decoding to a generic value happens only here, at the seam.
func wrapResult(output json.RawMessage) map[string]interface{} {
	var v interface{}
	if len(output) > 0 {
		if err := json.Unmarshal(output, &v); err != nil {
			v = string(output)
		}
	}
	return map[string]interface{}{outputKeyResult: v}
}
