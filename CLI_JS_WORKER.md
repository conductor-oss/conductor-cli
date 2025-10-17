# JavaScript Worker for Conductor CLI

⚠️ **EXPERIMENTAL FEATURE** - This feature is experimental and may change in future releases.

The `orkes worker run` command allows you to run JavaScript workers that poll for tasks and execute custom JavaScript code to process them.

## Overview

The JavaScript worker:
- Continuously polls for tasks of a specific type
- Executes a JavaScript file for each task
- Processes tasks in parallel using goroutines
- Automatically updates task status based on script execution results

## Usage

```bash
orkes worker run --type <task_type> <js_file>
```

### Required Arguments

- `<js_file>` - Path to the JavaScript file to execute for each task
- `--type` - Task type to poll for (required flag)

### Optional Flags

- `--count` - Number of tasks to poll in each batch (default: 1)
- `--worker-id` - Worker ID for identification
- `--domain` - Domain for task polling
- `--timeout` - Timeout in milliseconds (default: 100)

### Example

```bash
orkes worker run --type process_order --worker-id worker1 --count 5 worker.js
```

## JavaScript Worker Script

Your JavaScript file will be executed for each polled task. The task data is available via the `$.task` global object.

### Simple Example

```javascript
// worker.js - Simple task processor
(function() {
  var input = $.task.inputData;

  // Process the input
  var result = {
    processed: true,
    originalData: input
  };

  // Return result
  return {
    status: "COMPLETED",
    body: result
  };
})();
```

### Fetching External Data

```javascript
// worker.js - Fetch a cat fact from an API
(function() {
  // Make HTTP GET request
  var response = http.get("https://catfact.ninja/fact", {});

  // Check for errors
  if (response.error) {
    return {
      status: "FAILED",
      body: { error: "Failed to fetch cat fact: " + response.error }
    };
  }

  // Check HTTP status
  if (response.status !== 200) {
    return {
      status: "FAILED",
      body: { error: "HTTP error: " + response.status }
    };
  }

  // Extract just the fact field
  var catFact = response.body.fact;

  // Return success with the fact
  return {
    status: "COMPLETED",
    body: {
      fact: catFact,
      taskId: $.task.taskId,
      fetchedAt: new Date().toISOString()
    }
  };
})();
```

**Note:** Workers must use a self-invoking function `(function() { ... })()` or define and call a function to use `return` statements.

### Returning Status and Output

Your script can return an object with `status` and `body` fields to control the task outcome:

```javascript
// worker.js - Return custom status and output
(function() {
  var input = $.task.inputData;

  function processOrder(orderId) {
    // Your business logic here
    return { success: true, trackingNumber: "TRK-" + orderId };
  }

  try {
    // Process the task
    var result = processOrder(input.orderId);

    // Return success with output
    return {
      status: "COMPLETED",
      body: {
        orderId: input.orderId,
        result: result,
        processedAt: new Date().toISOString()
      }
    };
  } catch (error) {
    // Return failure with error details
    return {
      status: "FAILED",
      body: {
        error: error.message,
        orderId: input.orderId
      }
    };
  }
})();
```

### Status Values

The `status` field can be one of:
- `COMPLETED` - Task completed successfully
- `FAILED` - Task failed
- `FAILED_WITH_TERMINAL_ERROR` - Task failed with terminal error (no retries)
- `IN_PROGRESS` - Task is still in progress

### Return Value Behavior

| Return Value | Behavior |
|-------------|----------|
| `{status: "...", body: {...}}` | Updates task with specified status and output data |
| Any other value | Task marked as COMPLETED with the value in output |
| No return value (undefined) | Task marked as COMPLETED with empty output |
| Script throws error | Task marked as FAILED with error message |

## The $.task Object

The `$.task` object contains all information about the current task being processed.

### Key Fields

#### Task Identification
- `taskId` (string) - Unique identifier for this task
- `taskType` (string) - Type of the task
- `referenceTaskName` (string) - Reference name from workflow definition
- `taskDefName` (string) - Task definition name

#### Workflow Context
- `workflowInstanceId` (string) - ID of the workflow instance this task belongs to
- `workflowType` (string) - Type of the workflow
- `correlationId` (string) - Correlation ID for tracking
- `workflowPriority` (number) - Priority of the workflow

#### Task Data
- `inputData` (object) - Input data for the task (most commonly used)
- `outputData` (object) - Output data from the task
- `status` (string) - Current status of the task

#### Execution Details
- `workerId` (string) - ID of the worker processing this task
- `pollCount` (number) - Number of times this task has been polled
- `retryCount` (number) - Number of times this task has been retried
- `seq` (number) - Sequence number in the workflow

#### Timing Information
- `scheduledTime` (number) - When the task was scheduled (epoch milliseconds)
- `startTime` (number) - When the task started (epoch milliseconds)
- `endTime` (number) - When the task ended (epoch milliseconds)
- `updateTime` (number) - Last update time (epoch milliseconds)
- `queueWaitTime` (number) - Time spent in queue (milliseconds)

#### Configuration
- `responseTimeoutSeconds` (number) - Response timeout in seconds
- `callbackAfterSeconds` (number) - Callback delay in seconds
- `startDelayInSeconds` (number) - Start delay in seconds

#### State Flags
- `retried` (boolean) - Whether this task has been retried
- `executed` (boolean) - Whether this task has been executed
- `callbackFromWorker` (boolean) - Whether callback is from worker
- `loopOverTask` (boolean) - Whether this is a loop task

#### Advanced Fields
- `domain` (string) - Domain for the task
- `isolationGroupId` (string) - Isolation group identifier
- `executionNameSpace` (string) - Execution namespace
- `subWorkflowId` (string) - Sub-workflow ID if this task spawned a sub-workflow
- `iteration` (number) - Iteration number for loop tasks
- `taskDefinition` (object) - Full task definition metadata
- `workflowTask` (object) - Workflow task definition

### Example: Accessing Task Data

```javascript
// worker.js - Using various task fields
(function() {
  var task = $.task;

  // Access input data
  var orderId = task.inputData.orderId;
  var customerName = task.inputData.customerName;

  // Calculate processing time
  var startTime = new Date(task.startTime);
  var now = new Date();
  var processingTime = now - startTime;

  // Return result
  return {
    status: "COMPLETED",
    body: {
      taskId: task.taskId,
      workflowId: task.workflowInstanceId,
      taskType: task.taskType,
      orderId: orderId,
      customerName: customerName,
      processingTime: processingTime,
      retryCount: task.retryCount,
      isRetry: task.retryCount > 0
    }
  };
})();
```

## Error Handling

If your script throws an error, the task is automatically marked as FAILED:

```javascript
// worker.js - Error handling
(function() {
  function riskyOperation(data) {
    // Your risky operation here
    if (!data) throw new Error("No data provided");
    return { processed: data };
  }

  try {
    var result = riskyOperation($.task.inputData);

    return {
      status: "COMPLETED",
      body: result
    };
  } catch (error) {
    // Return explicit failure
    return {
      status: "FAILED",
      body: {
        error: error.message,
        stack: error.stack
      }
    };
  }
})();
```

## Best Practices

1. **Always validate input data**
   ```javascript
   (function() {
     var input = $.task.inputData;
     if (!input.orderId) {
       return {
         status: "FAILED",
         body: { error: "Missing orderId in input" }
       };
     }
     // ... rest of your code
   })();
   ```

2. **Use try-catch for error handling**
   ```javascript
   (function() {
     try {
       // Your logic
     } catch (error) {
       return { status: "FAILED", body: { error: error.message } };
     }
   })();
   ```

3. **Return meaningful output**
   ```javascript
   (function() {
     var processedData = { /* ... */ };
     return {
       status: "COMPLETED",
       body: {
         result: processedData,
         timestamp: new Date().toISOString(),
         workerId: $.task.workerId
       }
     };
   })();
   ```

4. **Handle retries gracefully**
   ```javascript
   (function() {
     if ($.task.retryCount > 3) {
       return {
         status: "FAILED_WITH_TERMINAL_ERROR",
         body: { error: "Max retries exceeded" }
       };
     }
     // ... rest of your code
   })();
   ```

## Parallel Processing

The worker processes multiple tasks in parallel using goroutines. Each task is executed in its own JavaScript VM instance, so there's no shared state between tasks.

```bash
# Poll 10 tasks at a time and process them in parallel
orkes worker run --type my_task --count 10 worker.js
```

## Continuous Polling

The worker runs in a continuous loop, polling for tasks and processing them until you stop it (Ctrl+C).

## Logging

The worker logs important events at the Go level:
- Task polling (how many tasks polled)
- Task processing start
- Task completion/failure
- Errors during script execution

Use `--verbose` flag for debug logging:
```bash
orkes --verbose worker run --type my_task worker.js
```

**Note:** JavaScript scripts cannot use `console.log()` - logging is handled by the Go worker process.

## Example: Complete Worker Script

```javascript
// worker.js - Complete example
(function() {
  var task = $.task;
  var input = task.inputData;

  // Helper functions
  function transformData(data) {
    return { transformed: true, data: data.toUpperCase() };
  }

  function validateData(data) {
    var isValid = data && data.length > 0;
    return { valid: isValid };
  }

  function enrichData(data) {
    return {
      original: data,
      enriched: {
        timestamp: Date.now(),
        processed: true
      }
    };
  }

  // Validate input
  if (!input.action || !input.data) {
    return {
      status: "FAILED",
      body: {
        error: "Missing required fields: action, data",
        received: input
      }
    };
  }

  // Process based on action
  try {
    var result;

    switch (input.action) {
      case "transform":
        result = transformData(input.data);
        break;
      case "validate":
        result = validateData(input.data);
        break;
      case "enrich":
        result = enrichData(input.data);
        break;
      default:
        return {
          status: "FAILED",
          body: { error: "Unknown action: " + input.action }
        };
    }

    return {
      status: "COMPLETED",
      body: {
        action: input.action,
        result: result,
        taskId: task.taskId,
        workflowId: task.workflowInstanceId,
        processedAt: new Date().toISOString()
      }
    };

  } catch (error) {
    return {
      status: "FAILED",
      body: {
        error: error.message,
        action: input.action,
        taskId: task.taskId
      }
    };
  }
})();
```
