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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/conductor-oss/conductor-cli/internal"
)

// skillPackageFile is the per-file manifest entry sent to the server.
type skillPackageFile struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType"`
}

type skillPackageSource struct {
	FullPath string
	RelPath  string
	Mode     os.FileMode
}

// skillManifest is the JSON manifest uploaded alongside the package zip. It is a
// typed struct so the wire shape is explicit and no map crosses a layer boundary.
type skillManifest struct {
	Name        string             `json:"name"`
	Version     string             `json:"version,omitempty"`
	Description string             `json:"description,omitempty"`
	Metadata    any                `json:"metadata,omitempty"`
	Model       string             `json:"model,omitempty"`
	AgentModels map[string]string  `json:"agentModels,omitempty"`
	Files       []skillPackageFile `json:"files"`
}

var (
	skillRegisterVersion     string
	skillRegisterModel       string
	skillRegisterAgentModels []string
)

var skillRegisterCmd = &cobra.Command{
	Use:          "register <path>",
	Short:        "Package and register a local skill with the server",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if !isSkillDirectory(path) {
			return fmt.Errorf("%q is not a skill directory (SKILL.md not found)", path)
		}
		pkg, files, err := buildSkillPackage(path)
		if err != nil {
			return err
		}
		manifest, err := buildSkillManifest(path, skillRegisterVersion, skillRegisterModel, skillRegisterAgentModels, files)
		if err != nil {
			return err
		}
		detail, err := internal.GetSkillService().Register(cmd.Context(), manifest, pkg)
		if err != nil {
			return err
		}
		fmt.Printf("Skill %s registered as version %s.\n", detail.Name, detail.Version)
		return nil
	},
}

// buildSkillManifest reads SKILL.md frontmatter and assembles the upload manifest.
func buildSkillManifest(skillDir, version, model string, agentModelFlags []string, files []skillPackageFile) (json.RawMessage, error) {
	data, err := os.ReadFile(filepath.Join(expandUserPath(skillDir), skillMarkdownFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", skillMarkdownFile, err)
	}
	frontmatter, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, err
	}
	name, _ := frontmatter["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("%s is missing the required 'name' field in its frontmatter", skillMarkdownFile)
	}
	description, _ := frontmatter["description"].(string)
	agentModels, err := parseAgentModelFlags(agentModelFlags)
	if err != nil {
		return nil, err
	}
	manifest := skillManifest{
		Name:        name,
		Version:     version,
		Description: description,
		Metadata:    frontmatter["metadata"],
		Model:       model,
		AgentModels: agentModels,
		Files:       files,
	}
	return json.Marshal(manifest)
}

const skillMarkdownFile = "SKILL.md"

// isSkillDirectory reports whether path is a directory containing a SKILL.md.
func isSkillDirectory(path string) bool {
	expanded := expandUserPath(path)
	info, err := os.Stat(expanded)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(expanded, skillMarkdownFile))
	return err == nil
}

// buildSkillPackage zips a skill directory deterministically and returns the bytes
// plus the per-file manifest. Secret files and ignored paths are excluded.
func buildSkillPackage(skillPath string) ([]byte, []skillPackageFile, error) {
	absPath, err := filepath.Abs(expandUserPath(skillPath))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve path: %w", err)
	}
	if absPath, err = filepath.EvalSymlinks(absPath); err != nil {
		return nil, nil, fmt.Errorf("resolve path: %w", err)
	}
	ignore, err := loadSkillPackageIgnore(absPath)
	if err != nil {
		return nil, nil, err
	}

	var sources []skillPackageSource
	walkErr := filepath.WalkDir(absPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(absPath, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if shouldExcludeSkillPackagePath(rel, true, ignore) || isDefaultGeneratedSkillDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !info.Mode().IsRegular() || shouldExcludeSkillPackagePath(rel, false, ignore) {
			return nil
		}
		sources = append(sources, skillPackageSource{FullPath: path, RelPath: rel, Mode: info.Mode()})
		return nil
	})
	if walkErr != nil {
		return nil, nil, fmt.Errorf("walk skill directory: %w", walkErr)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].RelPath < sources[j].RelPath })

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := make([]skillPackageFile, 0, len(sources))
	for _, src := range sources {
		data, readErr := os.ReadFile(src.FullPath)
		if readErr != nil {
			_ = zw.Close()
			return nil, nil, fmt.Errorf("read %s: %w", src.RelPath, readErr)
		}
		header := &zip.FileHeader{Name: src.RelPath, Method: zip.Deflate, Modified: time.Unix(0, 0).UTC()}
		header.SetMode(src.Mode.Perm())
		w, hErr := zw.CreateHeader(header)
		if hErr != nil {
			_ = zw.Close()
			return nil, nil, fmt.Errorf("create zip entry %s: %w", src.RelPath, hErr)
		}
		if _, wErr := w.Write(data); wErr != nil {
			_ = zw.Close()
			return nil, nil, fmt.Errorf("write zip entry %s: %w", src.RelPath, wErr)
		}
		sum := sha256.Sum256(data)
		files = append(files, skillPackageFile{
			Path:        src.RelPath,
			Size:        int64(len(data)),
			SHA256:      hex.EncodeToString(sum[:]),
			ContentType: guessContentType(src.RelPath),
		})
	}
	if err := zw.Close(); err != nil {
		return nil, nil, fmt.Errorf("close skill package: %w", err)
	}
	return buf.Bytes(), files, nil
}

// --- frontmatter ---

// parseFrontmatter extracts the YAML frontmatter (delimited by ---) from SKILL.md.
func parseFrontmatter(content string) (map[string]any, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("%s does not start with YAML frontmatter (---)", skillMarkdownFile)
	}
	rest := strings.TrimPrefix(content[3:], "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("%s frontmatter is not closed (missing second ---)", skillMarkdownFile)
	}
	var result map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &result); err != nil {
		return nil, fmt.Errorf("invalid YAML in frontmatter: %w", err)
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

// parseAgentModelFlags parses repeated name=model overrides.
func parseAgentModelFlags(flags []string) (map[string]string, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(flags))
	for _, f := range flags {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --agent-model value %q: expected name=model", f)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

// --- ignore matching ---

type skillPackageIgnoreMatcher struct {
	patterns []string
}

const skillIgnoreFile = ".agentspanignore"

func loadSkillPackageIgnore(skillRoot string) (skillPackageIgnoreMatcher, error) {
	matcher := skillPackageIgnoreMatcher{}
	data, err := os.ReadFile(filepath.Join(skillRoot, skillIgnoreFile))
	if err != nil {
		if os.IsNotExist(err) {
			return matcher, nil
		}
		return matcher, fmt.Errorf("read %s: %w", skillIgnoreFile, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			matcher.patterns = append(matcher.patterns, filepath.ToSlash(line))
		}
	}
	return matcher, nil
}

func shouldExcludeSkillPackagePath(relPath string, isDir bool, matcher skillPackageIgnoreMatcher) bool {
	relPath = filepath.ToSlash(strings.TrimPrefix(relPath, "./"))
	if relPath == "" || relPath == "." {
		return false
	}
	base := pathpkg.Base(relPath)
	if base == skillIgnoreFile || isDefaultSecretSkillFile(base) {
		return true
	}
	if isDir && isDefaultGeneratedSkillDir(base) {
		return true
	}
	return matcher.matches(relPath, isDir)
}

func (m skillPackageIgnoreMatcher) matches(relPath string, isDir bool) bool {
	for _, pattern := range m.patterns {
		if skillIgnorePatternMatches(pattern, relPath, isDir) {
			return true
		}
	}
	return false
}

func skillIgnorePatternMatches(pattern, relPath string, isDir bool) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" || strings.HasPrefix(pattern, "!") {
		return false
	}
	dirOnly := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	if dirOnly && !isDir {
		return relPath == pattern || strings.HasPrefix(relPath, pattern+"/")
	}
	if strings.Contains(pattern, "/") {
		if ok, err := pathpkg.Match(pattern, relPath); err == nil && ok {
			return true
		}
		return relPath == pattern || strings.HasPrefix(relPath, pattern+"/")
	}
	for _, part := range strings.Split(relPath, "/") {
		if ok, err := pathpkg.Match(pattern, part); err == nil && ok {
			return true
		}
	}
	return false
}

func isDefaultGeneratedSkillDir(name string) bool {
	switch name {
	case ".git", "__pycache__", "node_modules", ".venv", "venv", ".tox", "dist", "build", "target", ".gradle", ".pytest_cache", ".mypy_cache":
		return true
	default:
		return false
	}
}

func isDefaultSecretSkillFile(name string) bool {
	lower := strings.ToLower(name)
	if lower == ".ds_store" || lower == ".env" || strings.HasPrefix(lower, ".env.") {
		return true
	}
	switch lower {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "known_hosts":
		return true
	}
	for _, suffix := range []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".crt", ".cer"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func guessContentType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".txt"):
		return "text/plain"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"):
		return "application/yaml"
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		return "text/html"
	case strings.HasSuffix(lower, ".css"):
		return "text/css"
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".mjs"):
		return "text/javascript"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

func init() {
	skillRegisterCmd.Flags().StringVar(&skillRegisterVersion, "version", "", "Optional version label (defaults to a content hash on the server)")
	skillRegisterCmd.Flags().StringVar(&skillRegisterModel, "model", "", "Orchestrator and default model")
	skillRegisterCmd.Flags().StringArrayVar(&skillRegisterAgentModels, "agent-model", nil, "Sub-agent model override (name=model, repeatable)")
	skillCmd.AddCommand(skillRegisterCmd)
}
