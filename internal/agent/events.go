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

// parseSSE reads a text/event-stream body and emits one SSEEvent per record onto
// out, following WHATWG SSE framing: a blank line ends a record, ":" lines are
// comments (heartbeats), and multi-line data is joined with newlines. It returns the
// scanner error (or nil) when the stream ends; the caller owns closing out.
func parseSSE(r io.Reader, out chan<- SSEEvent) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, sseMaxLineBytes), sseMaxLineBytes)

	var id, event string
	var dataLines []string

	flush := func() {
		if len(dataLines) == 0 && event == "" {
			return
		}
		out <- SSEEvent{
			ID:   id,
			Type: EventType(event),
			Data: json.RawMessage(strings.Join(dataLines, "\n")),
		}
		id, event, dataLines = "", "", dataLines[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush()
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
	flush()
	return scanner.Err()
}

// sseFieldValue strips the field prefix and, per the WHATWG SSE spec, exactly one
// leading U+0020 space (not all whitespace).
func sseFieldValue(line, prefix string) string {
	return strings.TrimPrefix(strings.TrimPrefix(line, prefix), " ")
}
