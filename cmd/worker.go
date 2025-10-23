package cmd

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/dop251/goja"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	workerCmd = &cobra.Command{
		Use:     "worker",
		Short:   "Task worker management",
		Long:    "Commands for managing task workers",
		GroupID: "development",
	}

	workerJsCmd = &cobra.Command{
		Use:          "js <js_file>",
		Short:        "Run a JavaScript worker that polls and processes tasks (EXPERIMENTAL)",
		Long:         "⚠️  EXPERIMENTAL FEATURE - Run a JavaScript worker that continuously polls for tasks of a specific type and executes the provided JavaScript file for each task.",
		RunE:         runJsWorker,
		SilenceUsage: true,
		Example:      "orkes worker js --type my_task worker.js",
	}

	workerExecCmd = &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Poll and execute tasks using an external command",
		Long: `Continuously poll for tasks and execute them using an external command.

The worker runs in continuous mode, polling for tasks and executing them in
parallel goroutines (similar to JavaScript workers).

The task JSON is passed to the command via stdin. The command should read the task
from stdin and write a result JSON to stdout.

Environment variables set for the worker:
  TASK_TYPE      - Type of the task
  TASK_ID        - Task ID
  WORKFLOW_ID    - Workflow ID
  EXECUTION_ID   - Workflow execution ID (same as WORKFLOW_ID)

Expected stdout format:
  {
    "status": "COMPLETED|FAILED|IN_PROGRESS",
    "output": {"key": "value"},
    "logs": ["log line 1", "log line 2"],
    "reason": "failure reason (optional)"
  }

Exit codes:
  0: Task handled successfully (status determines success/failure)
  non-zero: Failure (task marked as FAILED)`,
		RunE:         execWorker,
		SilenceUsage: true,
		Example:      "worker exec --type greet_task python worker.py\nworker exec --type greet_task python worker.py --count 5\nworker exec --type greet_task ./worker.sh --verbose",
	}
)

type TaskResult struct {
	Status string                 `json:"status"`
	Body   map[string]interface{} `json:"body"`
}

func runJsWorker(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("JavaScript file path is required")
	}

	jsFile := args[0]
	taskType, _ := cmd.Flags().GetString("type")
	if taskType == "" {
		return fmt.Errorf("--type flag is required")
	}

	count, _ := cmd.Flags().GetInt32("count")
	workerId, _ := cmd.Flags().GetString("worker-id")
	domain, _ := cmd.Flags().GetString("domain")
	timeout, _ := cmd.Flags().GetInt32("timeout")

	// Read JavaScript file
	scriptContent, err := os.ReadFile(jsFile)
	if err != nil {
		return fmt.Errorf("error reading JavaScript file: %v", err)
	}

	fmt.Printf("Starting worker for task type: %s\n", taskType)
	fmt.Printf("JavaScript file: %s\n", jsFile)
	fmt.Printf("Worker ID: %s\n", workerId)

	// Continuous polling loop
	for {
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
			log.Errorf("Error polling tasks: %v", err)
			continue
		}

		if len(tasks) == 0 {
			log.Debug("No tasks available")
			continue
		}

		log.Infof("Polled %d task(s)", len(tasks))

		// Process tasks in goroutines
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(t model.Task) {
				defer wg.Done()
				processTask(t, string(scriptContent), taskClient)
			}(task)
		}

		wg.Wait()
	}
}

func processTask(task model.Task, script string, taskClient *client.TaskResourceApiService) {
	log.Infof("Processing task: %s (workflow: %s)", task.TaskId, task.WorkflowInstanceId)

	// Create Goja VM
	vm := goja.New()

	// Inject task into $ global object
	taskJSON, err := json.Marshal(task)
	if err != nil {
		log.Errorf("Error marshaling task: %v", err)
		updateTaskFailed(taskClient, task, fmt.Sprintf("Error marshaling task: %v", err))
		return
	}

	var taskObj interface{}
	err = json.Unmarshal(taskJSON, &taskObj)
	if err != nil {
		log.Errorf("Error unmarshaling task: %v", err)
		updateTaskFailed(taskClient, task, fmt.Sprintf("Error unmarshaling task: %v", err))
		return
	}

	// Set up $ object with task
	dollarObj := vm.NewObject()
	err = dollarObj.Set("task", taskObj)
	if err != nil {
		log.Errorf("Error setting task in $: %v", err)
		updateTaskFailed(taskClient, task, fmt.Sprintf("Error setting task: %v", err))
		return
	}
	err = vm.Set("$", dollarObj)
	if err != nil {
		log.Errorf("Error setting $ object: %v", err)
		updateTaskFailed(taskClient, task, fmt.Sprintf("Error setting $ object: %v", err))
		return
	}

	// Inject utility functions
	injectUtilities(vm)

	// Execute script
	result, err := vm.RunString(script)
	if err != nil {
		log.Errorf("Error executing script for task %s: %v", task.TaskId, err)
		updateTaskFailed(taskClient, task, fmt.Sprintf("Script execution error: %v", err))
		return
	}

	// Check if script returned a value
	if result != nil && !goja.IsUndefined(result) && !goja.IsNull(result) {
		// Try to parse the result as TaskResult
		resultJSON := result.Export()
		resultBytes, err := json.Marshal(resultJSON)
		if err != nil {
			log.Errorf("Error marshaling script result: %v", err)
			updateTaskCompleted(taskClient, task, map[string]interface{}{})
			return
		}

		var taskResult TaskResult
		err = json.Unmarshal(resultBytes, &taskResult)
		if err != nil {
			// Result is not in expected format, treat as completed with raw result
			log.Warnf("Script result not in expected format, treating as completed")
			updateTaskCompleted(taskClient, task, map[string]interface{}{"result": resultJSON})
			return
		}

		// Update task with returned status and body
		if taskResult.Body == nil {
			taskResult.Body = make(map[string]interface{})
		}
		updateTaskWithStatus(taskClient, task, taskResult.Status, taskResult.Body)
	} else {
		// No return value, mark as completed
		updateTaskCompleted(taskClient, task, map[string]interface{}{})
	}
}

func updateTaskCompleted(taskClient *client.TaskResourceApiService, task model.Task, output map[string]interface{}) {
	updateTaskWithStatus(taskClient, task, "COMPLETED", output)
}

func updateTaskFailed(taskClient *client.TaskResourceApiService, task model.Task, reason string) {
	output := map[string]interface{}{
		"error": reason,
	}
	updateTaskWithStatus(taskClient, task, "FAILED", output)
}

func updateTaskWithStatus(taskClient *client.TaskResourceApiService, task model.Task, status string, output map[string]interface{}) {
	log.Infof("Updating task %s with status: %s", task.TaskId, status)

	taskResult := &model.TaskResult{
		TaskId:             task.TaskId,
		WorkflowInstanceId: task.WorkflowInstanceId,
		WorkerId:           task.WorkerId,
		Status:             model.TaskResultStatus(status),
		OutputData:         output,
	}

	_, _, err := taskClient.UpdateTask(context.Background(), taskResult)
	if err != nil {
		log.Errorf("Error updating task %s: %v", task.TaskId, err)
		return
	}

	log.Infof("Task %s updated successfully with status: %s", task.TaskId, status)
}

// injectUtilities adds utility functions to the JavaScript VM
func injectUtilities(vm *goja.Runtime) {
	// HTTP utilities
	httpObj := vm.NewObject()
	httpObj.Set("get", func(url string, headers map[string]interface{}) map[string]interface{} {
		return httpRequest("GET", url, headers, "")
	})
	httpObj.Set("post", func(url string, headers map[string]interface{}, body string) map[string]interface{} {
		return httpRequest("POST", url, headers, body)
	})
	httpObj.Set("put", func(url string, headers map[string]interface{}, body string) map[string]interface{} {
		return httpRequest("PUT", url, headers, body)
	})
	httpObj.Set("delete", func(url string, headers map[string]interface{}) map[string]interface{} {
		return httpRequest("DELETE", url, headers, "")
	})
	vm.Set("http", httpObj)

	// Crypto utilities
	cryptoObj := vm.NewObject()
	cryptoObj.Set("md5", func(text string) string {
		hash := md5.Sum([]byte(text))
		return hex.EncodeToString(hash[:])
	})
	cryptoObj.Set("sha1", func(text string) string {
		hash := sha1.Sum([]byte(text))
		return hex.EncodeToString(hash[:])
	})
	cryptoObj.Set("sha256", func(text string) string {
		hash := sha256.Sum256([]byte(text))
		return hex.EncodeToString(hash[:])
	})
	cryptoObj.Set("base64Encode", func(text string) string {
		return base64.StdEncoding.EncodeToString([]byte(text))
	})
	cryptoObj.Set("base64Decode", func(text string) string {
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return ""
		}
		return string(decoded)
	})
	vm.Set("crypto", cryptoObj)

	// Utility functions
	utilObj := vm.NewObject()
	utilObj.Set("sleep", func(ms int) {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	})
	utilObj.Set("uuid", func() string {
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	})
	utilObj.Set("env", func(key string) string {
		return os.Getenv(key)
	})
	vm.Set("util", utilObj)

	// String utilities
	stringObj := vm.NewObject()
	stringObj.Set("toUpper", strings.ToUpper)
	stringObj.Set("toLower", strings.ToLower)
	stringObj.Set("trim", strings.TrimSpace)
	stringObj.Set("split", func(s, sep string) []string {
		return strings.Split(s, sep)
	})
	stringObj.Set("join", func(arr []string, sep string) string {
		return strings.Join(arr, sep)
	})
	stringObj.Set("replace", func(s, old, new string) string {
		return strings.ReplaceAll(s, old, new)
	})
	stringObj.Set("contains", strings.Contains)
	stringObj.Set("hasPrefix", strings.HasPrefix)
	stringObj.Set("hasSuffix", strings.HasSuffix)
	vm.Set("str", stringObj)
}

// httpRequest performs HTTP requests from JavaScript
func httpRequest(method, url string, headers map[string]interface{}, body string) map[string]interface{} {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": 0,
		}
	}

	// Set headers
	for key, value := range headers {
		if strVal, ok := value.(string); ok {
			req.Header.Set(key, strVal)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": 0,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": resp.StatusCode,
		}
	}

	// Try to parse as JSON
	var jsonBody interface{}
	if err := json.Unmarshal(respBody, &jsonBody); err == nil {
		return map[string]interface{}{
			"status": resp.StatusCode,
			"body":   jsonBody,
			"text":   string(respBody),
		}
	}

	// Return as text if not JSON
	return map[string]interface{}{
		"status": resp.StatusCode,
		"text":   string(respBody),
	}
}

// WorkerResult represents the expected output from a worker command
type WorkerResult struct {
	Status string                 `json:"status"` // COMPLETED | FAILED | IN_PROGRESS
	Output map[string]interface{} `json:"output,omitempty"`
	Logs   []string               `json:"logs,omitempty"`
	Reason string                 `json:"reason,omitempty"`
}

func execWorker(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Usage()
	}

	taskType, _ := cmd.Flags().GetString("type")
	if taskType == "" {
		return fmt.Errorf("--type flag is required")
	}

	workerCmd := args[0]
	workerArgs := args[1:]

	workerId, _ := cmd.Flags().GetString("worker-id")
	domain, _ := cmd.Flags().GetString("domain")
	pollTimeout, _ := cmd.Flags().GetInt32("poll-timeout")
	execTimeout, _ := cmd.Flags().GetInt32("exec-timeout")
	count, _ := cmd.Flags().GetInt32("count")

	taskClient := internal.GetTaskClient()

	// Continuous mode: poll and process in goroutines
	fmt.Printf("Starting worker for task type: %s\n", taskType)
	fmt.Printf("Command: %s %v\n", workerCmd, workerArgs)
	if workerId != "" {
		fmt.Printf("Worker ID: %s\n", workerId)
	}

	for {
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
		if pollTimeout > 0 {
			opts.Timeout = optional.NewInt32(pollTimeout)
		}

		tasks, _, err := taskClient.BatchPoll(context.Background(), taskType, opts)
		if err != nil {
			log.Errorf("Error polling tasks: %v", err)
			continue
		}

		if len(tasks) == 0 {
			log.Debug("No tasks available")
			continue
		}

		log.Infof("Polled %d task(s)", len(tasks))

		// Process tasks in parallel goroutines
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(t model.Task) {
				defer wg.Done()
				executeExternalWorker(t, workerCmd, workerArgs, workerId, domain, execTimeout, taskClient)
			}(task)
		}

		wg.Wait()
	}
}

func executeExternalWorker(task model.Task, workerCmd string, workerArgs []string, workerId, domain string, execTimeout int32, taskClient *client.TaskResourceApiService) {
	log.Infof("Processing task: %s (workflow: %s)", task.TaskId, task.WorkflowInstanceId)

	// Marshal task to JSON for stdin
	taskJSON, err := json.Marshal(task)
	if err != nil {
		log.Errorf("Error marshaling task: %v", err)
		updateExecTaskFailed(taskClient, task, workerId, fmt.Sprintf("error marshaling task: %v", err))
		return
	}

	// Execute worker command
	ctx := context.Background()
	if execTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(execTimeout)*time.Second)
		defer cancel()
	}

	execCmd := exec.CommandContext(ctx, workerCmd, workerArgs...)
	execCmd.Env = append(execCmd.Environ(),
		"TASK_TYPE="+task.TaskType,
		"TASK_ID="+task.TaskId,
		"WORKFLOW_ID="+task.WorkflowInstanceId,
		"EXECUTION_ID="+task.WorkflowInstanceId,
	)
	if domain != "" {
		execCmd.Env = append(execCmd.Env, "POLL_DOMAIN="+domain)
	}

	execCmd.Stdin = bytes.NewReader(taskJSON)

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	execErr := execCmd.Run()

	// Parse worker result
	var result WorkerResult
	if execErr != nil {
		// Worker failed with non-zero exit
		result = WorkerResult{
			Status: "FAILED",
			Reason: fmt.Sprintf("worker exec failed: %v; stderr=%s", execErr, stderr.String()),
			Logs:   []string{stderr.String()},
		}
	} else {
		// Parse stdout
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			result = WorkerResult{
				Status: "FAILED",
				Reason: fmt.Sprintf("invalid worker stdout JSON: %v; raw=%s", err, stdout.String()),
				Logs:   []string{stdout.String()},
			}
		}
	}

	// Validate status
	switch result.Status {
	case "COMPLETED", "FAILED", "IN_PROGRESS":
		// Valid status
	default:
		result.Status = "FAILED"
		result.Reason = fmt.Sprintf("invalid status from worker: %s", result.Status)
	}

	// Convert status string to TaskResultStatus
	var status model.TaskResultStatus
	switch result.Status {
	case "COMPLETED":
		status = model.CompletedTask
	case "FAILED":
		status = model.FailedTask
	case "IN_PROGRESS":
		status = model.InProgressTask
	default:
		status = model.FailedTask
	}

	// Update task with result
	taskResult := model.TaskResult{
		TaskId:             task.TaskId,
		WorkflowInstanceId: task.WorkflowInstanceId,
		Status:             status,
	}

	if result.Output != nil {
		taskResult.OutputData = result.Output
	}

	if len(result.Logs) > 0 {
		logs := make([]model.TaskExecLog, len(result.Logs))
		for i, logLine := range result.Logs {
			logs[i] = model.TaskExecLog{
				Log: logLine,
			}
		}
		taskResult.Logs = logs
	}

	if result.Reason != "" {
		taskResult.ReasonForIncompletion = result.Reason
	}

	if workerId != "" {
		taskResult.WorkerId = workerId
	}

	// Update task
	_, _, err = taskClient.UpdateTask(context.Background(), &taskResult)
	if err != nil {
		log.Errorf("Error updating task %s: %v", task.TaskId, err)
		return
	}

	log.Infof("Task %s completed with status: %s", task.TaskId, result.Status)
}

func updateExecTaskFailed(taskClient *client.TaskResourceApiService, task model.Task, workerId, reason string) {
	taskResult := model.TaskResult{
		TaskId:                  task.TaskId,
		WorkflowInstanceId:      task.WorkflowInstanceId,
		Status:                  model.FailedTask,
		ReasonForIncompletion:   reason,
		OutputData:              map[string]interface{}{"error": reason},
	}

	if workerId != "" {
		taskResult.WorkerId = workerId
	}

	_, _, err := taskClient.UpdateTask(context.Background(), &taskResult)
	if err != nil {
		log.Errorf("Error updating task %s as failed: %v", task.TaskId, err)
	}
}

func init() {
	// Worker JS flags
	workerJsCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerJsCmd.MarkFlagRequired("type")
	workerJsCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerJsCmd.Flags().String("worker-id", "", "Worker ID")
	workerJsCmd.Flags().String("domain", "", "Domain")
	workerJsCmd.Flags().Int32("timeout", 100, "Timeout in milliseconds")

	// Worker exec flags
	workerExecCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerExecCmd.MarkFlagRequired("type")
	workerExecCmd.Flags().String("worker-id", "", "Worker ID")
	workerExecCmd.Flags().String("domain", "", "Domain")
	workerExecCmd.Flags().Int32("poll-timeout", 100, "Poll timeout in milliseconds")
	workerExecCmd.Flags().Int32("exec-timeout", 0, "Execution timeout in seconds (0 = no timeout)")
	workerExecCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")

	workerCmd.AddCommand(workerJsCmd)
	workerCmd.AddCommand(workerExecCmd)
	rootCmd.AddCommand(workerCmd)
}
