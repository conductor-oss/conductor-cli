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

// Package agent is the CLI-owned client and service for the Conductor agent
// endpoints (/api/agent/*), which are served by the merged server but are not part
// of the conductor-go SDK. The package is layered: domain types here, the transport
// boundary in client.go, and use-case orchestration in service.go. No file paths or
// transport types cross into this package — callers pass parsed bytes and structs.
package agent

import "encoding/json"

// RunRequest starts an agent execution. Definition is an already-parsed inline agent
// config (the cmd layer read it from disk and marshaled it to JSON); it is opaque
// here. When Framework is non-empty the execution starts through the framework
// envelope (skill/openai/… agents) instead of the native agent-config envelope.
type RunRequest struct {
	Name       string
	Prompt     string
	Definition json.RawMessage
	Framework  string
	SessionID  string
}

// Execution identifies a started or running agent execution.
type Execution struct {
	ID        string
	AgentName string
	Status    string
}

// DeployResult reports the outcome of publishing (deploying) an agent definition
// without starting an execution. RequiredWorkers lists the tool task types the
// caller must serve for the deployed agent to run (empty for a native agent).
type DeployResult struct {
	AgentName       string
	RequiredWorkers []string
}

// AgentSummary is the list view of a registered agent.
type AgentSummary struct {
	Name        string   `json:"name"`
	Version     int      `json:"version"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// ExecutionFilter narrows an execution search. Start/Size paginate.
type ExecutionFilter struct {
	AgentName string
	Status    string
	FreeText  string
	Start     int
	Size      int
}

// ExecutionPage is one page of execution-search results.
type ExecutionPage struct {
	TotalHits int64              `json:"totalHits"`
	Results   []ExecutionSummary `json:"results"`
}

// ExecutionSummary is the list view of a single execution.
type ExecutionSummary struct {
	ExecutionID   string `json:"executionId"`
	AgentName     string `json:"agentName"`
	Version       int    `json:"version"`
	Status        string `json:"status"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	ExecutionTime int64  `json:"executionTime"`
}

// PruneRequest selects terminal executions to delete (or archive) by age.
type PruneRequest struct {
	OlderThanDays int
	Archive       bool
}

// PruneResult reports how many execution records were removed.
type PruneResult struct {
	Removed int
}

// HumanResponse answers a human-in-the-loop task.
type HumanResponse struct {
	Approved bool
	Reason   string
	Message  string
}
