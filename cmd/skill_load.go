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

	"github.com/spf13/cobra"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/agent"
)

var (
	skillLoadModel       string
	skillLoadAgentModels []string
	skillLoadSearchPaths []string
)

var skillLoadCmd = &cobra.Command{
	Use:   "load <path>",
	Short: "Package a local skill and deploy it as an agent",
	Long: `Read a local skill directory, package its contents into a skill agent
definition, and deploy it on the server for later execution via
'conductor agent run --name <skill>'. Unlike 'skill run', load only publishes the
agent; it does not start an execution.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if skillLoadModel == "" {
			return fmt.Errorf("--model is required for skill load")
		}
		agentModels, err := parseAgentModelFlags(skillLoadAgentModels)
		if err != nil {
			return err
		}

		cfg, local, err := BuildSkillPayload(args[0], PayloadOptions{
			Model:       skillLoadModel,
			AgentModels: agentModels,
			SearchPaths: skillLoadSearchPaths,
		})
		if err != nil {
			return err
		}

		raw, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("encode skill config: %w", err)
		}

		result, err := internal.GetAgentService().Deploy(cmd.Context(), frameworkSkill, raw)
		if err != nil {
			return err
		}
		return renderDeployResult(local.SkillName, result)
	},
}

// renderDeployResult prints a concise summary of a deployed skill agent, including
// the tool task types the caller must serve (via 'skill run'/'skill serve') for the
// agent to run end to end.
func renderDeployResult(skillName string, result agent.DeployResult) error {
	name := result.AgentName
	if name == "" {
		name = skillName
	}
	fmt.Printf("Skill %s deployed as agent %s.\n", skillName, name)
	if len(result.RequiredWorkers) > 0 {
		fmt.Println("Required workers:")
		for _, w := range result.RequiredWorkers {
			fmt.Printf("  - %s\n", w)
		}
	}
	return nil
}

func init() {
	skillLoadCmd.Flags().StringVar(&skillLoadModel, "model", "", "Orchestrator and default model (required)")
	skillLoadCmd.Flags().StringArrayVar(&skillLoadAgentModels, "agent-model", nil, "Sub-agent model override (name=model, repeatable)")
	skillLoadCmd.Flags().StringArrayVar(&skillLoadSearchPaths, "search-path", nil, "Cross-skill search directory (repeatable)")
	skillCmd.AddCommand(skillLoadCmd)
}
