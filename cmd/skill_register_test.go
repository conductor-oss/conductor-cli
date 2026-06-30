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

package cmd

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newSkillFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "SKILL.md", "---\nname: summarize\ndescription: Summarize text\n---\nBody text\n")
	writeFile(t, dir, "writer-agent.md", "agent body")
	writeFile(t, dir, "scripts/run.py", "print('hi')")
	writeFile(t, dir, "references/doc.md", "reference")
	writeFile(t, dir, ".agentspanignore", "*.tmp\n")
	writeFile(t, dir, "scratch.tmp", "ignored")
	writeFile(t, dir, ".env", "SECRET=x")
	writeFile(t, dir, "node_modules/dep/index.js", "junk")
	return dir
}

func zipNames(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	return names
}

func TestBuildSkillPackageIncludesAndExcludes(t *testing.T) {
	dir := newSkillFixture(t)

	pkg, files, err := buildSkillPackage(dir)
	if err != nil {
		t.Fatalf("buildSkillPackage: %v", err)
	}
	names := zipNames(t, pkg)

	for _, want := range []string{"SKILL.md", "writer-agent.md", "scripts/run.py", "references/doc.md"} {
		if !names[want] {
			t.Errorf("expected %q in package, got %v", want, names)
		}
	}
	for _, unwanted := range []string{"scratch.tmp", ".env", ".agentspanignore", "node_modules/dep/index.js"} {
		if names[unwanted] {
			t.Errorf("did not expect %q in package", unwanted)
		}
	}

	// Manifest entries carry checksum + content type and match the included files.
	if len(files) != len(names) {
		t.Errorf("manifest has %d files, package has %d", len(files), len(names))
	}
	for _, f := range files {
		if f.SHA256 == "" || f.ContentType == "" {
			t.Errorf("file entry missing checksum/content-type: %+v", f)
		}
	}
}

func TestBuildSkillManifestReadsFrontmatter(t *testing.T) {
	dir := newSkillFixture(t)
	_, files, err := buildSkillPackage(dir)
	if err != nil {
		t.Fatalf("buildSkillPackage: %v", err)
	}

	raw, err := buildSkillManifest(dir, "v1", "openai/gpt-4o", []string{"writer=anthropic/claude"}, files)
	if err != nil {
		t.Fatalf("buildSkillManifest: %v", err)
	}
	var m skillManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if m.Name != "summarize" || m.Description != "Summarize text" {
		t.Errorf("manifest = %+v", m)
	}
	if m.Version != "v1" || m.Model != "openai/gpt-4o" || m.AgentModels["writer"] != "anthropic/claude" {
		t.Errorf("manifest flags not applied: %+v", m)
	}
}

func TestBuildSkillManifestRequiresName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "SKILL.md", "---\ndescription: no name here\n---\nbody")
	if _, err := buildSkillManifest(dir, "", "", nil, nil); err == nil {
		t.Fatal("expected an error when SKILL.md has no name")
	}
}
