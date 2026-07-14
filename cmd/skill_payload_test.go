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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkillFile writes content to dir/rel, creating parent directories.
func writeSkillFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestBuildSkillPayloadTypedFields(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "SKILL.md", "---\nname: demo\nparams:\n  tone:\n    default: friendly\n  verbose: false\n---\nBody text.\n")
	writeSkillFile(t, dir, "planner-agent.md", "You plan.")
	writeSkillFile(t, dir, "scripts/greet.py", "print('hi')")
	writeSkillFile(t, dir, "scripts/build.sh", "echo build")
	writeSkillFile(t, dir, "references/guide.md", "guide")
	writeSkillFile(t, dir, "notes.txt", "root resource")

	cfg, local, err := BuildSkillPayload(dir, PayloadOptions{
		Model:       "gpt-x",
		AgentModels: map[string]string{"planner": "gpt-mini"},
	})
	if err != nil {
		t.Fatalf("BuildSkillPayload: %v", err)
	}

	if cfg.Model != "gpt-x" {
		t.Errorf("Model = %q, want gpt-x", cfg.Model)
	}
	if cfg.AgentModels["planner"] != "gpt-mini" {
		t.Errorf("AgentModels = %v", cfg.AgentModels)
	}
	if body := cfg.AgentFiles["planner"]; body != "You plan." {
		t.Errorf("AgentFiles[planner] = %q", body)
	}
	if got := cfg.Scripts["greet"]; got.Filename != "greet.py" || got.Language != "python" {
		t.Errorf("Scripts[greet] = %+v", got)
	}
	if got := cfg.Scripts["build"]; got.Filename != "build.sh" || got.Language != "bash" {
		t.Errorf("Scripts[build] = %+v", got)
	}
	if !containsString(cfg.ResourceFiles, "references/guide.md") || !containsString(cfg.ResourceFiles, "notes.txt") {
		t.Errorf("ResourceFiles = %v", cfg.ResourceFiles)
	}
	// SKILL.md and *-agent.md must never be listed as resources.
	if containsString(cfg.ResourceFiles, "SKILL.md") || containsString(cfg.ResourceFiles, "planner-agent.md") {
		t.Errorf("ResourceFiles leaked non-resource files: %v", cfg.ResourceFiles)
	}
	// Frontmatter defaults: {default: friendly} collapses to the value; scalar carries through.
	if cfg.DefaultParams["tone"].format() != "friendly" || cfg.DefaultParams["verbose"].format() != "false" {
		t.Errorf("DefaultParams = tone=%q verbose=%q", cfg.DefaultParams["tone"].format(), cfg.DefaultParams["verbose"].format())
	}
	// Params get injected into skillMd for server visibility.
	if !strings.Contains(cfg.SkillMd, "[Skill Parameters]") || !strings.Contains(cfg.SkillMd, "tone: friendly") {
		t.Errorf("skillMd missing injected params:\n%s", cfg.SkillMd)
	}
	if local.SkillName != "demo" {
		t.Errorf("LocalContext.SkillName = %q, want demo", local.SkillName)
	}
	if !filepath.IsAbs(local.SkillDir) {
		t.Errorf("LocalContext.SkillDir not absolute: %q", local.SkillDir)
	}
}

// TestBuildSkillPayloadParamOverride verifies overrides win over frontmatter defaults.
func TestBuildSkillPayloadParamOverride(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "SKILL.md", "---\nname: demo\nparams:\n  tone:\n    default: friendly\n---\nBody.\n")

	cfg, _, err := BuildSkillPayload(dir, PayloadOptions{
		Model:          "m",
		ParamOverrides: map[string]ParamValue{"tone": rawParamValue("terse")},
	})
	if err != nil {
		t.Fatalf("BuildSkillPayload: %v", err)
	}
	if cfg.Params["tone"].format() != "terse" {
		t.Errorf("merged param tone = %q, want terse", cfg.Params["tone"].format())
	}
	if !strings.Contains(cfg.SkillMd, "tone: terse") {
		t.Errorf("skillMd should carry the overridden param:\n%s", cfg.SkillMd)
	}
}

// TestBuildSkillPayloadWireLocalSplit asserts the wire type carries no local paths
// and the local context carries no server fields — the invariant that replaces the
// original's _skill-prefixed keys and stripLocalSkillFields step.
func TestBuildSkillPayloadWireLocalSplit(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "SKILL.md", "---\nname: demo\n---\nBody.\n")

	cfg, local, err := BuildSkillPayload(dir, PayloadOptions{Model: "m"})
	if err != nil {
		t.Fatalf("BuildSkillPayload: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wire := string(raw)
	for _, banned := range []string{"_skill", "skillPath", "skillDir", local.SkillDir} {
		if strings.Contains(wire, banned) {
			t.Errorf("wire config leaked %q: %s", banned, wire)
		}
	}
	// No local-context field name should appear as a JSON key either.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"sections", "skillDir", "skillName"} {
		if _, ok := probe[k]; ok {
			t.Errorf("wire config unexpectedly has key %q", k)
		}
	}
}

// TestBuildSkillPayloadCrossSkillRecursion checks a referenced sibling skill is
// packaged recursively as a nested typed config.
func TestBuildSkillPayloadCrossSkillRecursion(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	writeSkillFile(t, main, "SKILL.md", "---\nname: main\n---\nPlease invoke the helper skill to assist.\n")
	helper := filepath.Join(root, "helper")
	writeSkillFile(t, helper, "SKILL.md", "---\nname: helper\n---\nHelper body.\n")
	writeSkillFile(t, helper, "scripts/aid.sh", "echo aid")

	cfg, _, err := BuildSkillPayload(main, PayloadOptions{Model: "m"})
	if err != nil {
		t.Fatalf("BuildSkillPayload: %v", err)
	}
	ref, ok := cfg.CrossSkillRefs["helper"]
	if !ok {
		t.Fatalf("expected cross-skill ref 'helper', got %v", keysOfStringMap(cfg.CrossSkillRefs))
	}
	if _, ok := ref.Scripts["aid"]; !ok {
		t.Errorf("nested helper config missing its script: %+v", ref.Scripts)
	}
}

// TestBuildSkillPayloadCycleGuard ensures a circular reference is rejected, not
// recursed forever.
func TestBuildSkillPayloadCycleGuard(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "alpha")
	writeSkillFile(t, a, "SKILL.md", "---\nname: alpha\n---\nUse the beta skill.\n")
	b := filepath.Join(root, "beta")
	writeSkillFile(t, b, "SKILL.md", "---\nname: beta\n---\nUse the alpha skill.\n")

	_, _, err := BuildSkillPayload(a, PayloadOptions{Model: "m"})
	if err == nil {
		t.Fatal("expected a circular skill reference error")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error = %v, want a circular-reference error", err)
	}
}

// TestBuildSkillPayloadRequiresName rejects a SKILL.md without a name.
func TestBuildSkillPayloadRequiresName(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "SKILL.md", "---\ndescription: no name here\n---\nBody.\n")
	if _, _, err := BuildSkillPayload(dir, PayloadOptions{Model: "m"}); err == nil {
		t.Fatal("expected an error for a SKILL.md missing 'name'")
	}
}

// TestSplitSkillSectionsLargeBody exercises the section split above the threshold.
func TestSplitSkillSectionsLargeBody(t *testing.T) {
	filler := strings.Repeat("x", skillSectionSplitBytes)
	body := "## First Heading\n" + filler + "\n## Second Heading\nmore\n"
	sections := splitSkillSections(body)
	if _, ok := sections["first-heading"]; !ok {
		t.Errorf("missing first-heading section: %v", keysOfStringMap2(sections))
	}
	if _, ok := sections["second-heading"]; !ok {
		t.Errorf("missing second-heading section: %v", keysOfStringMap2(sections))
	}
	// Small bodies do not split.
	if splitSkillSections("## Tiny\nshort") != nil {
		t.Error("small body should not be split")
	}
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func keysOfStringMap(m map[string]SkillConfig) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func keysOfStringMap2(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
