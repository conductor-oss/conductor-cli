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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/deploy"
)

const (
	langPython     = "python"
	langTypeScript = "typescript"
)

// discoveredAgent is one agent reported by the discover subprocess.
type discoveredAgent struct {
	Name      string `json:"name"`
	Framework string `json:"framework"`
}

// deployResult is the outcome of deploying one agent, as reported by the subprocess.
type deployResult struct {
	AgentName      string  `json:"agent_name"`
	RegisteredName *string `json:"registered_name"`
	Success        bool    `json:"success"`
	Error          *string `json:"error"`
}

var (
	deployAgents   []string
	deployLanguage string
	deployPackage  string
	deployJSON     bool
)

var deployCmd = &cobra.Command{
	Use:          "deploy",
	Short:        "Deploy agents from your project to the Conductor server",
	GroupID:      "development",
	SilenceUsage: true,
	RunE:         runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	agentNames := cleanNames(deployAgents)

	language, err := detectLanguage(wd, deployLanguage)
	if err != nil {
		return err
	}

	pythonBin := ""
	if language == langPython {
		if pythonBin = findPythonBinary(wd); pythonBin == "" {
			return fmt.Errorf("no Python interpreter found; install Python or set the PYTHON environment variable")
		}
	}

	pkg, err := inferPackage(wd, language, deployPackage)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	t := internal.Transport()
	env, err := deploy.EnvBuilder{BaseURL: t.BaseURL, Tokens: t.Tokens, BaseEnv: os.Environ()}.Build(ctx)
	if err != nil {
		return err
	}
	runner := deploy.NewRunner()

	discovered, err := execDiscover(ctx, runner, env, language, pythonBin, wd, pkg)
	if err != nil {
		return err
	}
	if len(discovered) == 0 {
		return fmt.Errorf("no agents found in %q; define agents as module-level variables, or use --package/--path", pkg.Value)
	}
	allDiscovered := discovered

	if discovered, err = filterDiscoveredAgents(discovered, agentNames); err != nil {
		return err
	}

	if !deployJSON {
		fmt.Println(formatDiscoveryTable(discovered, pkg.Value))
		if !yes && !confirm("Deploy these agents?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	names := make([]string, len(discovered))
	for i, a := range discovered {
		names[i] = a.Name
	}

	results, err := execDeploy(ctx, runner, env, language, pythonBin, wd, pkg, names)
	if err != nil {
		return err
	}

	failed := 0
	for _, r := range results {
		if !r.Success {
			failed++
		}
	}

	if deployJSON {
		out := map[string]any{
			"discovered": allDiscovered,
			"deployed":   results,
			"summary":    map[string]int{"total": len(results), "succeeded": len(results) - failed, "failed": failed},
		}
		data, mErr := json.MarshalIndent(out, "", "  ")
		if mErr != nil {
			return mErr
		}
		fmt.Println(string(data))
	} else {
		fmt.Println(formatDeployOutput(results))
	}

	if failed > 0 {
		return fmt.Errorf("%d agent(s) failed to deploy", failed)
	}
	return nil
}

func cleanNames(names []string) []string {
	out := names[:0]
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	var answer string
	fmt.Scanln(&answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// detectLanguage resolves the project language from --language or marker files.
func detectLanguage(dir, override string) (string, error) {
	if override != "" {
		switch strings.ToLower(override) {
		case "python", "py":
			return langPython, nil
		case "typescript", "ts":
			return langTypeScript, nil
		default:
			return "", fmt.Errorf("unsupported language %q (supported: python, typescript)", override)
		}
	}

	hasPython := false
	for _, m := range []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			hasPython = true
			break
		}
	}
	hasTS := false
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		hasTS = true
	} else if hasTSDependency(filepath.Join(dir, "package.json")) {
		hasTS = true
	}

	switch {
	case hasPython && hasTS:
		return "", fmt.Errorf("both Python and TypeScript markers found; use --language to disambiguate")
	case hasPython:
		return langPython, nil
	case hasTS:
		return langTypeScript, nil
	default:
		return "", fmt.Errorf("no Python or TypeScript project markers found; use --language to specify")
	}
}

func hasTSDependency(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pkg map[string]any
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	for _, section := range []string{"dependencies", "devDependencies"} {
		deps, ok := pkg[section].(map[string]any)
		if !ok {
			continue
		}
		for _, dep := range []string{"typescript", "tsx", "ts-node"} {
			if _, found := deps[dep]; found {
				return true
			}
		}
	}
	return false
}

// packageInfo is the inferred package/path and how to pass it to the subprocess.
type packageInfo struct {
	Value  string
	IsPath bool // true => --path, false => --package
}

func inferPackage(dir, language, override string) (packageInfo, error) {
	if override != "" {
		isPath := strings.Contains(override, "/") || strings.HasPrefix(override, ".")
		return packageInfo{Value: override, IsPath: isPath}, nil
	}
	switch language {
	case langPython:
		if pkg, err := inferPythonPackage(dir); err == nil {
			return packageInfo{Value: pkg, IsPath: false}, nil
		}
		return packageInfo{Value: dir, IsPath: true}, nil
	case langTypeScript:
		if info, err := os.Stat(filepath.Join(dir, "src")); err == nil && info.IsDir() {
			return packageInfo{Value: "./src", IsPath: true}, nil
		}
		return packageInfo{Value: ".", IsPath: true}, nil
	default:
		return packageInfo{}, fmt.Errorf("cannot infer package for language %q", language)
	}
}

func inferPythonPackage(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return "", fmt.Errorf("pyproject.toml not found; use --package")
	}
	name := parsePyprojectName(string(data))
	if name == "" {
		return "", fmt.Errorf("no [project] name in pyproject.toml; use --package")
	}
	return strings.ReplaceAll(name, "-", "_"), nil // PEP 503 import name
}

// parsePyprojectName extracts name from the [project] table without a TOML dependency.
func parsePyprojectName(content string) string {
	inProject := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "[project]":
			inProject = true
		case inProject && strings.HasPrefix(trimmed, "["):
			return ""
		case inProject && strings.HasPrefix(trimmed, "name"):
			if parts := strings.SplitN(trimmed, "=", 2); len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			}
		}
	}
	return ""
}

func findPythonBinary(dir string) string {
	if p := os.Getenv("PYTHON"); p != "" {
		return p
	}
	for _, venv := range []string{".venv", "venv"} {
		candidate := filepath.Join(dir, venv, "bin", "python")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func execDiscover(ctx context.Context, runner deploy.Runner, env []string, language, pythonBin, projectDir string, pkg packageInfo) ([]discoveredAgent, error) {
	data, err := runLanguageTool(ctx, runner, env, language, pythonBin, projectDir, pkg, "discover", "agentspan.cli.discover", "discover.ts", nil)
	if err != nil && len(data) == 0 {
		return nil, err
	}
	var agents []discoveredAgent
	if uErr := json.Unmarshal(data, &agents); uErr != nil {
		return nil, fmt.Errorf("parse discovery result: %w", uErr)
	}
	return agents, nil
}

func execDeploy(ctx context.Context, runner deploy.Runner, env []string, language, pythonBin, projectDir string, pkg packageInfo, agentNames []string) ([]deployResult, error) {
	data, err := runLanguageTool(ctx, runner, env, language, pythonBin, projectDir, pkg, "deploy", "agentspan.cli.deploy", "deploy.ts", agentNames)
	if err != nil && len(data) == 0 {
		return nil, err
	}
	var results []deployResult
	if uErr := json.Unmarshal(data, &results); uErr != nil {
		return nil, fmt.Errorf("parse deploy result: %w", uErr)
	}
	return results, nil
}

// runLanguageTool invokes the python module or the TypeScript bin script for an
// operation, passing the package/path and optional agent names.
func runLanguageTool(ctx context.Context, runner deploy.Runner, env []string, language, pythonBin, projectDir string, pkg packageInfo, op, pyModule, tsScript string, agentNames []string) ([]byte, error) {
	switch language {
	case langPython:
		flag := "--package"
		if pkg.IsPath {
			flag = "--path"
		}
		args := []string{"-m", pyModule, flag, pkg.Value}
		args = appendAgents(args, agentNames)
		return runner.Run(ctx, env, pythonBin, args...)
	case langTypeScript:
		script, err := findTSBinScript(projectDir, tsScript)
		if err != nil {
			return nil, err
		}
		args := []string{"tsx", script, "--path", pkg.Value}
		args = appendAgents(args, agentNames)
		return runner.Run(ctx, env, "npx", args...)
	default:
		return nil, fmt.Errorf("unsupported language for %s: %s", op, language)
	}
}

func appendAgents(args, agentNames []string) []string {
	if len(agentNames) > 0 {
		args = append(args, "--agents", strings.Join(agentNames, ","))
	}
	return args
}

func filterDiscoveredAgents(agents []discoveredAgent, names []string) ([]discoveredAgent, error) {
	if len(names) == 0 {
		return agents, nil
	}
	byName := make(map[string]discoveredAgent, len(agents))
	for _, a := range agents {
		byName[a.Name] = a
	}
	var notFound []string
	var result []discoveredAgent
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		if a, ok := byName[n]; ok {
			result = append(result, a)
		} else {
			notFound = append(notFound, n)
		}
	}
	if len(notFound) > 0 {
		available := make([]string, 0, len(agents))
		for _, a := range agents {
			available = append(available, a.Name)
		}
		return nil, fmt.Errorf("agent(s) not found: %s (available: %s)", strings.Join(notFound, ", "), strings.Join(available, ", "))
	}
	return result, nil
}

func findTSBinScript(dir, name string) (string, error) {
	cur, _ := filepath.Abs(dir)
	for {
		for _, p := range []string{
			filepath.Join(cur, "node_modules", "@agentspan", "sdk", "cli-bin", name),
			filepath.Join(cur, "node_modules", "agentspan", "cli-bin", name),
			filepath.Join(cur, "cli-bin", name),
		} {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("cannot find %s under node_modules; install the AgentSpan SDK", name)
		}
		cur = parent
	}
}

func formatDiscoveryTable(agents []discoveredAgent, pkg string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\nDiscovered %d agent(s) in %s:\n\n", len(agents), pkg)
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tFRAMEWORK")
	for _, a := range agents {
		fmt.Fprintf(w, "%s\t%s\n", a.Name, a.Framework)
	}
	w.Flush()
	return buf.String()
}

func formatDeployOutput(results []deployResult) string {
	var buf bytes.Buffer
	succeeded, failed := 0, 0
	fmt.Fprintln(&buf)
	for _, r := range results {
		if r.Success {
			succeeded++
			fmt.Fprintf(&buf, "  ok  %s", r.AgentName)
			if r.RegisteredName != nil && *r.RegisteredName != "" && *r.RegisteredName != r.AgentName {
				fmt.Fprintf(&buf, " (registered as %s)", *r.RegisteredName)
			}
			fmt.Fprintln(&buf)
		} else {
			failed++
			msg := "unknown error"
			if r.Error != nil {
				msg = *r.Error
			}
			fmt.Fprintf(&buf, "  fail %s: %s\n", r.AgentName, msg)
		}
	}
	fmt.Fprintln(&buf)
	switch {
	case failed == 0:
		fmt.Fprintf(&buf, "All %d agent(s) deployed successfully.\n", succeeded)
	case succeeded == 0:
		fmt.Fprintf(&buf, "All %d agent(s) failed to deploy. Check 'conductor doctor'.\n", failed)
	default:
		fmt.Fprintf(&buf, "%d deployed, %d failed.\n", succeeded, failed)
	}
	if succeeded > 0 {
		fmt.Fprintln(&buf, "\nRun with: conductor agent run --name <agent> \"your prompt\"")
	}
	return buf.String()
}

func init() {
	deployCmd.Flags().StringSliceVarP(&deployAgents, "agents", "a", nil, "Comma-separated agent names to deploy (default: all)")
	deployCmd.Flags().StringVarP(&deployLanguage, "language", "l", "", "Project language: python or typescript")
	deployCmd.Flags().StringVarP(&deployPackage, "package", "p", "", "Package or path to scan for agents")
	deployCmd.Flags().BoolVar(&deployJSON, "json", false, "Output results as JSON")
	rootCmd.AddCommand(deployCmd)
}
