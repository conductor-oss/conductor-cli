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

package skillworker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func mustWrite(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// handle is a test helper that invokes a handler and returns the raw output.
func handle(t *testing.T, h ToolHandler, input string) (json.RawMessage, error) {
	t.Helper()
	return h.Handle(context.Background(), json.RawMessage(input))
}

// ---- read_skill_file ----

func TestReadSkillFileServesResourceAndSection(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "references/guide.md", "the guide")
	h := NewReadSkillFileHandler(dir, []string{"references/guide.md"}, map[string]string{"intro": "## Intro\nbody"})

	out, err := handle(t, h, `{"path":"references/guide.md"}`)
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	if unquote(t, out) != "the guide" {
		t.Errorf("resource body = %s", out)
	}

	out, err = handle(t, h, `{"path":"skill_section:intro"}`)
	if err != nil {
		t.Fatalf("read section: %v", err)
	}
	if unquote(t, out) != "## Intro\nbody" {
		t.Errorf("section body = %s", out)
	}
}

func TestReadSkillFileRejectsUnknownAndTraversal(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "SKILL.md", "secret")
	h := NewReadSkillFileHandler(dir, []string{"notes.txt"}, nil)

	// Unknown path → soft ERROR result (task completes), not a hard failure.
	out, err := handle(t, h, `{"path":"../../etc/passwd"}`)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if !strings.HasPrefix(unquote(t, out), "ERROR:") {
		t.Errorf("expected ERROR result, got %s", out)
	}

	// Missing path → hard error (task fails).
	if _, err := handle(t, h, `{}`); err == nil {
		t.Error("expected hard error for missing path")
	}
}

// TestSafeSkillPathBlocksEscape directly exercises the skill-dir boundary.
func TestSafeSkillPathBlocksEscape(t *testing.T) {
	dir := t.TempDir()
	if _, err := safeSkillPath(dir, "../outside"); err == nil {
		t.Error("expected traversal to be rejected")
	}
	mustWrite(t, dir, "ok.txt", "x")
	if _, err := safeSkillPath(dir, "ok.txt"); err != nil {
		t.Errorf("legitimate path rejected: %v", err)
	}
}

// ---- workspace: path safety ----

func TestSafeWorkspacePathTraversalAndSymlink(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "inside.txt", "x")
	if _, err := safeWorkspacePath(root, "inside.txt"); err != nil {
		t.Errorf("inside path rejected: %v", err)
	}
	if _, err := safeWorkspacePath(root, "../escape"); err == nil {
		t.Error("expected ../escape to be rejected")
	}
	if _, err := safeWorkspacePath(root, "/etc/passwd"); err == nil {
		t.Error("expected absolute path to be rejected")
	}

	// A symlink pointing outside the root must be rejected.
	outside := t.TempDir()
	mustWrite(t, outside, "secret.txt", "top secret")
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := safeWorkspacePath(root, "link"); err == nil {
		t.Error("expected symlink escaping the root to be rejected")
	}
}

// ---- workspace: list / read / search ----

func TestWorkspaceListReadSearch(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "a.txt", "hello world\nsecond line")
	mustWrite(t, root, "sub/b.md", "another file with hello")
	mustWrite(t, root, "node_modules/skip.js", "should be skipped")
	ws := WorkspaceConfig{Enabled: true, Roots: []WorkspaceRoot{{Name: "workspace", Path: root, Kind: KindWorkspace}}}

	// list
	out, err := handle(t, NewListWorkspaceFilesHandler(ws), `{}`)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var lr listResult
	mustUnmarshal(t, out, &lr)
	if !containsStr(lr.Files, "a.txt") || !containsStr(lr.Files, "sub/b.md") {
		t.Errorf("list missing files: %v", lr.Files)
	}
	if containsStr(lr.Files, "node_modules/skip.js") {
		t.Errorf("list should skip node_modules: %v", lr.Files)
	}

	// read
	out, err = handle(t, NewReadWorkspaceFileHandler(ws, 1<<20), `{"path":"a.txt"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var rr readResult
	mustUnmarshal(t, out, &rr)
	if !strings.Contains(rr.Content, "hello world") {
		t.Errorf("read content = %q", rr.Content)
	}

	// search (case-insensitive default) finds matches in both files
	out, err = handle(t, NewSearchWorkspaceHandler(ws, 1<<20), `{"query":"HELLO"}`)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var sr searchResult
	mustUnmarshal(t, out, &sr)
	if len(sr.Matches) < 2 {
		t.Errorf("expected >=2 matches, got %d: %+v", len(sr.Matches), sr.Matches)
	}
}

func TestWorkspaceUnknownRoot(t *testing.T) {
	ws := WorkspaceConfig{Enabled: true, Roots: []WorkspaceRoot{{Name: "workspace", Path: t.TempDir(), Kind: KindWorkspace}}}
	if _, err := handle(t, NewReadWorkspaceFileHandler(ws, 1<<20), `{"root":"nope","path":"x"}`); err == nil {
		t.Error("expected unknown-root error")
	}
}

// ---- scripts ----

func TestScriptHandlerRunsAndLimitsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test is POSIX-only")
	}
	dir := t.TempDir()
	mustWrite(t, dir, "SKILL.md", "---\nname: s\n---\nbody")
	mustWrite(t, dir, "scripts/echo.sh", "#!/bin/bash\necho hello-from-script")
	scriptPath := filepath.Join(dir, "scripts", "echo.sh")

	out, err := handle(t, NewScriptHandler(scriptPath, LangBash, WorkspaceConfig{}, ScriptOptions{}), `{"command":""}`)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	if !strings.Contains(unquote(t, out), "hello-from-script") {
		t.Errorf("script output = %s", out)
	}

	// Output limit truncates.
	limited := NewScriptHandler(scriptPath, LangBash, WorkspaceConfig{}, ScriptOptions{OutputLimit: 4})
	out, err = handle(t, limited, `{"command":""}`)
	if err != nil {
		t.Fatalf("script (limited): %v", err)
	}
	if !strings.Contains(unquote(t, out), "output truncated") {
		t.Errorf("expected truncation marker, got %s", out)
	}
}

func TestScriptHandlerTimeoutFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test is POSIX-only")
	}
	dir := t.TempDir()
	mustWrite(t, dir, "scripts/sleep.sh", "#!/bin/bash\nsleep 5")
	scriptPath := filepath.Join(dir, "scripts", "sleep.sh")

	_, err := handle(t, NewScriptHandler(scriptPath, LangBash, WorkspaceConfig{}, ScriptOptions{Timeout: 100 * time.Millisecond}), `{"command":""}`)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %v, want a timeout", err)
	}
}

// ---- helpers ----

func TestFlexIntAndBoolDecode(t *testing.T) {
	var li listInput
	if err := json.Unmarshal([]byte(`{"limit":"250"}`), &li); err != nil {
		t.Fatal(err)
	}
	if int(li.Limit) != 250 {
		t.Errorf("flexInt string = %d", li.Limit)
	}
	if err := json.Unmarshal([]byte(`{"limit":42}`), &li); err != nil {
		t.Fatal(err)
	}
	if int(li.Limit) != 42 {
		t.Errorf("flexInt number = %d", li.Limit)
	}

	var gd gitDiffInput
	if err := json.Unmarshal([]byte(`{"staged":"true"}`), &gd); err != nil {
		t.Fatal(err)
	}
	if !bool(gd.Staged) {
		t.Error("flexBool string 'true' should decode true")
	}
}

func TestApplyLimit(t *testing.T) {
	if got := applyLimit(0, 500, 5000); got != 500 {
		t.Errorf("default = %d", got)
	}
	if got := applyLimit(10000, 500, 5000); got != 5000 {
		t.Errorf("cap = %d", got)
	}
	if got := applyLimit(300, 500, 5000); got != 300 {
		t.Errorf("passthrough = %d", got)
	}
}

func unquote(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("output not a JSON string: %s", raw)
	}
	return s
}

func mustUnmarshal(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
