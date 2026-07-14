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
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/agent"
)

// Defaults that are policy, not server contract — named so they are not magic
// literals scattered through the code.
const (
	defaultExecutionSearchSize = 50
	defaultPruneOlderThanDays  = 30
	defaultInitModel           = "openai/gpt-4o"
	defaultInitMaxTurns        = 25
)

var agentCmd = &cobra.Command{
	Use:     "agent",
	Aliases: []string{"a"},
	Short:   "Manage and run agents",
	GroupID: "conductor",
}

// ---- agent list ----

var agentListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List registered agents",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := GetOutputFormat(cmd)
		if err != nil {
			return err
		}
		agents, err := internal.GetAgentService().List(cmd.Context())
		if err != nil {
			return err
		}
		return renderAgentList(agents, format)
	},
}

func renderAgentList(agents []agent.AgentSummary, format OutputFormat) error {
	switch format {
	case OutputFormatJSON:
		data, err := json.MarshalIndent(agents, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case OutputFormatCSV:
		w := NewCSVWriter()
		w.WriteHeader("NAME", "VERSION", "TYPE", "DESCRIPTION")
		for _, a := range agents {
			w.WriteRow(a.Name, strconv.Itoa(a.Version), a.Type, a.Description)
		}
		w.Flush()
	default:
		if len(agents) == 0 {
			fmt.Println("No agents found.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tTYPE\tDESCRIPTION")
		for _, a := range agents {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", a.Name, a.Version, a.Type, a.Description)
		}
		w.Flush()
	}
	return nil
}

// ---- agent get ----

var agentGetVersion int

var agentGetCmd = &cobra.Command{
	Use:          "get <name>",
	Short:        "Get an agent definition",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		def, err := internal.GetAgentService().Get(cmd.Context(), args[0], optionalVersion(cmd, agentGetVersion))
		if err != nil {
			return err
		}
		return printRawJSON(def)
	},
}

// ---- agent delete ----

var agentDeleteVersion int

var agentDeleteCmd = &cobra.Command{
	Use:          "delete <name>",
	Short:        "Delete an agent definition",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !confirmDeletion("agent", args[0]) {
			fmt.Println("Aborted.")
			return nil
		}
		if err := internal.GetAgentService().Delete(cmd.Context(), args[0], optionalVersion(cmd, agentDeleteVersion)); err != nil {
			return err
		}
		fmt.Printf("Deleted agent '%s'.\n", args[0])
		return nil
	},
}

// ---- agent compile ----

var agentCompileCmd = &cobra.Command{
	Use:          "compile <config-file>",
	Short:        "Compile an agent config and show its execution plan",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		def, err := loadAgentConfig(args[0])
		if err != nil {
			return err
		}
		plan, err := internal.GetAgentService().Compile(cmd.Context(), def)
		if err != nil {
			return err
		}
		return printRawJSON(plan)
	},
}

// ---- agent execution (search history) ----

var (
	execName   string
	execStatus string
	execSince  string
	execWindow string
)

var agentExecutionCmd = &cobra.Command{
	Use:   "execution",
	Short: "Search agent execution history",
	Long: `Search agent execution history with optional filters.

Time formats for --since and --window: 30s, 5m, 1h, 6h, 1d, 7d, 1mo, 1y`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := GetOutputFormat(cmd)
		if err != nil {
			return err
		}
		freeText, err := buildExecutionFreeText(execSince, execWindow)
		if err != nil {
			return err
		}
		page, err := internal.GetAgentService().SearchExecutions(cmd.Context(), agent.ExecutionFilter{
			AgentName: execName,
			Status:    execStatus,
			FreeText:  freeText,
			Start:     0,
			Size:      defaultExecutionSearchSize,
		})
		if err != nil {
			return err
		}
		return renderExecutionPage(page, format)
	},
}

func renderExecutionPage(page agent.ExecutionPage, format OutputFormat) error {
	switch format {
	case OutputFormatJSON:
		data, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case OutputFormatCSV:
		w := NewCSVWriter()
		w.WriteHeader("ID", "AGENT", "STATUS", "START_TIME", "DURATION")
		for _, e := range page.Results {
			w.WriteRow(e.ExecutionID, e.AgentName, e.Status, truncateTimestamp(e.StartTime), formatMillis(e.ExecutionTime))
		}
		w.Flush()
	default:
		if len(page.Results) == 0 {
			fmt.Println("No executions found.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tAGENT\tSTATUS\tSTART TIME\tDURATION")
		for _, e := range page.Results {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				e.ExecutionID, e.AgentName, e.Status, truncateTimestamp(e.StartTime), formatMillis(e.ExecutionTime))
		}
		w.Flush()
		fmt.Printf("\n%d of %d execution(s).\n", len(page.Results), page.TotalHits)
	}
	return nil
}

// ---- agent status ----

var agentStatusCmd = &cobra.Command{
	Use:          "status <execution-id>",
	Short:        "Get detailed status of an execution",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		detail, err := internal.GetAgentService().Status(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printRawJSON(detail)
	},
}

// ---- agent respond (human-in-the-loop) ----

var (
	respondApprove bool
	respondDeny    bool
	respondReason  string
	respondMessage string
)

var agentRespondCmd = &cobra.Command{
	Use:          "respond <execution-id>",
	Short:        "Respond to a human-in-the-loop task",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !respondApprove && !respondDeny {
			return fmt.Errorf("specify --approve or --deny")
		}
		if err := internal.GetAgentService().Respond(cmd.Context(), args[0], agent.HumanResponse{
			Approved: respondApprove,
			Reason:   respondReason,
			Message:  respondMessage,
		}); err != nil {
			return err
		}
		fmt.Printf("Response sent for execution '%s'.\n", args[0])
		return nil
	},
}

// ---- agent prune ----

var (
	pruneOlderThan int
	pruneArchive   bool
	pruneDryRun    bool
)

var agentPruneCmd = &cobra.Command{
	Use:          "prune",
	Short:        "Delete (or archive) old execution records",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if pruneDryRun {
			fmt.Printf("Dry run: would prune executions older than %d day(s) (archive=%t).\n", pruneOlderThan, pruneArchive)
			return nil
		}
		res, err := internal.GetAgentService().Prune(cmd.Context(), agent.PruneRequest{
			OlderThanDays: pruneOlderThan,
			Archive:       pruneArchive,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Pruned %d execution record(s).\n", res.Removed)
		return nil
	},
}

// ---- agent init (local scaffolding; no server) ----

var (
	initModel    string
	initStrategy string
	initFormat   string
)

var agentInitCmd = &cobra.Command{
	Use:          "init <agent-name>",
	Short:        "Create a starter agent config file",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		model := initModel
		if model == "" {
			model = defaultInitModel
		}
		cfg := map[string]any{
			"name":         name,
			"description":  fmt.Sprintf("%s agent", name),
			"model":        model,
			"instructions": fmt.Sprintf("You are %s, a helpful AI assistant.", name),
			"maxTurns":     defaultInitMaxTurns,
			"tools":        []any{},
		}
		if initStrategy != "" {
			cfg["strategy"] = initStrategy
		}

		var data []byte
		var err error
		ext := "yaml"
		if initFormat == "json" {
			ext = "json"
			data, err = json.MarshalIndent(cfg, "", "  ")
		} else {
			data, err = yaml.Marshal(cfg)
		}
		if err != nil {
			return err
		}

		filename := fmt.Sprintf("%s.%s", name, ext)
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Created %s\n", filename)
		fmt.Printf("Run with: conductor agent run --config %s \"your prompt here\"\n", filename)
		return nil
	},
}

// ---- helpers (file I/O and formatting live here, in the cmd layer) ----

// loadAgentConfig reads a YAML or JSON agent config from disk and returns it as
// JSON bytes. File access stays in the cmd layer; the service/client only ever see
// parsed bytes — no path crosses a layer boundary.
func loadAgentConfig(path string) (json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config file (tried YAML and JSON): %w", err)
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return raw, nil
}

// printRawJSON pretty-prints raw server JSON, falling back to the raw bytes when the
// payload is not indentable.
func printRawJSON(raw json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		fmt.Println(string(raw))
		return nil
	}
	fmt.Println(buf.String())
	return nil
}

// optionalVersion returns a pointer to v only when the version flag was set.
func optionalVersion(cmd *cobra.Command, v int) *int {
	if cmd.Flags().Changed("version") {
		return &v
	}
	return nil
}

func truncateTimestamp(ts string) string {
	const secondsPrecision = 19 // YYYY-MM-DDТHH:MM:SS
	if len(ts) > secondsPrecision {
		return ts[:secondsPrecision]
	}
	return ts
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return (time.Duration(ms) * time.Millisecond).String()
}

// buildExecutionFreeText converts the --since / --window flags into the server's
// freeText time-range query.
func buildExecutionFreeText(since, window string) (string, error) {
	freeText := ""
	if since != "" {
		dur, err := parseTimeSpec(since)
		if err != nil {
			return "", fmt.Errorf("invalid --since value: %w", err)
		}
		start := time.Now().Add(-dur).UnixMilli()
		freeText = fmt.Sprintf("startTime:[%d TO *]", start)
	}
	if window != "" {
		spec := window
		const relativePrefix = "now-"
		if len(spec) > len(relativePrefix) && spec[:len(relativePrefix)] == relativePrefix {
			spec = spec[len(relativePrefix):]
		}
		dur, err := parseTimeSpec(spec)
		if err != nil {
			return "", fmt.Errorf("invalid --window value: %w", err)
		}
		end := time.Now().UnixMilli()
		start := time.Now().Add(-dur).UnixMilli()
		q := fmt.Sprintf("startTime:[%d TO %d]", start, end)
		if freeText != "" {
			freeText += " AND " + q
		} else {
			freeText = q
		}
	}
	return freeText, nil
}

var timeSpecPattern = regexp.MustCompile(`^(\d+)(s|m|h|d|mo|y)$`)

// parseTimeSpec parses durations like 30s, 5m, 1h, 1d, 1mo, 1y.
func parseTimeSpec(s string) (time.Duration, error) {
	m := timeSpecPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("expected format like 30s, 5m, 1h, 1d, 1mo, 1y; got %q", s)
	}
	n, _ := strconv.Atoi(m[1])
	unit := map[string]time.Duration{
		"s":  time.Second,
		"m":  time.Minute,
		"h":  time.Hour,
		"d":  24 * time.Hour,
		"mo": 30 * 24 * time.Hour,
		"y":  365 * 24 * time.Hour,
	}[m[2]]
	return time.Duration(n) * unit, nil
}

func init() {
	agentGetCmd.Flags().IntVar(&agentGetVersion, "version", 0, "Agent version (default: latest)")
	agentDeleteCmd.Flags().IntVar(&agentDeleteVersion, "version", 0, "Agent version (default: latest)")

	AddOutputFlags(agentListCmd)

	agentExecutionCmd.Flags().StringVar(&execName, "name", "", "Filter by agent name")
	agentExecutionCmd.Flags().StringVar(&execStatus, "status", "", "Filter by status (RUNNING, COMPLETED, FAILED, ...)")
	agentExecutionCmd.Flags().StringVar(&execSince, "since", "", "Show executions since (e.g. 30m, 1h, 1d)")
	agentExecutionCmd.Flags().StringVar(&execWindow, "window", "", "Time window (e.g. now-1h, now-7d)")
	AddOutputFlags(agentExecutionCmd)

	agentRespondCmd.Flags().BoolVar(&respondApprove, "approve", false, "Approve the pending action")
	agentRespondCmd.Flags().BoolVar(&respondDeny, "deny", false, "Deny the pending action")
	agentRespondCmd.Flags().StringVar(&respondReason, "reason", "", "Reason for the decision")
	agentRespondCmd.Flags().StringVarP(&respondMessage, "message", "m", "", "Free-form message")
	agentRespondCmd.MarkFlagsMutuallyExclusive("approve", "deny")

	agentPruneCmd.Flags().IntVar(&pruneOlderThan, "older-than", defaultPruneOlderThanDays, "Delete executions older than N days")
	agentPruneCmd.Flags().BoolVar(&pruneArchive, "archive", false, "Archive tasks instead of hard-deleting")
	agentPruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "Show what would be pruned without deleting")

	agentInitCmd.Flags().StringVarP(&initModel, "model", "", "", "LLM model (default: "+defaultInitModel+")")
	agentInitCmd.Flags().StringVarP(&initStrategy, "strategy", "s", "", "Multi-agent strategy (handoff, sequential, parallel, ...)")
	agentInitCmd.Flags().StringVarP(&initFormat, "format", "f", "yaml", "Output format: yaml or json")

	agentCmd.AddCommand(
		agentListCmd,
		agentGetCmd,
		agentDeleteCmd,
		agentCompileCmd,
		agentExecutionCmd,
		agentStatusCmd,
		agentRespondCmd,
		agentPruneCmd,
		agentInitCmd,
	)
	rootCmd.AddCommand(agentCmd)
}
