# AI Agent Guidelines for Conductor CLI

This document provides guidelines for AI agents working with the Conductor CLI codebase.

## Project Overview

**Conductor CLI** (`conductor`) is a command-line interface for managing Netflix Conductor workflows, tasks, webhooks, schedules, secrets, and API gateway configurations. It's built in Go using the Cobra command framework.

## Architecture

```
conductor-cli/
├── main.go                    # Entry point
├── version.go                 # Build version info
├── cmd/                       # CLI commands (Cobra)
│   ├── root.go               # Root command, global flags, config
│   ├── workflow.go           # Workflow definition & execution commands
│   ├── task.go               # Task definition & execution commands
│   ├── scheduler.go          # Schedule management (Enterprise)
│   ├── secret.go             # Secret management (Enterprise)
│   ├── webhook_metadata.go   # Webhook management (Enterprise)
│   ├── api_gateway.go        # API Gateway management (Enterprise)
│   ├── config.go             # CLI configuration commands
│   ├── worker.go             # Task worker execution
│   ├── cached_token_manager.go # Token caching logic
│   └── token_validation.go   # JWT validation utilities
├── internal/
│   ├── settings.go           # API client factory/singleton
│   └── updater/              # Auto-update functionality
└── test/e2e/                 # End-to-end tests
```

## Key Patterns

### Command Structure
All commands follow the Cobra pattern:
```go
var exampleCmd = &cobra.Command{
    Use:          "example <args>",
    Short:        "Brief description",
    RunE:         exampleFunction,
    SilenceUsage: true,
}

func exampleFunction(cmd *cobra.Command, args []string) error {
    // 1. Get flags and validate input
    // 2. Get API client from internal package
    // 3. Make API call
    // 4. Handle errors with parseAPIError()
    // 5. Output result (table or JSON)
    return nil
}
```

### Error Handling
- Use `parseAPIError(err, "context message")` for API errors - it extracts user-friendly messages
- Use `parseJSONError(err, content, "context")` for JSON parsing errors
- Always return errors, don't print and continue

### Output Formats
- Default: Table format using `text/tabwriter`
- JSON: Available via `--json` flag for list commands
- Use `json.MarshalIndent` for JSON output and **always handle the error**

### Input Sources
Commands typically support multiple input sources (in priority order):
1. Command-line arguments
2. `--file` flag for JSON file input
3. Stdin (for piped input)
4. Individual flags

### Enterprise Features
Enterprise-only features should check:
```go
if !isEnterpriseServer() {
    return fmt.Errorf("Not supported in OSS Conductor")
}
```

## Code Standards

### Imports
Group imports in this order, separated by blank lines:
1. Standard library
2. Third-party packages
3. Internal packages

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/spf13/cobra"
    
    "github.com/orkes-io/conductor-cli/internal"
)
```

### Stdin Handling
Use `io.ReadAll(os.Stdin)` for cross-platform compatibility (not `os.ReadFile("/dev/stdin")`):
```go
stat, _ := os.Stdin.Stat()
if (stat.Mode() & os.ModeCharDevice) == 0 {
    data, err := io.ReadAll(os.Stdin)
    // ...
}
```

### Security
- Config directory should use `0700` permissions
- Config files should use `0600` permissions
- Never log credentials, even in debug mode
- Don't close `os.Stdin` - it's a global file descriptor

## Flag Standards and Conventions

### Global Flags (defined in root.go)

These flags are available to all commands:

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--server` | | string | Conductor server URL |
| `--auth-key` | | string | API key for authentication |
| `--auth-secret` | | string | API secret for authentication |
| `--auth-token` | | string | Auth token for authentication |
| `--server-type` | | string | Server type: `OSS` or `Enterprise` |
| `--config` | | string | Config file path |
| `--profile` | | string | Use specific profile (loads config-<profile>.yaml) |
| `--verbose` | `-v` | bool | Print verbose logs |
| `--yes` | `-y` | bool | Skip confirmation prompts |

### Standard Flag Patterns

When adding new commands, follow these conventions:

#### Output Control
| Flag | Short | Purpose | Example |
|------|-------|---------|---------|
| `--json` | `-j` | Output as JSON instead of table | `workflow list --json` |

**Important**: Always use `--json` for JSON output, never `--complete` or other variations.

#### Resource Identification
| Flag | Short | Purpose | Example |
|------|-------|---------|---------|
| `--name` | `-n` | Resource name | `schedule create -n my_schedule` |
| `--workflow` | `-w` | Workflow name | `workflow search -w my_workflow` |
| `--workflow-id` | | Workflow execution ID | `task signal --workflow-id abc123` |
| `--version` | | Resource version | `workflow start --version 1` |

#### Input Data
| Flag | Short | Purpose | Example |
|------|-------|---------|---------|
| `--input` | `-i` | JSON input data as string | `workflow start -i '{"key":"value"}'` |
| `--file` | `-f` | JSON input from file | `workflow create -f workflow.json` |
| `--value` | | Simple string value | `secret put mykey --value myvalue` |
| `--output` | | Task output data | `task update-execution --output '{}'` |

#### Filtering and Search
| Flag | Short | Purpose | Example |
|------|-------|---------|---------|
| `--count` | `-c` | Number of results to return | `workflow search -c 50` |
| `--status` | `-s` | Filter by status | `workflow search -s COMPLETED` |
| `--start-time-after` | | Filter by start time | `workflow search --start-time-after "2024-01-01"` |
| `--start-time-before` | | Filter by end time | `workflow search --start-time-before "2024-12-31"` |

#### Execution Control
| Flag | Short | Purpose | Example |
|------|-------|---------|---------|
| `--sync` | | Execute synchronously | `workflow start --sync` |
| `--timeout` | | Timeout in milliseconds | `worker js --timeout 100` |
| `--wait-until` | `-u` | Wait until task completes | `workflow start --sync --wait-until task_ref` |

#### Boolean Modifiers
| Flag | Short | Purpose | Example |
|------|-------|---------|---------|
| `--force` | | Force overwrite | `workflow create --force` |
| `--paused` | `-p` | Create in paused state | `schedule create --paused` |
| `--archive` | `-a` | Archive instead of delete | `workflow delete-execution --archive` |
| `--use-latest` | | Use latest definition | `workflow restart --use-latest` |

### Flag Definition Examples

#### Basic string flag:
```go
cmd.Flags().String("name", "", "Resource name")
```

#### String flag with shorthand:
```go
cmd.Flags().StringP("workflow", "w", "", "Workflow name")
```

#### Boolean flag:
```go
cmd.Flags().Bool("json", false, "Output as JSON")
```

#### Boolean flag with shorthand:
```go
cmd.Flags().BoolP("yes", "y", false, "Skip confirmation")
```

#### Integer flag:
```go
cmd.Flags().Int32("count", 10, "Number of results")
cmd.Flags().Int32P("count", "c", 10, "Number of results")  // with shorthand
```

#### String array (repeatable flag):
```go
cmd.Flags().StringArray("tag", []string{}, "Tag in key:value format (repeatable)")
// Usage: --tag env:prod --tag team:backend
```

#### String slice (comma-separated):
```go
cmd.Flags().StringSlice("cors-allowed-origins", nil, "CORS allowed origins")
// Usage: --cors-allowed-origins origin1,origin2
```

### Marking Required Flags

```go
cmd.Flags().String("name", "", "Resource name (required)")
cmd.MarkFlagRequired("name")
```

### Mutually Exclusive Flags

```go
cmd.Flags().String("input", "", "Input as JSON string")
cmd.Flags().String("file", "", "Input from file")
cmd.MarkFlagsMutuallyExclusive("input", "file")
```

### Reading Flag Values

```go
// String
name, _ := cmd.Flags().GetString("name")

// Bool  
jsonOutput, _ := cmd.Flags().GetBool("json")

// Int32
count, _ := cmd.Flags().GetInt32("count")

// StringArray
tags, _ := cmd.Flags().GetStringArray("tag")

// StringSlice
origins, _ := cmd.Flags().GetStringSlice("cors-allowed-origins")
```

### Flag Naming Rules

1. **Use kebab-case**: `--workflow-id`, not `--workflowId` or `--workflow_id`
2. **Be descriptive**: `--start-time-after`, not `--sta`
3. **Use common shorthand conventions**:
   - `-n` for name
   - `-w` for workflow
   - `-c` for count
   - `-s` for status
   - `-i` for input
   - `-f` for file
   - `-v` for verbose
   - `-y` for yes
   - `-j` for json
   - `-p` for paused/pretty
   - `-a` for archive
4. **Avoid conflicts**: Check existing global flags before adding shorthand

### Status Values

For `--status` flags, valid values are:
- `COMPLETED`
- `FAILED`
- `PAUSED`
- `RUNNING`
- `TERMINATED`
- `TIMED_OUT`

For task status updates:
- `IN_PROGRESS`
- `COMPLETED`
- `FAILED`
- `FAILED_WITH_TERMINAL_ERROR`

## Testing

Run tests:
```bash
go test ./...
```

Build:
```bash
go build -o conductor
```

## Common Tasks

### Adding a New Command

1. Create the command variable with `&cobra.Command{}`
2. Implement the handler function returning `error`
3. Add flags in `init()` following the flag standards above
4. Register with parent command via `AddCommand()`
5. Use `parseAPIError()` for API errors
6. Support both table and JSON output for list commands

### Adding a New API Endpoint

1. Check if the client method exists in `conductor-go` SDK
2. Add factory function to `internal/settings.go` if needed
3. Implement command following existing patterns
4. Handle errors consistently with `parseAPIError()`

## Reference Documentation

- **CLAUDE.md**: Comprehensive CLI reference for AI assistants
- **README.md**: User-facing documentation
- **Conductor Go SDK**: https://github.com/conductor-sdk/conductor-go
