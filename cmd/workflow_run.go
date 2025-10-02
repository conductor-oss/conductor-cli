package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/google/uuid"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	// Execution command group
	executionCmd = &cobra.Command{
		Use:   "execution",
		Short: "Workflow execution management",
		Long:  "Commands for managing workflow executions",
	}

	// Execution subcommands
	searchExecutionCmd = &cobra.Command{
		Use:          "search",
		Short:        "Search for workflow executions",
		RunE:         searchWorkflowExecutions,
		SilenceUsage: true,
		Example:      "execution search [flags] search_text",
	}

	statusExecutionCmd = &cobra.Command{
		Use:          "status",
		Short:        "Get workflow execution status",
		RunE:         getWorkflowExecutionStatus,
		SilenceUsage: true,
		Example:      "execution status [workflow_id] [workflow_id2]...",
	}

	getExecutionCmd = &cobra.Command{
		Use:          "get",
		Short:        "Get full workflow execution details",
		RunE:         getWorkflowExecution,
		SilenceUsage: true,
		Example:      "execution get [flags] [workflow_id] [workflow_id2]...",
	}

	startExecutionCmd = &cobra.Command{
		Use:          "start",
		Short:        "Start workflow execution",
		RunE:         startWorkflow,
		SilenceUsage: true,
		Example:      "execution start [flags]",
	}

	executeExecutionCmd = &cobra.Command{
		Use:          "execute",
		Short:        "Execute workflow and get output",
		RunE:         startWorkflow,
		SilenceUsage: true,
		Example:      "execution execute [flags]",
	}

	terminateExecutionCmd = &cobra.Command{
		Use:          "terminate",
		Short:        "Terminate a running workflow execution",
		RunE:         terminateWorkflow,
		SilenceUsage: true,
		Example:      "execution terminate [flags]",
	}

	pauseExecutionCmd = &cobra.Command{
		Use:          "pause <workflow_id>",
		Short:        "Pause a running workflow execution",
		RunE:         pauseWorkflow,
		SilenceUsage: true,
		Example:      "execution pause [workflow_id]",
	}

	resumeExecutionCmd = &cobra.Command{
		Use:          "resume <workflow_id>",
		Short:        "Resume a paused workflow execution",
		RunE:         resumeWorkflow,
		SilenceUsage: true,
		Example:      "execution resume [workflow_id]",
	}

	deleteExecutionCmd = &cobra.Command{
		Use:          "delete <workflow_id>",
		Short:        "Delete a workflow execution",
		RunE:         deleteWorkflowExecution,
		SilenceUsage: true,
		Example:      "execution delete [workflow_id]\nexecution delete --archive [workflow_id]",
	}

)

// parseTimeToEpochMillis parses human-readable time formats to epoch milliseconds
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

func searchWorkflowExecutions(cmd *cobra.Command, args []string) error {

	workflowClient := internal.GetWorkflowClient()

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
		return err
	}

	for _, item := range results.Results {
		fmt.Println(item.WorkflowId)
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
			return getStateErr
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
				return err
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
				return err
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

	opts := &client.WorkflowResourceApiStartWorkflowOpts{}
	if version > 0 {
		opts.Version = optional.NewInt32(int32(version))
	}
	if correlationId != "" {
		opts.CorrelationId = optional.NewString(correlationId)
	}

	workflowClient := internal.GetWorkflowClient()
	executeSync, notFound := cmd.Flags().GetBool("sync")
	if notFound != nil && executeSync {
		requestId, _ := uuid.NewRandom()
		request := model.StartWorkflowRequest{
			Name:          workflowName,
			Version:       version,
			CorrelationId: "",
			Input:         inputMap,
			Priority:      0,
		}
		waitUntil, _ := cmd.Flags().GetString("wait-until")
		log.Debug("wait until ", waitUntil)
		run, _, execErr := workflowClient.ExecuteWorkflow(context.Background(), request, requestId.String(), workflowName, version, waitUntil)
		if execErr != nil {
			return execErr
		}
		data, jsonError := json.MarshalIndent(run, "", "   ")
		if jsonError != nil {
			return jsonError
		}
		fmt.Println(string(data))

	} else {
		workflowId, _, startErr := workflowClient.StartWorkflow(cmd.Context(), inputMap, workflowName, opts)
		if startErr != nil {
			return startErr
		}
		fmt.Println(workflowId)
	}

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

func init() {
	searchExecutionCmd.Flags().Int32P("count", "c", 10, "No of workflow executions to return (max 1000)")
	searchExecutionCmd.Flags().StringP("status", "s", "", "Filter by status one of (COMPLETED, FAILED, PAUSED, RUNNING, TERMINATED, TIMED_OUT)")
	searchExecutionCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	searchExecutionCmd.Flags().String("start-time-after", "", "Filter executions started after this time (YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, or epoch ms)")
	searchExecutionCmd.Flags().String("start-time-before", "", "Filter executions started before this time (YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, or epoch ms)")

	startExecutionCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	startExecutionCmd.Flags().StringP("input", "i", "", "Input json")
	startExecutionCmd.Flags().StringP("file", "f", "", "Input file with json data")
	startExecutionCmd.Flags().Int32("version", 0, "Workflow version (optional)")
	startExecutionCmd.MarkFlagsMutuallyExclusive("input", "file")

	executeExecutionCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	executeExecutionCmd.Flags().StringP("input", "i", "", "Input json")
	executeExecutionCmd.Flags().StringP("file", "f", "", "Input file with json data")
	executeExecutionCmd.Flags().StringP("wait-until", "u", "", "Wait until task completes (instead of entire workflow)")
	executeExecutionCmd.Flags().Int32("version", 1, "Workflow version (optional)")
	executeExecutionCmd.Flags().BoolP("sync", "s", true, "Run synchronously")
	executeExecutionCmd.MarkFlagsMutuallyExclusive("input", "file")

	getExecutionCmd.Flags().BoolP("complete", "c", false, "Include complete details")
	deleteExecutionCmd.Flags().BoolP("archive", "a", false, "Archive the workflow execution instead of removing it completely")

	executionCmd.AddCommand(
		searchExecutionCmd,
		statusExecutionCmd,
		getExecutionCmd,
		startExecutionCmd,
		executeExecutionCmd,
		terminateExecutionCmd,
		pauseExecutionCmd,
		resumeExecutionCmd,
		deleteExecutionCmd,
	)

	// Add execution command to root
	rootCmd.AddCommand(executionCmd)
}
