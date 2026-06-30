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
	"encoding/json"
	"fmt"
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
		return svc.StreamExecution(ctx, exec.ID, "", terminalSink{})
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
		return internal.GetAgentService().StreamExecution(ctx, args[0], streamLastEventID, terminalSink{})
	},
}

// terminalSink renders streamed agent events to stdout. It is the cmd-layer
// presentation of agent.EventSink; the service and client know nothing about it.
type terminalSink struct{}

func (terminalSink) OnEvent(e agent.SSEEvent) error {
	data := map[string]any{}
	_ = json.Unmarshal(e.Data, &data)

	switch e.ResolvedType() {
	case agent.EventThinking:
		fmt.Printf("  [thinking] %s\n", truncate(mapStr(data, "message"), truncThinking))
	case agent.EventToolCall:
		fmt.Printf("  [tool] %s(%s)\n", mapStr(data, "toolName"), truncate(mapStr(data, "input"), truncToolInput))
	case agent.EventToolResult:
		fmt.Printf("  [result] %s -> %s\n", mapStr(data, "toolName"), truncate(mapStr(data, "result"), truncToolResult))
	case agent.EventHandoff:
		fmt.Printf("  [handoff] -> %s\n", mapStr(data, "agentName"))
	case agent.EventMessage:
		if content := mapStr(data, "content"); content != "" {
			fmt.Print(content)
		}
	case agent.EventWaiting:
		fmt.Printf("  [waiting] human input required (execution: %s)\n", mapStr(data, "executionId"))
	case agent.EventGuardrailPass:
		fmt.Printf("  [guardrail] PASS %s\n", mapStr(data, "guardrailName"))
	case agent.EventGuardrailFail:
		fmt.Printf("  [guardrail] FAIL %s: %s\n", mapStr(data, "guardrailName"), mapStr(data, "reason"))
	case agent.EventError:
		fmt.Printf("  [error] %s\n", mapStr(data, "message"))
	case agent.EventDone:
		if out := mapStr(data, "output"); out != "" {
			fmt.Println()
			fmt.Println(out)
		}
	default:
		if t := e.ResolvedType(); t != "" {
			fmt.Printf("  [%s] %s\n", t, truncate(string(e.Data), truncEventData))
		}
	}
	return nil
}

// mapStr returns a string field from a decoded event payload, JSON-encoding non-string values.
func mapStr(data map[string]any, key string) string {
	v, ok := data[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
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
