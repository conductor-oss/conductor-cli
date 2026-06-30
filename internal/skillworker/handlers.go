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
	"fmt"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tool names (the "{tool}" half of a "{skillName}__{tool}" task type). Shared with
// the cmd registry that maps them to these handlers.
const (
	ToolReadSkillFile     = "read_skill_file"
	ToolListWorkspace     = "list_workspace_files"
	ToolReadWorkspaceFile = "read_workspace_file"
	ToolSearchWorkspace   = "search_workspace"
	ToolGitStatus         = "git_status"
	ToolGitDiff           = "git_diff"
)

// Script languages — the vocabulary produced by the payload builder's extension
// table and consumed by executeScript, kept in one place to avoid drift.
const (
	LangPython = "python"
	LangNode   = "node"
	LangRuby   = "ruby"
	LangGo     = "go"
	LangBatch  = "batch"
	LangBash   = "bash"
)

// Script execution defaults and the script-facing environment variables. The
// AGENTSPAN_* names are a stable skill-script contract (a script reads them to find
// its skill dir and workspace roots), so they are kept verbatim as named constants.
const (
	defaultScriptTimeout     = 300 * time.Second
	defaultScriptOutputLimit = 10 << 20 // 10 MiB

	envSkillDir             = "AGENTSPAN_SKILL_DIR"
	envWorkspaceDir         = "AGENTSPAN_WORKSPACE_DIR"
	envFilesystemRootPrefix = "AGENTSPAN_FILESYSTEM_ROOT_"
)

// Workspace tool limits and the git command timeout.
const (
	listDefaultLimit   = 500
	listMaxLimit       = 5000
	readMaxLimit       = 5 << 20 // 5 MiB
	searchDefaultLimit = 100
	searchMaxLimit     = 1000
	searchLineMax      = 500
	gitCommandTimeout  = 30 * time.Second
)

// skillSectionPrefix marks a read_skill_file request for a pre-split SKILL.md
// section ("skill_section:{slug}") rather than a resource file.
const skillSectionPrefix = "skill_section:"

// toolFunc adapts a function to ToolHandler.
type toolFunc func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)

func (f toolFunc) Handle(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	return f(ctx, input)
}

// ---- read_skill_file ----

type readSkillFileInput struct {
	Path string `json:"path"`
}

// NewReadSkillFileHandler serves skill resource files and pre-split SKILL.md
// sections. Only paths in resourceFiles (and "skill_section:{slug}" for each known
// section) are allowed; expected failures (unknown/unreadable path) are returned as
// an "ERROR: ..." result string so the agent can see them, matching the original.
func NewReadSkillFileHandler(skillDir string, resourceFiles []string, sections map[string]string) ToolHandler {
	allowed := make(map[string]bool, len(resourceFiles)+len(sections))
	for _, f := range resourceFiles {
		allowed[f] = true
	}
	for name := range sections {
		allowed[skillSectionPrefix+name] = true
	}
	return toolFunc(func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in readSkillFileInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("decode input: %w", err)
		}
		if in.Path == "" {
			return nil, fmt.Errorf("missing 'path' parameter")
		}
		path := normalizeSkillResourcePath(in.Path)
		if !allowed[path] {
			return jsonString(fmt.Sprintf("ERROR: '%s' not found. Available: %v", path, sortedKeys(allowed)))
		}
		if strings.HasPrefix(path, skillSectionPrefix) {
			name := strings.TrimPrefix(path, skillSectionPrefix)
			if section, ok := sections[name]; ok {
				return jsonString(section)
			}
			return jsonString(fmt.Sprintf("ERROR: section '%s' not found", name))
		}
		fullPath, err := safeSkillPath(skillDir, path)
		if err != nil {
			return jsonString(fmt.Sprintf("ERROR: %v", err))
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return jsonString(fmt.Sprintf("ERROR: failed to read '%s': %v", path, err))
		}
		return jsonString(string(data))
	})
}

func normalizeSkillResourcePath(path string) string {
	if strings.HasPrefix(path, skillSectionPrefix) {
		return path
	}
	return pathpkg.Clean(strings.ReplaceAll(path, "\\", "/"))
}

// safeSkillPath joins relPath onto absSkillDir, rejecting escapes (skill-dir
// boundary, symlink-resolved).
func safeSkillPath(absSkillDir, relPath string) (string, error) {
	cleanRel := filepath.Clean(relPath)
	if filepath.IsAbs(cleanRel) || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("'%s' is outside the skill directory", relPath)
	}
	target := filepath.Join(absSkillDir, cleanRel)
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return target, nil // path may not exist yet; join is already contained
	}
	resolvedDir, err := filepath.EvalSymlinks(absSkillDir)
	if err != nil {
		resolvedDir = absSkillDir
	}
	if rel, err := filepath.Rel(resolvedDir, resolvedTarget); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("'%s' is outside the skill directory", relPath)
	}
	return resolvedTarget, nil
}

// ---- scripts ----

type scriptInput struct {
	Command string `json:"command"`
}

// ScriptOptions bounds a script execution (zero values fall back to the defaults).
type ScriptOptions struct {
	Timeout     time.Duration
	OutputLimit int
}

// NewScriptHandler runs one skill script. Script failure is returned as an error
// whose message includes the captured output, so the failed task's reason carries
// the diagnostic output.
func NewScriptHandler(scriptPath, language string, ws WorkspaceConfig, opts ScriptOptions) ToolHandler {
	return toolFunc(func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in scriptInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("decode input: %w", err)
		}
		out, err := executeScript(ctx, scriptPath, language, in.Command, ws, opts)
		if err != nil {
			return nil, err
		}
		return jsonString(out)
	})
}

func executeScript(ctx context.Context, scriptPath, language, command string, ws WorkspaceConfig, opts ScriptOptions) (string, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultScriptTimeout
	}
	outputLimit := opts.OutputLimit
	if outputLimit <= 0 {
		outputLimit = defaultScriptOutputLimit
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := buildScriptCmd(ctx, language, scriptPath, command)
	if skillRoot := skillRootFromScriptPath(scriptPath); skillRoot != "" {
		cmd.Dir = skillRoot
		cmd.Env = scriptEnv(skillRoot, ws)
	}

	output := &limitedOutputBuffer{limit: outputLimit}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	out := output.String()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("script timed out after %s\n%s", timeout, out)
		}
		return out, fmt.Errorf("script failed: %w\n%s", err, out)
	}
	return out, nil
}

func buildScriptCmd(ctx context.Context, language, scriptPath, command string) *exec.Cmd {
	args := splitCommandArgs(command)
	switch language {
	case LangPython:
		py := "python3"
		if _, err := exec.LookPath("python3"); err != nil {
			py = "python" // Windows ships python, not python3
		}
		return exec.CommandContext(ctx, py, append([]string{scriptPath}, args...)...)
	case LangNode:
		return exec.CommandContext(ctx, "node", append([]string{scriptPath}, args...)...)
	case LangRuby:
		return exec.CommandContext(ctx, "ruby", append([]string{scriptPath}, args...)...)
	case LangGo:
		return exec.CommandContext(ctx, "go", append([]string{"run", scriptPath}, args...)...)
	case LangBatch:
		return exec.CommandContext(ctx, "cmd", append([]string{"/c", scriptPath}, args...)...)
	default: // LangBash / shell
		if runtime.GOOS != "windows" {
			return exec.CommandContext(ctx, "bash", append([]string{scriptPath}, args...)...)
		}
		if _, err := exec.LookPath("bash"); err == nil {
			return exec.CommandContext(ctx, "bash", append([]string{scriptPath}, args...)...)
		}
		return exec.CommandContext(ctx, "cmd", append([]string{"/c", scriptPath}, args...)...)
	}
}

// skillRootFromScriptPath returns the skill directory for a "…/scripts/foo" path,
// or "" when the script is not under a scripts/ directory.
func skillRootFromScriptPath(scriptPath string) string {
	scriptsDir := filepath.Dir(scriptPath)
	if filepath.Base(scriptsDir) != scriptsDirName {
		return ""
	}
	return filepath.Dir(scriptsDir)
}

// scriptsDirName mirrors the payload builder's scripts directory name.
const scriptsDirName = "scripts"

func scriptEnv(skillRoot string, ws WorkspaceConfig) []string {
	env := append(os.Environ(), envSkillDir+"="+skillRoot)
	if wsRoot, ok := ws.Root(workspaceRootName); ok {
		env = append(env, envWorkspaceDir+"="+wsRoot.Path)
	}
	for _, root := range ws.Roots {
		env = append(env, envFilesystemRootPrefix+filesystemEnvName(root.Name)+"="+root.Path)
	}
	return env
}

var filesystemEnvReplacer = regexp.MustCompile(`[^A-Za-z0-9]+`)

func filesystemEnvName(name string) string {
	return strings.ToUpper(strings.Trim(filesystemEnvReplacer.ReplaceAllString(name, "_"), "_"))
}

// splitCommandArgs splits a command string into arguments, honoring single/double
// quotes and backslash escapes.
func splitCommandArgs(command string) []string {
	var args []string
	var current strings.Builder
	inSingle, inDouble, escaped := false, false, false
	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t' || r == '\n') && !inSingle && !inDouble:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

type limitedOutputBuffer struct {
	buf       strings.Builder
	limit     int
	truncated bool
}

func (b *limitedOutputBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.truncated = true
		_, _ = b.buf.Write(p[:remaining])
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *limitedOutputBuffer) String() string {
	out := b.buf.String()
	if b.truncated {
		out += fmt.Sprintf("\n[output truncated after %d bytes]", b.limit)
	}
	return out
}

// ---- workspace tools ----

type listInput struct {
	Root  string  `json:"root"`
	Path  string  `json:"path"`
	Glob  string  `json:"glob"`
	Limit flexInt `json:"limit"`
}

type listResult struct {
	Root      string   `json:"root"`
	Path      string   `json:"path"`
	Files     []string `json:"files"`
	Truncated bool     `json:"truncated"`
}

// NewListWorkspaceFilesHandler lists files under a workspace root, filtered by glob.
func NewListWorkspaceFilesHandler(ws WorkspaceConfig) ToolHandler {
	return toolFunc(func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in listInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("decode input: %w", err)
		}
		root, err := workspaceRootFromInput(ws, in.Root)
		if err != nil {
			return nil, err
		}
		res, err := listWorkspaceFiles(root, in.Path, in.Glob, applyLimit(int(in.Limit), listDefaultLimit, listMaxLimit))
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	})
}

func listWorkspaceFiles(root WorkspaceRoot, pathValue, pattern string, limit int) (listResult, error) {
	rootPath := resolvedWorkspaceRootPath(root.Path)
	startPath, err := safeWorkspacePath(root.Path, defaultString(pathValue, "."))
	if err != nil {
		return listResult{}, err
	}
	info, err := os.Stat(startPath)
	if err != nil {
		return listResult{}, err
	}
	if !info.IsDir() {
		return listResult{}, fmt.Errorf("path is not a directory: %s", pathValue)
	}

	files := []string{}
	truncated := false
	err = filepath.WalkDir(startPath, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == startPath {
			return nil
		}
		if entry.IsDir() && shouldSkipWorkspaceDir(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootPath, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !matchesWorkspacePattern(pattern, rel) {
			return nil
		}
		files = append(files, rel)
		if limit > 0 && len(files) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return listResult{}, err
	}
	return listResult{Root: root.Name, Path: defaultString(pathValue, "."), Files: files, Truncated: truncated}, nil
}

type readInput struct {
	Root  string  `json:"root"`
	Path  string  `json:"path"`
	Limit flexInt `json:"limit"`
}

type readResult struct {
	Root      string `json:"root"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// NewReadWorkspaceFileHandler reads one workspace file (bounded by fileLimit).
func NewReadWorkspaceFileHandler(ws WorkspaceConfig, fileLimit int) ToolHandler {
	return toolFunc(func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in readInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("decode input: %w", err)
		}
		root, err := workspaceRootFromInput(ws, in.Root)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.Path) == "" {
			return nil, fmt.Errorf("missing 'path' parameter")
		}
		res, err := readWorkspaceFile(root, in.Path, applyLimit(int(in.Limit), fileLimit, readMaxLimit))
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	})
}

func readWorkspaceFile(root WorkspaceRoot, pathValue string, limit int) (readResult, error) {
	rootPath := resolvedWorkspaceRootPath(root.Path)
	fullPath, err := safeWorkspacePath(root.Path, pathValue)
	if err != nil {
		return readResult{}, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return readResult{}, err
	}
	if info.IsDir() {
		return readResult{}, fmt.Errorf("path is a directory: %s", pathValue)
	}
	content, truncated, err := readLimitedTextFile(fullPath, limit)
	if err != nil {
		return readResult{}, err
	}
	rel, _ := filepath.Rel(rootPath, fullPath)
	return readResult{Root: root.Name, Path: filepath.ToSlash(rel), Content: content, Truncated: truncated}, nil
}

type searchInput struct {
	Root       string    `json:"root"`
	Path       string    `json:"path"`
	Glob       string    `json:"glob"`
	Query      string    `json:"query"`
	IgnoreCase *flexBool `json:"ignoreCase"`
	Limit      flexInt   `json:"limit"`
}

type searchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type searchResult struct {
	Root      string        `json:"root"`
	Query     string        `json:"query"`
	Matches   []searchMatch `json:"matches"`
	Truncated bool          `json:"truncated"`
}

// NewSearchWorkspaceHandler does a substring search across a workspace root.
func NewSearchWorkspaceHandler(ws WorkspaceConfig, fileLimit int) ToolHandler {
	return toolFunc(func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in searchInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("decode input: %w", err)
		}
		root, err := workspaceRootFromInput(ws, in.Root)
		if err != nil {
			return nil, err
		}
		if in.Query == "" {
			return nil, fmt.Errorf("missing 'query' parameter")
		}
		res, err := searchWorkspace(root, in.Path, in.Glob, in.Query,
			flexBoolValue(in.IgnoreCase, true), applyLimit(int(in.Limit), searchDefaultLimit, searchMaxLimit), fileLimit)
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	})
}

func searchWorkspace(root WorkspaceRoot, pathValue, pattern, query string, ignoreCase bool, limit, fileLimit int) (searchResult, error) {
	rootPath := resolvedWorkspaceRootPath(root.Path)
	startPath, err := safeWorkspacePath(root.Path, defaultString(pathValue, "."))
	if err != nil {
		return searchResult{}, err
	}
	info, err := os.Stat(startPath)
	if err != nil {
		return searchResult{}, err
	}
	if !info.IsDir() {
		return searchResult{}, fmt.Errorf("path is not a directory: %s", pathValue)
	}

	needle := query
	if ignoreCase {
		needle = strings.ToLower(needle)
	}
	matches := []searchMatch{}
	truncated := false
	err = filepath.WalkDir(startPath, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == startPath {
			return nil
		}
		if entry.IsDir() && shouldSkipWorkspaceDir(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootPath, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !matchesWorkspacePattern(pattern, rel) {
			return nil
		}
		content, _, err := readLimitedTextFile(current, fileLimit)
		if err != nil || strings.Contains(content, "\x00") {
			return nil
		}
		for i, line := range strings.Split(content, "\n") {
			haystack := line
			if ignoreCase {
				haystack = strings.ToLower(haystack)
			}
			if strings.Contains(haystack, needle) {
				matches = append(matches, searchMatch{Path: rel, Line: i + 1, Text: trimLongLine(line, searchLineMax)})
				if limit > 0 && len(matches) >= limit {
					truncated = true
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil {
		return searchResult{}, err
	}
	return searchResult{Root: root.Name, Query: query, Matches: matches, Truncated: truncated}, nil
}

type gitStatusInput struct {
	Root string `json:"root"`
}

type gitDiffInput struct {
	Root   string   `json:"root"`
	Base   string   `json:"base"`
	Path   string   `json:"path"`
	Staged flexBool `json:"staged"`
}

type gitResult struct {
	Root   string `json:"root"`
	Output string `json:"output"`
}

// NewGitStatusHandler runs "git status --short" on a workspace root.
func NewGitStatusHandler(ws WorkspaceConfig, fileLimit int) ToolHandler {
	return toolFunc(func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in gitStatusInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("decode input: %w", err)
		}
		root, err := workspaceRootFromInput(ws, in.Root)
		if err != nil {
			return nil, err
		}
		res, err := runGitWorkspaceCommand(ctx, root, []string{"status", "--short"}, fileLimit)
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	})
}

// NewGitDiffHandler runs "git diff" (optionally staged / against a base / for a
// path) on a workspace root.
func NewGitDiffHandler(ws WorkspaceConfig, fileLimit int) ToolHandler {
	return toolFunc(func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in gitDiffInput
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, fmt.Errorf("decode input: %w", err)
		}
		root, err := workspaceRootFromInput(ws, in.Root)
		if err != nil {
			return nil, err
		}
		args := []string{"diff", "--no-ext-diff", "--color=never"}
		if bool(in.Staged) {
			args = append(args, "--cached")
		}
		if base := strings.TrimSpace(in.Base); base != "" {
			args = append(args, base)
		}
		if pathValue := strings.TrimSpace(in.Path); pathValue != "" {
			fullPath, err := safeWorkspacePath(root.Path, pathValue)
			if err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(resolvedWorkspaceRootPath(root.Path), fullPath)
			if err != nil {
				return nil, err
			}
			args = append(args, "--", filepath.ToSlash(rel))
		}
		res, err := runGitWorkspaceCommand(ctx, root, args, fileLimit)
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	})
}

func runGitWorkspaceCommand(ctx context.Context, root WorkspaceRoot, args []string, limit int) (gitResult, error) {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root.Path}, args...)...)
	output := &limitedOutputBuffer{limit: limit}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	out := output.String()
	if ctx.Err() == context.DeadlineExceeded {
		return gitResult{}, fmt.Errorf("git command timed out after %s\n%s", gitCommandTimeout, out)
	}
	if err != nil {
		return gitResult{}, fmt.Errorf("git command failed: %w\n%s", err, out)
	}
	return gitResult{Root: root.Name, Output: out}, nil
}

func workspaceRootFromInput(ws WorkspaceConfig, rootName string) (WorkspaceRoot, error) {
	if root, ok := ws.Root(rootName); ok {
		return root, nil
	}
	names := make([]string, 0, len(ws.Roots))
	for _, r := range ws.Roots {
		names = append(names, r.Name)
	}
	return WorkspaceRoot{}, fmt.Errorf("unknown filesystem root %q; available: %s", rootName, strings.Join(names, ", "))
}

// ---- shared helpers ----

// jsonString marshals s as a JSON string (the result form for read_skill_file and
// script handlers).
func jsonString(s string) (json.RawMessage, error) {
	b, err := json.Marshal(s)
	return b, err
}

// applyLimit clamps a caller-supplied limit: non-positive falls back to def, and a
// positive max caps the value (mirrors the original intInput semantics).
func applyLimit(value, def, max int) int {
	if value <= 0 {
		value = def
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// flexInt decodes a JSON number or numeric string; anything else (including null)
// decodes to 0, which callers treat as "unset" and replace with a default.
type flexInt int

func (n *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	if v, err := strconv.Atoi(s); err == nil {
		*n = flexInt(v)
		return nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		*n = flexInt(int(f))
		return nil
	}
	*n = 0
	return nil
}

// flexBool decodes a JSON bool or a "true"/"false" string; anything else is false.
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	*b = flexBool(s == "true")
	return nil
}

// flexBoolValue returns p's value, or def when p is nil (absent).
func flexBoolValue(p *flexBool, def bool) bool {
	if p == nil {
		return def
	}
	return bool(*p)
}
