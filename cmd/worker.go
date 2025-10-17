package cmd

import (
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
		Use:   "worker",
		Short: "Worker commands (EXPERIMENTAL)",
		Long:  "⚠️  EXPERIMENTAL FEATURE - Worker features are experimental and may change in future releases.\n\nManage and run workers for task processing.",
	}

	workerRunCmd = &cobra.Command{
		Use:          "run <js_file>",
		Short:        "Run a JavaScript worker that polls and processes tasks",
		Long:         "Run a JavaScript worker that continuously polls for tasks of a specific type and executes the provided JavaScript file for each task.",
		RunE:         runWorker,
		SilenceUsage: true,
		Example:      "orkes worker run --type my_task worker.js",
	}
)

type TaskResult struct {
	Status string                 `json:"status"`
	Body   map[string]interface{} `json:"body"`
}

func runWorker(cmd *cobra.Command, args []string) error {
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

func init() {
	workerRunCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerRunCmd.MarkFlagRequired("type")
	workerRunCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerRunCmd.Flags().String("worker-id", "", "Worker ID")
	workerRunCmd.Flags().String("domain", "", "Domain")
	workerRunCmd.Flags().Int32("timeout", 100, "Timeout in milliseconds")

	workerCmd.AddCommand(workerRunCmd)
	rootCmd.AddCommand(workerCmd)
}
