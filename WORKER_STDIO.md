# STDIO Workers

The CLI supports executing tasks using external worker programs written in any language. 

This allows you to implement task workers in Python, Node.js, shell scripts, or any executable.

## How it Works

The `worker stdio` command continuously polls for tasks and executes them in parallel goroutines:

1. **Continuously polls** for tasks of the specified type
2. **Passes** the full task JSON to your worker via **stdin**
3. **Sets** environment variables with task metadata
4. **Reads** the result JSON from your worker's **stdout**
5. **Updates** the task in Conductor with the result
6. **Executes** each task in a separate goroutine for parallel processing

## Usage

```bash
orkes worker stdio --type <task_type> <command> [args...]
```

**Flags:**
- `--type`: Task type to poll for (required)
- `--worker-id`: Worker identifier
- `--domain`: Poll domain
- `--poll-timeout`: Poll timeout in milliseconds (default: 100)
- `--exec-timeout`: Worker execution timeout in seconds (0 = no timeout)
- `--count`: Number of tasks to poll in each batch (default: 1)
- `--verbose`: Print task and result JSON to stdout

## Worker Contract

**Input (stdin):** Full task JSON from Conductor

**Environment variables:**
- `TASK_TYPE` - Type of the task
- `TASK_ID` - Task ID
- `WORKFLOW_ID` - Workflow instance ID
- `EXECUTION_ID` - Workflow execution ID (same as WORKFLOW_ID)
- `POLL_DOMAIN` - Domain (if specified)

**Output (stdout):** JSON result:
```json
{
  "status": "COMPLETED|FAILED|IN_PROGRESS",
  "output": {"key": "value"},
  "logs": ["log line 1", "log line 2"],
  "reason": "failure reason (optional)"
}
```

**Exit codes:**
- `0` - Task handled successfully (status field determines success/failure)
- `non-zero` - Failure (task automatically marked as FAILED)

## Example: Python Worker

Create a simple Python worker that greets a user:

```python
#!/usr/bin/env python3
import sys
import json

# Read task from stdin
task = json.load(sys.stdin)

# Get input parameters
input_data = task.get('inputData', {})
name = input_data.get('name', 'World')

# Process the task
message = f"Hello {name}"

# Return result to stdout
result = {
    "status": "COMPLETED",
    "output": {
        "message": message
    },
    "logs": [f"Processed greeting for {name}"]
}

print(json.dumps(result))
```

Save as `worker.py` and make it executable:

```bash
chmod +x worker.py
```

Run the worker:

```bash
# Start worker for 'greet_task'
orkes worker stdio --type greet_task python3 worker.py

# Poll multiple tasks per batch (poll 5 tasks at a time)
orkes worker stdio --type greet_task python3 worker.py --count 5

# With worker ID and domain
orkes worker stdio --type greet_task python3 worker.py --worker-id worker-1 --domain production

# With execution timeout (30 seconds per task)
orkes worker stdio --type greet_task python3 worker.py --exec-timeout 30
```

## Example: Shell Script Worker

```bash
#!/bin/bash
# worker.sh

# Read task JSON from stdin
task=$(cat)

# Extract name from input (using jq)
name=$(echo "$task" | jq -r '.inputData.name // "World"')

# Process
message="Hello $name"

# Return result
jq -n \
  --arg msg "$message" \
  '{
    "status": "COMPLETED",
    "output": {"message": $msg},
    "logs": ["Processed greeting"]
  }'
```

## Parallel Execution

The worker automatically runs in continuous mode:

1. Continuously polls for tasks
2. Executes each task in a separate goroutine (parallel execution)
3. Automatically retries on polling errors
4. Logs all task processing with timestamps

**Batch polling for higher throughput:**

```bash
# Poll 10 tasks at a time and process them in parallel
orkes worker stdio --type greet_task python3 worker.py --count 10
```

## Error Handling

If your worker exits with a non-zero code or produces invalid JSON, the task will be marked as FAILED with details in the reason field:

```python
#!/usr/bin/env python3
import sys
import json

try:
    task = json.load(sys.stdin)
    # ... process task ...
    print(json.dumps({"status": "COMPLETED", "output": result}))
except Exception as e:
    # Return failure status
    print(json.dumps({
        "status": "FAILED",
        "reason": str(e),
        "logs": [f"Error: {e}"]
    }))
    sys.exit(0)  # Exit 0 so CLI reads the failure status
```

## Advanced Examples

### Node.js Worker

```javascript
#!/usr/bin/env node
// worker.js

const fs = require('fs');

// Read task from stdin
let input = '';
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  const task = JSON.parse(input);
  const name = task.inputData?.name || 'World';

  const result = {
    status: 'COMPLETED',
    output: {
      message: `Hello ${name}`
    },
    logs: [`Processed greeting for ${name}`]
  };

  console.log(JSON.stringify(result));
});
```

### Go Worker

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Task struct {
	InputData map[string]interface{} `json:"inputData"`
}

type Result struct {
	Status string                 `json:"status"`
	Output map[string]interface{} `json:"output"`
	Logs   []string               `json:"logs"`
}

func main() {
	// Read task from stdin
	input, _ := io.ReadAll(os.Stdin)
	var task Task
	json.Unmarshal(input, &task)

	// Get name from input
	name := "World"
	if n, ok := task.InputData["name"].(string); ok {
		name = n
	}

	// Create result
	result := Result{
		Status: "COMPLETED",
		Output: map[string]interface{}{
			"message": fmt.Sprintf("Hello %s", name),
		},
		Logs: []string{fmt.Sprintf("Processed greeting for %s", name)},
	}

	// Output result
	json.NewEncoder(os.Stdout).Encode(result)
}
```

## Best Practices

1. **Always validate input**: Check that required fields are present before processing
2. **Use proper error handling**: Catch exceptions and return FAILED status with details
3. **Log important events**: Use the logs array to track what happened
4. **Handle timeouts gracefully**: Be aware of `--exec-timeout` setting
5. **Make workers idempotent**: Tasks may be retried, ensure your worker can handle this
6. **Return meaningful output**: Include useful data in the output field
7. **Use environment variables**: Access `TASK_ID`, `WORKFLOW_ID` etc. when needed
8. **Exit with code 0**: Always exit with 0 and use status field for task outcome

## Comparison with JavaScript Workers

| Feature | Generic Workers (exec) | JavaScript Workers (js) |
|---------|----------------------|------------------------|
| Languages | Any (Python, Node, Go, etc.) | JavaScript only |
| Dependencies | Full access to language ecosystem | Limited (Goja ES5.1+) |
| Setup | Requires external executable | Built-in, no setup |
| Performance | Process per task (overhead) | In-process (faster) |
| HTTP Calls | Use language's HTTP library | Built-in `http` object |
| File System | Full access | No access |
| Best For | Complex logic, heavy dependencies | Lightweight tasks, quick scripts |
