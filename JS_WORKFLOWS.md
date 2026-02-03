# JavaScript Workflows (Experimental)

⚠️ **This feature is highly experimental and subject to change.**

## Overview

The CLI supports defining Conductor workflows using JavaScript instead of JSON. This allows you to write workflows in a more programmatic way with helper functions and inline task definitions.

## Requirements

Your JavaScript file **must** define a `workflow()` function that returns an object containing:
- `name`: Workflow name
- `description`: Workflow description
- `version`: Workflow version number
- `tasks`: Array of task definitions
- `inputParameters`: (optional) Array of input parameter names
- `outputParameters`: (optional) Output parameter mappings

## Creating a JavaScript Workflow

```bash
conductor workflow create workflow.js --js
```

The `--js` flag tells the CLI to process the file as JavaScript.

## Example Workflow

```javascript
function workflow() {
    return {
        name: "my_js_workflow",
        description: "Sample JS Workflow",
        version: 1,
        tasks: [
            {
                name: "process_data",
                taskReferenceName: "process_data_ref",
                type: "SIMPLE",
                inputParameters: {
                    data: "${workflow.input.data}"
                }
            },
            {
                name: "inline_fn",
                type: "INLINE",
                inputParameters: {},
                function: function transform() {
                    return {
                        result: "transformed"
                    };
                }
            },
            {
                // WAIT task - taskReferenceName auto-generated from name
                type: "WAIT",
                duration: "30s"
            },
            {
                name: "final_task",
                type: "SIMPLE",
                inputParameters: {
                    input: "${inline_fn.output.result}"
                }
            }
        ],
        inputParameters: ["data"],
        outputParameters: {
            result: "${final_task.output}"
        }
    };
}
```

## Special Features

### Inline Functions

You can define inline JavaScript functions that will be automatically converted to `INLINE` tasks with GraalJS evaluator:

```javascript
{
    name: "inline_fn",
    type: "INLINE",
    inputParameters: {},
    function: function myFunction() {
        // Your JavaScript code here
        return {
            result: "some value"
        };
    }
}
```

The transformation will:
1. Convert the task `type` to `"INLINE"`
2. Stringify the function and set it as the `expression` parameter
3. Set `evaluatorType` to `"graaljs"`

### WAIT Tasks

For `WAIT` tasks, you can simply specify the duration:

```javascript
{
    type: "WAIT",
    duration: "30s"
}
```

The transformation will:
1. Auto-generate the task `name` as `"WAIT"` if not provided
2. Move `duration` to `inputParameters.duration`

### Auto-generated Task References

If you don't provide a `taskReferenceName`, it will be automatically generated from the task `name`.

## How It Works

When you run `conductor workflow create workflow.js --js`, the CLI:

1. Loads your JavaScript file
2. Executes the `workflow()` function in a JavaScript VM (goja)
3. Applies transformations to the returned object:
   - Converts inline functions to INLINE tasks
   - Processes WAIT tasks
   - Auto-generates missing taskReferenceNames
4. Converts the result to JSON
5. Registers the workflow with Conductor

## Limitations

- The JavaScript runtime is sandboxed (goja)
- No access to Node.js modules or external dependencies
- Only the `workflow()` function is executed
- Error handling is basic - syntax errors may not be clearly reported

## Updating Workflows

To update an existing JavaScript workflow:

```bash
conductor workflow update workflow.js --js
```

## Tips

1. Always initialize `inputParameters: {}` for tasks with inline functions
2. Test your JavaScript syntax before uploading
3. Use descriptive task names for better readability
4. Remember that this is experimental - fallback to JSON if you encounter issues
