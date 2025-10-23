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
		Use:     "execution",
		Short:   "Workflow and Task execution",
		Long:    "Commands for managing workflow and task execution",
		GroupID: "execution",
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
		Short:        "Start workflow execution asynchronously",
		Long:         "Start workflow execution asynchronously and return the workflow ID immediately without waiting for completion.",
		RunE:         startWorkflow,
		SilenceUsage: true,
		Example:      "execution start [flags]",
	}

	executeExecutionCmd = &cobra.Command{
		Use:          "execute",
		Short:        "Execute workflow synchronously",
		Long:         "Execute workflow synchronously and wait for completion, returning the full workflow execution output.",
		RunE:         executeWorkflow,
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

	restartExecutionCmd = &cobra.Command{
		Use:          "restart <workflow_id>",
		Short:        "Restart a completed workflow",
		RunE:         restartWorkflow,
		SilenceUsage: true,
		Example:      "execution restart [workflow_id]\nexecution restart --use-latest [workflow_id]",
	}

	retryExecutionCmd = &cobra.Command{
		Use:          "retry <workflow_id>",
		Short:        "Retry the last failed task",
		RunE:         retryWorkflow,
		SilenceUsage: true,
		Example:      "execution retry [workflow_id]",
	}

	skipTaskExecutionCmd = &cobra.Command{
		Use:          "skip-task <workflow_id> <task_reference_name>",
		Short:        "Skip a task in a running workflow",
		RunE:         skipTask,
		SilenceUsage: true,
		Example:      "execution skip-task [workflow_id] [task_ref_name]",
	}

	rerunExecutionCmd = &cobra.Command{
		Use:          "rerun <workflow_id>",
		Short:        "Rerun workflow from a specific task",
		RunE:         rerunWorkflow,
		SilenceUsage: true,
		Example:      "execution rerun [workflow_id] --task-id [task_id]",
	}

	jumpExecutionCmd = &cobra.Command{
		Use:          "jump <workflow_id> <task_reference_name>",
		Short:        "Jump workflow execution to given task",
		RunE:         jumpToTask,
		SilenceUsage: true,
		Example:      "execution jump [workflow_id] [task_ref_name]",
	}

	updateStateExecutionCmd = &cobra.Command{
		Use:          "update-state <workflow_id>",
		Short:        "Update workflow state (variables and tasks)",
		RunE:         updateWorkflowState,
		SilenceUsage: true,
		Example:      "execution update-state [workflow_id] --variables '{\"key\":\"value\"}'",
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
		opts.Version = optional.NewInt32(version)
	}
	if correlationId != "" {
		opts.CorrelationId = optional.NewString(correlationId)
	}

	workflowClient := internal.GetWorkflowClient()
	workflowId, _, startErr := workflowClient.StartWorkflow(cmd.Context(), inputMap, workflowName, opts)
	if startErr != nil {
		return startErr
	}
	fmt.Println(workflowId)

	return nil
}

func executeWorkflow(cmd *cobra.Command, args []string) error {
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

	workflowClient := internal.GetWorkflowClient()
	run, _, execErr := workflowClient.ExecuteWorkflow(context.Background(), request, requestId.String(), workflowName, version, waitUntil)
	if execErr != nil {
		return execErr
	}

	data, jsonError := json.MarshalIndent(run, "", "   ")
	if jsonError != nil {
		return jsonError
	}
	fmt.Println(string(data))

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
	searchExecutionCmd.Flags().Int32P("count", "c", 10, "No of workflow executions to return (max 1000)")
	searchExecutionCmd.Flags().StringP("status", "s", "", "Filter by status one of (COMPLETED, FAILED, PAUSED, RUNNING, TERMINATED, TIMED_OUT)")
	searchExecutionCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	searchExecutionCmd.Flags().String("start-time-after", "", "Filter executions started after this time (YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, or epoch ms)")
	searchExecutionCmd.Flags().String("start-time-before", "", "Filter executions started before this time (YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, or epoch ms)")
	searchExecutionCmd.Flags().Bool("json", false, "Output complete JSON instead of table")

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
	executeExecutionCmd.MarkFlagsMutuallyExclusive("input", "file")

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
		restartExecutionCmd,
		retryExecutionCmd,
		skipTaskExecutionCmd,
		rerunExecutionCmd,
		jumpExecutionCmd,
		updateStateExecutionCmd,
	)

	// Add execution command to root
	rootCmd.AddCommand(executionCmd)
}
