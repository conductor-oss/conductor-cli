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
	"strings"
	"testing"
)

func collectSSE(t *testing.T, body string) []SSEEvent {
	t.Helper()
	out := make(chan SSEEvent, 16)
	go func() {
		_ = parseSSE(strings.NewReader(body), out)
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
