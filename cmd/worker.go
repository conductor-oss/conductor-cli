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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/authentication"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	"github.com/dop251/goja"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		Example:      "conductor worker js --type my_task worker.js",
	}

	workerStdioCmd = &cobra.Command{
		Use:   "stdio <command> [args...]",
		Short: "Poll tasks and execute command via stdin/stdout",
		Long: `CLI polls tasks and executes the command. The task is passed in the standard input and the result is expected in the standard output.

The worker runs in continuous mode, polling for tasks and executing them in
parallel goroutines.

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
		Example:      "worker stdio --type greet_task python worker.py\nworker stdio --type greet_task python worker.py --count 5\nworker stdio --type greet_task ./worker.sh --verbose",
	}

	workerRemoteCmd = &cobra.Command{
		Use:   "remote",
		Short: "Run a worker from the job-runner registry (EXPERIMENTAL)",
		Long: `⚠️  EXPERIMENTAL FEATURE - Download and execute a worker from the Orkes Conductor job-runner.

The worker is downloaded from the configured Conductor server and cached locally for
subsequent runs. Use --refresh to force re-download from the registry.

Supported worker languages:
  - NODEJS: JavaScript/Node.js workers (executed using built-in JavaScript engine)
  - PYTHON: Python workers (executed using python3 interpreter)

The worker runs in continuous mode, polling for tasks and executing them in parallel.`,
		RunE:         runRemoteWorker,
		SilenceUsage: true,
		Example:      "conductor worker remote --type greet_task\nconductor worker remote --type greet_task --count 5 --refresh\nconductor worker remote --type greet_task --worker-id worker-1 --domain prod",
	}

	workerListRemoteCmd = &cobra.Command{
		Use:          "list-remote",
		Short:        "List available workers in the job-runner registry (EXPERIMENTAL)",
		Long:         `⚠️  EXPERIMENTAL FEATURE - List all available workers in the Orkes Conductor job-runner registry.`,
		RunE:         listRemoteWorkers,
		SilenceUsage: true,
		Example:      "conductor worker list-remote\nconductor worker list-remote --namespace production",
	}
)

type TaskResult struct {
	Status string                 `json:"status"`
	Body   map[string]interface{} `json:"body"`
}

// WorkerCodeResponse represents the response from the job-runner worker-code API
type WorkerCodeResponse struct {
	Id           string    `json:"id"`
	UserId       string    `json:"userId"`
	Namespace    string    `json:"namespace"`
	TaskName     string    `json:"taskName"`
	Language     string    `json:"language"`
	Code         string    `json:"code"`
	Version      int       `json:"version"`
	Description  string    `json:"description"`
	Dependencies []string  `json:"dependencies"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatedBy    string    `json:"createdBy"`
}

// WorkerMetadata represents cached worker metadata
type WorkerMetadata struct {
	TaskName     string    `json:"taskName"`
	Language     string    `json:"language"`
	Version      int       `json:"version"`
	WorkerCodeId string    `json:"workerCodeId"`
	CachedAt     time.Time `json:"cachedAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
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

	scriptContent, err := os.ReadFile(jsFile)
	if err != nil {
		return fmt.Errorf("error reading JavaScript file: %v", err)
	}

	fmt.Printf("Starting worker for task type: %s\n", taskType)
	fmt.Printf("JavaScript file: %s\n", jsFile)
	fmt.Printf("Worker ID: %s\n", workerId)

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

	vm := goja.New()

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

	injectUtilities(vm)

	result, err := vm.RunString(script)
	if err != nil {
		log.Errorf("Error executing script for task %s: %v", task.TaskId, err)
		updateTaskFailed(taskClient, task, fmt.Sprintf("Script execution error: %v", err))
		return
	}

	if result != nil && !goja.IsUndefined(result) && !goja.IsNull(result) {
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
			log.Warnf("Script result not in expected format, treating as completed")
			updateTaskCompleted(taskClient, task, map[string]interface{}{"result": resultJSON})
			return
		}

		if taskResult.Body == nil {
			taskResult.Body = make(map[string]interface{})
		}
		updateTaskWithStatus(taskClient, task, taskResult.Status, taskResult.Body)
	} else {
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

	var jsonBody interface{}
	if err := json.Unmarshal(respBody, &jsonBody); err == nil {
		return map[string]interface{}{
			"status": resp.StatusCode,
			"body":   jsonBody,
			"text":   string(respBody),
		}
	}

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
	verbose, _ := cmd.Flags().GetBool("verbose")

	taskClient := internal.GetTaskClient()

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
				executeExternalWorker(t, workerCmd, workerArgs, workerId, domain, execTimeout, verbose, taskClient)
			}(task)
		}

		wg.Wait()
	}
}

func executeExternalWorker(task model.Task, workerCmd string, workerArgs []string, workerId, domain string, execTimeout int32, verbose bool, taskClient *client.TaskResourceApiService) {
	log.Infof("Processing task: %s (workflow: %s)", task.TaskId, task.WorkflowInstanceId)

	taskJSON, err := json.Marshal(task)
	if err != nil {
		log.Errorf("Error marshaling task: %v", err)
		updateExecTaskFailed(taskClient, task, workerId, fmt.Sprintf("error marshaling task: %v", err))
		return
	}

	if verbose {
		fmt.Println("=== Task Input ===")
		fmt.Println(string(taskJSON))
		fmt.Println("==================")
	}

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

	serverUrl := viper.GetString("server")
	if serverUrl != "" {
		serverUrl = strings.TrimSuffix(serverUrl, "/")
		if !strings.HasSuffix(serverUrl, "/api") {
			serverUrl = serverUrl + "/api"
		}
		execCmd.Env = append(execCmd.Env, "CONDUCTOR_SERVER_URL="+serverUrl)
	}

	authKey := viper.GetString("auth-key")
	authSecret := viper.GetString("auth-secret")
	if authKey != "" {
		execCmd.Env = append(execCmd.Env, "CONDUCTOR_ACCESS_KEY_ID="+authKey)
	}
	if authSecret != "" {
		execCmd.Env = append(execCmd.Env, "CONDUCTOR_ACCESS_KEY_SECRET="+authSecret)
	}

	authToken := viper.GetString("auth-token")
	if authToken != "" {
		execCmd.Env = append(execCmd.Env, "CONDUCTOR_AUTH_TOKEN="+authToken)
	}

	execCmd.Stdin = bytes.NewReader(taskJSON)

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
	execCmd.Stderr = io.MultiWriter(&stderr, os.Stderr)

	execErr := execCmd.Run()

	var result WorkerResult
	if execErr != nil {
		stderrOutput := stderr.String()
		log.Errorf("Worker execution failed: %v", execErr)
		if stderrOutput != "" {
			log.Errorf("Worker stderr:\n%s", stderrOutput)
		}

		result = WorkerResult{
			Status: "FAILED",
			Reason: fmt.Sprintf("worker execution failed: %v", execErr),
			Logs:   []string{stderrOutput},
		}
	} else {
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			stdoutOutput := stdout.String()
			log.Errorf("Failed to parse worker output as JSON: %v", err)
			log.Errorf("Worker stdout:\n%s", stdoutOutput)

			result = WorkerResult{
				Status: "FAILED",
				Reason: fmt.Sprintf("invalid worker stdout JSON: %v", err),
				Logs:   []string{stdoutOutput},
			}
		}
	}

	if verbose {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		if result.Status == "FAILED" {
			fmt.Println("=== Task Result (Error) ===")
			fmt.Println(string(resultJSON))
			fmt.Println("===========================")
		} else {
			fmt.Println("=== Task Result ===")
			fmt.Println(string(resultJSON))
			fmt.Println("===================")
		}
	}

	switch result.Status {
	case "COMPLETED", "FAILED", "IN_PROGRESS":
	default:
		result.Status = "FAILED"
		result.Reason = fmt.Sprintf("invalid status from worker: %s", result.Status)
	}

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

func listRemoteWorkers(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	namespace, _ := cmd.Flags().GetString("namespace")
	if namespace == "" {
		namespace = "default"
	}

	serverUrl := viper.GetString("server")
	if serverUrl == "" {
		serverUrl = "http://localhost:8080/api"
	}
	serverUrl = strings.TrimSuffix(serverUrl, "/")
	if !strings.HasSuffix(serverUrl, "/api") {
		serverUrl = serverUrl + "/api"
	}

	apiUrl := fmt.Sprintf("%s/worker-code?namespace=%s", serverUrl, namespace)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := addWorkerAuthHeaders(req); err != nil {
		return fmt.Errorf("failed to add auth headers: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed (401 Unauthorized) - verify your credentials")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list workers: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var workers []WorkerCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&workers); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(workers) == 0 {
		fmt.Printf("No workers found in namespace: %s\n", namespace)
		return nil
	}

	fmt.Printf("\nAvailable Workers in namespace '%s':\n\n", namespace)
	fmt.Printf("%-30s %-12s %-10s %-50s\n", "TASK NAME", "LANGUAGE", "VERSION", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 105))

	for _, worker := range workers {
		description := worker.Description
		if len(description) > 47 {
			description = description[:47] + "..."
		}
		fmt.Printf("%-30s %-12s %-10d %-50s\n",
			worker.TaskName,
			worker.Language,
			worker.Version,
			description)
	}

	fmt.Printf("\nTotal: %d workers\n", len(workers))
	return nil
}

func runRemoteWorker(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	taskType, _ := cmd.Flags().GetString("type")
	if taskType == "" {
		return fmt.Errorf("--type flag is required")
	}

	refresh, _ := cmd.Flags().GetBool("refresh")

	// Get or download worker code
	workerFile, language, err := getRemoteWorker(taskType, refresh)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// Execute based on language
	switch language {
	case "NODEJS":
		return executeJsWorkerFromFile(cmd, workerFile, taskType)
	case "PYTHON":
		return executePythonWorkerFromFile(cmd, workerFile, taskType)
	default:
		return fmt.Errorf("unsupported worker language: %s (supported: NODEJS, PYTHON)", language)
	}
}

func getRemoteWorker(taskName string, refresh bool) (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".conductor-cli", "workers", taskName)
	metadataFile := filepath.Join(cacheDir, ".metadata.json")

	if !refresh {
		if metadata, err := loadMetadata(metadataFile); err == nil {
			workerFile := getWorkerFile(cacheDir, metadata.Language)
			if fileExists(workerFile) {
				log.Infof("Using cached worker '%s' (version %d)", taskName, metadata.Version)
				return workerFile, metadata.Language, nil
			}
		}
	}

	log.Infof("Downloading worker '%s' from registry...", taskName)

	serverUrl := viper.GetString("server")
	if serverUrl == "" {
		serverUrl = "http://localhost:8080/api"
	}
	serverUrl = strings.TrimSuffix(serverUrl, "/")
	if !strings.HasSuffix(serverUrl, "/api") {
		serverUrl = serverUrl + "/api"
	}
	apiUrl := fmt.Sprintf("%s/worker-code/by-name/%s", serverUrl, taskName)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	if err := addWorkerAuthHeaders(req); err != nil {
		return "", "", fmt.Errorf("failed to add auth headers: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", "", fmt.Errorf("worker '%s' not found in registry", taskName)
	}
	if resp.StatusCode == 401 {
		return "", "", fmt.Errorf("authentication failed (401 Unauthorized) - verify your credentials")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("failed to download worker: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var response WorkerCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	workerFile := getWorkerFile(cacheDir, response.Language)
	if err := os.WriteFile(workerFile, []byte(response.Code), 0600); err != nil {
		return "", "", fmt.Errorf("failed to save worker code: %w", err)
	}

	metadata := WorkerMetadata{
		TaskName:     response.TaskName,
		Language:     response.Language,
		Version:      response.Version,
		WorkerCodeId: response.Id,
		CachedAt:     time.Now(),
		UpdatedAt:    response.UpdatedAt,
	}
	metadataJson, _ := json.MarshalIndent(metadata, "", "  ")
	os.WriteFile(metadataFile, metadataJson, 0600)

	log.Infof("Worker downloaded successfully (version %d)", response.Version)

	if response.Language == "PYTHON" {
		log.Infof("Setting up Python environment and installing dependencies...")
		if err := setupPythonEnvironment(cacheDir, response.Dependencies); err != nil {
			log.Warnf("Failed to set up Python environment: %v", err)
			log.Warnf("You may need to manually install Python dependencies")
		} else {
			log.Infof("Python environment ready")
		}
	}

	return workerFile, response.Language, nil
}

func addWorkerAuthHeaders(req *http.Request) error {
	authToken := viper.GetString("auth-token")
	if authToken != "" {
		req.Header.Set("X-Authorization", authToken)
		return nil
	}

	cachedToken := viper.GetString("cached-token")
	if cachedToken != "" {
		req.Header.Set("X-Authorization", cachedToken)
		return nil
	}

	authKey := viper.GetString("auth-key")
	authSecret := viper.GetString("auth-secret")
	if authKey != "" && authSecret != "" {
		token, err := exchangeKeySecretForToken(authKey, authSecret)
		if err != nil {
			return fmt.Errorf("failed to get token from key+secret: %w", err)
		}
		req.Header.Set("X-Authorization", token)
		return nil
	}

	return nil
}

func exchangeKeySecretForToken(key, secret string) (string, error) {
	serverUrl := viper.GetString("server")
	if serverUrl == "" {
		serverUrl = "http://localhost:8080/api"
	}

	serverUrl = strings.TrimSuffix(serverUrl, "/")
	if !strings.HasSuffix(serverUrl, "/api") {
		serverUrl = serverUrl + "/api"
	}

	httpSettings := settings.NewHttpSettings(serverUrl)
	authSettings := settings.NewAuthenticationSettings(key, secret)

	tokenResponse, _, err := authentication.GetToken(*authSettings, httpSettings, &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		return "", fmt.Errorf("authentication failed: %w", err)
	}

	return tokenResponse.Token, nil
}

func getWorkerFile(cacheDir, language string) string {
	ext := map[string]string{
		"NODEJS": ".js",
		"PYTHON": ".py",
		"JAVA":   ".java",
		"GO":     ".go",
	}
	extension := ext[language]
	if extension == "" {
		extension = ".txt"
	}
	return filepath.Join(cacheDir, "worker"+extension)
}

func loadMetadata(metadataFile string) (*WorkerMetadata, error) {
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, err
	}

	var metadata WorkerMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func executeJsWorkerFromFile(cmd *cobra.Command, workerFile, taskType string) error {
	count, _ := cmd.Flags().GetInt32("count")
	workerId, _ := cmd.Flags().GetString("worker-id")
	domain, _ := cmd.Flags().GetString("domain")
	timeout, _ := cmd.Flags().GetInt32("timeout")

	scriptContent, err := os.ReadFile(workerFile)
	if err != nil {
		return fmt.Errorf("error reading worker file: %v", err)
	}

	log.Infof("Starting JavaScript worker for task type: %s", taskType)
	if workerId != "" {
		log.Infof("Worker ID: %s", workerId)
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

func setupPythonEnvironment(cacheDir string, dependencies []string) error {
	venvDir := filepath.Join(cacheDir, "venv")
	requirementsFile := filepath.Join(cacheDir, "requirements.txt")

	allDependencies := append([]string{"conductor-python"}, dependencies...)

	venvExists := fileExists(venvDir)
	requirementsExists := fileExists(requirementsFile)

	var existingRequirements []string
	if requirementsExists {
		data, err := os.ReadFile(requirementsFile)
		if err == nil {
			existingRequirements = strings.Split(strings.TrimSpace(string(data)), "\n")
		}
	}

	requirementsChanged := !equalStringSlices(allDependencies, existingRequirements)

	requirementsContent := strings.Join(allDependencies, "\n")
	if err := os.WriteFile(requirementsFile, []byte(requirementsContent), 0644); err != nil {
		return fmt.Errorf("failed to write requirements.txt: %w", err)
	}

	if !venvExists {
		log.Infof("Creating Python virtual environment...")
		log.Debugf("Running: python3 -m venv %s", venvDir)
		createVenvCmd := exec.Command("python3", "-m", "venv", venvDir)
		createVenvCmd.Dir = cacheDir
		if output, err := createVenvCmd.CombinedOutput(); err != nil {
			log.Errorf("Failed to create venv. Output:\n%s", string(output))
			return fmt.Errorf("failed to create venv: %w", err)
		}
		log.Infof("Virtual environment created successfully")
	}

	if !venvExists || requirementsChanged {
		log.Infof("Installing dependencies: %v", allDependencies)

		pipPath := filepath.Join(venvDir, "bin", "pip")
		if !fileExists(pipPath) {
			return fmt.Errorf("pip not found in venv: %s", pipPath)
		}

		log.Debugf("Running: %s install -r %s", pipPath, requirementsFile)
		installCmd := exec.Command(pipPath, "install", "-r", requirementsFile)
		installCmd.Dir = cacheDir

		output, err := installCmd.CombinedOutput()
		log.Debugf("Pip output:\n%s", string(output))

		if err != nil {
			log.Errorf("Failed to install dependencies. Pip output:\n%s", string(output))
			return fmt.Errorf("failed to install dependencies: %w", err)
		}

		log.Infof("Dependencies installed successfully")
	} else {
		log.Debugf("Virtual environment already configured, skipping dependency installation")
	}

	return nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func executePythonWorkerFromFile(cmd *cobra.Command, workerFile, taskType string) error {
	count, _ := cmd.Flags().GetInt32("count")
	workerId, _ := cmd.Flags().GetString("worker-id")
	domain, _ := cmd.Flags().GetString("domain")
	execTimeout, _ := cmd.Flags().GetInt32("timeout")

	pythonCmd := "python3"
	cacheDir := filepath.Dir(workerFile)
	venvPython := filepath.Join(cacheDir, "venv", "bin", "python")

	if fileExists(venvPython) {
		pythonCmd = venvPython
		log.Infof("Using virtual environment Python: %s", venvPython)
	} else {
		log.Infof("Using system Python: python3")
	}

	log.Infof("Starting Python worker for task type: %s", taskType)
	if workerId != "" {
		log.Infof("Worker ID: %s", workerId)
	}

	taskClient := internal.GetTaskClient()

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
		if execTimeout > 0 {
			opts.Timeout = optional.NewInt32(execTimeout)
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
				executeExternalWorker(t, pythonCmd, []string{workerFile}, workerId, domain, execTimeout, false, taskClient)
			}(task)
		}

		wg.Wait()
	}
}

func init() {
	workerJsCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerJsCmd.MarkFlagRequired("type")
	workerJsCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerJsCmd.Flags().String("worker-id", "", "Worker ID")
	workerJsCmd.Flags().String("domain", "", "Domain")
	workerJsCmd.Flags().Int32("timeout", 100, "Timeout in milliseconds")

	workerStdioCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerStdioCmd.MarkFlagRequired("type")
	workerStdioCmd.Flags().String("worker-id", "", "Worker ID")
	workerStdioCmd.Flags().String("domain", "", "Domain")
	workerStdioCmd.Flags().Int32("poll-timeout", 100, "Poll timeout in milliseconds")
	workerStdioCmd.Flags().Int32("exec-timeout", 0, "Execution timeout in seconds (0 = no timeout)")
	workerStdioCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerStdioCmd.Flags().Bool("verbose", false, "Print task and result JSON to stdout")

	workerRemoteCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerRemoteCmd.MarkFlagRequired("type")
	workerRemoteCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerRemoteCmd.Flags().String("worker-id", "", "Worker ID")
	workerRemoteCmd.Flags().String("domain", "", "Domain")
	workerRemoteCmd.Flags().Int32("timeout", 100, "Timeout in milliseconds")
	workerRemoteCmd.Flags().Bool("refresh", false, "Force refresh worker from registry (ignore cache)")

	workerListRemoteCmd.Flags().String("namespace", "default", "Namespace to list workers from")

	workerCmd.AddCommand(workerJsCmd)
	workerCmd.AddCommand(workerStdioCmd)
	workerCmd.AddCommand(workerRemoteCmd)
	workerCmd.AddCommand(workerListRemoteCmd)
	rootCmd.AddCommand(workerCmd)
}
