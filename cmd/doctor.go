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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/providers"
)

// aiProvider describes an LLM provider and the env vars that configure it.
//
// It deliberately names no models. doctor can know which providers are configured;
// which model ids a provider currently serves is not something a CLI release can keep
// true, and stale ids fail at execution time on the server rather than at validation
// (see #103).
type aiProvider struct {
	name       string
	serverName string            // identifier the server reports for this provider
	envVars    []string          // all must be set for the provider to be "configured"
	warns      []providerWarning // conditional warnings, checked when the provider is opted into
}

type providerWarning struct {
	condition func() bool
	message   string
	fix       string
}

// aiProviders is the data-driven registry doctor reports on. Adding a provider is a
// data change, not new control flow.
var aiProviders = []aiProvider{
	{name: "OpenAI", serverName: "openai", envVars: []string{"OPENAI_API_KEY"}},
	{name: "Anthropic", serverName: "anthropic", envVars: []string{"ANTHROPIC_API_KEY"}},
	{
		name:       "Google Gemini",
		serverName: "gemini",
		envVars:    []string{"GEMINI_API_KEY", "GOOGLE_CLOUD_PROJECT"},
		warns: []providerWarning{{
			condition: func() bool {
				return os.Getenv("GEMINI_API_KEY") != "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == ""
			},
			message: "GEMINI_API_KEY is set but GOOGLE_CLOUD_PROJECT is missing",
			fix:     "export GOOGLE_CLOUD_PROJECT=your-gcp-project-id",
		}},
	},
	{
		name:       "Azure OpenAI",
		serverName: "azureopenai",
		envVars:    []string{"AZURE_OPENAI_API_KEY", "AZURE_OPENAI_ENDPOINT"},
		warns: []providerWarning{{
			condition: func() bool {
				return os.Getenv("AZURE_OPENAI_API_KEY") != "" && os.Getenv("AZURE_OPENAI_DEPLOYMENT") == ""
			},
			message: "AZURE_OPENAI_DEPLOYMENT is not set (required to route requests)",
			fix:     "export AZURE_OPENAI_DEPLOYMENT=your-deployment-name",
		}},
	},
	{
		name:       "AWS Bedrock",
		serverName: "aws_bedrock",
		envVars:    []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		warns: []providerWarning{{
			condition: func() bool {
				return os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_DEFAULT_REGION") == "" && os.Getenv("AWS_REGION") == ""
			},
			message: "No AWS region set — defaults to us-east-1",
			fix:     "export AWS_DEFAULT_REGION=us-east-1",
		}},
	},
	{name: "Mistral", serverName: "mistral", envVars: []string{"MISTRAL_API_KEY"}},
	{name: "Cohere", serverName: "cohere", envVars: []string{"COHERE_API_KEY"}},
	{name: "Grok", serverName: "grok", envVars: []string{"XAI_API_KEY"}},
	{name: "Perplexity", serverName: "perplexity", envVars: []string{"PERPLEXITY_API_KEY"}},
	{name: "Hugging Face", serverName: "huggingface", envVars: []string{"HUGGINGFACE_API_KEY"}},
	{name: "Stability AI", serverName: "stabilityai", envVars: []string{"STABILITY_API_KEY"}},
}

// serverOnlyProviders are reported by the server but configured without env vars, so
// they have no entry in the local registry. Only their display name is needed.
var serverOnlyProviders = map[string]string{"ollama": "Ollama"}

// displayProviderName maps a server provider identifier to the CLI's display name, so
// one provider reads as one provider rather than two spellings of it. Unknown names
// pass through unchanged — a provider the server gained since this release should be
// reported, not hidden.
func displayProviderName(serverName string) string {
	for _, p := range aiProviders {
		if p.serverName == serverName {
			return p.name
		}
	}
	if name, ok := serverOnlyProviders[serverName]; ok {
		return name
	}
	return serverName
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

	// Agents run on the server, so ask it what it can dial before reporting what this
	// shell happens to hold. The transport imposes no timeout of its own; a
	// diagnostic must not hang on an unreachable server.
	ctx, cancel := context.WithTimeout(cmd.Context(), providerStatusTimeout)
	defer cancel()
	st, err := providers.Fetch(ctx, t)
	printServerProviders(os.Stdout, st, err)

	configured, warnings := printLocalProviders(os.Stdout)
	issues += warnings

	fmt.Printf("\n%d AI provider(s) configured in this environment", configured)
	if issues > 0 {
		fmt.Printf(", %d warning(s)", issues)
	}
	fmt.Println(".")
	return nil
}

// providerStatusTimeout bounds the provider-status lookup. doctor reports on a server
// that may be unreachable, which is itself a finding rather than a reason to block.
//
// The bound is generous because the endpoint is measurably slow: a local OSS server
// took 7-8s to answer, and not because of the ollama reachability probe it advertises
// (a refused connect returns immediately). A timeout tight enough to feel snappy would
// report "no providers" on a server that has them, which is the very confusion this
// section exists to remove.
const providerStatusTimeout = 20 * time.Second

// printServerProviders reports the providers the server has configured. Neither a
// host-managed deployment nor a server without the endpoint is a fault in the user's
// setup, so both are stated plainly and neither counts as a warning.
func printServerProviders(w io.Writer, st providers.Status, err error) {
	fmt.Fprintln(w, "\nAI Providers (server)")

	switch {
	case errors.Is(err, providers.ErrUnsupported):
		fmt.Fprintln(w, "  --  This server does not report provider status")
		fmt.Fprintln(w, "      (AI integrations are disabled, or the server predates the endpoint)")
		return
	case err != nil:
		fmt.Fprintf(w, "  --  Could not read provider status: %v\n", err)
		return
	case st.ManagedByHost:
		fmt.Fprintln(w, "  --  Provider configuration is owned by the host deployment")
		fmt.Fprintln(w, "      (per-provider detail is not reported by this server)")
		return
	case len(st.Providers) == 0:
		fmt.Fprintln(w, "  --  The server reported no providers")
		return
	}

	for _, p := range st.Providers {
		mark := "--"
		if p.Configured {
			mark = "ok"
		}
		fmt.Fprintf(w, "  %s  %s%s\n", mark, displayProviderName(p.Name), serverProviderDetail(p))
	}
}

// serverProviderDetail renders the extras the server reports for URL-based providers.
// Reachability is probed from the server's own network, which is precisely the fact a
// client cannot determine for itself.
func serverProviderDetail(p providers.Provider) string {
	if p.BaseURL == "" && p.Reachable == nil {
		return ""
	}
	parts := []string{}
	if p.BaseURL != "" {
		parts = append(parts, p.BaseURL)
	}
	if p.Reachable != nil {
		if *p.Reachable {
			parts = append(parts, "reachable")
		} else {
			parts = append(parts, "unreachable from the server")
		}
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// printLocalProviders reports what this shell has configured, which still governs the
// deploy and worker paths. It returns the configured count and the number of warnings
// raised.
func printLocalProviders(w io.Writer) (configured, warnings int) {
	fmt.Fprintln(w, "\nAI Providers (local environment)")
	for _, p := range aiProviders {
		if isProviderConfigured(p) {
			configured++
			fmt.Fprintf(w, "  ok  %s (%s)\n", p.name, strings.Join(p.envVars, ", "))
		} else {
			fmt.Fprintf(w, "  --  %s (%s)\n", p.name, strings.Join(p.envVars, ", "))
		}
		if providerOptedIn(p) {
			for _, warn := range p.warns {
				if warn.condition() {
					fmt.Fprintf(w, "      !  %s\n", warn.message)
					fmt.Fprintf(w, "         %s\n", warn.fix)
					warnings++
				}
			}
		}
	}
	return configured, warnings
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
