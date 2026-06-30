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
	"fmt"
)

// frameworkSkill is the framework marker inferred for skill-backed agent definitions.
const frameworkSkill = "skill"

// EventSink receives streamed events. It decouples streaming from rendering: the cmd
// layer supplies a terminal sink, the TUI a sink that posts UI messages, tests a
// recorder. Returning an error stops the stream.
type EventSink interface {
	OnEvent(SSEEvent) error
}

// Service is the agent use-case layer. It depends only on Client and is free of
// presentation and transport concerns.
type Service interface {
	Run(ctx context.Context, req RunRequest) (Execution, error)
	Deploy(ctx context.Context, framework string, rawConfig json.RawMessage) (DeployResult, error)
	StreamExecution(ctx context.Context, executionID, lastEventID string, sink EventSink) error
	List(ctx context.Context) ([]AgentSummary, error)
	Get(ctx context.Context, name string, version *int) (json.RawMessage, error)
	Delete(ctx context.Context, name string, version *int) error
	Compile(ctx context.Context, def json.RawMessage) (json.RawMessage, error)
	SearchExecutions(ctx context.Context, filter ExecutionFilter) (ExecutionPage, error)
	GetExecution(ctx context.Context, id string) (json.RawMessage, error)
	Status(ctx context.Context, id string) (json.RawMessage, error)
	Respond(ctx context.Context, id string, resp HumanResponse) error
	Prune(ctx context.Context, req PruneRequest) (PruneResult, error)
}

// NewService returns a Service backed by the given Client.
func NewService(c Client) Service {
	return &service{client: c}
}

type service struct {
	client Client
}

// Run starts an agent execution. In name-mode (Name set, no inline Definition) it
// first fetches the registered definition and, when that definition declares a
// framework, starts it through the framework envelope — mirroring the AgentSpan CLI.
func (s *service) Run(ctx context.Context, req RunRequest) (Execution, error) {
	if req.Name == "" && len(req.Definition) == 0 {
		return Execution{}, fmt.Errorf("run requires either an inline definition or a registered agent name")
	}
	if req.Name != "" && len(req.Definition) == 0 {
		def, err := s.client.Get(ctx, req.Name, nil)
		if err != nil {
			return Execution{}, err
		}
		req.Definition = def
		req.Framework = detectFramework(def)
	}
	return s.client.Run(ctx, req)
}

// Deploy publishes a framework agent definition without starting it.
func (s *service) Deploy(ctx context.Context, framework string, rawConfig json.RawMessage) (DeployResult, error) {
	return s.client.Deploy(ctx, framework, rawConfig)
}

// StreamExecution streams an execution's events into the sink until the stream ends.
// A context cancellation (e.g. Ctrl-C) is treated as a clean stop, not an error.
func (s *service) StreamExecution(ctx context.Context, executionID, lastEventID string, sink EventSink) error {
	events, errc := s.client.Stream(ctx, executionID, lastEventID)
	for evt := range events {
		if err := sink.OnEvent(evt); err != nil {
			return err
		}
	}
	if err := <-errc; err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (s *service) List(ctx context.Context) ([]AgentSummary, error) {
	return s.client.List(ctx)
}

func (s *service) Get(ctx context.Context, name string, version *int) (json.RawMessage, error) {
	return s.client.Get(ctx, name, version)
}

func (s *service) Delete(ctx context.Context, name string, version *int) error {
	return s.client.Delete(ctx, name, version)
}

func (s *service) Compile(ctx context.Context, def json.RawMessage) (json.RawMessage, error) {
	return s.client.Compile(ctx, def)
}

func (s *service) SearchExecutions(ctx context.Context, filter ExecutionFilter) (ExecutionPage, error) {
	return s.client.SearchExecutions(ctx, filter)
}

func (s *service) GetExecution(ctx context.Context, id string) (json.RawMessage, error) {
	return s.client.GetExecution(ctx, id)
}

func (s *service) Status(ctx context.Context, id string) (json.RawMessage, error) {
	return s.client.Status(ctx, id)
}

func (s *service) Respond(ctx context.Context, id string, resp HumanResponse) error {
	return s.client.Respond(ctx, id, resp)
}

func (s *service) Prune(ctx context.Context, req PruneRequest) (PruneResult, error) {
	return s.client.Prune(ctx, req)
}

// detectFramework returns the stored framework marker on an agent definition, or ""
// for a native agent. Framework agents (skill/openai/…) start via a different envelope.
func detectFramework(def json.RawMessage) string {
	var probe struct {
		Framework string `json:"_framework"`
		SkillMd   string `json:"skillMd"`
	}
	if json.Unmarshal(def, &probe) != nil {
		return ""
	}
	if probe.Framework != "" {
		return probe.Framework
	}
	if probe.SkillMd != "" {
		return frameworkSkill
	}
	return ""
}
