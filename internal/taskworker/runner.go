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
	"fmt"

	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
)

// RunnerOptions carries the poll and reporting settings that the worker commands expose
// as flags. Each poll field is applied only when set, because always sending a field —
// an empty Workerid, for instance — would change the outgoing request.
type RunnerOptions struct {
	WorkerID      string
	Domain        string
	Count         int32
	PollTimeoutMs int32
	// UseTaskWorkerID reports the polled task's own worker id on the result instead of
	// WorkerID. JavaScript workers behave this way and workflows can observe it.
	UseTaskWorkerID bool
}

// conductorRunner adapts Conductor's TaskResourceApiService to Runner. It is the ONLY
// place model.* and *client.TaskResourceApiService appear in this package — the loop and
// the handlers see only taskworker.Task and taskworker.Result.
type conductorRunner struct {
	client *client.TaskResourceApiService
	opts   RunnerOptions
}

// NewConductorRunner returns a Runner backed by the Conductor task client, which the cmd
// layer supplies via internal.GetTaskClient().
func NewConductorRunner(taskClient *client.TaskResourceApiService, opts RunnerOptions) Runner {
	return &conductorRunner{client: taskClient, opts: opts}
}

func (r *conductorRunner) Poll(ctx context.Context, taskType string) ([]PolledTask, error) {
	opts := &client.TaskResourceApiBatchPollOpts{}
	if r.opts.WorkerID != "" {
		opts.Workerid = optional.NewString(r.opts.WorkerID)
	}
	if r.opts.Domain != "" {
		opts.Domain = optional.NewString(r.opts.Domain)
	}
	if r.opts.Count > 0 {
		opts.Count = optional.NewInt32(r.opts.Count)
	}
	if r.opts.PollTimeoutMs > 0 {
		opts.Timeout = optional.NewInt32(r.opts.PollTimeoutMs)
	}

	tasks, _, err := r.client.BatchPoll(ctx, taskType, opts)
	if err != nil {
		return nil, err
	}

	polled := make([]PolledTask, 0, len(tasks))
	for _, t := range tasks {
		polled = append(polled, taskFromModel(t))
	}
	return polled, nil
}

func (r *conductorRunner) Update(ctx context.Context, t Task, res Result) error {
	_, _, err := r.client.UpdateTask(ctx, ToTaskResult(t, res, r.opts))
	return err
}

// taskFromModel converts one SDK task, carrying any conversion failure on the task
// itself so the loop can fail it individually instead of dropping its batch peers.
func taskFromModel(t model.Task) PolledTask {
	task := Task{
		ID:         t.TaskId,
		WorkflowID: t.WorkflowInstanceId,
		Type:       t.TaskType,
		WorkerID:   t.WorkerId,
	}

	raw, err := json.Marshal(t)
	if err != nil {
		return PolledTask{Task: task, Err: fmt.Errorf("marshal task: %w", err)}
	}
	task.Raw = raw
	return PolledTask{Task: task}
}

// ToTaskResult maps a Result onto the SDK's TaskResult. It is a pure function so the
// mapping — the part most likely to drift during a refactor — can be table-tested
// without a live client.
func ToTaskResult(t Task, r Result, opts RunnerOptions) *model.TaskResult {
	result := &model.TaskResult{
		TaskId:             t.ID,
		WorkflowInstanceId: t.WorkflowID,
		Status:             model.TaskResultStatus(r.Status),
		OutputData:         r.Output,
	}

	if opts.UseTaskWorkerID {
		result.WorkerId = t.WorkerID
	} else if opts.WorkerID != "" {
		result.WorkerId = opts.WorkerID
	}

	if r.Reason != "" {
		result.ReasonForIncompletion = r.Reason
	}

	if len(r.Logs) > 0 {
		logs := make([]model.TaskExecLog, len(r.Logs))
		for i, line := range r.Logs {
			logs[i] = model.TaskExecLog{Log: line}
		}
		result.Logs = logs
	}

	return result
}
