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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal/skill"
	"github.com/conductor-oss/conductor-cli/internal/skillworker"
)

func TestResolveSkillWorkspaceConfig(t *testing.T) {
	wsDir := t.TempDir()
	extraDir := t.TempDir()

	skillNoWorkspace = false
	skillWorkspaceDir = wsDir
	skillFileSystems = []string{"docs=" + extraDir}
	t.Cleanup(func() { skillFileSystems = nil })

	cfg, err := resolveSkillWorkspaceConfig()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !cfg.Enabled || len(cfg.Roots) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if _, ok := cfg.Root("workspace"); !ok {
		t.Error("missing default workspace root")
	}
	if _, ok := cfg.Root("docs"); !ok {
		t.Error("missing named filesystem root")
	}

	// --no-workspace with no filesystems disables the workspace.
	skillNoWorkspace = true
	skillFileSystems = nil
	cfg, err = resolveSkillWorkspaceConfig()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("expected disabled workspace, got %+v", cfg)
	}
	skillNoWorkspace = false

	// Invalid --filesystem spec is rejected.
	skillFileSystems = []string{"bad-spec"}
	if _, err := resolveSkillWorkspaceConfig(); err == nil {
		t.Error("expected an error for a malformed --filesystem value")
	}
	skillFileSystems = nil
}

func TestParseParamOverrides(t *testing.T) {
	out, err := parseParamOverrides([]string{"tone=terse", "verbose=true", "keep=false"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out["tone"].format() != "terse" {
		t.Errorf("tone = %q", out["tone"].format())
	}
	if out["verbose"].format() != "true" {
		t.Errorf("verbose = %q", out["verbose"].format())
	}
	// bool values marshal as JSON bools, not strings.
	b, _ := json.Marshal(out["verbose"])
	if string(b) != "true" {
		t.Errorf("verbose JSON = %s", b)
	}
	if _, err := parseParamOverrides([]string{"noequals"}); err == nil {
		t.Error("expected an error for a param without '='")
	}
}

func TestBuildSkillWorkerRegistry(t *testing.T) {
	cfg := SkillConfig{
		ResourceFiles: []string{"notes.txt"},
		Scripts:       map[string]ScriptInfo{"greet": {Filename: "greet.sh", Language: skillworker.LangBash}},
		CrossSkillRefs: map[string]SkillConfig{
			"helper": {Scripts: map[string]ScriptInfo{"aid": {Filename: "aid.sh", Language: skillworker.LangBash}}},
		},
	}
	local := LocalContext{
		SkillName:   "main",
		SkillDir:    t.TempDir(),
		Sections:    map[string]string{"intro": "## Intro"},
		CrossSkills: map[string]LocalContext{"helper": {SkillName: "helper", SkillDir: t.TempDir()}},
	}
	ws := skillworker.WorkspaceConfig{Enabled: true, Roots: []skillworker.WorkspaceRoot{{Name: "workspace", Path: t.TempDir(), Kind: skillworker.KindWorkspace}}}

	reg := buildSkillWorkerRegistry(cfg, local, ws, skillworker.ScriptOptions{}, 1<<20)

	for _, taskType := range []string{
		"main__read_skill_file", "main__greet",
		"main__list_workspace_files", "main__read_workspace_file",
		"main__search_workspace", "main__git_status", "main__git_diff",
		"helper__read_skill_file", "helper__aid",
	} {
		if _, ok := reg[taskType]; !ok {
			t.Errorf("registry missing task type %q (have %v)", taskType, registryKeys(reg))
		}
	}
}

func TestBuildSkillWorkerRegistryNoWorkspace(t *testing.T) {
	cfg := SkillConfig{Scripts: map[string]ScriptInfo{"greet": {Filename: "greet.sh", Language: skillworker.LangBash}}}
	local := LocalContext{SkillName: "main", SkillDir: t.TempDir()}
	reg := buildSkillWorkerRegistry(cfg, local, skillworker.WorkspaceConfig{}, skillworker.ScriptOptions{}, 1<<20)

	if _, ok := reg["main__list_workspace_files"]; ok {
		t.Error("workspace tools should not be registered when the workspace is disabled")
	}
	if _, ok := reg["main__read_skill_file"]; !ok {
		t.Error("read_skill_file should always be registered")
	}
}

// ---- materialize ----

type fakeSkillService struct {
	detail    skill.Detail
	pkg       []byte
	downloads int
}

func (f *fakeSkillService) List(context.Context, bool) ([]skill.Summary, error) { return nil, nil }
func (f *fakeSkillService) Get(context.Context, string, string) (skill.Detail, error) {
	return f.detail, nil
}
func (f *fakeSkillService) DownloadPackage(context.Context, string, string) ([]byte, error) {
	f.downloads++
	return f.pkg, nil
}
func (f *fakeSkillService) Register(context.Context, json.RawMessage, []byte) (skill.Detail, error) {
	return skill.Detail{}, nil
}
func (f *fakeSkillService) Delete(context.Context, string, string) error { return nil }

func TestMaterializeLocalDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, detail, err := materializeSkill(context.Background(), &fakeSkillService{}, dir, "")
	if err != nil {
		t.Fatalf("materialize local: %v", err)
	}
	if got != dir || detail != nil {
		t.Errorf("local dir should be used as-is: got=%q detail=%v", got, detail)
	}
}

func TestMaterializeRegisteredSkillCaches(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate the config-home cache

	pkg := skillZip(t, map[string]string{"SKILL.md": "---\nname: reg\n---\nbody"})
	sum := sha256.Sum256(pkg)
	svc := &fakeSkillService{
		detail: skill.Detail{Name: "reg", Version: "1", Checksum: hex.EncodeToString(sum[:])},
		pkg:    pkg,
	}

	dir, detail, err := materializeSkill(context.Background(), svc, "reg", "")
	if err != nil {
		t.Fatalf("materialize registered: %v", err)
	}
	if detail == nil || detail.Name != "reg" {
		t.Fatalf("detail = %+v", detail)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("cached skill missing SKILL.md: %v", err)
	}
	if svc.downloads != 1 {
		t.Errorf("expected 1 download, got %d", svc.downloads)
	}

	// Second call hits the cache — no additional download.
	dir2, _, err := materializeSkill(context.Background(), svc, "reg", "")
	if err != nil {
		t.Fatalf("materialize (cache hit): %v", err)
	}
	if dir2 != dir {
		t.Errorf("cache-hit dir = %q, want %q", dir2, dir)
	}
	if svc.downloads != 1 {
		t.Errorf("cache hit re-downloaded: downloads = %d", svc.downloads)
	}
}

func TestMaterializeChecksumMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := &fakeSkillService{
		detail: skill.Detail{Name: "reg", Version: "1", Checksum: "deadbeef"},
		pkg:    skillZip(t, map[string]string{"SKILL.md": "---\nname: reg\n---\n"}),
	}
	if _, _, err := materializeSkill(context.Background(), svc, "reg", ""); err == nil {
		t.Fatal("expected a checksum mismatch error")
	}
}

// skillZip builds an in-memory skill package zip from path→content.
func skillZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func registryKeys(reg map[string]skillworker.ToolHandler) []string {
	keys := make([]string, 0, len(reg))
	for k := range reg {
		keys = append(keys, k)
	}
	return keys
}
