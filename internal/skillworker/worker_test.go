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
	"sync"
	"testing"
	"time"
)

// handlerFunc adapts a function to ToolHandler.
type handlerFunc func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)

func (f handlerFunc) Handle(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	return f(ctx, input)
}

// fakeRunner is a TaskRunner that returns queued tasks then reports empty polls. It
// records terminal updates and can cancel the loop once the queue is drained.
type fakeRunner struct {
	mu           sync.Mutex
	queue        []Task
	pollCount    int
	completed    []completedCall
	failed       []failedCall
	stopAfterOne context.CancelFunc // cancel the loop after the first terminal update
}

type completedCall struct {
	task   Task
	output json.RawMessage
}

type failedCall struct {
	task   Task
	reason string
}

func (r *fakeRunner) Poll(ctx context.Context, taskType string) (Task, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pollCount++
	if len(r.queue) == 0 {
		return Task{}, false, nil
	}
	t := r.queue[0]
	r.queue = r.queue[1:]
	return t, true, nil
}

func (r *fakeRunner) Complete(ctx context.Context, t Task, output json.RawMessage) error {
	r.mu.Lock()
	r.completed = append(r.completed, completedCall{task: t, output: output})
	r.mu.Unlock()
	if r.stopAfterOne != nil {
		r.stopAfterOne()
	}
	return nil
}

func (r *fakeRunner) Fail(ctx context.Context, t Task, reason string) error {
	r.mu.Lock()
	r.failed = append(r.failed, failedCall{task: t, reason: reason})
	r.mu.Unlock()
	if r.stopAfterOne != nil {
		r.stopAfterOne()
	}
	return nil
}

func TestWorkerRunCompletesTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fr := &fakeRunner{
		queue:        []Task{{ID: "t1", WorkflowID: "w1", Input: json.RawMessage(`{"path":"a"}`)}},
		stopAfterOne: cancel,
	}
	var gotInput json.RawMessage
	h := handlerFunc(func(_ context.Context, in json.RawMessage) (json.RawMessage, error) {
		gotInput = in
		return json.RawMessage(`"file body"`), nil
	})

	NewWorker(fr).Run(ctx, "demo__read_skill_file", h)

	if string(gotInput) != `{"path":"a"}` {
		t.Errorf("handler input = %s", gotInput)
	}
	if len(fr.completed) != 1 || len(fr.failed) != 0 {
		t.Fatalf("completed=%d failed=%d", len(fr.completed), len(fr.failed))
	}
	got := fr.completed[0]
	if got.task.ID != "t1" || string(got.output) != `"file body"` {
		t.Errorf("completed call = %+v", got)
	}
}

func TestWorkerRunFailsTaskOnHandlerError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fr := &fakeRunner{
		queue:        []Task{{ID: "t2", WorkflowID: "w2"}},
		stopAfterOne: cancel,
	}
	h := handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("boom")
	})

	NewWorker(fr).Run(ctx, "demo__script", h)

	if len(fr.failed) != 1 || len(fr.completed) != 0 {
		t.Fatalf("completed=%d failed=%d", len(fr.completed), len(fr.failed))
	}
	if fr.failed[0].reason != "boom" || fr.failed[0].task.ID != "t2" {
		t.Errorf("fail call = %+v", fr.failed[0])
	}
}

func TestWorkerRunStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fr := &fakeRunner{} // always reports empty polls
	done := make(chan struct{})
	go func() {
		NewWorker(fr).Run(ctx, "demo__x", handlerFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return nil, nil
		}))
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

func TestTaskType(t *testing.T) {
	if got := TaskType("demo", "read_skill_file"); got != "demo__read_skill_file" {
		t.Errorf("TaskType = %q", got)
	}
}
