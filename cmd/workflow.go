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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/google/uuid"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var workflowCmd = &cobra.Command{
	Use:     "workflow",
	Short:   "Workflow definition and execution management",
	GroupID: "conductor",
}

var (
	// Workflow Definition Management Commands
	listWorkflowsMetadataCmd = &cobra.Command{
		Use:          "list",
		Short:        "List workflows",
		RunE:         listWorkflow,
		SilenceUsage: true,
	}

	getWorkflowMetadataCmd = &cobra.Command{
		Use:          "get <workflow_name> [version]",
		Short:        "Get Workflow",
		RunE:         getWorkflowMetadata,
		SilenceUsage: true,
	}

	getAllWorkflowMetadataCmd = &cobra.Command{
		Use:          "get_all",
		Short:        "Get All Workflows",
		RunE:         getAllWorkflowMetadata,
		SilenceUsage: true,
	}

	updateWorkflowMetadataCmd = &cobra.Command{
		Use:          "update <workflow_definition.json>",
		Short:        "Update Workflow",
		Long:         "Update an existing workflow from a JSON or JavaScript file.\n\nJavaScript format (--js flag) is experimental and subject to change.",
		RunE:         updateWorkflowMetadata,
		SilenceUsage: true,
	}

	createWorkflowMetadataCmd = &cobra.Command{
		Use:          "create <workflow_definition.json>",
		Short:        "Create Workflow",
		Long:         "Create a workflow from a JSON or JavaScript file.\n\nJavaScript format (--js flag) is experimental and subject to change.",
		RunE:         createWorkflowMetadata,
		SilenceUsage: true,
	}

	deleteWorkflowMetadataCmd = &cobra.Command{
		Use:          "delete <workflow_name> <version>",
		Short:        "Delete Workflow",
		RunE:         deleteWorkflowMetadata,
		SilenceUsage: true,
	}

	// Workflow Execution Management Commands
	searchExecutionCmd = &cobra.Command{
		Use:          "search",
		Short:        "Search for workflow executions",
		RunE:         searchWorkflowExecutions,
		SilenceUsage: true,
		Example:      "workflow search [flags] search_text",
	}

	statusExecutionCmd = &cobra.Command{
		Use:          "status <workflow_id>",
		Short:        "Get workflow execution status",
		RunE:         getWorkflowExecutionStatus,
		SilenceUsage: true,
		Example:      "workflow status [workflow_id] [workflow_id2]...",
	}

	getExecutionCmd = &cobra.Command{
		Use:          "get-execution <workflow_id>",
		Short:        "Get full workflow execution details",
		RunE:         getWorkflowExecution,
		SilenceUsage: true,
		Example:      "workflow get-execution [flags] [workflow_id] [workflow_id2]...",
	}

	startExecutionCmd = &cobra.Command{
		Use:          "start",
		Short:        "Start workflow execution",
		Long:         "Start workflow execution. Use --sync flag to execute synchronously and wait for completion.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			sync, _ := cmd.Flags().GetBool("sync")
			version, _ := cmd.Flags().GetInt32("version")
			if sync && version == 0 {
				return fmt.Errorf("--version is required when using --sync flag")
			}
			return nil
		},
		RunE:         startWorkflow,
		SilenceUsage: true,
		Example:      "workflow start --workflow my_workflow\nworkflow start --workflow my_workflow --sync --version 1",
	}

	terminateExecutionCmd = &cobra.Command{
		Use:          "terminate <workflow_id>",
		Short:        "Terminate a running workflow execution",
		RunE:         terminateWorkflow,
		SilenceUsage: true,
		Example:      "workflow terminate [workflow_id]",
	}

	pauseExecutionCmd = &cobra.Command{
		Use:          "pause <workflow_id>",
		Short:        "Pause a running workflow execution",
		RunE:         pauseWorkflow,
		SilenceUsage: true,
		Example:      "workflow pause [workflow_id]",
	}

	resumeExecutionCmd = &cobra.Command{
		Use:          "resume <workflow_id>",
		Short:        "Resume a paused workflow execution",
		RunE:         resumeWorkflow,
		SilenceUsage: true,
		Example:      "workflow resume [workflow_id]",
	}

	deleteExecutionCmd = &cobra.Command{
		Use:          "delete-execution <workflow_id>",
		Short:        "Delete a workflow execution",
		RunE:         deleteWorkflowExecution,
		SilenceUsage: true,
		Example:      "workflow delete-execution [workflow_id]\nworkflow delete-execution --archive [workflow_id]",
	}

	restartExecutionCmd = &cobra.Command{
		Use:          "restart <workflow_id>",
		Short:        "Restart a completed workflow",
		RunE:         restartWorkflow,
		SilenceUsage: true,
		Example:      "workflow restart [workflow_id]\nworkflow restart --use-latest [workflow_id]",
	}

	retryExecutionCmd = &cobra.Command{
		Use:          "retry <workflow_id>",
		Short:        "Retry the last failed task",
		RunE:         retryWorkflow,
		SilenceUsage: true,
		Example:      "workflow retry [workflow_id]",
	}

	skipTaskExecutionCmd = &cobra.Command{
		Use:          "skip-task <workflow_id> <task_reference_name>",
		Short:        "Skip a task in a running workflow",
		RunE:         skipTask,
		SilenceUsage: true,
		Example:      "workflow skip-task [workflow_id] [task_ref_name]",
	}

	rerunExecutionCmd = &cobra.Command{
		Use:          "rerun <workflow_id>",
		Short:        "Rerun workflow from a specific task",
		RunE:         rerunWorkflow,
		SilenceUsage: true,
		Example:      "workflow rerun [workflow_id] --task-id [task_id]",
	}

	jumpExecutionCmd = &cobra.Command{
		Use:          "jump <workflow_id> <task_reference_name>",
		Short:        "Jump workflow execution to given task",
		RunE:         jumpToTask,
		SilenceUsage: true,
		Example:      "workflow jump [workflow_id] [task_ref_name]",
	}

	updateStateExecutionCmd = &cobra.Command{
		Use:          "update-state <workflow_id>",
		Short:        "Update workflow state (variables and tasks)",
		RunE:         updateWorkflowState,
		SilenceUsage: true,
		Example:      "workflow update-state [workflow_id] --variables '{\"key\":\"value\"}'",
	}
)
func listWorkflow(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	metadataClient := internal.GetMetadataClient()
	workflows, _, err := metadataClient.GetAll(context.Background())
	if err != nil {
		return parseAPIError(err, "Failed to list workflows")
	}

	if jsonOutput {
		data, err := json.MarshalIndent(workflows, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling workflows: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION")
	for _, workflow := range workflows {
		description := workflow.Description
		if description == "" {
			description = "-"
		}
		// Truncate long descriptions
		if len(description) > 50 {
			description = description[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n",
			workflow.Name,
			workflow.Version,
			description,
		)
	}
	w.Flush()

	return nil
}

func getWorkflowMetadata(cmd *cobra.Command, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return cmd.Usage()
	}

	metadataClient := internal.GetMetadataClient()
	name := args[0]

	var versionOpts *client.MetadataResourceApiGetOpts
	var errorMsg string

	if len(args) == 2 {
		version, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version '%s': must be a number", args[1])
		}
		versionOpts = &client.MetadataResourceApiGetOpts{Version: optional.NewInt32(int32(version))}
		errorMsg = fmt.Sprintf("Failed to get workflow '%s' version %d", name, version)
	} else {
		versionOpts = &client.MetadataResourceApiGetOpts{}
		errorMsg = fmt.Sprintf("Failed to get workflow '%s'", name)
	}

	metadata, _, err := metadataClient.Get(context.Background(), html.EscapeString(name), versionOpts)
	if err != nil {
		return parseAPIError(err, errorMsg)
	}

	bytes, _ := json.MarshalIndent(metadata, "", "   ")
	fmt.Println(string(bytes))
	return nil
}

func getAllWorkflowMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	var expr string
	var regex *regexp.Regexp
	if len(args) == 1 {
		var err error
		expr = args[0]
		regex, err = regexp.Compile(expr)
		if err != nil {
			return err
		}

	}

	metadata, _, err := metadataClient.GetAll(context.Background())
	if err != nil {
		return parseAPIError(err, "Failed to get workflows")
	}
	fmt.Println("[")
	for i, data := range metadata {
		if regex != nil {
			if regex.Match([]byte(data.Name)) {
				bytes, _ := json.MarshalIndent(data, "", "   ")
				fmt.Println(string(bytes))
			}
		} else {
			bytes, _ := json.MarshalIndent(data, "", "   ")
			fmt.Println(string(bytes))
		}
		if i < len(metadata)-1 {
			fmt.Print(",")
		}
	}
	fmt.Println("]")

	return nil
}

func updateWorkflowMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	var data []byte
	var err error
	if len(args) == 1 {
		data, err = os.ReadFile(args[0])
		if err != nil {
			return err
		}
	} else {
		// Check if stdin has data available
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("error checking stdin: %v", err)
		}

		// If running interactively (no pipe/redirect), show usage
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return cmd.Usage()
		}

		data = read()
		if len(data) == 0 {
			return fmt.Errorf("no workflow data received from stdin")
		}
	}
	var workflowDefs []model.WorkflowDef

	javascript, _ := cmd.Flags().GetBool("js")
	if javascript {
		js := string(data)
		workflowJson, conversionError := getWorkflowFromJS(js)
		if conversionError != nil {
			return conversionError
		}
		data = []byte(workflowJson)
	}

	var workflowDef model.WorkflowDef
	err = json.Unmarshal(data, &workflowDef)
	if err != nil {
		return parseJSONError(err, string(data), "workflow definition")
	}
	workflowDefs = append(workflowDefs, workflowDef)
	_, err = metadataClient.Update(context.Background(), workflowDefs)
	if err != nil {
		return parseAPIError(err, "Failed to update workflow")
	}
	return nil
}

func createWorkflowMetadata(cmd *cobra.Command, args []string) error {
	var data []byte
	var err error
	if len(args) == 1 {
		data, err = os.ReadFile(args[0])
		if err != nil {
			fmt.Println("Error reading file ", err.Error())
			return err
		}
	} else {
		// Check if stdin has data available
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("error checking stdin: %v", err)
		}

		// If running interactively (no pipe/redirect), show usage
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return cmd.Usage()
		}

		data = read()
		if len(data) == 0 {
			return fmt.Errorf("no workflow data received from stdin")
		}
	}
	js := string(data)
	var json string
	javascript, _ := cmd.Flags().GetBool("js")
	if javascript {
		json, err = getWorkflowFromJS(js)
		data = []byte(json)
		if err != nil {
			return err
		}
	}
	err = registerWorkflow(data, true)
	return err
}

func registerWorkflow(data []byte, force bool) error {
	metadataClient := internal.GetMetadataClient()
	var workflowDef model.WorkflowDef
	err := json.Unmarshal(data, &workflowDef)
	if err != nil {
		return parseJSONError(err, string(data), "workflow definition")
	}
	_, err = metadataClient.RegisterWorkflowDef(context.Background(), force, workflowDef)
	if err != nil {
		return parseAPIError(err, "Failed to create workflow")
	}
	return nil
}

func deleteWorkflowMetadata(cmd *cobra.Command, args []string) error {
	return _deleteWorkflowMetadata(cmd, args)
}
func _deleteWorkflowMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	if len(args) == 0 {
		// Check if stdin has data available
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("error checking stdin: %v", err)
		}

		// If running interactively (no pipe/redirect), show usage
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return cmd.Usage()
		}

		workflows := readLines()
		if len(workflows) == 0 {
			return fmt.Errorf("no workflow data received from stdin")
		}
		log.Info("Read ", len(workflows), " from console")
		r := regexp.MustCompile(`[^\s"]+|"([^"]*)"`)
		if !yes {
			// When piped input, auto-confirm since user can't interact
			fmt.Printf("Auto-confirming deletion of %d workflow(s) from piped input. Use --yes to skip confirmation.\n", len(workflows))
		}
		for _, workflow := range workflows {
			a := r.FindAllString(workflow, -1)
			if len(a) != 2 {
				return errors.New("no version specified")
			}
			name := a[0]
			version, err := strconv.Atoi(a[1])
			if err != nil {
				return fmt.Errorf("invalid version '%s' for workflow '%s': %v", a[1], name, err)
			}
			log.Info("Deleting workflow: ", name, " version: ", version)
			_, err = metadataClient.UnregisterWorkflowDef(context.Background(), name, int32(version))
			if err != nil {
				return parseAPIError(err, fmt.Sprintf("Failed to delete workflow '%s' version %d", name, version))
			}
		}
		return nil
	} else if len(args) == 2 {
		name := args[0]
		version, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}

		// Confirm deletion
		resourceName := fmt.Sprintf("%s version %d", name, version)
		if !confirmDeletion("workflow", resourceName) {
			fmt.Println("Deletion cancelled")
			return nil
		}

		_, err = metadataClient.UnregisterWorkflowDef(context.Background(), name, int32(version))
		if err != nil {
			return parseAPIError(err, fmt.Sprintf("Failed to delete workflow '%s' version %d", name, version))
		}
		fmt.Printf("Workflow '%s' version %d deleted successfully\n", name, version)
		return nil
	}

	return cmd.Usage()

}

// parseJSONError provides helpful error messages for JSON parsing failures
func parseJSONError(err error, jsonContent string, contextName string) error {
	errStr := err.Error()

	// Common JSON syntax error patterns
	if strings.Contains(errStr, "invalid character") && strings.Contains(errStr, "in string literal") {
		// Find the approximate line number by counting newlines
		lines := strings.Split(jsonContent, "\n")

		// Look for unterminated strings (missing quotes)
		for i, line := range lines {
			// Simple heuristic: look for lines with odd number of quotes
			quoteCount := strings.Count(line, "\"") - strings.Count(line, "\\\"")
			if quoteCount%2 != 0 && strings.Contains(line, ":") {
				return fmt.Errorf("JSON syntax error in %s: unterminated string on line %d\nLine content: %s\nHint: Check for missing closing quote (\") on this line", contextName, i+1, strings.TrimSpace(line))
			}
		}

		return fmt.Errorf("JSON syntax error in %s: %s\nHint: Check for unterminated strings (missing quotes)", contextName, errStr)
	}

	if strings.Contains(errStr, "unexpected end of JSON input") {
		return fmt.Errorf("JSON syntax error in %s: unexpected end of file\nHint: Check for missing closing braces } or brackets ]", contextName)
	}

	if strings.Contains(errStr, "invalid character") {
		return fmt.Errorf("JSON syntax error in %s: %s\nHint: Check for invalid characters, missing commas, or malformed values", contextName, errStr)
	}

	// Fallback for other JSON errors
	return fmt.Errorf("Invalid %s format: %s", contextName, errStr)
}

// parseAPIError extracts useful error information from API responses
func parseAPIError(err error, defaultMsg string) error {
	errStr := err.Error()

	// Try to extract JSON from error message
	// Error format: "error: {...}, body: {...}"
	var jsonStr string
	if strings.Contains(errStr, "body: {") {
		// Extract the body part
		parts := strings.Split(errStr, "body: ")
		if len(parts) > 1 {
			jsonStr = parts[1]
		}
	} else if strings.Contains(errStr, "error: {") {
		// Extract the error part
		parts := strings.Split(errStr, "error: ")
		if len(parts) > 1 {
			jsonStr = strings.Split(parts[1], ", body:")[0]
		}
	}

	if jsonStr != "" {
		// Try to parse the JSON with validation errors
		var errorResponse struct {
			Status           int    `json:"status"`
			Message          string `json:"message"`
			Error            string `json:"error"`
			ValidationErrors []struct {
				Path    string `json:"path"`
				Message string `json:"message"`
			} `json:"validationErrors"`
		}

		if json.Unmarshal([]byte(jsonStr), &errorResponse) == nil {
			// Check for authentication errors
			if errorResponse.Error == "INVALID_TOKEN" || errorResponse.Error == "ERROR_WHILE_FETCHING" {
				message := errorResponse.Message
				if message == "" {
					message = "Authentication failed"
				}
				return fmt.Errorf("%s\nPlease check your authentication settings. Run 'conductor config save' to configure credentials", message)
			}

			if errorResponse.Message != "" {
				message := fmt.Sprintf("%s: %s", defaultMsg, errorResponse.Message)

				// Add validation error details if available
				if len(errorResponse.ValidationErrors) > 0 {
					message += "\nValidation errors:"
					for _, validationErr := range errorResponse.ValidationErrors {
						if validationErr.Path != "" {
							message += fmt.Sprintf("\n  - %s: %s", validationErr.Path, validationErr.Message)
						} else {
							message += fmt.Sprintf("\n  - %s", validationErr.Message)
						}
					}
				}

				if errorResponse.Status > 0 {
					message += fmt.Sprintf(" (status: %d)", errorResponse.Status)
				}

				return fmt.Errorf(message)
			}
		}
	}

	// Fallback to original error if parsing fails
	return fmt.Errorf("%s: %v", defaultMsg, err)
}

func read() []byte {
	var b bytes.Buffer
	in := bufio.NewReader(os.Stdin)
	defer os.Stdin.Close()
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			// If we have a partial line (no newline at EOF), include it
			if len(line) > 0 {
				b.WriteString(line)
			}
			break
		}
		b.WriteString(line)
	}
	return b.Bytes()
}

func readString() string {
	var b bytes.Buffer
	in := bufio.NewReader(os.Stdin)
	defer os.Stdin.Close()
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			// If we have a partial line (no newline at EOF), include it
			if len(line) > 0 {
				b.WriteString(line)
			}
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

func readLines() []string {
	var lines = make([]string, 0)
	in := bufio.NewReader(os.Stdin)
	defer os.Stdin.Close()
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			// If we have a partial line (no newline at EOF), include it
			if len(line) > 0 {
				line = strings.TrimSpace(line)
				if len(line) > 0 {
					lines = append(lines, line)
				}
			}
			break
		}
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

var script = `
const wf = workflow();
wf.tasks.forEach(t => {
    if(t.inputParameters == null) t.inputParameters = {};
    if(t.function != null) {
        t.type = "INLINE";
        t.inputParameters["expression"] = t.function.toString() + "\n" + t.function.name + "();";
        t.inputParameters["evaluatorType"] = "graaljs";
    } else if (t.type == "WAIT") {
        if(t.name == null) t.name = "WAIT";
        if(t.duration != null) {
            t.inputParameters["duration"] = t.duration;
        }
    }
    if(t.taskReferenceName == null) {
        t.taskReferenceName = t.name;
    }

});
JSON.stringify(wf);
`

func getWorkflowFromJS(js string) (string, error) {
	vm := goja.New()
	new(require.Registry).Enable(vm)
	console.Enable(vm)

	updatedScript := js + "\n" + script
	prog, err := goja.Compile("", updatedScript, true)
	if err != nil {
		fmt.Printf("Error compiling the script %v ", err)
		return "", err
	}
	value, err := vm.RunProgram(prog)
	return value.String(), err
}
func parseTimeToEpochMillis(timeStr string) (int64, error) {
	if timeStr == "" {
		return 0, fmt.Errorf("empty time string")
	}

	// Try common formats
	formats := []string{
		"2006-01-02 15:04:05",  // YYYY-MM-DD HH:MM:SS
		"2006-01-02T15:04:05Z", // RFC3339 UTC
		"2006-01-02T15:04:05",  // RFC3339 without timezone
		"2006-01-02 15:04",     // YYYY-MM-DD HH:MM
		"2006-01-02",           // YYYY-MM-DD
		"01/02/2006 15:04:05",  // MM/DD/YYYY HH:MM:SS
		"01/02/2006",           // MM/DD/YYYY
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t.Unix() * 1000, nil // Convert to milliseconds
		}
	}

	// Try parsing as epoch milliseconds directly
	if epochMs, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
		return epochMs, nil
	}

	return 0, fmt.Errorf("unable to parse time '%s'. Supported formats: YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, MM/DD/YYYY, epoch milliseconds", timeStr)
}

// debugSearchWorkflows makes a raw HTTP request to debug server response
func debugSearchWorkflows(freeText, query string, count int32) error {
	serverURL := viper.GetString("server")
	if serverURL == "" {
		serverURL = "http://localhost:8080/api"
	}
	serverURL = strings.TrimSuffix(serverURL, "/")
	if !strings.HasSuffix(serverURL, "/api") {
		serverURL = serverURL + "/api"
	}

	// Build URL with query parameters
	params := neturl.Values{}
	params.Set("start", "0")
	params.Set("size", strconv.Itoa(int(count)))
	params.Set("freeText", freeText)
	params.Set("sort", "startTime:DESC")
	if query != "" {
		params.Set("query", query)
	}

	searchURL := fmt.Sprintf("%s/workflow/search?%s", serverURL, params.Encode())
	fmt.Printf("DEBUG: Request URL: %s\n\n", searchURL)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add auth header if available
	authToken := viper.GetString("auth-token")
	if authToken != "" {
		req.Header.Set("X-Authorization", authToken)
	}
	cachedToken := viper.GetString("cached-token")
	if cachedToken != "" {
		req.Header.Set("X-Authorization", cachedToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("DEBUG: Response Status: %d\n\n", resp.StatusCode)
	fmt.Printf("DEBUG: Raw Response Body:\n%s\n", string(body))

	// Try to pretty print if it's JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err == nil {
		prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
		fmt.Printf("\nDEBUG: Pretty JSON:\n%s\n", string(prettyJSON))
	}

	return nil
}

func searchWorkflowExecutions(cmd *cobra.Command, args []string) error {
	debug, _ := cmd.Flags().GetBool("debug")

	freeText := "*"
	if len(args) == 1 {
		freeText = args[0]
	}

	count, _ := cmd.Flags().GetInt32("count")
	if count > 1000 {
		//fmt.Println("count exceeds max allowed 1000.  Will only show the first 1000 matching results")
		//count = 1000
	} else if count == 0 {
		count = 10
	}

	// Build query dynamically with AND conditions
	var queryParts []string

	// Workflow name filter
	workflowName, _ := cmd.Flags().GetString("workflow")
	if workflowName != "" {
		queryParts = append(queryParts, "workflowType IN ("+workflowName+")")
	}

	// Status filter
	status, _ := cmd.Flags().GetString("status")
	if status != "" {
		queryParts = append(queryParts, "status IN ("+status+")")
	}

	// Start time filter (after)
	startTimeAfter, _ := cmd.Flags().GetString("start-time-after")
	if startTimeAfter != "" {
		startTimeAfterMs, err := parseTimeToEpochMillis(startTimeAfter)
		if err != nil {
			return fmt.Errorf("invalid start-time-after: %v", err)
		}
		queryParts = append(queryParts, "startTime>"+strconv.FormatInt(startTimeAfterMs, 10))
	}

	// Start time filter (before)
	startTimeBefore, _ := cmd.Flags().GetString("start-time-before")
	if startTimeBefore != "" {
		startTimeBeforeMs, err := parseTimeToEpochMillis(startTimeBefore)
		if err != nil {
			return fmt.Errorf("invalid start-time-before: %v", err)
		}
		queryParts = append(queryParts, "startTime<"+strconv.FormatInt(startTimeBeforeMs, 10))
	}

	// Combine all query parts with AND
	query := strings.Join(queryParts, " AND ")

	// Debug mode: make raw HTTP request to see server response
	if debug {
		return debugSearchWorkflows(freeText, query, count)
	}

	workflowClient := internal.GetWorkflowClient()

	searchOpts := client.WorkflowResourceApiSearchOpts{
		Start:    optional.NewInt32(0),
		Size:     optional.NewInt32(count),
		FreeText: optional.NewString(freeText),
		Sort:     optional.NewString("startTime:DESC"),
	}

	// Only add query if we have conditions
	if query != "" {
		searchOpts.Query = optional.NewString(query)
	}

	results, _, err := workflowClient.Search(context.Background(), &searchOpts)
	if err != nil {
		return parseAPIError(err, "Failed to search workflows")
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		data, err := json.MarshalIndent(results, "", "   ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Table output
	fmt.Printf("%-25s %-38s %-30s %-25s %-15s\n", "START TIME", "WORKFLOW ID", "WORKFLOW NAME", "END TIME", "STATUS")
	for _, item := range results.Results {
		startTime := item.StartTime
		if startTime == "" {
			startTime = "-"
		}
		endTime := item.EndTime
		if endTime == "" {
			endTime = "-"
		}
		fmt.Printf("%-25s %-38s %-30s %-25s %-15s\n",
			startTime,
			item.WorkflowId,
			item.WorkflowType,
			endTime,
			item.Status,
		)
	}

	return nil
}

func getWorkflowExecutionStatus(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}
	workflowClient := internal.GetWorkflowClient()

	for i := 0; i < len(args); i++ {
		id := args[i]
		status, _, getStateErr := workflowClient.GetWorkflowState(context.Background(), id, true, true)
		if getStateErr != nil {
			return parseAPIError(getStateErr, fmt.Sprintf("Failed to get workflow status for '%s'", id))
		}
		fmt.Println(status.Status)
	}
	return nil
}

func getWorkflowExecution(cmd *cobra.Command, args []string) error {

	if len(args) == 0 {
		return cmd.Usage()
	}
	workflowClient := internal.GetWorkflowClient()

	full, _ := cmd.Flags().GetBool("complete")
	for i := 0; i < len(args); i++ {
		id := args[i]
		if full {

			options := &client.WorkflowResourceApiGetExecutionStatusOpts{IncludeTasks: optional.NewBool(true)}
			status, _, err := workflowClient.GetExecutionStatus(context.Background(), id, options)
			if err != nil {
				return parseAPIError(err, fmt.Sprintf("Failed to get workflow execution for '%s'", id))
			}
			data, marshallError := json.MarshalIndent(status, "", "   ")
			if marshallError != nil {
				return marshallError
			}
			fmt.Println(string(data))

		} else {
			options := &client.WorkflowResourceApiGetExecutionStatusOpts{IncludeTasks: optional.NewBool(false)}
			status, _, err := workflowClient.GetExecutionStatus(context.Background(), id, options)
			if err != nil {
				return parseAPIError(err, fmt.Sprintf("Failed to get workflow execution for '%s'", id))
			}
			// Remove workflowDefinition from output
			status.WorkflowDefinition = nil
			data, marshallError := json.MarshalIndent(status, "", "   ")
			if marshallError != nil {
				return marshallError
			}
			fmt.Println(string(data))
		}

	}
	return nil
}

func terminateWorkflow(cmd *cobra.Command, args []string) error {

	if len(args) == 0 {
		return cmd.Usage()
	}
	workflowClient := internal.GetWorkflowClient()
	for i := 0; i < len(args); i++ {
		id := args[i]
		fmt.Println(id)
		options := &client.WorkflowResourceApiTerminateOpts{
			Reason: optional.NewString("Terminated by background process"),
		}
		_, err := workflowClient.Terminate(context.Background(), id, options)
		if err != nil {
			fmt.Println("error terminating", id, err.Error())
		}
	}

	return nil
}

func startWorkflow(cmd *cobra.Command, args []string) error {
	workflowName, _ := cmd.Flags().GetString("workflow")
	version, _ := cmd.Flags().GetInt32("version")
	input, _ := cmd.Flags().GetString("input")
	inputFile, _ := cmd.Flags().GetString("file")
	correlationId, _ := cmd.Flags().GetString("correlation")
	sync, _ := cmd.Flags().GetBool("sync")

	if workflowName == "" {
		if len(args) == 1 {
			workflowName = args[0]
		} else {
			return cmd.Usage()
		}
	}

	var inputJson []byte
	var err error

	if input != "" {
		inputJson = []byte(input)
	} else if inputFile != "" {
		inputJson, err = os.ReadFile(inputFile)
		if err != nil {
			return err
		}
	}

	if inputJson == nil {
		inputJson = []byte("{}")
	}

	var inputMap map[string]interface{}
	err = json.Unmarshal(inputJson, &inputMap)
	if err != nil {
		return err
	}

	workflowClient := internal.GetWorkflowClient()

	// Synchronous execution
	if sync {
		requestId, _ := uuid.NewRandom()
		request := model.StartWorkflowRequest{
			Name:          workflowName,
			Version:       version,
			CorrelationId: correlationId,
			Input:         inputMap,
			Priority:      0,
		}

		waitUntil, _ := cmd.Flags().GetString("wait-until")
		log.Debug("wait until ", waitUntil)

		run, _, execErr := workflowClient.ExecuteWorkflow(context.Background(), request, requestId.String(), workflowName, version, waitUntil)
		if execErr != nil {
			return parseAPIError(execErr, "Failed to start workflow")
		}

		data, jsonError := json.MarshalIndent(run, "", "   ")
		if jsonError != nil {
			return jsonError
		}
		fmt.Println(string(data))
		return nil
	}

	// Asynchronous execution
	opts := &client.WorkflowResourceApiStartWorkflowOpts{}
	if version > 0 {
		opts.Version = optional.NewInt32(version)
	}
	if correlationId != "" {
		opts.CorrelationId = optional.NewString(correlationId)
	}

	workflowId, _, startErr := workflowClient.StartWorkflow(cmd.Context(), inputMap, workflowName, opts)
	if startErr != nil {
		return parseAPIError(startErr, "Failed to start workflow")
	}
	fmt.Println(workflowId)

	return nil
}
func pauseWorkflow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	workflowClient := internal.GetWorkflowClient()
	for i := 0; i < len(args); i++ {
		id := args[i]
		_, err := workflowClient.PauseWorkflow(context.Background(), id)
		if err != nil {
			fmt.Printf("error pausing workflow %s: %s\n", id, err.Error())
		} else {
			fmt.Printf("workflow %s paused successfully\n", id)
		}
	}

	return nil
}

func resumeWorkflow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	workflowClient := internal.GetWorkflowClient()
	for i := 0; i < len(args); i++ {
		id := args[i]
		_, err := workflowClient.ResumeWorkflow(context.Background(), id)
		if err != nil {
			fmt.Printf("error resuming workflow %s: %s\n", id, err.Error())
		} else {
			fmt.Printf("workflow %s resumed successfully\n", id)
		}
	}

	return nil
}

func deleteWorkflowExecution(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	archive, _ := cmd.Flags().GetBool("archive")

	workflowClient := internal.GetWorkflowClient()
	for i := 0; i < len(args); i++ {
		workflowId := args[i]

		// Confirm deletion
		if !confirmDeletion("workflow execution", workflowId) {
			fmt.Printf("Skipping deletion of workflow execution '%s'\n", workflowId)
			continue
		}

		options := &client.WorkflowResourceApiDeleteOpts{
			ArchiveWorkflow: optional.NewBool(archive),
		}
		_, err := workflowClient.Delete(context.Background(), workflowId, options)
		if err != nil {
			fmt.Printf("error deleting workflow execution %s: %s\n", workflowId, err.Error())
		} else {
			fmt.Printf("workflow execution %s deleted successfully\n", workflowId)
		}
	}

	return nil
}

func restartWorkflow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	useLatest, _ := cmd.Flags().GetBool("use-latest")

	workflowClient := internal.GetWorkflowClient()
	for i := 0; i < len(args); i++ {
		workflowId := args[i]

		options := &client.WorkflowResourceApiRestartOpts{
			UseLatestDefinitions: optional.NewBool(useLatest),
		}
		_, err := workflowClient.Restart(context.Background(), workflowId, options)
		if err != nil {
			fmt.Printf("error restarting workflow %s: %s\n", workflowId, err.Error())
		} else {
			fmt.Printf("workflow %s restarted successfully\n", workflowId)
		}
	}

	return nil
}

func retryWorkflow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	resumeSubworkflowTasks, _ := cmd.Flags().GetBool("resume-subworkflow-tasks")

	workflowClient := internal.GetWorkflowClient()
	for i := 0; i < len(args); i++ {
		workflowId := args[i]

		options := &client.WorkflowResourceApiRetryOpts{
			ResumeSubworkflowTasks: optional.NewBool(resumeSubworkflowTasks),
		}
		_, err := workflowClient.Retry(context.Background(), workflowId, options)
		if err != nil {
			fmt.Printf("error retrying workflow %s: %s\n", workflowId, err.Error())
		} else {
			fmt.Printf("workflow %s retry initiated successfully\n", workflowId)
		}
	}

	return nil
}

func skipTask(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return cmd.Usage()
	}

	workflowId := args[0]
	taskReferenceName := args[1]

	taskInput, _ := cmd.Flags().GetString("task-input")
	taskOutput, _ := cmd.Flags().GetString("task-output")

	var inputMap map[string]interface{}
	var outputMap map[string]interface{}

	if taskInput != "" {
		err := json.Unmarshal([]byte(taskInput), &inputMap)
		if err != nil {
			return fmt.Errorf("invalid task-input JSON: %v", err)
		}
	}

	if taskOutput != "" {
		err := json.Unmarshal([]byte(taskOutput), &outputMap)
		if err != nil {
			return fmt.Errorf("invalid task-output JSON: %v", err)
		}
	}

	skipTaskRequest := model.SkipTaskRequest{
		TaskInput:  inputMap,
		TaskOutput: outputMap,
	}

	workflowClient := internal.GetWorkflowClient()
	_, err := workflowClient.SkipTaskFromWorkflow(context.Background(), workflowId, taskReferenceName, skipTaskRequest)
	if err != nil {
		return fmt.Errorf("error skipping task %s in workflow %s: %v", taskReferenceName, workflowId, err)
	}

	fmt.Printf("task %s in workflow %s skipped successfully\n", taskReferenceName, workflowId)
	return nil
}

func rerunWorkflow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	workflowId := args[0]
	taskId, _ := cmd.Flags().GetString("task-id")
	correlationId, _ := cmd.Flags().GetString("correlation-id")
	taskInput, _ := cmd.Flags().GetString("task-input")
	workflowInput, _ := cmd.Flags().GetString("workflow-input")

	var taskInputMap map[string]interface{}
	var workflowInputMap map[string]interface{}

	if taskInput != "" {
		err := json.Unmarshal([]byte(taskInput), &taskInputMap)
		if err != nil {
			return fmt.Errorf("invalid task-input JSON: %v", err)
		}
	}

	if workflowInput != "" {
		err := json.Unmarshal([]byte(workflowInput), &workflowInputMap)
		if err != nil {
			return fmt.Errorf("invalid workflow-input JSON: %v", err)
		}
	}

	rerunRequest := model.RerunWorkflowRequest{
		ReRunFromTaskId:     taskId,
		ReRunFromWorkflowId: workflowId,
		CorrelationId:       correlationId,
		TaskInput:           taskInputMap,
		WorkflowInput:       workflowInputMap,
	}

	workflowClient := internal.GetWorkflowClient()
	newWorkflowId, _, err := workflowClient.Rerun(context.Background(), rerunRequest, workflowId)
	if err != nil {
		return fmt.Errorf("error rerunning workflow %s: %v", workflowId, err)
	}

	fmt.Println(newWorkflowId)
	return nil
}

func jumpToTask(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return cmd.Usage()
	}

	workflowId := args[0]
	taskReferenceName := args[1]
	taskInput, _ := cmd.Flags().GetString("task-input")

	var inputMap map[string]interface{}
	if taskInput != "" {
		err := json.Unmarshal([]byte(taskInput), &inputMap)
		if err != nil {
			return fmt.Errorf("invalid task-input JSON: %v", err)
		}
	}

	if inputMap == nil {
		inputMap = make(map[string]interface{})
	}

	opts := &client.WorkflowResourceApiJumpToTaskOpts{
		TaskReferenceName: optional.NewString(taskReferenceName),
	}

	workflowClient := internal.GetWorkflowClient()
	_, err := workflowClient.JumpToTask(context.Background(), inputMap, workflowId, opts)
	if err != nil {
		return fmt.Errorf("error jumping to task %s in workflow %s: %v", taskReferenceName, workflowId, err)
	}

	fmt.Printf("workflow %s jumped to task %s successfully\n", workflowId, taskReferenceName)
	return nil
}

func updateWorkflowState(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	workflowId := args[0]
	requestId, _ := cmd.Flags().GetString("request-id")
	waitUntilTaskRef, _ := cmd.Flags().GetString("wait-until-task-ref")
	waitForSeconds, _ := cmd.Flags().GetInt32("wait-for-seconds")
	variables, _ := cmd.Flags().GetString("variables")
	taskUpdates, _ := cmd.Flags().GetString("task-updates")

	if requestId == "" {
		reqId, _ := uuid.NewRandom()
		requestId = reqId.String()
	}

	stateUpdate := model.WorkflowStateUpdate{}

	if variables != "" {
		var varsMap map[string]interface{}
		err := json.Unmarshal([]byte(variables), &varsMap)
		if err != nil {
			return fmt.Errorf("invalid variables JSON: %v", err)
		}
		stateUpdate.Variables = varsMap
	}

	if taskUpdates != "" {
		var taskResult model.TaskResult
		err := json.Unmarshal([]byte(taskUpdates), &taskResult)
		if err != nil {
			return fmt.Errorf("invalid task-updates JSON: %v", err)
		}
		stateUpdate.TaskResult = &taskResult
	}

	opts := &client.WorkflowResourceApiUpdateWorkflowAndTaskStateOpts{
		WaitUntilTaskRef: optional.NewString(waitUntilTaskRef),
		WaitForSeconds:   optional.NewInt32(waitForSeconds),
	}

	workflowClient := internal.GetWorkflowClient()
	workflow, _, err := workflowClient.UpdateWorkflowAndTaskState(context.Background(), stateUpdate, requestId, workflowId, opts)
	if err != nil {
		return fmt.Errorf("error updating workflow state for %s: %v", workflowId, err)
	}

	data, _ := json.MarshalIndent(workflow, "", "   ")
	fmt.Println(string(data))
	return nil
}


func init() {
	rootCmd.AddCommand(workflowCmd)

	// Definition management flags
	createWorkflowMetadataCmd.Flags().Bool("force", false, "--force overwrite existing workflow")
	createWorkflowMetadataCmd.Flags().Bool("js", false, "Input is javascript file")
	createWorkflowMetadataCmd.Flags().Bool("json", true, "Input is json file")
	createWorkflowMetadataCmd.MarkFlagsMutuallyExclusive("js", "json")
	listWorkflowsMetadataCmd.Flags().Bool("json", false, "Print complete JSON output")

	// Execution management flags
	searchExecutionCmd.Flags().Int32P("count", "c", 10, "No of workflow executions to return (max 1000)")
	searchExecutionCmd.Flags().StringP("status", "s", "", "Filter by status one of (COMPLETED, FAILED, PAUSED, RUNNING, TERMINATED, TIMED_OUT)")
	searchExecutionCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	searchExecutionCmd.Flags().String("start-time-after", "", "Filter executions started after this time (YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, or epoch ms)")
	searchExecutionCmd.Flags().String("start-time-before", "", "Filter executions started before this time (YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, or epoch ms)")
	searchExecutionCmd.Flags().Bool("json", false, "Output complete JSON instead of table")
	searchExecutionCmd.Flags().Bool("debug", false, "Print raw server response for debugging")

	startExecutionCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	startExecutionCmd.Flags().StringP("input", "i", "", "Input json")
	startExecutionCmd.Flags().StringP("file", "f", "", "Input file with json data")
	startExecutionCmd.Flags().Int32("version", 0, "Workflow version (optional)")
	startExecutionCmd.Flags().StringP("correlation", "", "", "Correlation ID")
	startExecutionCmd.Flags().Bool("sync", false, "Execute synchronously and wait for completion")
	startExecutionCmd.Flags().StringP("wait-until", "u", "", "Wait until task completes (only with --sync)")
	startExecutionCmd.MarkFlagsMutuallyExclusive("input", "file")

	getExecutionCmd.Flags().BoolP("complete", "c", false, "Include complete details")
	deleteExecutionCmd.Flags().BoolP("archive", "a", false, "Archive the workflow execution instead of removing it completely")

	restartExecutionCmd.Flags().Bool("use-latest", false, "Use latest workflow definition when restarting")
	retryExecutionCmd.Flags().Bool("resume-subworkflow-tasks", false, "Resume subworkflow tasks")
	skipTaskExecutionCmd.Flags().String("task-input", "", "Task input as JSON string")
	skipTaskExecutionCmd.Flags().String("task-output", "", "Task output as JSON string")

	rerunExecutionCmd.Flags().String("task-id", "", "Task ID to rerun from")
	rerunExecutionCmd.Flags().String("correlation-id", "", "Correlation ID for the rerun")
	rerunExecutionCmd.Flags().String("task-input", "", "Task input as JSON string")
	rerunExecutionCmd.Flags().String("workflow-input", "", "Workflow input as JSON string")

	jumpExecutionCmd.Flags().String("task-input", "", "Task input as JSON string")

	updateStateExecutionCmd.Flags().String("request-id", "", "Request ID (auto-generated if not provided)")
	updateStateExecutionCmd.Flags().String("wait-until-task-ref", "", "Wait until this task reference completes")
	updateStateExecutionCmd.Flags().Int32("wait-for-seconds", 10, "Wait for seconds")
	updateStateExecutionCmd.Flags().String("variables", "", "Variables to update as JSON string")
	updateStateExecutionCmd.Flags().String("task-updates", "", "Task updates as JSON string")

	workflowCmd.AddCommand(
		// Definition management
		listWorkflowsMetadataCmd,
		getWorkflowMetadataCmd,
		getAllWorkflowMetadataCmd,
		updateWorkflowMetadataCmd,
		createWorkflowMetadataCmd,
		deleteWorkflowMetadataCmd,
		// Execution management
		searchExecutionCmd,
		statusExecutionCmd,
		getExecutionCmd,
		startExecutionCmd,
		terminateExecutionCmd,
		pauseExecutionCmd,
		resumeExecutionCmd,
		deleteExecutionCmd,
		restartExecutionCmd,
		retryExecutionCmd,
		skipTaskExecutionCmd,
		rerunExecutionCmd,
		jumpExecutionCmd,
		updateStateExecutionCmd,
	)
}
