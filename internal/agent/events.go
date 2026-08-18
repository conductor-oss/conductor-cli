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
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// EventType enumerates the streamed agent event kinds. Defining them as named
// constants keeps event handling free of magic strings — the SSE parser tags each
// event and the renderers switch on these.
type EventType string

const (
	EventThinking      EventType = "thinking"
	EventToolCall      EventType = "tool_call"
	EventToolResult    EventType = "tool_result"
	EventHandoff       EventType = "handoff"
	EventMessage       EventType = "message"
	EventWaiting       EventType = "waiting"
	EventGuardrailPass EventType = "guardrail_pass"
	EventGuardrailFail EventType = "guardrail_fail"
	EventError         EventType = "error"
	EventDone          EventType = "done"
)

// IsTerminal reports whether an event is the last one an execution can emit. The
// server keeps a terminal execution's SSE connection open indefinitely, so the
// stream has to end on the event itself rather than on a close that never comes.
func (t EventType) IsTerminal() bool {
	return t == EventDone || t == EventError
}

// SSE framing tokens and limits.
const (
	sseMaxLineBytes = 1024 * 1024 // generous line buffer for large data frames
	fieldID         = "id:"
	fieldEvent      = "event:"
	fieldData       = "data:"
	fieldComment    = ":"
)

// SSEEvent is one decoded Server-Sent Event. Data stays raw so the transport layer
// never needs to know each event's payload schema — the presentation layer decodes it.
type SSEEvent struct {
	ID   string
	Type EventType
	Data json.RawMessage
}

// ResolvedType returns the event's type, preferring the SSE event field and falling
// back to a "type" field inside the data payload (some events carry the kind there).
func (e SSEEvent) ResolvedType() EventType {
	if e.Type != "" {
		return e.Type
	}
	var probe struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(e.Data, &probe) == nil {
		return EventType(probe.Type)
	}
	return ""
}

// EventPayload mirrors the server's AgentSSEEvent: one envelope shared by every event
// kind, each filling only the fields it needs. Renderers read these fields instead of
// looking keys up in a map, so a field the server renames becomes a compile error
// rather than a silently blank line.
//
// Kinds the CLI does not model (context_condensed, subagent_start/stop) still decode
// into the common fields; a renderer that does not know them falls back to raw data.
type EventPayload struct {
	ID          int64     `json:"id"`
	Type        EventType `json:"type"`
	ExecutionID string    `json:"executionId"`
	// Content carries the human-readable text of thinking, message, error and
	// guardrail_fail events — the server has no separate "message" or "reason" field.
	Content string `json:"content"`
	// ToolName names the tool of tool_call/tool_result, and the failing task ref of error.
	ToolName      string         `json:"toolName"`
	Args          RawValue       `json:"args"`   // tool_call arguments
	Result        RawValue       `json:"result"` // tool_result payload
	Target        string         `json:"target"` // handoff destination agent
	Output        RawValue       `json:"output"` // done payload
	GuardrailName string         `json:"guardrailName"`
	PendingTool   map[string]any `json:"pendingTool"` // waiting: the tool awaiting a human
	Timestamp     int64          `json:"timestamp"`
}

// RawValue is a payload field the server types as a free-form object — tool
// arguments, tool results, the final output. Keeping it raw lets the payload stay
// typed without pinning down schemas the server does not fix.
type RawValue json.RawMessage

// UnmarshalJSON and MarshalJSON keep the bytes verbatim, as json.RawMessage does —
// a named type does not inherit its methods, and without them encoding/json would
// treat the underlying []byte as base64.
func (v *RawValue) UnmarshalJSON(data []byte) error {
	*v = append((*v)[:0], data...)
	return nil
}

func (v RawValue) MarshalJSON() ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	return v, nil
}

// String renders the value for display: a JSON string yields its text, anything else
// its compact JSON encoding. Non-strings round-trip through a generic decode so keys
// are ordered deterministically regardless of the order the server sent them in.
func (v RawValue) String() string {
	if len(v) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(v, &text) == nil {
		return text
	}
	var generic any
	if json.Unmarshal(v, &generic) != nil {
		return string(v)
	}
	encoded, err := json.Marshal(generic)
	if err != nil {
		return string(v)
	}
	return string(encoded)
}

// Payload decodes the event data into the typed payload. A malformed or empty body
// yields the zero payload rather than an error: a renderer shows what arrived, it
// does not police the wire format.
func (e SSEEvent) Payload() EventPayload {
	var p EventPayload
	_ = json.Unmarshal(e.Data, &p)
	return p
}

// parseSSE reads a text/event-stream body and emits one SSEEvent per record onto
// out, following WHATWG SSE framing: a blank line ends a record, ":" lines are
// comments (heartbeats), and multi-line data is joined with newlines. It returns the
// scanner error (or nil) when the stream ends; the caller owns closing out.
//
// Every send selects on ctx, so a consumer that walks away mid-stream — the stream
// ends on a terminal event, or the user hits Ctrl-C — cannot strand this goroutine
// on a send into a full buffer. Abandoning a send returns ctx.Err().
func parseSSE(ctx context.Context, r io.Reader, out chan<- SSEEvent) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, sseMaxLineBytes), sseMaxLineBytes)

	var id, event string
	var dataLines []string

	// flush reports whether the record was delivered; false means ctx is done.
	flush := func() bool {
		if len(dataLines) == 0 && event == "" {
			return true
		}
		select {
		case out <- SSEEvent{
			ID:   id,
			Type: EventType(event),
			Data: json.RawMessage(strings.Join(dataLines, "\n")),
		}:
		case <-ctx.Done():
			return false
		}
		id, event, dataLines = "", "", dataLines[:0]
		return true
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if !flush() {
				return ctx.Err()
			}
		case strings.HasPrefix(line, fieldComment):
			// comment / heartbeat — ignore
		case strings.HasPrefix(line, fieldID):
			id = sseFieldValue(line, fieldID)
		case strings.HasPrefix(line, fieldEvent):
			event = sseFieldValue(line, fieldEvent)
		case strings.HasPrefix(line, fieldData):
			dataLines = append(dataLines, sseFieldValue(line, fieldData))
		}
	}
	if !flush() {
		return ctx.Err()
	}
	return scanner.Err()
}

// sseFieldValue strips the field prefix and, per the WHATWG SSE spec, exactly one
// leading U+0020 space (not all whitespace).
func sseFieldValue(line, prefix string) string {
	return strings.TrimPrefix(strings.TrimPrefix(line, prefix), " ")
}
