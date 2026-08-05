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
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRunner is a scripted Runner. Each Poll returns the next entry from batches, then
// reports empty polls forever.
type fakeRunner struct {
	mu      sync.Mutex
	batches [][]PolledTask
	errs    []error
	polls   atomic.Int32
	updates []update
}

type update struct {
	task   Task
	result Result
}

func (f *fakeRunner) Poll(ctx context.Context, taskType string) ([]PolledTask, error) {
	n := int(f.polls.Add(1)) - 1

	f.mu.Lock()
	defer f.mu.Unlock()
	if n < len(f.errs) && f.errs[n] != nil {
		return nil, f.errs[n]
	}
	if n < len(f.batches) {
		return f.batches[n], nil
	}
	return nil, nil
}

func (f *fakeRunner) Update(ctx context.Context, t Task, r Result) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, update{task: t, result: r})
	return nil
}

func (f *fakeRunner) recorded() []update {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]update, len(f.updates))
	copy(out, f.updates)
	return out
}

func task(id string) Task {
	return Task{ID: id, WorkflowID: "wf-1", Type: "greet", Raw: json.RawMessage(`{"taskId":"` + id + `"}`)}
}

func okHandler() Handler {
	return HandlerFunc(func(ctx context.Context, t Task) Result {
		return Result{Status: StatusCompleted, Output: map[string]interface{}{"id": t.ID}}
	})
}

// runFor starts Run and cancels it once cancel() reports true, so tests never rely on
// wall-clock sleeps to decide the loop has done enough.
func runFor(t *testing.T, w *Worker, h Handler, until func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx, "greet", h)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for !until() {
		select {
		case <-deadline:
			cancel()
			t.Fatal("condition not reached within 2s")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

func TestRunDispatchesPolledTasks(t *testing.T) {
	r := &fakeRunner{batches: [][]PolledTask{{{Task: task("t1")}}}}
	w := NewWorker(r, Config{PollBackoff: time.Millisecond})

	runFor(t, w, okHandler(), func() bool { return len(r.recorded()) >= 1 })

	got := r.recorded()
	if got[0].task.ID != "t1" {
		t.Errorf("task ID = %q, want t1", got[0].task.ID)
	}
	if got[0].result.Status != StatusCompleted {
		t.Errorf("status = %q, want COMPLETED", got[0].result.Status)
	}
}

func TestRunBacksOffOnPollErrorWithoutSpinning(t *testing.T) {
	r := &fakeRunner{errs: []error{errors.New("boom"), errors.New("boom"), errors.New("boom")}}
	w := NewWorker(r, Config{PollBackoff: 50 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	w.Run(ctx, "greet", okHandler())

	// With a 50ms backoff over ~220ms, a correct loop polls a handful of times. A loop
	// without backoff would poll thousands of times.
	if polls := r.polls.Load(); polls > 10 {
		t.Errorf("polled %d times in 220ms with a 50ms backoff — loop is not backing off", polls)
	}
}

func TestRunBacksOffOnEmptyPollWithoutSpinning(t *testing.T) {
	r := &fakeRunner{} // always empty
	w := NewWorker(r, Config{PollBackoff: 50 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	w.Run(ctx, "greet", okHandler())

	if polls := r.polls.Load(); polls > 10 {
		t.Errorf("polled %d times in 220ms with a 50ms backoff — loop is not backing off", polls)
	}
}

func TestRunReturnsWhenContextCancelledMidBackoff(t *testing.T) {
	r := &fakeRunner{} // always empty, so the loop sits in backoff
	w := NewWorker(r, Config{PollBackoff: time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx, "greet", okHandler())
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let it reach the backoff sleep
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return while waiting in backoff — cancellation is not interrupting the sleep")
	}
}

func TestRunFailedTaskDoesNotStopLoop(t *testing.T) {
	r := &fakeRunner{batches: [][]PolledTask{
		{{Task: task("bad")}},
		{{Task: task("good")}},
	}}
	w := NewWorker(r, Config{PollBackoff: time.Millisecond})

	h := HandlerFunc(func(ctx context.Context, t Task) Result {
		if t.ID == "bad" {
			return Failure("handler said no")
		}
		return Result{Status: StatusCompleted}
	})

	runFor(t, w, h, func() bool { return len(r.recorded()) >= 2 })

	got := r.recorded()
	if got[0].result.Status != StatusFailed || got[0].result.Reason != "handler said no" {
		t.Errorf("first update = %+v, want FAILED with reason", got[0].result)
	}
	if got[1].result.Status != StatusCompleted {
		t.Errorf("second update = %+v, want COMPLETED — loop did not survive the failure", got[1].result)
	}
}

func TestRunHandlerPanicFailsOnlyThatTask(t *testing.T) {
	r := &fakeRunner{batches: [][]PolledTask{
		{{Task: task("panics")}},
		{{Task: task("fine")}},
	}}
	w := NewWorker(r, Config{PollBackoff: time.Millisecond})

	h := HandlerFunc(func(ctx context.Context, t Task) Result {
		if t.ID == "panics" {
			panic("kaboom")
		}
		return Result{Status: StatusCompleted}
	})

	runFor(t, w, h, func() bool { return len(r.recorded()) >= 2 })

	got := r.recorded()
	if got[0].result.Status != StatusFailed {
		t.Errorf("panicking task status = %q, want FAILED", got[0].result.Status)
	}
	if got[1].result.Status != StatusCompleted {
		t.Errorf("loop did not survive a handler panic")
	}
}

func TestRunConversionErrorFailsOnlyThatTaskAndPeersStillRun(t *testing.T) {
	r := &fakeRunner{batches: [][]PolledTask{{
		{Task: task("broken"), Err: errors.New("marshal task: nope")},
		{Task: task("peer")},
	}}}
	w := NewWorker(r, Config{PollBackoff: time.Millisecond})

	runFor(t, w, okHandler(), func() bool { return len(r.recorded()) >= 2 })

	byID := map[string]Result{}
	for _, u := range r.recorded() {
		byID[u.task.ID] = u.result
	}

	if got := byID["broken"]; got.Status != StatusFailed || got.Reason != "marshal task: nope" {
		t.Errorf("broken task = %+v, want FAILED carrying the conversion error", got)
	}
	if got := byID["peer"]; got.Status != StatusCompleted {
		t.Errorf("peer task = %+v, want COMPLETED — a bad task discarded its batch peers", got)
	}
}

func TestRunBatchExecutesConcurrently(t *testing.T) {
	const n = 4
	batch := make([]PolledTask, 0, n)
	for i := 0; i < n; i++ {
		batch = append(batch, PolledTask{Task: task(string(rune('a' + i)))})
	}
	r := &fakeRunner{batches: [][]PolledTask{batch}}
	w := NewWorker(r, Config{PollBackoff: time.Millisecond})

	// Every handler waits on the same barrier. If the batch ran serially the barrier
	// would never be satisfied and the test would time out.
	var wg sync.WaitGroup
	wg.Add(n)
	h := HandlerFunc(func(ctx context.Context, t Task) Result {
		wg.Done()
		wg.Wait()
		return Result{Status: StatusCompleted}
	})

	barrierMet := make(chan struct{})
	go func() { wg.Wait(); close(barrierMet) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx, "greet", h)

	select {
	case <-barrierMet:
	case <-time.After(2 * time.Second):
		t.Fatal("batch tasks did not run concurrently — all 4 never ran at once")
	}
}

func TestUpdateStillDeliversAfterContextCancelled(t *testing.T) {
	r := &fakeRunner{batches: [][]PolledTask{{{Task: task("t1")}}}}
	w := NewWorker(r, Config{PollBackoff: time.Millisecond})

	// The handler cancels the loop context before returning, mimicking Ctrl-C landing
	// while a task is in flight. The result must still be reported.
	ctx, cancel := context.WithCancel(context.Background())
	h := HandlerFunc(func(hctx context.Context, t Task) Result {
		cancel()
		return Result{Status: StatusCompleted}
	})

	done := make(chan struct{})
	go func() { w.Run(ctx, "greet", h); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}

	got := r.recorded()
	if len(got) != 1 {
		t.Fatalf("recorded %d updates, want 1 — a result finished during shutdown was dropped", len(got))
	}
	if got[0].result.Status != StatusCompleted {
		t.Errorf("status = %q, want COMPLETED", got[0].result.Status)
	}
}

func TestInputData(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "present", raw: `{"taskId":"t","inputData":{"name":"Miguel"}}`, want: `{"name":"Miguel"}`},
		{name: "absent yields null", raw: `{"taskId":"t"}`, want: `null`},
		{name: "explicit null", raw: `{"inputData":null}`, want: `null`},
		{name: "empty object", raw: `{"inputData":{}}`, want: `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Task{Raw: json.RawMessage(tt.raw)}.InputData()
			if err != nil {
				t.Fatalf("InputData() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("InputData() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestInputDataMalformedRawErrors(t *testing.T) {
	if _, err := (Task{Raw: json.RawMessage(`not json`)}).InputData(); err == nil {
		t.Error("InputData() on malformed Raw returned nil error")
	}
}
