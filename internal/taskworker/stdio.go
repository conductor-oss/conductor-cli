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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	log "github.com/sirupsen/logrus"
)

// stdioResult is the JSON contract a stdio worker writes to its stdout.
type stdioResult struct {
	Status string                 `json:"status"`
	Output map[string]interface{} `json:"output,omitempty"`
	Logs   []string               `json:"logs,omitempty"`
	Reason string                 `json:"reason,omitempty"`
}

// StdioOptions configures a StdioHandler.
type StdioOptions struct {
	// Command and Args are the worker program to run, once per task.
	Command string
	Args    []string
	// Env is appended to the child's environment. It is passed in rather than read from
	// viper so this package does not depend on process-global config, and so tests can
	// assert what the child receives without mutating global state.
	Env []string
	// Domain, when set, is exported to the child as POLL_DOMAIN.
	Domain string
	// ExecTimeout bounds a single task's execution. Zero means no timeout.
	ExecTimeout time.Duration
	// Verbose prints the task JSON and the result JSON to stdout.
	Verbose bool
}

// StdioHandler runs an external program per task: the full task JSON goes in on stdin,
// a result JSON comes back on stdout. It is safe for concurrent use — each Handle call
// builds its own command and buffers.
type StdioHandler struct {
	opts StdioOptions
}

// NewStdioHandler returns a Handler that executes opts.Command for each task.
func NewStdioHandler(opts StdioOptions) *StdioHandler {
	return &StdioHandler{opts: opts}
}

func (h *StdioHandler) Handle(ctx context.Context, t Task) Result {
	log.Infof("Processing task: %s (workflow: %s)", t.ID, t.WorkflowID)

	if h.opts.Verbose {
		fmt.Println("=== Task Input ===")
		fmt.Println(string(t.Raw))
		fmt.Println("==================")
	}

	// The child is deliberately detached from the loop's cancellation. A task already
	// running should finish and report its real result, which is what Run promises
	// ("returns once the in-flight batch finishes"). Killing it on Ctrl-C instead makes
	// the worker report a FAILED it inflicted on itself — and because results are
	// delivered on a context that outlives cancellation, the server sees that failure
	// and consumes one of the task's retries. A child that will not finish is handled by
	// the second interrupt, which exits the process outright.
	execCtx := context.WithoutCancel(ctx)
	if h.opts.ExecTimeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(execCtx, h.opts.ExecTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(execCtx, h.opts.Command, h.opts.Args...)
	cmd.Env = append(cmd.Environ(),
		"TASK_TYPE="+t.Type,
		"TASK_ID="+t.ID,
		"WORKFLOW_ID="+t.WorkflowID,
		"EXECUTION_ID="+t.WorkflowID,
	)
	if h.opts.Domain != "" {
		cmd.Env = append(cmd.Env, "POLL_DOMAIN="+h.opts.Domain)
	}
	cmd.Env = append(cmd.Env, h.opts.Env...)

	cmd.Stdin = bytes.NewReader(t.Raw)

	// The child's streams are both captured and echoed, so a worker's own output stays
	// visible in the terminal while still being available for parsing and for logs.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
	cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)

	result := h.runAndParse(cmd, &stdout, &stderr)

	log.Infof("Task %s handled with status: %s", t.ID, result.Status)
	return result
}

// printResultBanner reports a result under --verbose. The banner distinguishes failures
// so they stand out in a stream of task output.
func printResultBanner(result Result) {
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	if result.Status == StatusFailed {
		fmt.Println("=== Task Result (Error) ===")
		fmt.Println(string(resultJSON))
		fmt.Println("===========================")
		return
	}
	fmt.Println("=== Task Result ===")
	fmt.Println(string(resultJSON))
	fmt.Println("===================")
}

// runAndParse executes the child and turns its outcome into a Result.
func (h *StdioHandler) runAndParse(cmd *exec.Cmd, stdout, stderr *bytes.Buffer) Result {
	if err := cmd.Run(); err != nil {
		stderrOutput := stderr.String()
		log.Errorf("Worker execution failed: %v", err)
		if stderrOutput != "" {
			log.Errorf("Worker stderr:\n%s", stderrOutput)
		}
		failure := Result{
			Status: StatusFailed,
			Reason: fmt.Sprintf("worker execution failed: %v", err),
			Logs:   []string{stderrOutput},
		}
		if h.opts.Verbose {
			printResultBanner(failure)
		}
		return failure
	}

	var parsed stdioResult
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		stdoutOutput := stdout.String()
		log.Errorf("Failed to parse worker output as JSON: %v", err)
		log.Errorf("Worker stdout:\n%s", stdoutOutput)
		failure := Result{
			Status: StatusFailed,
			Reason: fmt.Sprintf("invalid worker stdout JSON: %v", err),
			Logs:   []string{stdoutOutput},
		}
		if h.opts.Verbose {
			printResultBanner(failure)
		}
		return failure
	}

	// Reported before normalisation, so a worker that returned an unrecognised status
	// sees what it actually sent rather than the rewritten failure — which is the whole
	// point of asking for verbose output.
	if h.opts.Verbose {
		printResultBanner(Result{
			Status: Status(parsed.Status),
			Output: parsed.Output,
			Logs:   parsed.Logs,
			Reason: parsed.Reason,
		})
	}

	return normalizeStdioResult(parsed)
}

// normalizeStdioResult constrains a stdio worker's status to the three values the stdio
// contract documents. Unlike JavaScript workers — which forward any status straight
// through — an unrecognised status here fails the task.
func normalizeStdioResult(parsed stdioResult) Result {
	result := Result{
		Status: Status(parsed.Status),
		Output: parsed.Output,
		Logs:   parsed.Logs,
		Reason: parsed.Reason,
	}

	switch result.Status {
	case StatusCompleted, StatusFailed, StatusInProgress:
		return result
	}

	// Carried over verbatim from the pre-convergence implementation, including its
	// quirk: Status is overwritten before the reason is formatted, so the message always
	// reads "invalid status from worker: FAILED" rather than naming the offending value.
	// Preserved here to keep the convergence a pure refactor; see issue #92 for the
	// follow-up that fixes the message.
	result.Status = StatusFailed
	result.Reason = fmt.Sprintf("invalid status from worker: %s", result.Status)
	return result
}
