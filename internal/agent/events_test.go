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

package agent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func collectSSE(t *testing.T, body string) []SSEEvent {
	t.Helper()
	out := make(chan SSEEvent, 16)
	go func() {
		_ = parseSSE(context.Background(), strings.NewReader(body), out)
		close(out)
	}()
	var got []SSEEvent
	for e := range out {
		got = append(got, e)
	}
	return got
}

func TestParseSSEFramingAndComments(t *testing.T) {
	// A heartbeat comment, a typed event, then a multi-line data record.
	body := ": keep-alive\n" +
		"event: message\n" +
		"data: {\"content\":\"hi\"}\n" +
		"\n" +
		"event: done\n" +
		"data: line1\n" +
		"data: line2\n" +
		"\n"

	got := collectSSE(t, body)
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(got), got)
	}
	if got[0].Type != EventMessage || string(got[0].Data) != `{"content":"hi"}` {
		t.Errorf("event 0 = %+v", got[0])
	}
	if got[1].Type != EventDone || string(got[1].Data) != "line1\nline2" {
		t.Errorf("event 1 = %+v", got[1])
	}
}

func TestResolvedTypeFallsBackToDataType(t *testing.T) {
	e := SSEEvent{Data: []byte(`{"type":"thinking"}`)}
	if e.ResolvedType() != EventThinking {
		t.Errorf("ResolvedType = %q, want thinking", e.ResolvedType())
	}
}

// Regression guard for #102: a consumer that walks away must not strand the parser
// on a send. Nobody reads out here, so the send can only complete via cancellation.
func TestParseSSEAbandonsSendWhenContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- parseSSE(ctx, strings.NewReader("event: done\ndata: {}\n\n"), make(chan SSEEvent))
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("parseSSE = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parseSSE stayed blocked on a send after the context was cancelled")
	}
}

func TestIsTerminal(t *testing.T) {
	for _, tt := range []struct {
		typ  EventType
		want bool
	}{
		{EventDone, true},
		{EventError, true},
		{EventMessage, false},
		{EventThinking, false},
		{"", false},
	} {
		if got := tt.typ.IsTerminal(); got != tt.want {
			t.Errorf("%q.IsTerminal() = %v, want %v", tt.typ, got, tt.want)
		}
	}
}

// The payload mirrors the server's AgentSSEEvent; this pins the wire names down.
func TestPayloadDecodesServerFieldNames(t *testing.T) {
	e := SSEEvent{Data: []byte(`{"id":7,"type":"tool_call","executionId":"exec-1",
		"content":"why","toolName":"lookup","args":{"q":"x"},"result":"ok",
		"target":"billing","output":{"result":"done"},"guardrailName":"pii","timestamp":42}`)}

	p := e.Payload()
	if p.ID != 7 || p.Type != "tool_call" || p.ExecutionID != "exec-1" || p.Timestamp != 42 {
		t.Errorf("envelope = %+v", p)
	}
	if p.Content != "why" || p.ToolName != "lookup" || p.Target != "billing" || p.GuardrailName != "pii" {
		t.Errorf("text fields = %+v", p)
	}
	if got := p.Args.String(); got != `{"q":"x"}` {
		t.Errorf("args = %q", got)
	}
	if got := p.Result.String(); got != "ok" {
		t.Errorf("result = %q", got)
	}
	if got := p.Output.String(); got != `{"result":"done"}` {
		t.Errorf("output = %q", got)
	}
}

func TestPayloadOfMalformedDataIsZero(t *testing.T) {
	if p := (SSEEvent{Data: []byte("not json")}).Payload(); !reflect.DeepEqual(p, EventPayload{}) {
		t.Errorf("payload = %+v, want zero", p)
	}
}

func TestRawValueString(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   RawValue
		want string
	}{
		{"absent", nil, ""},
		{"string", RawValue(`"hello"`), "hello"},
		{"number", RawValue(`3`), "3"},
		{"object keys sorted", RawValue(`{"b":1,"a":2}`), `{"a":2,"b":1}`},
		{"not json", RawValue(`{oops`), "{oops"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
