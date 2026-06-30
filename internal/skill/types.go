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

// Package skill is the CLI-owned client and service for the Conductor skill
// endpoints (/api/skills/*). It mirrors the layering of the agent package. This
// stage covers the server-backed management operations (list/get/pull/delete);
// register/run/serve, which need the local skill packaging-and-execution engine,
// build on the same client in a later step.
package skill

import "encoding/json"

// Summary is the list view of a registered skill.
type Summary struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	PackageSize   int64  `json:"packageSize"`
	FileCount     int    `json:"fileCount"`
	ScriptCount   int    `json:"scriptCount"`
	SubAgentCount int    `json:"subAgentCount"`
	ResourceCount int    `json:"resourceCount"`
}

// FileEntry describes one file inside a skill package.
type FileEntry struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType"`
}

// Detail is the full server-side skill package record. RawConfig and Metadata stay
// as raw JSON so their free-form shapes do not leak as untyped maps across a boundary.
type Detail struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Checksum    string          `json:"checksum"`
	Status      string          `json:"status"`
	OwnerID     string          `json:"ownerId"`
	CreatedAt   *int64          `json:"createdAt"`
	UpdatedAt   *int64          `json:"updatedAt"`
	PackageSize int64           `json:"packageSize"`
	FileCount   int             `json:"fileCount"`
	Files       []FileEntry     `json:"files"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	RawConfig   json.RawMessage `json:"rawConfig,omitempty"`
}
