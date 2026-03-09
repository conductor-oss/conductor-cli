package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Task represents the input task structure
type Task struct {
	TaskId             string                 `json:"taskId"`
	WorkflowInstanceId string                 `json:"workflowInstanceId"`
	TaskType           string                 `json:"taskType"`
	InputData          map[string]interface{} `json:"inputData"`
}

// Result represents the output result structure
type Result struct {
	Status string                 `json:"status"`
	Output map[string]interface{} `json:"output,omitempty"`
	Logs   []string               `json:"logs,omitempty"`
	Reason string                 `json:"reason,omitempty"`
}

// Simple stdio worker example for Orkes Conductor CLI.
//
// This worker reads a task from stdin, processes it, and returns a result to stdout.
func main() {
	// Read task from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		outputError(fmt.Sprintf("Failed to read stdin: %v", err))
		return
	}

	// Parse task JSON
	var task Task
	if err := json.Unmarshal(input, &task); err != nil {
		outputError(fmt.Sprintf("Failed to parse task JSON: %v", err))
		return
	}

	// Get input parameters
	name := "World"
	if n, ok := task.InputData["name"].(string); ok && n != "" {
		name = n
	}

	// Get task metadata
	taskID := task.TaskId
	if taskID == "" {
		taskID = "unknown"
	}
	workflowID := task.WorkflowInstanceId
	if workflowID == "" {
		workflowID = "unknown"
	}
	taskType := task.TaskType
	if taskType == "" {
		taskType = "unknown"
	}

	// Process the task
	message := fmt.Sprintf("Hello, %s!", name)

	// Create result
	result := Result{
		Status: "COMPLETED",
		Output: map[string]interface{}{
			"message":    message,
			"taskId":     taskID,
			"workflowId": workflowID,
		},
		Logs: []string{
			fmt.Sprintf("Processing task %s of type %s", taskID, taskType),
			fmt.Sprintf("Workflow: %s", workflowID),
			fmt.Sprintf("Generated greeting for %s", name),
		},
	}

	// Output result to stdout
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		outputError(fmt.Sprintf("Failed to encode result: %v", err))
	}
}

// outputError outputs an error result
func outputError(message string) {
	result := Result{
		Status: "FAILED",
		Reason: message,
		Logs:   []string{fmt.Sprintf("Error processing task: %s", message)},
	}
	json.NewEncoder(os.Stdout).Encode(result)
}
