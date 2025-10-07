package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
)

var (
	// Task command group under execution
	taskUpdateCmd = &cobra.Command{
		Use:          "task-update",
		Short:        "Update a task by reference name",
		RunE:         updateTaskByRefName,
		SilenceUsage: true,
		Example:      "execution task-update --workflow-id <id> --task-ref-name <name> --status COMPLETED --output '{\"key\":\"value\"}'",
	}

	taskSignalCmd = &cobra.Command{
		Use:          "task-signal",
		Short:        "Signal a task asynchronously",
		RunE:         signalTaskAsync,
		SilenceUsage: true,
		Example:      "execution task-signal --workflow-id <id> --status COMPLETED --output '{\"key\":\"value\"}'",
	}

	taskSignalSyncCmd = &cobra.Command{
		Use:          "task-signal-sync",
		Short:        "Signal a task synchronously",
		RunE:         signalTaskSync,
		SilenceUsage: true,
		Example:      "execution task-signal-sync --workflow-id <id> --status COMPLETED --output '{\"key\":\"value\"}'",
	}

	taskPollCmd = &cobra.Command{
		Use:          "task-poll <task_type>",
		Short:        "Batch poll for tasks of a certain type",
		RunE:         pollTasks,
		SilenceUsage: true,
		Example:      "execution task-poll my_task_type --count 5 --worker-id worker1",
	}
)

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
	// Task update flags
	taskUpdateCmd.Flags().String("workflow-id", "", "Workflow ID (required)")
	taskUpdateCmd.Flags().String("task-ref-name", "", "Task reference name (required)")
	taskUpdateCmd.Flags().String("status", "", "Task status: IN_PROGRESS, COMPLETED, FAILED, FAILED_WITH_TERMINAL_ERROR (required)")
	taskUpdateCmd.Flags().String("output", "", "Task output as JSON string")
	taskUpdateCmd.Flags().String("worker-id", "", "Worker ID (optional)")

	// Task signal (async) flags
	taskSignalCmd.Flags().String("workflow-id", "", "Workflow ID (required)")
	taskSignalCmd.Flags().String("status", "", "Task status: IN_PROGRESS, COMPLETED, FAILED, FAILED_WITH_TERMINAL_ERROR (required)")
	taskSignalCmd.Flags().String("output", "", "Task output as JSON string")

	// Task signal (sync) flags
	taskSignalSyncCmd.Flags().String("workflow-id", "", "Workflow ID (required)")
	taskSignalSyncCmd.Flags().String("status", "", "Task status: IN_PROGRESS, COMPLETED, FAILED, FAILED_WITH_TERMINAL_ERROR (required)")
	taskSignalSyncCmd.Flags().String("output", "", "Task output as JSON string")

	// Task poll flags
	taskPollCmd.Flags().Int32("count", 1, "Number of tasks to poll")
	taskPollCmd.Flags().String("worker-id", "", "Worker ID")
	taskPollCmd.Flags().String("domain", "", "Domain")
	taskPollCmd.Flags().Int32("timeout", 100, "Timeout in milliseconds")

	// Add task commands to execution command
	executionCmd.AddCommand(
		taskUpdateCmd,
		taskSignalCmd,
		taskSignalSyncCmd,
		taskPollCmd,
	)
}
