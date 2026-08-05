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

package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// workerFlagCmd builds a throwaway command carrying the same flags a worker subcommand
// registers, so the flag plumbing can be tested without running a worker.
func workerFlagCmd(t *testing.T, execTimeout bool, execTimeoutDefault int32, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "fake"}
	cmd.Flags().String("worker-id", "", "")
	cmd.Flags().String("domain", "", "")
	cmd.Flags().Int32("count", 1, "")
	addPollTimeoutFlags(cmd, execTimeout, execTimeoutDefault)

	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v) error = %v", args, err)
	}
	return cmd
}

// TestWorkerPollFlagsTimeoutAlias is the regression test for issue #91: `worker remote`
// fed a single --timeout value to both the poll wait (milliseconds) and the execution
// budget (seconds), so one number meant two different things in two different units.
func TestWorkerPollFlagsTimeoutAlias(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPollMs   int32
		wantExecSecs time.Duration
	}{
		{
			name:         "no flags uses defaults",
			args:         nil,
			wantPollMs:   100,
			wantExecSecs: 0,
		},
		{
			name:         "poll-timeout alone",
			args:         []string{"--poll-timeout", "500"},
			wantPollMs:   500,
			wantExecSecs: 0,
		},
		{
			name:         "deprecated timeout alias maps to poll only",
			args:         []string{"--timeout", "4000"},
			wantPollMs:   4000,
			wantExecSecs: 0,
		},
		{
			name:         "poll-timeout wins when both are given",
			args:         []string{"--timeout", "4000", "--poll-timeout", "250"},
			wantPollMs:   250,
			wantExecSecs: 0,
		},
		{
			name:         "exec-timeout is independent of the poll wait",
			args:         []string{"--poll-timeout", "250", "--exec-timeout", "30"},
			wantPollMs:   250,
			wantExecSecs: 30 * time.Second,
		},
		{
			// The #91 bug: one value must not become both a millisecond poll wait and a
			// second-denominated execution budget.
			name:         "timeout alias does not also set the exec budget",
			args:         []string{"--timeout", "100"},
			wantPollMs:   100,
			wantExecSecs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := workerFlagCmd(t, true, 0, tt.args...)
			opts, execTimeout := workerPollFlags(cmd)

			if opts.PollTimeoutMs != tt.wantPollMs {
				t.Errorf("PollTimeoutMs = %d, want %d", opts.PollTimeoutMs, tt.wantPollMs)
			}
			if execTimeout != tt.wantExecSecs {
				t.Errorf("execTimeout = %v, want %v", execTimeout, tt.wantExecSecs)
			}
		})
	}
}

// TestWorkerPollFlagsRemoteExecTimeoutDefault pins the default that preserves the old
// effective behaviour: --timeout 100 previously reached the child as a 100 second kill
// timer, so defaulting remote's --exec-timeout to 0 would let a hanging worker run forever.
func TestWorkerPollFlagsRemoteExecTimeoutDefault(t *testing.T) {
	cmd := workerFlagCmd(t, true, 100)
	_, execTimeout := workerPollFlags(cmd)

	if execTimeout != 100*time.Second {
		t.Errorf("execTimeout = %v, want 100s", execTimeout)
	}
}

// TestWorkerPollFlagsWithoutExecTimeout covers `worker js`, which registers no
// --exec-timeout because a goja script cannot be interrupted. Reading the flag must not
// panic on the command that lacks it.
func TestWorkerPollFlagsWithoutExecTimeout(t *testing.T) {
	cmd := workerFlagCmd(t, false, 0, "--poll-timeout", "300")
	opts, execTimeout := workerPollFlags(cmd)

	if opts.PollTimeoutMs != 300 {
		t.Errorf("PollTimeoutMs = %d, want 300", opts.PollTimeoutMs)
	}
	if execTimeout != 0 {
		t.Errorf("execTimeout = %v, want 0 when the flag is not registered", execTimeout)
	}
}

func TestWorkerPollFlagsPassesThroughIdentity(t *testing.T) {
	cmd := workerFlagCmd(t, true, 0, "--worker-id", "w1", "--domain", "prod", "--count", "5")
	opts, _ := workerPollFlags(cmd)

	if opts.WorkerID != "w1" {
		t.Errorf("WorkerID = %q, want w1", opts.WorkerID)
	}
	if opts.Domain != "prod" {
		t.Errorf("Domain = %q, want prod", opts.Domain)
	}
	if opts.Count != 5 {
		t.Errorf("Count = %d, want 5", opts.Count)
	}
	if opts.UseTaskWorkerID {
		t.Error("UseTaskWorkerID = true, want false — only JavaScript workers set it")
	}
}

// TestJsRunnerOptionsReportsTaskWorkerID pins that JavaScript workers report the polled
// task's own worker id, which is observable on the task result.
func TestJsRunnerOptionsReportsTaskWorkerID(t *testing.T) {
	cmd := workerFlagCmd(t, false, 0, "--worker-id", "ignored-for-js")
	opts, _ := workerPollFlags(cmd)

	if got := jsRunnerOptions(opts); !got.UseTaskWorkerID {
		t.Error("jsRunnerOptions() did not set UseTaskWorkerID")
	}
}

// TestAddPollTimeoutFlagsRegistration checks the flag surface each worker command exposes:
// --timeout must be hidden everywhere, and --exec-timeout absent where it cannot be honoured.
func TestAddPollTimeoutFlagsRegistration(t *testing.T) {
	withExec := workerFlagCmd(t, true, 0)
	if withExec.Flags().Lookup("exec-timeout") == nil {
		t.Error("--exec-timeout not registered when requested")
	}

	withoutExec := workerFlagCmd(t, false, 0)
	if withoutExec.Flags().Lookup("exec-timeout") != nil {
		t.Error("--exec-timeout registered on a command that cannot honour it")
	}

	alias := withExec.Flags().Lookup("timeout")
	if alias == nil {
		t.Fatal("--timeout alias not registered")
	}
	if !alias.Hidden {
		t.Error("--timeout is not hidden; the deprecated alias should not appear in help")
	}
	if alias.Deprecated == "" {
		t.Error("--timeout is not marked deprecated")
	}
}
