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
	"regexp"
	"text/tabwriter"

	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:     "task",
	Short:   "Task definition and execution management",
	GroupID: "conductor",
}

var (
	// Task Definition Management Commands
	listTaskMetadataCmd = &cobra.Command{
		Use:          "list",
		Short:        "List Tasks Definitions",
		RunE:         listTasks,
		SilenceUsage: true,
	}

	getTaskMetadataCmd = &cobra.Command{
		Use:          "get <task_type>",
		Short:        "Get Task Definition",
		RunE:         getTask,
		SilenceUsage: true,
	}

	deleteTaskMetadataCmd = &cobra.Command{
		Use:          "delete <task_type>",
		Short:        "Delete task",
		RunE:         deleteTaskMetadata,
		SilenceUsage: true,
	}

	createTaskMetadataCmd = &cobra.Command{
		Use:          "create <task_definition.json>",
		Short:        "Create task",
		RunE:         createTask,
		SilenceUsage: true,
	}

	updateTaskMetadataCmd = &cobra.Command{
		Use:          "update <task_definition.json>",
		Short:        "Update task",
		RunE:         updateTask,
		SilenceUsage: true,
	}

	getAllTaskMetadataCmd = &cobra.Command{
		Use:          "get_all",
		Short:        "Get all task definitions",
		RunE:         getAllTasks,
		SilenceUsage: true,
	}

	// Task Execution Management Commands
	taskPollCmd = &cobra.Command{
		Use:          "poll <task_type>",
		Short:        "Batch poll for tasks of a certain type",
		RunE:         pollTasks,
		SilenceUsage: true,
		Example:      "task poll my_task_type --count 5 --worker-id worker1",
	}

	taskUpdateExecutionCmd = &cobra.Command{
		Use:          "update-execution",
		Short:        "Update a task by reference name",
		RunE:         updateTaskByRefName,
		SilenceUsage: true,
		Example:      "task update-execution --workflow-id <id> --task-ref-name <name> --status COMPLETED --output '{\"key\":\"value\"}'",
	}

	taskSignalCmd = &cobra.Command{
		Use:          "signal",
		Short:        "Signal a task asynchronously",
		RunE:         signalTaskAsync,
		SilenceUsage: true,
		Example:      "task signal --workflow-id <id> --status COMPLETED --output '{\"key\":\"value\"}'",
	}

	taskSignalSyncCmd = &cobra.Command{
		Use:          "signal-sync",
		Short:        "Signal a task synchronously",
		RunE:         signalTaskSync,
		SilenceUsage: true,
		Example:      "task signal-sync --workflow-id <id> --status COMPLETED --output '{\"key\":\"value\"}'",
	}
)

func listTasks(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	metadataClient := internal.GetMetadataClient()
	tasks, _, err := metadataClient.GetTaskDefs(context.Background())
	if err != nil {
		return parseAPIError(err, "Failed to list tasks")
	}

	if jsonOutput {
		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling tasks: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tEXECUTABLE\tDESCRIPTION\tOWNER\tTIMEOUT POLICY\tTIMEOUT (s)\tRETRY COUNT\tRESPONSE TIMEOUT (s)")
	for _, task := range tasks {
		// Determine if task is executable (has executionNameSpace or ownerApp)
		executable := "no"
		if task.ExecutionNameSpace != "" || task.OwnerApp != "" {
			executable = "yes"
		}

		description := task.Description
		if description == "" {
			description = "-"
		}
		// Truncate long descriptions
		if len(description) > 30 {
			description = description[:27] + "..."
		}

		ownerEmail := task.OwnerEmail
		if ownerEmail == "" {
			ownerEmail = "-"
		}

		timeoutPolicy := task.TimeoutPolicy
		if timeoutPolicy == "" {
			timeoutPolicy = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\n",
			task.Name,
			executable,
			description,
			ownerEmail,
			timeoutPolicy,
			task.TimeoutSeconds,
			task.RetryCount,
			task.ResponseTimeoutSeconds,
		)
	}
	w.Flush()

	return nil
}

func getTask(cmd *cobra.Command, args []string) error {

	metadataClient := internal.GetMetadataClient()
	if len(args) != 1 {
		return cmd.Usage()
	}
	taskType := args[0]
	task, _, err := metadataClient.GetTaskDef(context.Background(), taskType)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to get task '%s'", taskType))
	}
	bytes, _ := json.MarshalIndent(task, "", "   ")
	fmt.Println(string(bytes))
	return nil
}

func createTask(cmd *cobra.Command, args []string) error {

	metadataClient := internal.GetMetadataClient()
	var taskDefs []model.TaskDef
	var bytes []byte
	var err error
	if len(args) == 1 {
		file := args[0]
		bytes, err = os.ReadFile(file)
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

		bytes = read()
	}
	err = json.Unmarshal(bytes, &taskDefs)
	if err != nil {
		var taskDef model.TaskDef
		err = json.Unmarshal(bytes, &taskDef)
		if err != nil {
			return err
		}
		taskDefs = append(taskDefs, taskDef)
	}
	_, err = metadataClient.RegisterTaskDef(context.Background(), taskDefs)
	if err != nil {
		return parseAPIError(err, "Failed to create task")
	}
	return nil
}

func updateTask(cmd *cobra.Command, args []string) error {

	metadataClient := internal.GetMetadataClient()
	var taskDef model.TaskDef
	var bytes []byte
	var err error
	if len(args) == 1 {
		file := args[0]
		bytes, err = os.ReadFile(file)
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

		bytes = read()
	}
	err = json.Unmarshal(bytes, &taskDef)
	if err != nil {
		return err
	}
	_, err = metadataClient.UpdateTaskDef(context.Background(), taskDef)
	if err != nil {
		return parseAPIError(err, "Failed to update task")
	}
	return nil
}

func getAllTasks(cmd *cobra.Command, args []string) error {

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

	taskDefs, _, err := metadataClient.GetTaskDefs(context.Background())
	if err != nil {
		return parseAPIError(err, "Failed to get tasks")
	}

	fmt.Println("[")
	for _, data := range taskDefs {
		if regex != nil {
			if regex.Match([]byte(data.Name)) {
				bytes, _ := json.MarshalIndent(data, "", "   ")
				fmt.Println(string(bytes))
			}
		} else {
			bytes, _ := json.MarshalIndent(data, "", "   ")
			fmt.Println(string(bytes))
		}

	}
	fmt.Println("]")

	return nil
}

func deleteTaskMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	if len(args) == 0 {
		return cmd.Usage()
	}
	for i := 0; i < len(args); i++ {
		name := args[i]

		// Confirm deletion
		if !confirmDeletion("task", name) {
			fmt.Printf("Skipping deletion of task '%s'\n", name)
			continue
		}

		_, err := metadataClient.UnregisterTaskDef(context.Background(), name)
		if err != nil {
			return parseAPIError(err, fmt.Sprintf("Failed to delete task '%s'", name))
		}
		fmt.Printf("Task '%s' deleted successfully\n", name)
	}

	return nil
}


func updateTaskByRefName(cmd *cobra.Command, args []string) error {
	workflowId, _ := cmd.Flags().GetString("workflow-id")
	taskRefName, _ := cmd.Flags().GetString("task-ref-name")
	status, _ := cmd.Flags().GetString("status")
	output, _ := cmd.Flags().GetString("output")
	workerId, _ := cmd.Flags().GetString("worker-id")

	if workflowId == "" || taskRefName == "" || status == "" {
		return fmt.Errorf("--workflow-id, --task-ref-name, and --status are required")
	}

	var outputMap map[string]interface{}
	if output != "" {
		err := json.Unmarshal([]byte(output), &outputMap)
		if err != nil {
			return fmt.Errorf("invalid output JSON: %v", err)
		}
	} else {
		outputMap = make(map[string]interface{})
	}

	taskClient := internal.GetTaskClient()

	var result string
	var err error

	if workerId != "" {
		result, _, err = taskClient.UpdateTaskByRefNameWithWorkerId(
			context.Background(),
			outputMap,
			workflowId,
			taskRefName,
			status,
			optional.NewString(workerId),
		)
	} else {
		result, _, err = taskClient.UpdateTaskByRefName(
			context.Background(),
			outputMap,
			workflowId,
			taskRefName,
			status,
		)
	}

	if err != nil {
		return fmt.Errorf("error updating task: %v", err)
	}

	fmt.Println(result)
	return nil
}

func signalTaskAsync(cmd *cobra.Command, args []string) error {
	workflowId, _ := cmd.Flags().GetString("workflow-id")
	status, _ := cmd.Flags().GetString("status")
	output, _ := cmd.Flags().GetString("output")

	if workflowId == "" || status == "" {
		return fmt.Errorf("--workflow-id and --status are required")
	}

	var outputMap map[string]interface{}
	if output != "" {
		err := json.Unmarshal([]byte(output), &outputMap)
		if err != nil {
			return fmt.Errorf("invalid output JSON: %v", err)
		}
	} else {
		outputMap = make(map[string]interface{})
	}

	taskClient := internal.GetTaskClient()
	_, err := taskClient.SignalAsync(context.Background(), outputMap, workflowId, status)
	if err != nil {
		return fmt.Errorf("error signaling task: %v", err)
	}

	fmt.Printf("Task signal sent asynchronously to workflow %s with status %s\n", workflowId, status)
	return nil
}

func signalTaskSync(cmd *cobra.Command, args []string) error {
	workflowId, _ := cmd.Flags().GetString("workflow-id")
	status, _ := cmd.Flags().GetString("status")
	output, _ := cmd.Flags().GetString("output")

	if workflowId == "" || status == "" {
		return fmt.Errorf("--workflow-id and --status are required")
	}

	var outputMap map[string]interface{}
	if output != "" {
		err := json.Unmarshal([]byte(output), &outputMap)
		if err != nil {
			return fmt.Errorf("invalid output JSON: %v", err)
		}
	} else {
		outputMap = make(map[string]interface{})
	}

	// Convert status string to WorkflowStatus enum
	var statusEnum model.WorkflowStatus
	switch status {
	case "IN_PROGRESS":
		statusEnum = model.RunningWorkflow
	case "COMPLETED":
		statusEnum = model.CompletedWorkflow
	case "FAILED", "FAILED_WITH_TERMINAL_ERROR":
		statusEnum = model.FailedWorkflow
	default:
		return fmt.Errorf("invalid status: %s. Must be one of: IN_PROGRESS, COMPLETED, FAILED, FAILED_WITH_TERMINAL_ERROR", status)
	}

	taskClient := internal.GetTaskClient()
	response, err := taskClient.Signal(context.Background(), outputMap, workflowId, statusEnum)
	if err != nil {
		return fmt.Errorf("error signaling task: %v", err)
	}

	data, _ := json.MarshalIndent(response, "", "   ")
	fmt.Println(string(data))
	return nil
}

func pollTasks(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	taskType := args[0]
	count, _ := cmd.Flags().GetInt32("count")
	workerId, _ := cmd.Flags().GetString("worker-id")
	domain, _ := cmd.Flags().GetString("domain")
	timeout, _ := cmd.Flags().GetInt32("timeout")

	opts := &client.TaskResourceApiBatchPollOpts{}
	if workerId != "" {
		opts.Workerid = optional.NewString(workerId)
	}
	if domain != "" {
		opts.Domain = optional.NewString(domain)
	}
	if count > 0 {
		opts.Count = optional.NewInt32(count)
	}
	if timeout > 0 {
		opts.Timeout = optional.NewInt32(timeout)
	}

	taskClient := internal.GetTaskClient()
	tasks, _, err := taskClient.BatchPoll(context.Background(), taskType, opts)
	if err != nil {
		return fmt.Errorf("error polling tasks: %v", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks available")
		return nil
	}

	data, _ := json.MarshalIndent(tasks, "", "   ")
	fmt.Println(string(data))
	return nil
}

func init() {
	rootCmd.AddCommand(taskCmd)

	// Definition management flags
	listTaskMetadataCmd.Flags().Bool("json", false, "Print complete JSON output")

	// Execution management flags
	taskUpdateExecutionCmd.Flags().String("workflow-id", "", "Workflow ID (required)")
	taskUpdateExecutionCmd.Flags().String("task-ref-name", "", "Task reference name (required)")
	taskUpdateExecutionCmd.Flags().String("status", "", "Task status: IN_PROGRESS, COMPLETED, FAILED, FAILED_WITH_TERMINAL_ERROR (required)")
	taskUpdateExecutionCmd.Flags().String("output", "", "Task output as JSON string")
	taskUpdateExecutionCmd.Flags().String("worker-id", "", "Worker ID (optional)")

	taskSignalCmd.Flags().String("workflow-id", "", "Workflow ID (required)")
	taskSignalCmd.Flags().String("status", "", "Task status: IN_PROGRESS, COMPLETED, FAILED, FAILED_WITH_TERMINAL_ERROR (required)")
	taskSignalCmd.Flags().String("output", "", "Task output as JSON string")

	taskSignalSyncCmd.Flags().String("workflow-id", "", "Workflow ID (required)")
	taskSignalSyncCmd.Flags().String("status", "", "Task status: IN_PROGRESS, COMPLETED, FAILED, FAILED_WITH_TERMINAL_ERROR (required)")
	taskSignalSyncCmd.Flags().String("output", "", "Task output as JSON string")

	taskPollCmd.Flags().Int32("count", 1, "Number of tasks to poll")
	taskPollCmd.Flags().String("worker-id", "", "Worker ID")
	taskPollCmd.Flags().String("domain", "", "Domain")
	taskPollCmd.Flags().Int32("timeout", 100, "Timeout in milliseconds")

	taskCmd.AddCommand(
		// Definition management
		listTaskMetadataCmd,
		getTaskMetadataCmd,
		getAllTaskMetadataCmd,
		createTaskMetadataCmd,
		updateTaskMetadataCmd,
		deleteTaskMetadataCmd,
		// Execution management
		taskPollCmd,
		taskUpdateExecutionCmd,
		taskSignalCmd,
		taskSignalSyncCmd,
	)
}
