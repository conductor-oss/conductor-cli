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

// Package skillworker is the local tool-worker runtime for skill run/serve. When a
// skill agent runs on the server, it dispatches tool tasks (read_skill_file, each
// script, workspace tools) back to the CLI; the tools run locally and their results are
// returned.
//
// The poll→handle→update loop itself lives in internal/taskworker, shared with the
// worker commands. This package owns the tool logic (handlers.go) and the adapter that
// puts it on that loop (adapter.go).
package skillworker

import (
	"context"
	"encoding/json"
)

// Worker protocol constants — fixed by the skill agent/server contract, so they are
// named constants, never inline literals.
const (
	taskTypeSep     = "__"     // task type is "{skillName}__{tool}"
	outputKeyResult = "result" // handler output is wrapped as {result: <output>}
)

// TaskType builds the "{skillName}__{tool}" task type dispatched for a skill tool.
func TaskType(skillName, tool string) string {
	return skillName + taskTypeSep + tool
}

// ToolHandler handles one dispatched tool task. IO is json.RawMessage, so no map
// crosses this boundary; the concrete tool logic lives in the handlers.
type ToolHandler interface {
	Handle(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}
