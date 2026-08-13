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
	"bytes"
	"strings"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal/agent"
)

// Regression guard for #116: every payload below is in the server's AgentSSEEvent
// shape, so a renderer reading a field the server does not send shows up as a blank
// line here rather than in someone's terminal.
func TestTerminalSinkRendersServerPayloads(t *testing.T) {
	tests := []struct {
		name string
		typ  agent.EventType
		data string
		want string
	}{
		{
			name: "thinking",
			typ:  agent.EventThinking,
			data: `{"type":"thinking","executionId":"e1","content":"probe_agent_llm"}`,
			want: "  [thinking] probe_agent_llm\n",
		},
		{
			name: "tool_call",
			typ:  agent.EventToolCall,
			data: `{"type":"tool_call","toolName":"lookup","args":{"q":"orders"}}`,
			want: "  [tool] lookup({\"q\":\"orders\"})\n",
		},
		{
			name: "tool_result",
			typ:  agent.EventToolResult,
			data: `{"type":"tool_result","toolName":"lookup","result":"3 orders"}`,
			want: "  [result] lookup -> 3 orders\n",
		},
		{
			name: "handoff",
			typ:  agent.EventHandoff,
			data: `{"type":"handoff","target":"billing_agent"}`,
			want: "  [handoff] -> billing_agent\n",
		},
		{
			name: "message",
			typ:  agent.EventMessage,
			data: `{"type":"message","content":"Yes."}`,
			want: "Yes.",
		},
		{
			name: "waiting",
			typ:  agent.EventWaiting,
			data: `{"type":"waiting","executionId":"e1","pendingTool":{"name":"approve"}}`,
			want: "  [waiting] human input required (execution: e1)\n",
		},
		{
			name: "guardrail_pass",
			typ:  agent.EventGuardrailPass,
			data: `{"type":"guardrail_pass","guardrailName":"pii"}`,
			want: "  [guardrail] PASS pii\n",
		},
		{
			name: "guardrail_fail carries its detail in content",
			typ:  agent.EventGuardrailFail,
			data: `{"type":"guardrail_fail","guardrailName":"pii","content":"found an email address"}`,
			want: "  [guardrail] FAIL pii: found an email address\n",
		},
		{
			name: "guardrail_fail without detail leaves no dangling separator",
			typ:  agent.EventGuardrailFail,
			data: `{"type":"guardrail_fail","guardrailName":"pii"}`,
			want: "  [guardrail] FAIL pii\n",
		},
		{
			name: "error",
			typ:  agent.EventError,
			data: `{"type":"error","toolName":"agent_llm","content":"model call failed: 429"}`,
			want: "  [error] model call failed: 429\n",
		},
		{
			name: "done",
			typ:  agent.EventDone,
			data: `{"type":"done","output":{"result":"Yes.","finishReason":"STOP"}}`,
			want: "\n{\"finishReason\":\"STOP\",\"result\":\"Yes.\"}\n",
		},
		{
			name: "unmodelled type falls back to raw data",
			typ:  "context_condensed",
			data: `{"type":"context_condensed","content":"token_limit"}`,
			want: "  [context_condensed] {\"type\":\"context_condensed\",\"content\":\"token_limit\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			sink := terminalSink{w: &out}

			if err := sink.OnEvent(agent.SSEEvent{Type: tt.typ, Data: []byte(tt.data)}); err != nil {
				t.Fatalf("OnEvent: %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("rendered %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTerminalSinkTruncatesLongFields(t *testing.T) {
	var out bytes.Buffer
	sink := terminalSink{w: &out}
	long := strings.Repeat("x", truncThinking+10)

	if err := sink.OnEvent(agent.SSEEvent{
		Type: agent.EventThinking,
		Data: []byte(`{"type":"thinking","content":"` + long + `"}`),
	}); err != nil {
		t.Fatalf("OnEvent: %v", err)
	}
	if want := "  [thinking] " + strings.Repeat("x", truncThinking) + "...\n"; out.String() != want {
		t.Errorf("rendered %q, want %q", out.String(), want)
	}
}
