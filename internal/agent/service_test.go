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
	"encoding/json"
	"errors"
	"testing"
)

// fakeClient records calls and returns canned values; it implements Client so the
// service can be tested in isolation.
type fakeClient struct {
	getDef        json.RawMessage
	getCalled     bool
	lastRun       RunRequest
	lastDeployFwk string
	lastDeployCfg json.RawMessage
	deployResult  DeployResult
	streamEvents  []SSEEvent
	streamErr     error
}

func (f *fakeClient) CheckSupported(ctx context.Context) error { return nil }

func (f *fakeClient) Run(ctx context.Context, req RunRequest) (Execution, error) {
	f.lastRun = req
	return Execution{ID: "exec-1", AgentName: req.Name}, nil
}

func (f *fakeClient) Deploy(ctx context.Context, framework string, rawConfig json.RawMessage) (DeployResult, error) {
	f.lastDeployFwk = framework
	f.lastDeployCfg = rawConfig
	return f.deployResult, nil
}

func (f *fakeClient) Stream(ctx context.Context, id, lastEventID string) (<-chan SSEEvent, <-chan error) {
	events := make(chan SSEEvent, len(f.streamEvents))
	errc := make(chan error, 1)
	for _, e := range f.streamEvents {
		events <- e
	}
	close(events)
	errc <- f.streamErr
	close(errc)
	return events, errc
}

func (f *fakeClient) Get(ctx context.Context, name string, version *int) (json.RawMessage, error) {
	f.getCalled = true
	return f.getDef, nil
}

func (f *fakeClient) List(ctx context.Context) ([]AgentSummary, error)      { return nil, nil }
func (f *fakeClient) Delete(ctx context.Context, name string, v *int) error { return nil }
func (f *fakeClient) Compile(ctx context.Context, d json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}
func (f *fakeClient) SearchExecutions(ctx context.Context, _ ExecutionFilter) (ExecutionPage, error) {
	return ExecutionPage{}, nil
}
func (f *fakeClient) GetExecution(ctx context.Context, id string) (json.RawMessage, error) {
	return nil, nil
}
func (f *fakeClient) Status(ctx context.Context, id string) (json.RawMessage, error) {
	return nil, nil
}
func (f *fakeClient) Respond(ctx context.Context, id string, r HumanResponse) error { return nil }
func (f *fakeClient) Prune(ctx context.Context, req PruneRequest) (PruneResult, error) {
	return PruneResult{}, nil
}

func TestRunNameModeFetchesDefinitionAndDetectsFramework(t *testing.T) {
	fc := &fakeClient{getDef: json.RawMessage(`{"name":"x","_framework":"openai"}`)}
	svc := NewService(fc)

	if _, err := svc.Run(context.Background(), RunRequest{Name: "x", Prompt: "hi"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !fc.getCalled {
		t.Error("expected name-mode Run to fetch the definition")
	}
	if fc.lastRun.Framework != "openai" {
		t.Errorf("framework = %q, want openai", fc.lastRun.Framework)
	}
	if string(fc.lastRun.Definition) != string(fc.getDef) {
		t.Errorf("definition = %s, want %s", fc.lastRun.Definition, fc.getDef)
	}
}

func TestRunRequiresNameOrDefinition(t *testing.T) {
	svc := NewService(&fakeClient{})
	if _, err := svc.Run(context.Background(), RunRequest{Prompt: "hi"}); err == nil {
		t.Fatal("expected an error when neither name nor definition is provided")
	}
}

func TestDeployPassesThroughToClient(t *testing.T) {
	fc := &fakeClient{deployResult: DeployResult{AgentName: "my-skill", RequiredWorkers: []string{"my-skill__run"}}}
	res, err := NewService(fc).Deploy(context.Background(), "skill", json.RawMessage(`{"skillMd":"hi"}`))
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if fc.lastDeployFwk != "skill" || string(fc.lastDeployCfg) != `{"skillMd":"hi"}` {
		t.Errorf("client saw framework=%q cfg=%s", fc.lastDeployFwk, fc.lastDeployCfg)
	}
	if res.AgentName != "my-skill" || len(res.RequiredWorkers) != 1 {
		t.Errorf("result = %+v", res)
	}
}

type recordingSink struct {
	events []SSEEvent
}

func (r *recordingSink) OnEvent(e SSEEvent) error {
	r.events = append(r.events, e)
	return nil
}

func TestStreamExecutionForwardsEventsAndIgnoresCancel(t *testing.T) {
	fc := &fakeClient{
		streamEvents: []SSEEvent{{Type: EventMessage}, {Type: EventDone}},
		streamErr:    context.Canceled, // simulate Ctrl-C; must be treated as a clean stop
	}
	sink := &recordingSink{}
	if err := NewService(fc).StreamExecution(context.Background(), "exec-1", "", sink); err != nil {
		t.Fatalf("StreamExecution: %v", err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("forwarded %d events, want 2", len(sink.events))
	}
}

func TestStreamExecutionReturnsRealError(t *testing.T) {
	fc := &fakeClient{streamErr: errors.New("boom")}
	if err := NewService(fc).StreamExecution(context.Background(), "exec-1", "", &recordingSink{}); err == nil {
		t.Fatal("expected the terminal stream error to surface")
	}
}
