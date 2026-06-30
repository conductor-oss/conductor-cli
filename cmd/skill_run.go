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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/agent"
	"github.com/conductor-oss/conductor-cli/internal/skillworker"
)

// Skill run/serve flag defaults.
const (
	defaultScriptTimeoutSeconds = 300
	defaultScriptOutputLimit    = 10 << 20 // 10 MiB
	defaultWorkspaceDir         = "."
	defaultWorkspaceFileLimit   = 1 << 20 // 1 MiB
)

var (
	// run only
	skillRunModel       string
	skillRunAgentModels []string
	skillRunParams      []string
	// run + serve
	skillRunVersion         string
	skillSearchPaths        []string
	skillScriptTimeout      int
	skillScriptOutputLimit  int
	skillWorkspaceDir       string
	skillNoWorkspace        bool
	skillFileSystems        []string
	skillWorkspaceFileLimit int
)

var skillRunCmd = &cobra.Command{
	Use:   "run <path-or-name> <prompt>",
	Short: "Run a local or registered skill and stream its output",
	Long: `Run a local skill directory or a server-registered skill by name. The CLI
starts local tool workers (read_skill_file, scripts, workspace tools), starts the
skill agent on the server, and streams its execution. Workers run for the duration
of the execution.`,
	Args:         cobra.MinimumNArgs(2),
	SilenceUsage: true,
	RunE:         runSkillRun,
}

var skillServeCmd = &cobra.Command{
	Use:   "serve <path-or-name>",
	Short: "Start local tool workers for a skill without running it",
	Long: `Start the local tool workers for a skill (read_skill_file, scripts, workspace
tools) and block until interrupted. Use this to serve a skill's tools while it is
run elsewhere (e.g. from the UI).`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runSkillServe,
}

func runSkillRun(cmd *cobra.Command, args []string) error {
	if skillRunModel == "" {
		return fmt.Errorf("--model is required for skill run")
	}
	prompt := strings.Join(args[1:], " ")

	agentModels, err := parseAgentModelFlags(skillRunAgentModels)
	if err != nil {
		return err
	}
	params, err := parseParamOverrides(skillRunParams)
	if err != nil {
		return err
	}
	ws, err := resolveSkillWorkspaceConfig()
	if err != nil {
		return err
	}

	dir, _, err := materializeSkill(cmd.Context(), internal.GetSkillService(), args[0], skillRunVersion)
	if err != nil {
		return err
	}
	cfg, local, err := BuildSkillPayload(dir, PayloadOptions{
		Model:          skillRunModel,
		AgentModels:    agentModels,
		SearchPaths:    skillSearchPaths,
		ParamOverrides: params,
	})
	if err != nil {
		return err
	}
	cfg.Workspace = ws.WireConfig()

	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode skill config: %w", err)
	}

	// One signal-aware context governs both the workers and the stream; cancelling
	// it (Ctrl-C) stops everything. Workers are also cancelled when the execution
	// ends normally.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	startSkillWorkers(workerCtx, buildSkillWorkerRegistry(cfg, local, ws, scriptOptions(), skillWorkspaceFileLimit))

	svc := internal.GetAgentService()
	exec, err := svc.Run(ctx, agent.RunRequest{Framework: frameworkSkill, Definition: raw, Prompt: prompt})
	if err != nil {
		return err
	}
	fmt.Printf("Skill: %s (Execution: %s)\n\n", exec.AgentName, exec.ID)

	streamErr := svc.StreamExecution(ctx, exec.ID, "", terminalSink{})
	cancelWorkers()
	return streamErr
}

func runSkillServe(cmd *cobra.Command, args []string) error {
	ws, err := resolveSkillWorkspaceConfig()
	if err != nil {
		return err
	}
	dir, _, err := materializeSkill(cmd.Context(), internal.GetSkillService(), args[0], skillRunVersion)
	if err != nil {
		return err
	}
	cfg, local, err := BuildSkillPayload(dir, PayloadOptions{SearchPaths: skillSearchPaths})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startSkillWorkers(ctx, buildSkillWorkerRegistry(cfg, local, ws, scriptOptions(), skillWorkspaceFileLimit))

	fmt.Printf("Serving workers for skill %s. Press Ctrl-C to stop.\n", local.SkillName)
	<-ctx.Done()
	return nil
}

// scriptOptions builds the script-execution bounds from the shared flags.
func scriptOptions() skillworker.ScriptOptions {
	return skillworker.ScriptOptions{
		Timeout:     time.Duration(skillScriptTimeout) * time.Second,
		OutputLimit: skillScriptOutputLimit,
	}
}

// startSkillWorkers launches one polling worker goroutine per registered task type.
// They run until ctx is cancelled.
func startSkillWorkers(ctx context.Context, registry map[string]skillworker.ToolHandler) {
	taskClient := internal.GetTaskClient()
	for taskType, handler := range registry {
		w := skillworker.NewWorker(skillworker.NewConductorRunner(taskClient))
		go w.Run(ctx, taskType, handler)
	}
}

// buildSkillWorkerRegistry maps every "{skillName}__{tool}" task type the skill (and
// its resolved cross-skills) needs to the handler that serves it locally.
func buildSkillWorkerRegistry(cfg SkillConfig, local LocalContext, ws skillworker.WorkspaceConfig, opts skillworker.ScriptOptions, fileLimit int) map[string]skillworker.ToolHandler {
	reg := map[string]skillworker.ToolHandler{}
	addSkillWorkerHandlers(reg, cfg, local, ws, opts, fileLimit)
	return reg
}

func addSkillWorkerHandlers(reg map[string]skillworker.ToolHandler, cfg SkillConfig, local LocalContext, ws skillworker.WorkspaceConfig, opts skillworker.ScriptOptions, fileLimit int) {
	name := local.SkillName
	if name == "" {
		return
	}
	reg[skillworker.TaskType(name, skillworker.ToolReadSkillFile)] = skillworker.NewReadSkillFileHandler(local.SkillDir, cfg.ResourceFiles, local.Sections)
	for tool, info := range cfg.Scripts {
		scriptPath := filepath.Join(local.SkillDir, scriptsDirName, info.Filename)
		reg[skillworker.TaskType(name, tool)] = skillworker.NewScriptHandler(scriptPath, info.Language, ws, opts)
	}
	if ws.Enabled {
		reg[skillworker.TaskType(name, skillworker.ToolListWorkspace)] = skillworker.NewListWorkspaceFilesHandler(ws)
		reg[skillworker.TaskType(name, skillworker.ToolReadWorkspaceFile)] = skillworker.NewReadWorkspaceFileHandler(ws, fileLimit)
		reg[skillworker.TaskType(name, skillworker.ToolSearchWorkspace)] = skillworker.NewSearchWorkspaceHandler(ws, fileLimit)
		reg[skillworker.TaskType(name, skillworker.ToolGitStatus)] = skillworker.NewGitStatusHandler(ws, fileLimit)
		reg[skillworker.TaskType(name, skillworker.ToolGitDiff)] = skillworker.NewGitDiffHandler(ws, fileLimit)
	}
	for refName, refCfg := range cfg.CrossSkillRefs {
		if refLocal, ok := local.CrossSkills[refName]; ok {
			addSkillWorkerHandlers(reg, refCfg, refLocal, ws, opts, fileLimit)
		}
	}
}

// resolveSkillWorkspaceConfig builds the workspace configuration from the shared
// flags. Flag parsing stays in the cmd layer; skillworker validates each root.
func resolveSkillWorkspaceConfig() (skillworker.WorkspaceConfig, error) {
	cfg := skillworker.WorkspaceConfig{}
	seen := map[string]bool{}
	addRoot := func(name, pathValue, kind string) error {
		root, err := skillworker.NewWorkspaceRoot(name, expandUserPath(pathValue), kind)
		if err != nil {
			return err
		}
		if seen[root.Name] {
			return fmt.Errorf("duplicate filesystem root %q", root.Name)
		}
		seen[root.Name] = true
		cfg.Roots = append(cfg.Roots, root)
		return nil
	}

	if !skillNoWorkspace {
		workspacePath := skillWorkspaceDir
		if strings.TrimSpace(workspacePath) == "" {
			workspacePath = defaultWorkspaceDir
		}
		if err := addRoot(skillworker.KindWorkspace, workspacePath, skillworker.KindWorkspace); err != nil {
			return cfg, err
		}
	}
	for _, spec := range skillFileSystems {
		name, pathValue, ok := strings.Cut(spec, "=")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(pathValue) == "" {
			return cfg, fmt.Errorf("invalid --filesystem value %q: expected name=path", spec)
		}
		if err := addRoot(strings.TrimSpace(name), strings.TrimSpace(pathValue), skillworker.KindFilesystem); err != nil {
			return cfg, err
		}
	}
	cfg.Enabled = len(cfg.Roots) > 0
	return cfg, nil
}

// parseParamOverrides parses repeated --param key=value flags into typed values.
func parseParamOverrides(flags []string) (map[string]ParamValue, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	result := make(map[string]ParamValue, len(flags))
	for _, f := range flags {
		name, value, ok := strings.Cut(f, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid --param value %q: expected key=value", f)
		}
		result[name] = parseParamValue(value)
	}
	return result, nil
}

// parseParamValue coerces "true"/"false" to a bool, keeping everything else as a
// string (faithful to the original).
func parseParamValue(value string) ParamValue {
	switch strings.ToLower(value) {
	case "true":
		return rawParamValue(true)
	case "false":
		return rawParamValue(false)
	default:
		return rawParamValue(value)
	}
}

func init() {
	skillRunCmd.Flags().StringVar(&skillRunModel, "model", "", "Orchestrator and default model (required)")
	skillRunCmd.Flags().StringArrayVar(&skillRunAgentModels, "agent-model", nil, "Sub-agent model override (name=model, repeatable)")
	skillRunCmd.Flags().StringArrayVar(&skillRunParams, "param", nil, "Skill parameter override (key=value, repeatable)")
	addSkillWorkerFlags(skillRunCmd)

	addSkillWorkerFlags(skillServeCmd)

	skillCmd.AddCommand(skillRunCmd, skillServeCmd)
}

// addSkillWorkerFlags binds the flags shared by run and serve.
func addSkillWorkerFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(&skillSearchPaths, "search-path", nil, "Cross-skill search directory (repeatable)")
	cmd.Flags().StringVar(&skillRunVersion, "version", "", "Registered skill version or checksum prefix")
	cmd.Flags().IntVar(&skillScriptTimeout, "script-timeout", defaultScriptTimeoutSeconds, "Skill script timeout in seconds")
	cmd.Flags().IntVar(&skillScriptOutputLimit, "script-output-limit", defaultScriptOutputLimit, "Maximum bytes captured from skill script output")
	cmd.Flags().StringVar(&skillWorkspaceDir, "workspace", defaultWorkspaceDir, "Workspace directory exposed to workspace tools")
	cmd.Flags().BoolVar(&skillNoWorkspace, "no-workspace", false, "Disable exposing the current workspace")
	cmd.Flags().StringArrayVar(&skillFileSystems, "filesystem", nil, "Additional read-only filesystem root name=path (repeatable)")
	cmd.Flags().IntVar(&skillWorkspaceFileLimit, "workspace-file-limit", defaultWorkspaceFileLimit, "Maximum bytes returned by workspace file tools")
}
