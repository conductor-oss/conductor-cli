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
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/conductor-oss/conductor-cli/internal/transport"
)

// Endpoint paths. These are fixed by the server API contract, so they are named
// constants rather than configuration — but never inline literals at call sites.
// Paths are relative to the transport BaseURL, which already includes the "/api"
// prefix (see cmd/root.go) — mirroring the conductor-go SDK, whose base is also
// ".../api" and whose resource paths omit it. Do not re-add "/api" here.
const (
	pathAgents     = "/agent"                  // base; "/{name}" appended
	pathStart      = "/agent/start"            //
	pathDeploy     = "/agent/deploy"           // publish/activate without starting
	pathCompile    = "/agent/compile"          //
	pathList       = "/agent/list"             //
	pathExecutions = "/agent/executions"       // base; "/{id}" appended
	pathPrune      = "/agent/executions/prune" //
	pathStatusFmt  = "/agent/%s/status"        // execution id
	pathRespondFmt = "/agent/%s/respond"       // execution id
	pathStreamFmt  = "/agent/stream/%s"        // execution id (SSE; used by Stream)
)

// Query-parameter keys and fixed values used by the agent endpoints.
const (
	queryVersion       = "version"
	queryStart         = "start"
	querySize          = "size"
	querySort          = "sort"
	queryAgentName     = "agentName"
	queryStatus        = "status"
	queryFreeText      = "freeText"
	queryOlderThanDays = "olderThanDays"
	queryArchiveTasks  = "archiveTasks"
	sortExecutions     = "startTime:DESC"
	valueTrue          = "true"
)

// Streaming (SSE) constants.
const (
	sseChannelBuffer  = 100
	headerAccept      = "Accept"
	headerLastEventID = "Last-Event-ID"
	mimeEventStream   = "text/event-stream"
)

// Client is the transport boundary for agent endpoints. It returns domain types or
// raw JSON for free-form payloads — never *http.Response or transport types.
type Client interface {
	Run(ctx context.Context, req RunRequest) (Execution, error)
	Deploy(ctx context.Context, framework string, rawConfig json.RawMessage) (DeployResult, error)
	Stream(ctx context.Context, executionID, lastEventID string) (<-chan SSEEvent, <-chan error)
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

// NewClient returns a Client backed by the shared transport.
func NewClient(t transport.Config) Client {
	return &restClient{t: t}
}

type restClient struct {
	t transport.Config
}

// Wire DTOs — private to the client so the JSON shape never leaks across a boundary.

type startRequest struct {
	AgentConfig json.RawMessage `json:"agentConfig,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
	SessionID   string          `json:"sessionId,omitempty"`
}

type frameworkRequest struct {
	Framework string          `json:"framework"`
	RawConfig json.RawMessage `json:"rawConfig,omitempty"`
	Prompt    string          `json:"prompt,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
}

type startResponse struct {
	ExecutionID string `json:"executionId"`
	AgentName   string `json:"agentName"`
}

type deployRequest struct {
	Framework string          `json:"framework"`
	RawConfig json.RawMessage `json:"rawConfig,omitempty"`
}

type deployResponse struct {
	AgentName       string   `json:"agentName"`
	RequiredWorkers []string `json:"requiredWorkers,omitempty"`
}

type respondRequest struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
	Message  string `json:"message,omitempty"`
}

type pruneResponse struct {
	Deleted int `json:"deleted"`
}

func (c *restClient) Run(ctx context.Context, req RunRequest) (Execution, error) {
	var body any
	if req.Framework != "" {
		body = frameworkRequest{Framework: req.Framework, RawConfig: req.Definition, Prompt: req.Prompt, SessionID: req.SessionID}
	} else {
		body = startRequest{AgentConfig: req.Definition, Prompt: req.Prompt, SessionID: req.SessionID}
	}
	var sr startResponse
	if err := c.t.DoJSON(ctx, http.MethodPost, pathStart, body, &sr); err != nil {
		return Execution{}, err
	}
	return Execution{ID: sr.ExecutionID, AgentName: sr.AgentName}, nil
}

// Deploy publishes and activates a framework agent definition without starting an
// execution — the non-interactive counterpart of Run. The definition is opaque
// bytes assembled by the caller; the framework marker selects the server envelope.
func (c *restClient) Deploy(ctx context.Context, framework string, rawConfig json.RawMessage) (DeployResult, error) {
	body := deployRequest{Framework: framework, RawConfig: rawConfig}
	var dr deployResponse
	if err := c.t.DoJSON(ctx, http.MethodPost, pathDeploy, body, &dr); err != nil {
		return DeployResult{}, err
	}
	return DeployResult{AgentName: dr.AgentName, RequiredWorkers: dr.RequiredWorkers}, nil
}

// Stream opens an SSE connection for an execution and returns an event channel and
// a single-error channel. The caller ranges the events; when it closes, the error
// channel carries the terminal error (nil on a clean end). Cancelling ctx ends the
// stream — that surfaces as a context error which the service treats as a clean stop.
func (c *restClient) Stream(ctx context.Context, executionID, lastEventID string) (<-chan SSEEvent, <-chan error) {
	events := make(chan SSEEvent, sseChannelBuffer)
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		defer close(events)

		header := http.Header{}
		header.Set(headerAccept, mimeEventStream)
		if lastEventID != "" {
			header.Set(headerLastEventID, lastEventID)
		}
		resp, err := c.t.Do(ctx, http.MethodGet, fmt.Sprintf(pathStreamFmt, url.PathEscape(executionID)), nil, header)
		if err != nil {
			errc <- err
			return
		}
		defer resp.Body.Close()
		errc <- parseSSE(resp.Body, events)
	}()
	return events, errc
}

func (c *restClient) List(ctx context.Context) ([]AgentSummary, error) {
	var out []AgentSummary
	if err := c.t.DoJSON(ctx, http.MethodGet, pathList, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restClient) Get(ctx context.Context, name string, version *int) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.t.DoJSON(ctx, http.MethodGet, agentPath(name, version), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restClient) Delete(ctx context.Context, name string, version *int) error {
	return c.t.DoJSON(ctx, http.MethodDelete, agentPath(name, version), nil, nil)
}

func (c *restClient) Compile(ctx context.Context, def json.RawMessage) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.t.DoJSON(ctx, http.MethodPost, pathCompile, def, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restClient) SearchExecutions(ctx context.Context, filter ExecutionFilter) (ExecutionPage, error) {
	q := url.Values{}
	q.Set(queryStart, strconv.Itoa(filter.Start))
	q.Set(querySize, strconv.Itoa(filter.Size))
	q.Set(querySort, sortExecutions)
	if filter.AgentName != "" {
		q.Set(queryAgentName, filter.AgentName)
	}
	if filter.Status != "" {
		q.Set(queryStatus, filter.Status)
	}
	if filter.FreeText != "" {
		q.Set(queryFreeText, filter.FreeText)
	}
	var page ExecutionPage
	if err := c.t.DoJSON(ctx, http.MethodGet, pathExecutions+"?"+q.Encode(), nil, &page); err != nil {
		return ExecutionPage{}, err
	}
	return page, nil
}

func (c *restClient) GetExecution(ctx context.Context, id string) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.t.DoJSON(ctx, http.MethodGet, pathExecutions+"/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restClient) Status(ctx context.Context, id string) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.t.DoJSON(ctx, http.MethodGet, fmt.Sprintf(pathStatusFmt, url.PathEscape(id)), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restClient) Respond(ctx context.Context, id string, resp HumanResponse) error {
	body := respondRequest{Approved: resp.Approved, Reason: resp.Reason, Message: resp.Message}
	return c.t.DoJSON(ctx, http.MethodPost, fmt.Sprintf(pathRespondFmt, url.PathEscape(id)), body, nil)
}

func (c *restClient) Prune(ctx context.Context, req PruneRequest) (PruneResult, error) {
	q := url.Values{}
	q.Set(queryOlderThanDays, strconv.Itoa(req.OlderThanDays))
	if req.Archive {
		q.Set(queryArchiveTasks, valueTrue)
	}
	var pr pruneResponse
	if err := c.t.DoJSON(ctx, http.MethodPost, pathPrune+"?"+q.Encode(), nil, &pr); err != nil {
		return PruneResult{}, err
	}
	return PruneResult{Removed: pr.Deleted}, nil
}

// agentPath builds the /agent/{name} path (relative to the /api BaseURL) with an
// optional ?version=N.
func agentPath(name string, version *int) string {
	p := pathAgents + "/" + url.PathEscape(name)
	if version != nil {
		q := url.Values{}
		q.Set(queryVersion, strconv.Itoa(*version))
		p += "?" + q.Encode()
	}
	return p
}
