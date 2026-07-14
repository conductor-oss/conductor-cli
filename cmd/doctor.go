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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/conductor-oss/conductor-cli/internal"
)

// aiProvider describes an LLM provider and the env vars that configure it.
type aiProvider struct {
	name    string
	envVars []string          // all must be set for the provider to be "configured"
	warns   []providerWarning // conditional warnings, checked when the provider is opted into
	models  []string          // example model ids
}

type providerWarning struct {
	condition func() bool
	message   string
	fix       string
}

// aiProviders is the data-driven registry doctor reports on. Adding a provider is a
// data change, not new control flow.
var aiProviders = []aiProvider{
	{name: "OpenAI", envVars: []string{"OPENAI_API_KEY"}, models: []string{"openai/gpt-4o", "openai/gpt-4o-mini"}},
	{name: "Anthropic", envVars: []string{"ANTHROPIC_API_KEY"}, models: []string{"anthropic/claude-sonnet-4-20250514", "anthropic/claude-3-5-sonnet-20241022"}},
	{
		name:    "Google Gemini",
		envVars: []string{"GEMINI_API_KEY", "GOOGLE_CLOUD_PROJECT"},
		warns: []providerWarning{{
			condition: func() bool {
				return os.Getenv("GEMINI_API_KEY") != "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == ""
			},
			message: "GEMINI_API_KEY is set but GOOGLE_CLOUD_PROJECT is missing",
			fix:     "export GOOGLE_CLOUD_PROJECT=your-gcp-project-id",
		}},
		models: []string{"google_gemini/gemini-2.0-flash", "google_gemini/gemini-1.5-pro"},
	},
	{
		name:    "Azure OpenAI",
		envVars: []string{"AZURE_OPENAI_API_KEY", "AZURE_OPENAI_ENDPOINT"},
		warns: []providerWarning{{
			condition: func() bool {
				return os.Getenv("AZURE_OPENAI_API_KEY") != "" && os.Getenv("AZURE_OPENAI_DEPLOYMENT") == ""
			},
			message: "AZURE_OPENAI_DEPLOYMENT is not set (required to route requests)",
			fix:     "export AZURE_OPENAI_DEPLOYMENT=your-deployment-name",
		}},
		models: []string{"azure_openai/gpt-4o"},
	},
	{
		name:    "AWS Bedrock",
		envVars: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		warns: []providerWarning{{
			condition: func() bool {
				return os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_DEFAULT_REGION") == "" && os.Getenv("AWS_REGION") == ""
			},
			message: "No AWS region set — defaults to us-east-1",
			fix:     "export AWS_DEFAULT_REGION=us-east-1",
		}},
		models: []string{"aws_bedrock/anthropic.claude-3-5-sonnet-20241022-v2:0"},
	},
	{name: "Mistral", envVars: []string{"MISTRAL_API_KEY"}, models: []string{"mistral/mistral-large-latest"}},
	{name: "Cohere", envVars: []string{"COHERE_API_KEY"}, models: []string{"cohere/command-r-plus"}},
	{name: "Grok", envVars: []string{"XAI_API_KEY"}, models: []string{"grok/grok-3"}},
	{name: "Perplexity", envVars: []string{"PERPLEXITY_API_KEY"}, models: []string{"perplexity/sonar-pro"}},
	{name: "Hugging Face", envVars: []string{"HUGGINGFACE_API_KEY"}, models: []string{"hugging_face/meta-llama/Llama-3-70b-chat-hf"}},
	{name: "Stability AI", envVars: []string{"STABILITY_API_KEY"}, models: []string{"stabilityai/sd3.5-large"}},
}

var doctorCmd = &cobra.Command{
	Use:          "doctor",
	Short:        "Check the runtime and AI provider configuration",
	GroupID:      "development",
	SilenceUsage: true,
	RunE:         runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	issues := 0

	fmt.Println("Runtime")
	if ok, ver := commandVersion("java", "-version"); ok {
		fmt.Printf("  ok  Java %s\n", ver)
	} else {
		fmt.Println("  --  Java not found (required by a local Conductor server; not needed for a remote server)")
	}
	if ok, ver := pythonVersion(); ok {
		fmt.Printf("  ok  Python %s\n", ver)
	} else {
		fmt.Println("  --  Python not found (optional, needed for the Python SDK)")
	}

	fmt.Println("\nConductor server")
	t := internal.Transport()
	fmt.Printf("  ok  Server: %s\n", t.BaseURL)
	if t.Tokens != nil {
		fmt.Println("  ok  Authentication configured")
	} else {
		fmt.Println("  --  No authentication configured (anonymous; OSS only)")
	}

	fmt.Println("\nAI Providers")
	configured := 0
	for _, p := range aiProviders {
		opted := providerOptedIn(p)
		if isProviderConfigured(p) {
			configured++
			fmt.Printf("  ok  %s (%s)\n", p.name, strings.Join(p.envVars, ", "))
			for _, m := range p.models {
				fmt.Printf("        %s\n", m)
			}
		} else {
			fmt.Printf("  --  %s (%s)\n", p.name, strings.Join(p.envVars, ", "))
		}
		if opted {
			for _, w := range p.warns {
				if w.condition() {
					fmt.Printf("      !  %s\n", w.message)
					fmt.Printf("         %s\n", w.fix)
					issues++
				}
			}
		}
	}

	fmt.Printf("\n%d AI provider(s) configured", configured)
	if issues > 0 {
		fmt.Printf(", %d warning(s)", issues)
	}
	fmt.Println(".")
	return nil
}

// isProviderConfigured reports whether every required env var for a provider is set.
func isProviderConfigured(p aiProvider) bool {
	for _, env := range p.envVars {
		if os.Getenv(env) == "" {
			return false
		}
	}
	return true
}

// providerOptedIn reports whether the user set at least one of a provider's env vars
// (so its warnings are worth showing even if not fully configured).
func providerOptedIn(p aiProvider) bool {
	for _, env := range p.envVars {
		if os.Getenv(env) != "" {
			return true
		}
	}
	return false
}

// commandVersion reports whether a command is on PATH and its first version line.
func commandVersion(name string, args ...string) (bool, string) {
	path, err := exec.LookPath(name)
	if err != nil {
		return false, ""
	}
	out, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return true, ""
	}
	return true, firstLine(string(out))
}

// pythonVersion probes python3 then python.
func pythonVersion() (bool, string) {
	for _, name := range []string{"python3", "python"} {
		if ok, ver := commandVersion(name, "--version"); ok {
			return true, ver
		}
	}
	return false, ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
