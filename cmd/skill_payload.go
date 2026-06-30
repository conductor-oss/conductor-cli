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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/conductor-oss/conductor-cli/internal/skillworker"
)

// frameworkSkill is the framework marker for skill-backed agent definitions; it is
// the envelope selector sent to /api/agent/deploy and /api/agent/start.
const frameworkSkill = "skill"

// skillSectionSplitBytes is the SKILL.md body size (~50 KB) above which the builder
// pre-splits the instructions into per-heading sections that the read_skill_file
// worker can serve on demand instead of shipping the whole body inline.
const skillSectionSplitBytes = 50 << 10

const (
	scriptsDirName        = "scripts"            // optional executables directory inside a skill
	agentFileSuffix       = "-agent.md"          // sub-agent instruction files: "{name}-agent.md"
	defaultScriptLanguage = skillworker.LangBash // language for scripts with an unknown extension
)

// scriptLanguageByExt maps a script file extension to its execution language. The
// language vocabulary is owned by skillworker (which executes scripts), so the two
// sides cannot drift. Extensions absent here fall back to defaultScriptLanguage.
var scriptLanguageByExt = map[string]string{
	".py":  skillworker.LangPython,
	".sh":  skillworker.LangBash,
	".bat": skillworker.LangBatch,
	".cmd": skillworker.LangBatch,
	".js":  skillworker.LangNode,
	".mjs": skillworker.LangNode,
	".ts":  skillworker.LangNode,
	".rb":  skillworker.LangRuby,
	".go":  skillworker.LangGo,
}

// skillResourceDirs are the sub-directories whose files are exposed as skill
// resources (in addition to eligible root-level files).
var skillResourceDirs = []string{"references", "examples", "assets"}

// agentsSkillsSubpath is the conventional per-project / per-user skill library
// location (".agents/skills") searched when resolving cross-skill references.
var agentsSkillsSubpath = filepath.Join(".agents", "skills")

// crossSkillRefPattern matches prose like "invoke the foo skill" / "use bar skill"
// in a SKILL.md body, capturing the referenced skill name.
var crossSkillRefPattern = regexp.MustCompile(`(?i)(?:invoke|use|call)\s+(?:the\s+)?([a-z][a-z0-9-]*)\s+skill`)

// sectionHeadingPrefix marks a level-2 markdown heading; the body is split into
// sections at each line that begins with it.
const sectionHeadingPrefix = "## "

// SkillConfig is the server-bound skill definition. Every field is serialized to
// the deploy/start rawConfig; there are no local-only fields (paths and pre-split
// sections live in LocalContext instead), so no strip step is needed before the
// wire — the type is the contract.
type SkillConfig struct {
	Model          string                 `json:"model,omitempty"`
	AgentModels    map[string]string      `json:"agentModels,omitempty"`
	SkillMd        string                 `json:"skillMd"`
	AgentFiles     map[string]string      `json:"agentFiles,omitempty"`     // agentName → body
	Scripts        map[string]ScriptInfo  `json:"scripts,omitempty"`        // toolName → {filename,language}
	ResourceFiles  []string               `json:"resourceFiles,omitempty"`  // skill-relative paths
	CrossSkillRefs map[string]SkillConfig `json:"crossSkillRefs,omitempty"` // refName → nested config
	DefaultParams  map[string]ParamValue  `json:"defaultParams,omitempty"`
	Params         map[string]ParamValue  `json:"params,omitempty"`
	// Workspace is set only by the run/serve worker runtime; load never populates it.
	// The wire type is owned by skillworker (which serves the workspace tools).
	Workspace *skillworker.WorkspaceWire `json:"workspace,omitempty"`
}

// ScriptInfo describes one discovered script tool.
type ScriptInfo struct {
	Filename string `json:"filename"`
	Language string `json:"language"`
}

// ParamValue is one skill-parameter value. Skill params are scalars: a --param
// override parses to a bool ("true"/"false") or a string, and a SKILL.md
// frontmatter default carries through its parsed YAML scalar. ParamValue keeps the
// value typed at the wire boundary (SkillConfig holds no bare interface{} map) and
// both marshals to, and stringifies as, its underlying scalar.
type ParamValue struct {
	v any
}

func rawParamValue(v any) ParamValue { return ParamValue{v: v} }

// MarshalJSON emits the underlying scalar.
func (p ParamValue) MarshalJSON() ([]byte, error) { return json.Marshal(p.v) }

// format renders the value for the [Skill Parameters] block injected into SKILL.md.
func (p ParamValue) format() string { return fmt.Sprint(p.v) }

// LocalContext is the never-serialized side of a built skill. It stays on the CLI
// side and feeds the run/serve workers: the skill name is the worker task-type
// prefix, SkillDir roots local file/script tools, Sections lets the read_skill_file
// worker serve "skill_section:{slug}" requests for large bodies, and CrossSkills
// carries the same context for each resolved cross-skill so their script/file
// workers can be started too.
type LocalContext struct {
	SkillName   string
	SkillDir    string
	Sections    map[string]string       // slug → SKILL.md section body
	CrossSkills map[string]LocalContext // refName → nested local context
}

// PayloadOptions carries the caller-resolved (in cmd) inputs the builder needs.
type PayloadOptions struct {
	Model          string
	AgentModels    map[string]string
	SearchPaths    []string
	ParamOverrides map[string]ParamValue
}

// BuildSkillPayload reads a local skill directory and produces its typed wire
// config plus the local-only context. All filesystem access stays here in the cmd
// layer; the returned SkillConfig is pure data ready to marshal for deploy/start.
func BuildSkillPayload(dir string, opts PayloadOptions) (SkillConfig, LocalContext, error) {
	return buildSkillPayloadInternal(dir, opts, map[string]bool{})
}

func buildSkillPayloadInternal(dir string, opts PayloadOptions, seen map[string]bool) (SkillConfig, LocalContext, error) {
	absPath, err := filepath.Abs(expandUserPath(dir))
	if err != nil {
		return SkillConfig{}, LocalContext{}, fmt.Errorf("resolve path: %w", err)
	}
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return SkillConfig{}, LocalContext{}, fmt.Errorf("resolve path: %w", err)
	}

	skillMdContent, err := os.ReadFile(filepath.Join(absPath, skillMarkdownFile))
	if err != nil {
		if os.IsNotExist(err) {
			return SkillConfig{}, LocalContext{}, fmt.Errorf("directory %q is not a valid skill: %s not found", absPath, skillMarkdownFile)
		}
		return SkillConfig{}, LocalContext{}, fmt.Errorf("read %s: %w", skillMarkdownFile, err)
	}

	frontmatter, err := parseFrontmatter(string(skillMdContent))
	if err != nil {
		return SkillConfig{}, LocalContext{}, fmt.Errorf("parse %s frontmatter: %w", skillMarkdownFile, err)
	}
	skillName, _ := frontmatter["name"].(string)
	if skillName == "" {
		return SkillConfig{}, LocalContext{}, fmt.Errorf("%s missing required 'name' field in frontmatter", skillMarkdownFile)
	}

	agentFiles, err := discoverAgentFiles(absPath)
	if err != nil {
		return SkillConfig{}, LocalContext{}, fmt.Errorf("discover agent files: %w", err)
	}
	scripts, err := discoverScripts(absPath)
	if err != nil {
		return SkillConfig{}, LocalContext{}, fmt.Errorf("discover scripts: %w", err)
	}
	resourceFiles, err := collectResourceFiles(absPath)
	if err != nil {
		return SkillConfig{}, LocalContext{}, fmt.Errorf("collect resource files: %w", err)
	}
	crossRefs, crossLocals, err := resolveCrossSkills(string(skillMdContent), absPath, opts, seen)
	if err != nil {
		return SkillConfig{}, LocalContext{}, err
	}

	// Params: frontmatter defaults overlaid with caller overrides, then rendered
	// into SKILL.md so the server-side orchestrator sees them.
	defaultParams := extractDefaultParams(frontmatter)
	mergedParams := mergeParams(defaultParams, opts.ParamOverrides)
	skillMd := string(skillMdContent)
	if len(mergedParams) > 0 {
		skillMd = skillMd + "\n\n" + formatParamMap(mergedParams) + "\n"
	}

	sections := splitSkillSections(extractBody(skillMd))

	cfg := SkillConfig{
		Model:          opts.Model,
		AgentModels:    opts.AgentModels,
		SkillMd:        skillMd,
		AgentFiles:     agentFiles,
		Scripts:        scripts,
		ResourceFiles:  resourceFiles,
		CrossSkillRefs: crossRefs,
		DefaultParams:  defaultParams,
		Params:         mergedParams,
	}
	local := LocalContext{SkillName: skillName, SkillDir: absPath, Sections: sections, CrossSkills: crossLocals}
	return cfg, local, nil
}

// discoverAgentFiles globs "*-agent.md" and maps agent name → file body.
func discoverAgentFiles(skillDir string) (map[string]string, error) {
	matches, err := filepath.Glob(filepath.Join(skillDir, "*"+agentFileSuffix))
	if err != nil {
		return nil, fmt.Errorf("glob agent files: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(matches))
	for _, match := range matches {
		base := filepath.Base(match)
		content, err := os.ReadFile(match)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		result[strings.TrimSuffix(base, agentFileSuffix)] = string(content)
	}
	return result, nil
}

// discoverScripts lists files in scripts/ and maps tool name → script info. The
// directory is optional.
func discoverScripts(skillDir string) (map[string]ScriptInfo, error) {
	entries, err := os.ReadDir(filepath.Join(skillDir, scriptsDirName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read scripts directory: %w", err)
	}
	result := make(map[string]ScriptInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		toolName := strings.TrimSuffix(filename, filepath.Ext(filename))
		if toolName == "" {
			toolName = filename
		}
		result[toolName] = ScriptInfo{Filename: filename, Language: detectScriptLanguage(filename)}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// detectScriptLanguage maps a filename to its language via scriptLanguageByExt,
// defaulting to defaultScriptLanguage.
func detectScriptLanguage(filename string) string {
	if lang, ok := scriptLanguageByExt[strings.ToLower(filepath.Ext(filename))]; ok {
		return lang
	}
	return defaultScriptLanguage
}

// collectResourceFiles lists resource-dir files plus eligible root files (excluding
// SKILL.md and *-agent.md) as skill-relative slash paths, honoring the ignore file.
func collectResourceFiles(skillDir string) ([]string, error) {
	ignore, err := loadSkillPackageIgnore(skillDir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, subdir := range skillResourceDirs {
		dir := filepath.Join(skillDir, subdir)
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(skillDir, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if shouldExcludeSkillPackagePath(rel, false, ignore) {
				return nil
			}
			result = append(result, rel)
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", subdir, walkErr)
		}
	}

	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return nil, fmt.Errorf("read skill directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if shouldExcludeSkillPackagePath(name, false, ignore) || name == skillMarkdownFile || strings.HasSuffix(name, agentFileSuffix) {
			continue
		}
		result = append(result, name)
	}
	return result, nil
}

// resolveCrossSkills packages referenced sibling/project/user skills recursively,
// guarding against cycles. It returns the typed wire configs (no local fields leak
// into them — what removes the original's strip step) paired with their local
// contexts (so run/serve can start each cross-skill's workers).
func resolveCrossSkills(skillMd, skillDir string, opts PayloadOptions, seen map[string]bool) (map[string]SkillConfig, map[string]LocalContext, error) {
	refNames := referencedSkillNames(skillMd)
	if len(refNames) == 0 {
		return nil, nil, nil
	}

	searchDirs := []string{filepath.Dir(skillDir), filepath.Join(".", agentsSkillsSubpath)}
	if home, err := os.UserHomeDir(); err == nil {
		searchDirs = append(searchDirs, filepath.Join(home, agentsSkillsSubpath))
	}
	searchDirs = append(searchDirs, opts.SearchPaths...)

	refs := make(map[string]SkillConfig)
	locals := make(map[string]LocalContext)
	for _, refName := range refNames {
		refDir, ok := findSkillDir(refName, searchDirs)
		if !ok {
			continue
		}
		refAbs, err := filepath.Abs(refDir)
		if err != nil {
			return nil, nil, err
		}
		refAbs, err = filepath.EvalSymlinks(refAbs)
		if err != nil {
			return nil, nil, err
		}
		if refAbs == skillDir {
			continue
		}
		if seen[refAbs] {
			return nil, nil, fmt.Errorf("circular skill reference detected: %s", refName)
		}
		nextSeen := make(map[string]bool, len(seen)+1)
		for k, v := range seen {
			nextSeen[k] = v
		}
		nextSeen[skillDir] = true

		refCfg, refLocal, err := buildSkillPayloadInternal(refAbs, opts, nextSeen)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve cross-skill %q: %w", refName, err)
		}
		refs[refName] = refCfg
		locals[refName] = refLocal
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}
	return refs, locals, nil
}

// referencedSkillNames returns the de-duplicated, sorted skill names referenced in
// the SKILL.md body prose.
func referencedSkillNames(skillMd string) []string {
	matches := crossSkillRefPattern.FindAllStringSubmatch(extractBody(skillMd), -1)
	seen := make(map[string]bool)
	var names []string
	for _, m := range matches {
		name := strings.ToLower(m[1])
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// findSkillDir returns the first search dir that contains "{name}/SKILL.md".
func findSkillDir(name string, dirs []string) (string, bool) {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(expandUserPath(dir), name)
		if _, err := os.Stat(filepath.Join(candidate, skillMarkdownFile)); err == nil {
			return candidate, true
		}
	}
	return "", false
}

// extractBody returns the markdown body after the YAML frontmatter (mirrors the
// register command's parseFrontmatter, which returns the parsed head).
func extractBody(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return content
	}
	rest := strings.TrimPrefix(content[3:], "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return content
	}
	return strings.TrimPrefix(rest[end+4:], "\n")
}

// extractDefaultParams reads frontmatter "params" defaults. A param may be given as
// a scalar or as a "{default: ...}" object; both collapse to a ParamValue.
func extractDefaultParams(frontmatter map[string]any) map[string]ParamValue {
	params, ok := frontmatter["params"].(map[string]any)
	if !ok {
		return nil
	}
	defaults := make(map[string]ParamValue, len(params))
	for name, raw := range params {
		if def, ok := raw.(map[string]any); ok {
			if value, exists := def["default"]; exists {
				defaults[name] = rawParamValue(value)
				continue
			}
		}
		defaults[name] = rawParamValue(raw)
	}
	if len(defaults) == 0 {
		return nil
	}
	return defaults
}

// mergeParams overlays overrides onto defaults (overrides win).
func mergeParams(defaults, overrides map[string]ParamValue) map[string]ParamValue {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]ParamValue, len(defaults)+len(overrides))
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// formatParamMap renders a deterministic "[Skill Parameters]" block.
func formatParamMap(params map[string]ParamValue) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("[Skill Parameters]\n")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(params[k].format())
	}
	return sb.String()
}

// splitSkillSections breaks a large body into slug → "## section" bodies so the
// read_skill_file worker can serve them on demand. It scans line by line, starting
// a new section at each heading line (Go's RE2 has no look-ahead, so a split regex
// is not an option). Small bodies, and bodies with no headings, return nil.
func splitSkillSections(body string) map[string]string {
	if len(body) <= skillSectionSplitBytes {
		return nil
	}
	sections := make(map[string]string)
	var block []string
	flush := func() {
		if len(block) == 0 {
			return
		}
		section := strings.TrimSpace(strings.Join(block, "\n"))
		block = block[:0]
		if !strings.HasPrefix(section, sectionHeadingPrefix) {
			return // discard the pre-heading preamble
		}
		firstLine := strings.SplitN(section, "\n", 2)[0]
		slug := slugifyHeading(strings.TrimSpace(strings.TrimPrefix(firstLine, sectionHeadingPrefix)))
		if slug != "" {
			sections[slug] = section
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, sectionHeadingPrefix) {
			flush()
		}
		block = append(block, line)
	}
	flush()
	if len(sections) == 0 {
		return nil
	}
	return sections
}

// slugifyHeading lowercases text and collapses runs of spaces/dashes into single
// dashes, keeping only [a-z0-9-].
func slugifyHeading(text string) string {
	text = strings.ToLower(text)
	var sb strings.Builder
	lastDash := false
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '\t' || r == '-':
			if !lastDash && sb.Len() > 0 {
				sb.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(sb.String(), "-")
}
