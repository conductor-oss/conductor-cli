using System;
using System.Collections.Generic;
using System.Text.Json;

/// <summary>
/// Simple stdio worker example for Orkes Conductor CLI.
///
/// This worker reads a task from stdin, processes it, and returns a result to stdout.
/// </summary>
class SimpleWorker
{
    class Task
    {
        public string? TaskId { get; set; }
        public string? WorkflowInstanceId { get; set; }
        public string? TaskType { get; set; }
        public Dictionary<string, object>? InputData { get; set; }
    }

    class Result
    {
        public string Status { get; set; } = "COMPLETED";
        public Dictionary<string, object>? Output { get; set; }
        public List<string>? Logs { get; set; }
        public string? Reason { get; set; }
    }

    static void Main()
    {
        try
        {
            // Read task from stdin
            string taskJson = Console.In.ReadToEnd();

            // Parse task JSON
            var task = JsonSerializer.Deserialize<Task>(taskJson);

            // Get input parameters
            string name = "World";
            if (task?.InputData != null && task.InputData.ContainsKey("name"))
            {
                var nameValue = task.InputData["name"];
                if (nameValue != null)
                {
                    name = nameValue.ToString() ?? "World";
                }
            }

            // Get task metadata
            string taskId = task?.TaskId ?? "unknown";
            string workflowId = task?.WorkflowInstanceId ?? "unknown";
            string taskType = task?.TaskType ?? "unknown";

            // Process the task
            string message = $"Hello, {name}!";

            // Create result
            var result = new Result
            {
                Status = "COMPLETED",
                Output = new Dictionary<string, object>
                {
                    { "message", message },
                    { "taskId", taskId },
                    { "workflowId", workflowId }
                },
                Logs = new List<string>
                {
                    $"Processing task {taskId} of type {taskType}",
                    $"Workflow: {workflowId}",
                    $"Generated greeting for {name}"
                }
            };

            // Output result to stdout
            string resultJson = JsonSerializer.Serialize(result);
            Console.WriteLine(resultJson);
        }
        catch (Exception ex)
        {
            // Return failure result on error
            var errorResult = new Result
            {
                Status = "FAILED",
                Reason = ex.Message,
                Logs = new List<string> { $"Error processing task: {ex.Message}" }
            };

            string errorJson = JsonSerializer.Serialize(errorResult);
            Console.WriteLine(errorJson);
        }
    }
}
