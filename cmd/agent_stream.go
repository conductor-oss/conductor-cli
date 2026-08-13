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
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/agent"
)

// Display truncation widths for streamed events — named so they are not magic
// literals at the call sites.
const (
	truncThinking   = 120
	truncToolInput  = 100
	truncToolResult = 200
	truncEventData  = 150
)

var (
	runName     string
	runConfig   string
	runSession  string
	runNoStream bool
)

var agentRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Start an agent and stream its output",
	Long: `Start an agent by --name or --config with a prompt and stream its execution
events in real time. Use --no-stream to start it and just print the execution id.`,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := strings.Join(args, " ")
		svc := internal.GetAgentService()

		var req agent.RunRequest
		switch {
		case runConfig != "":
			def, err := loadAgentConfig(runConfig)
			if err != nil {
				return err
			}
			req = agent.RunRequest{Definition: def, Prompt: prompt, SessionID: runSession}
		case runName != "":
			req = agent.RunRequest{Name: runName, Prompt: prompt, SessionID: runSession}
		default:
			return fmt.Errorf("specify either --name or --config")
		}

		exec, err := svc.Run(cmd.Context(), req)
		if err != nil {
			return err
		}
		fmt.Printf("Agent: %s (Execution: %s)\n", exec.AgentName, exec.ID)
		if runNoStream {
			return nil
		}

		fmt.Println()
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		return svc.StreamExecution(ctx, exec.ID, "", newTerminalSink())
	},
}

var streamLastEventID string

var agentStreamCmd = &cobra.Command{
	Use:          "stream <execution-id>",
	Short:        "Stream events from a running execution",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		return internal.GetAgentService().StreamExecution(ctx, args[0], streamLastEventID, newTerminalSink())
	},
}

// terminalSink renders streamed agent events to a writer, stdout in production. It is
// the cmd-layer presentation of agent.EventSink; the service and client know nothing
// about it. Every field it reads comes from the typed payload, so a server-side rename
// breaks the build instead of quietly printing a blank line.
type terminalSink struct {
	w io.Writer
}

func newTerminalSink() terminalSink {
	return terminalSink{w: os.Stdout}
}

func (s terminalSink) OnEvent(e agent.SSEEvent) error {
	p := e.Payload()

	switch e.ResolvedType() {
	case agent.EventThinking:
		fmt.Fprintf(s.w, "  [thinking] %s\n", truncate(p.Content, truncThinking))
	case agent.EventToolCall:
		fmt.Fprintf(s.w, "  [tool] %s(%s)\n", p.ToolName, truncate(p.Args.String(), truncToolInput))
	case agent.EventToolResult:
		fmt.Fprintf(s.w, "  [result] %s -> %s\n", p.ToolName, truncate(p.Result.String(), truncToolResult))
	case agent.EventHandoff:
		fmt.Fprintf(s.w, "  [handoff] -> %s\n", p.Target)
	case agent.EventMessage:
		if p.Content != "" {
			fmt.Fprint(s.w, p.Content)
		}
	case agent.EventWaiting:
		fmt.Fprintf(s.w, "  [waiting] human input required (execution: %s)\n", p.ExecutionID)
	case agent.EventGuardrailPass:
		fmt.Fprintf(s.w, "  [guardrail] PASS %s\n", p.GuardrailName)
	case agent.EventGuardrailFail:
		// The failure detail rides in content; a server that omits it leaves the
		// name standing alone rather than trailing an empty separator.
		if p.Content != "" {
			fmt.Fprintf(s.w, "  [guardrail] FAIL %s: %s\n", p.GuardrailName, p.Content)
		} else {
			fmt.Fprintf(s.w, "  [guardrail] FAIL %s\n", p.GuardrailName)
		}
	case agent.EventError:
		fmt.Fprintf(s.w, "  [error] %s\n", p.Content)
	case agent.EventDone:
		if out := p.Output.String(); out != "" {
			fmt.Fprintln(s.w)
			fmt.Fprintln(s.w, out)
		}
	default:
		if t := e.ResolvedType(); t != "" {
			fmt.Fprintf(s.w, "  [%s] %s\n", t, truncate(string(e.Data), truncEventData))
		}
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func init() {
	agentRunCmd.Flags().StringVar(&runName, "name", "", "Name of a registered agent to run")
	agentRunCmd.Flags().StringVar(&runConfig, "config", "", "Path to an agent config file (YAML/JSON)")
	agentRunCmd.Flags().StringVar(&runSession, "session", "", "Session id for conversation continuity")
	agentRunCmd.Flags().BoolVar(&runNoStream, "no-stream", false, "Start the agent and print the execution id without streaming")

	agentStreamCmd.Flags().StringVar(&streamLastEventID, "last-event-id", "", "Resume streaming after this event id")

	agentCmd.AddCommand(agentRunCmd, agentStreamCmd)
}
