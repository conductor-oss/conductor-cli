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
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
)

// Workspace root kinds and the reserved default root name.
const (
	KindWorkspace     = "workspace"  // the primary, editable working directory
	KindFilesystem    = "filesystem" // an additional named read-only root
	workspaceRootName = "workspace"  // Root("") and AGENTSPAN_WORKSPACE_DIR resolve to this
)

// defaultTextReadLimit bounds a single text read when no limit is supplied.
const defaultTextReadLimit = 1 << 20 // 1 MiB

// workspaceRootNamePattern constrains a root name to a filesystem/env-safe set.
var workspaceRootNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// WorkspaceConfig is the set of local roots the workspace tools expose. It is
// resolved from flags in the cmd layer and passed to the handlers; the server only
// ever sees the wire form (WireConfig) and tool outputs, never these paths.
type WorkspaceConfig struct {
	Enabled bool
	Roots   []WorkspaceRoot
}

// WorkspaceRoot is one named, absolute, symlink-resolved local root.
type WorkspaceRoot struct {
	Name string
	Path string
	Kind string
}

// WorkspaceWire is the workspace section of a skill config as sent to the server:
// the enable flag and the root names/kinds only — never their local paths.
type WorkspaceWire struct {
	Enabled bool                `json:"enabled"`
	Roots   []WorkspaceRootWire `json:"roots,omitempty"`
}

// WorkspaceRootWire is one named workspace root on the wire (no path).
type WorkspaceRootWire struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

// NewWorkspaceRoot validates and resolves one root. The name must be
// filesystem/env-safe and the path must be an existing directory; the returned
// path is absolute and symlink-resolved. File I/O is expected here — this is the
// local CLI runtime layer.
func NewWorkspaceRoot(name, pathValue, kind string) (WorkspaceRoot, error) {
	if !workspaceRootNamePattern.MatchString(name) {
		return WorkspaceRoot{}, fmt.Errorf("invalid filesystem root name %q: use letters, numbers, dot, underscore, or dash", name)
	}
	absPath, err := filepath.Abs(pathValue)
	if err != nil {
		return WorkspaceRoot{}, fmt.Errorf("resolve filesystem root %q: %w", name, err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return WorkspaceRoot{}, fmt.Errorf("filesystem root %q does not exist: %w", name, err)
	}
	if !info.IsDir() {
		return WorkspaceRoot{}, fmt.Errorf("filesystem root %q is not a directory: %s", name, absPath)
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}
	return WorkspaceRoot{Name: name, Path: absPath, Kind: kind}, nil
}

// Root returns the named root, or the first root for an empty name.
func (c WorkspaceConfig) Root(name string) (WorkspaceRoot, bool) {
	if name == "" && len(c.Roots) > 0 {
		return c.Roots[0], true
	}
	for _, root := range c.Roots {
		if root.Name == name {
			return root, true
		}
	}
	return WorkspaceRoot{}, false
}

// WireConfig produces the server-bound workspace section (no local paths), or nil
// when the workspace is disabled.
func (c WorkspaceConfig) WireConfig() *WorkspaceWire {
	if !c.Enabled {
		return nil
	}
	roots := make([]WorkspaceRootWire, 0, len(c.Roots))
	for _, r := range c.Roots {
		roots = append(roots, WorkspaceRootWire{Name: r.Name, Kind: r.Kind})
	}
	return &WorkspaceWire{Enabled: true, Roots: roots}
}

// safeWorkspacePath joins relPath onto absRoot and rejects any path that escapes
// the root (after resolving symlinks) — the workspace security boundary.
func safeWorkspacePath(absRoot, relPath string) (string, error) {
	cleanRel := filepath.Clean(filepath.FromSlash(defaultString(relPath, ".")))
	if filepath.IsAbs(cleanRel) || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("'%s' is outside the filesystem root", relPath)
	}
	target := filepath.Join(absRoot, cleanRel)
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		resolvedRoot = absRoot
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		resolvedTarget = target
	}
	if rel, err := filepath.Rel(resolvedRoot, resolvedTarget); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("'%s' is outside the filesystem root", relPath)
	}
	return resolvedTarget, nil
}

// resolvedWorkspaceRootPath returns absRoot with symlinks resolved (used to compute
// relative paths consistently with safeWorkspacePath's resolved targets).
func resolvedWorkspaceRootPath(absRoot string) string {
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		return resolved
	}
	return absRoot
}

// readLimitedTextFile reads up to limit bytes and reports whether it truncated.
func readLimitedTextFile(pathValue string, limit int) (string, bool, error) {
	if limit <= 0 {
		limit = defaultTextReadLimit
	}
	file, err := os.Open(pathValue)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return "", false, err
	}
	truncated := len(data) > limit
	if truncated {
		data = data[:limit]
	}
	return string(data), truncated, nil
}

// shouldSkipWorkspaceDir reports directories the workspace walk never descends into.
func shouldSkipWorkspaceDir(name string) bool {
	switch name {
	case ".git", "node_modules", "__pycache__", ".venv", "venv", ".tox", "dist", "build", "target", ".gradle", ".idea", ".mypy_cache", ".pytest_cache":
		return true
	default:
		return false
	}
}

// matchesWorkspacePattern reports whether relPath matches a glob (supporting **),
// or true when the pattern is empty.
func matchesWorkspacePattern(pattern, relPath string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return true
	}
	relPath = filepath.ToSlash(relPath)
	if ok, err := pathpkg.Match(pattern, relPath); err == nil && ok {
		return true
	}
	if strings.Contains(pattern, "**") {
		re := regexp.QuoteMeta(pattern)
		re = strings.ReplaceAll(re, `\*\*`, `.*`)
		re = strings.ReplaceAll(re, `\*`, `[^/]*`)
		re = strings.ReplaceAll(re, `\?`, `[^/]`)
		ok, err := regexp.MatchString("^"+re+"$", relPath)
		return err == nil && ok
	}
	if strings.HasPrefix(pattern, "*") || strings.HasSuffix(pattern, "*") {
		return strings.Contains(relPath, strings.Trim(pattern, "*"))
	}
	return relPath == pattern
}

// trimLongLine caps a line length for search-match display.
func trimLongLine(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...[truncated]"
}

// defaultString returns fallback when value is blank.
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
