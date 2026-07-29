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

// Package taskworker is the single poll→execute→update loop shared by every worker
// flavour in the CLI: stdio workers, JavaScript (goja) workers, registry workers, and
// skill tool workers. It is layered — the loop and its two interfaces live here, the
// Conductor SDK is confined to the runner bridge, and each flavour's execution logic
// lives in a Handler.
//
// Before this package existed the loop was hand-copied four times in cmd/worker.go with
// no backoff, no context cancellation, and no tests.
package taskworker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// defaultPollBackoff is the idle wait between polls that return no task or an error.
// Runners also long-poll the server, so this is a hot-loop backstop rather than the
// primary pacing mechanism.
const defaultPollBackoff = 100 * time.Millisecond

// Status is the task status reported back to Conductor. It is deliberately an open
// string type rather than a closed enum: JavaScript workers forward whatever status
// their script returns straight through, and FAILED_WITH_TERMINAL_ERROR is documented
// and in use. Handlers that need to constrain the set normalise it themselves.
type Status string

const (
	StatusCompleted               Status = "COMPLETED"
	StatusFailed                  Status = "FAILED"
	StatusInProgress              Status = "IN_PROGRESS"
	StatusFailedWithTerminalError Status = "FAILED_WITH_TERMINAL_ERROR"
)

// Task is one polled task, decoupled from the SDK's model.Task.
type Task struct {
	ID         string
	WorkflowID string
	Type       string
	// WorkerID is the worker id carried by the polled task. JavaScript workers report
	// this value back on the result rather than the configured --worker-id.
	WorkerID string
	// Raw is json.Marshal(model.Task) — the whole task, not the raw HTTP body and not
	// just inputData. Stdio workers write it to the child's stdin and goja workers
	// expose it as $.task, so both depend on it being the SDK struct's marshalling.
	Raw json.RawMessage
}

// InputData returns just the task's inputData, for handlers that want the input rather
// than the whole task. A task with no inputData yields "null", matching what the skill
// worker produced previously by marshalling a nil map.
func (t Task) InputData() (json.RawMessage, error) {
	var envelope struct {
		InputData json.RawMessage `json:"inputData"`
	}
	if err := json.Unmarshal(t.Raw, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.InputData) == 0 {
		return json.RawMessage("null"), nil
	}
	return envelope.InputData, nil
}

// Result is the outcome of executing one task.
//
// There is no error return alongside it: every outcome is a Result, so there is exactly
// one way to express failure. Handlers build their own failure Results because the
// flavours shape failures differently — JavaScript workers report the message under an
// "error" output key, stdio workers use ReasonForIncompletion plus logs — and those
// differences are observable by workflows.
type Result struct {
	Status Status
	Output map[string]interface{}
	Logs   []string
	Reason string
}

// Failure builds a Result for the common shape: FAILED with a reason and no output.
func Failure(reason string) Result {
	return Result{Status: StatusFailed, Reason: reason}
}

// Handler executes one task.
//
// A Handler MUST be safe for concurrent use: one Handler is shared across all the
// goroutines of a batch poll.
type Handler interface {
	Handle(ctx context.Context, t Task) Result
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, t Task) Result

func (f HandlerFunc) Handle(ctx context.Context, t Task) Result { return f(ctx, t) }

// PolledTask is one entry from a poll. Conversion happens per task so that a single
// malformed task fails on its own instead of discarding its whole batch.
type PolledTask struct {
	Task Task
	// Err is set when this task could not be converted from the SDK model. The loop
	// fails such a task individually and continues with the rest of the batch.
	Err error
}

// Runner is the server boundary: polling for work and reporting results. The production
// implementation wraps the Conductor SDK; tests inject a fake.
type Runner interface {
	Poll(ctx context.Context, taskType string) ([]PolledTask, error)
	Update(ctx context.Context, t Task, r Result) error
}

// Config tunes the loop.
type Config struct {
	// PollBackoff is the wait after an empty or failed poll. Zero uses the default.
	PollBackoff time.Duration
}

// Worker runs the poll→execute→update loop for a single task type over a Runner.
type Worker struct {
	runner Runner
	cfg    Config
}

// NewWorker returns a Worker backed by the given Runner.
func NewWorker(runner Runner, cfg Config) *Worker {
	if cfg.PollBackoff <= 0 {
		cfg.PollBackoff = defaultPollBackoff
	}
	return &Worker{runner: runner, cfg: cfg}
}

// Run polls taskType and dispatches each task to h until ctx is cancelled.
//
// Cancellation is not immediate: Run returns once the in-flight batch finishes. A
// handler that blocks — a goja script has no interrupt wired, for instance — delays
// shutdown for as long as it runs.
//
// Transient poll failures back off and retry rather than stop the loop, and a failing
// task affects only itself.
func (w *Worker) Run(ctx context.Context, taskType string, h Handler) {
	for {
		if ctx.Err() != nil {
			return
		}

		polled, err := w.runner.Poll(ctx, taskType)
		if err != nil || len(polled) == 0 {
			if !sleep(ctx, w.cfg.PollBackoff) {
				return
			}
			continue
		}

		w.runBatch(ctx, polled, h)
	}
}

// runBatch executes every task in a poll batch concurrently and waits for all of them.
//
// Waiting for the whole batch before polling again preserves the pre-existing --count
// semantics: the next poll is gated on the slowest task in the batch. Decoupling poll
// cadence from execution would be a behaviour change and is deliberately not done here.
func (w *Worker) runBatch(ctx context.Context, polled []PolledTask, h Handler) {
	done := make(chan struct{})
	var pending int

	for _, p := range polled {
		pending++
		go func(p PolledTask) {
			defer func() { done <- struct{}{} }()
			w.runOne(ctx, p, h)
		}(p)
	}

	for i := 0; i < pending; i++ {
		<-done
	}
}

// runOne executes a single task and reports its result. A conversion error from the poll
// seam, or a panic in the handler, fails that task rather than the loop.
func (w *Worker) runOne(ctx context.Context, p PolledTask, h Handler) {
	if p.Err != nil {
		w.update(ctx, p.Task, Failure(p.Err.Error()))
		return
	}

	result := w.safeHandle(ctx, p.Task, h)
	w.update(ctx, p.Task, result)
}

// safeHandle runs the handler, converting a panic into a failed task so that one
// misbehaving handler cannot take the whole worker process down.
func (w *Worker) safeHandle(ctx context.Context, t Task, h Handler) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Failure(fmt.Sprintf("worker panicked: %v", r))
		}
	}()
	return h.Handle(ctx, t)
}

// update reports a result, deliberately detached from the loop's cancellation.
//
// A task that finished while the user was pressing Ctrl-C still has its result
// delivered; using the cancelled context would abandon completed work and leave the
// task in-flight until the server times it out.
func (w *Worker) update(ctx context.Context, t Task, r Result) {
	if err := w.runner.Update(context.WithoutCancel(ctx), t, r); err != nil {
		log.Errorf("Error updating task %s: %v", t.ID, err)
	}
}

// sleep waits d or until ctx is cancelled; it returns false if ctx was cancelled, which
// keeps the loop responsive to Ctrl-C during idle waits.
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
