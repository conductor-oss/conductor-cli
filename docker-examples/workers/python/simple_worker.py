#!/usr/bin/env python3
"""
Simple stdio worker example for Orkes Conductor CLI.

This worker reads a task from stdin, processes it, and returns a result to stdout.
"""
import sys
import json

def main():
    # Read task from stdin
    task = json.load(sys.stdin)

    # Get input parameters
    input_data = task.get('inputData', {})
    name = input_data.get('name', 'World')

    # Get task metadata
    task_id = task.get('taskId', 'unknown')
    workflow_id = task.get('workflowInstanceId', 'unknown')
    task_type = task.get('taskType', 'unknown')

    # Process the task
    message = f"Hello, {name}!"

    # Return result to stdout
    result = {
        "status": "COMPLETED",
        "output": {
            "message": message,
            "taskId": task_id,
            "workflowId": workflow_id
        },
        "logs": [
            f"Processing task {task_id} of type {task_type}",
            f"Workflow: {workflow_id}",
            f"Generated greeting for {name}"
        ]
    }

    print(json.dumps(result))

if __name__ == '__main__':
    try:
        main()
    except Exception as e:
        # Return failure result on error
        result = {
            "status": "FAILED",
            "reason": str(e),
            "logs": [f"Error processing task: {e}"]
        }
        print(json.dumps(result))
