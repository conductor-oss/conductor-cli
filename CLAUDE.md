# Orkes CLI - AI Assistant Reference

> Optimized reference for LLMs and AI assistants using the Orkes Conductor CLI.

## Quick Overview

Orkes CLI (`orkes`) is a command-line tool for managing Netflix Conductor workflows, executions, tasks, webhooks, and schedules. It connects to Conductor server instances for workflow orchestration.

## Installation

```bash
# Homebrew (macOS/Linux)
brew install conductor-oss/conductor-tools/orkes

# Manual download from: https://github.com/conductor-oss/conductor-cli/releases
```

## Authentication

**Three methods** (precedence: command-line flags > environment variables > config file):

| Method | Command-line Flags | Environment Variables |
|--------|-------------------|----------------------|
| **Auth Token** (recommended) | `--auth-token <token>` | `CONDUCTOR_AUTH_TOKEN` |
| **API Key + Secret** | `--auth-key <key> --auth-secret <secret>` | `CONDUCTOR_AUTH_KEY`, `CONDUCTOR_AUTH_SECRET` |
| **Config File** | `--config <path>` | N/A |

**Server URL:** `--server <url>` or `CONDUCTOR_SERVER_URL` (default: `http://localhost:8080/api`)

## Profile Management

Manage multiple environments (dev, staging, prod) using profiles.

| Operation | Command | Result |
|-----------|---------|--------|
| **Save default profile** | `orkes --server <url> --auth-token <token> --save-config workflow list` | Creates `~/.conductor-cli/config.yaml` |
| **Save named profile** | `orkes --server <url> --auth-token <token> --save-config=prod workflow list` | Creates `~/.conductor-cli/config-prod.yaml` |
| **Use profile (flag)** | `orkes --profile prod workflow list` | Loads `config-prod.yaml` |
| **Use profile (env)** | `ORKES_PROFILE=prod orkes workflow list` | Loads `config-prod.yaml` |

**Precedence:** `--profile` flag > `ORKES_PROFILE` env var > default config

**Profile directory:** `~/.conductor-cli/`
- `config.yaml` - default profile
- `config-<name>.yaml` - named profiles

## Command Reference

### Workflow Commands

| Command | Description | Required Args | Example |
|---------|-------------|---------------|---------|
| `workflow list` | List all workflows | None | `orkes workflow list` |
| `workflow get <name>` | Get workflow definition | workflow name | `orkes workflow get my_workflow` |
| `workflow get <name> <version>` | Get specific version | name, version | `orkes workflow get my_workflow 2` |
| `workflow create <file>` | Create/register workflow | JSON file path | `orkes workflow create workflow.json --force` |
| `workflow update <file>` | Update workflow | JSON file path | `orkes workflow update workflow.json` |
| `workflow delete <name> <version>` | Delete workflow | name, version | `orkes workflow delete my_workflow 1` |

**Flag:** `--force` - Overwrite existing workflow when creating

### Execution Commands

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| `execution start --workflow <name>` | Start workflow | workflow name | `--input`, `--file`, `--version` | `orkes execution start --workflow my_workflow` |
| `execution start` with input | Start with input data | workflow name, input JSON | | `orkes execution start --workflow my_workflow --input '{"key":"value"}'` |
| `execution start` with file | Start with input file | workflow name, file path | | `orkes execution start --workflow my_workflow --file input.json` |
| `execution status <id>` | Get execution status | workflow ID | | `orkes execution status abc-123` |
| `execution get <id>` | Get full execution details | workflow ID | | `orkes execution get abc-123` |
| `execution terminate <id>` | Terminate execution | workflow ID | | `orkes execution terminate abc-123` |
| `execution pause <id>` | Pause execution | workflow ID | | `orkes execution pause abc-123` |
| `execution resume <id>` | Resume paused execution | workflow ID | | `orkes execution resume abc-123` |
| `execution restart <id>` | Restart completed workflow | workflow ID | | `orkes execution restart abc-123` |
| `execution rerun <id>` | Rerun from failed task | workflow ID | | `orkes execution rerun abc-123` |
| `execution retry <id>` | Retry last failed task | workflow ID | | `orkes execution retry abc-123` |
| `execution delete <id>` | Delete execution | workflow ID | | `orkes execution delete abc-123` |
| `execution search` | Search executions | None | `--workflow`, `--status`, `--count`, `--start-time-after`, `--start-time-before` | `orkes execution search --workflow my_workflow --status FAILED` |

**Status values:** `RUNNING`, `COMPLETED`, `FAILED`, `TERMINATED`, `TIMED_OUT`, `PAUSED`

### Task Commands

| Command | Description | Required Args | Example |
|---------|-------------|---------------|---------|
| `execution task-poll <type>` | Poll for tasks | task type | `orkes execution task-poll my_task --count 5` |
| `execution task-update` | Update task by ref name | workflow-id, task-ref-name, status | `orkes execution task-update --workflow-id abc --task-ref-name task1 --status COMPLETED` |

### Webhook Commands

| Command | Description | Required Args | Example |
|---------|-------------|---------------|---------|
| `webhook list` | List webhooks | None | `orkes webhook list` |
| `webhook get <id>` | Get webhook details | webhook ID | `orkes webhook get webhook-id` |
| `webhook create` | Create webhook | name, source-platform, verifier | `orkes webhook create --name hook1 --source-platform Custom --verifier HEADER_BASED` |
| `webhook update <id>` | Update webhook | webhook ID, file | `orkes webhook update id --file webhook.json` |
| `webhook delete <id>` | Delete webhook | webhook ID | `orkes webhook delete webhook-id` |

### Schedule Commands

| Command | Description | Required Args | Example |
|---------|-------------|---------------|---------|
| `schedule list` | List schedules | None | `orkes schedule list` |
| `schedule get <name>` | Get schedule details | schedule name | `orkes schedule get my_schedule` |
| `schedule create <file>` | Create schedule | JSON file | `orkes schedule create schedule.json` |
| `schedule delete <name>` | Delete schedule | schedule name | `orkes schedule delete my_schedule` |
| `schedule pause <name>` | Pause schedule | schedule name | `orkes schedule pause my_schedule` |
| `schedule resume <name>` | Resume schedule | schedule name | `orkes schedule resume my_schedule` |

### Other Commands

| Command | Description | Example |
|---------|-------------|---------|
| `update` | Update CLI to latest version | `orkes update` |
| `--version` | Show CLI version | `orkes --version` |
| `--help` | Show help | `orkes --help` or `orkes workflow --help` |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error (connection failed, authentication failed, not found, etc.) |

## Output Format

- **Default:** Human-readable text
- **Workflow ID extraction:** UUIDs in format `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` (36 characters with hyphens)
- **Status output:** Single line with status value (e.g., `RUNNING`, `COMPLETED`)
- **List output:** One item per line (workflow names, IDs, etc.)
- **JSON output:** Not currently available (use text parsing)

**Important:** To parse output reliably, redirect stderr to `/dev/null` to suppress update notifications and warnings:
```bash
orkes workflow list 2>/dev/null
WORKFLOW_ID=$(orkes execution start --workflow my_workflow 2>/dev/null | grep -oE '[a-f0-9-]{36}')
```

## Input Format

### Workflow Input Data

Workflows can accept input data in two ways:

**1. Inline JSON (--input flag):**
```bash
orkes execution start --workflow my_workflow --input '{"key":"value","count":42}'
```

**2. JSON File (--file flag):**
```bash
# input.json
{
  "orderId": "12345",
  "customerId": "cust_001",
  "items": [
    {"sku": "ITEM-001", "quantity": 2}
  ]
}

# Start with file
orkes execution start --workflow my_workflow --file input.json
```

### Workflow Definition Format

Workflow definitions are JSON files with structure:

```json
{
  "name": "my_workflow",
  "version": 1,
  "tasks": [
    {
      "name": "task_1",
      "taskReferenceName": "task_1_ref",
      "type": "SIMPLE",
      "inputParameters": {}
    }
  ]
}
```

See [Conductor documentation](https://orkes.io/content) for complete workflow definition schema.

## Common Patterns

### 1. Deploy workflow to production

```bash
# Save production profile
orkes --server https://prod.conductor.io/api \
     --auth-token prod-token-123 \
     --save-config=production \
     workflow list

# Deploy workflow
orkes --profile production workflow create workflow.json --force
```

### 2. Start and monitor execution

```bash
# Start workflow and capture ID
WORKFLOW_ID=$(orkes execution start --workflow my_workflow 2>/dev/null | grep -oE '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}')

# Start with input data
WORKFLOW_ID=$(orkes execution start --workflow my_workflow --input '{"orderId":"12345","customerId":"cust_001"}' 2>/dev/null | grep -oE '[a-f0-9-]{36}')

# Start with input from file
WORKFLOW_ID=$(orkes execution start --workflow my_workflow --file input.json 2>/dev/null | grep -oE '[a-f0-9-]{36}')

# Check status
orkes execution status "$WORKFLOW_ID"

# Get full details
orkes execution get "$WORKFLOW_ID"
```

### 3. Multi-environment workflow

```bash
# Deploy to dev
ORKES_PROFILE=dev orkes workflow create workflow.json --force

# Test in dev
ORKES_PROFILE=dev orkes execution start --workflow my_workflow

# Deploy to prod after testing
ORKES_PROFILE=prod orkes workflow create workflow.json --force
```

### 4. Handle workflow failure

```bash
# Check status
STATUS=$(orkes execution status "$WORKFLOW_ID" 2>/dev/null)

if [ "$STATUS" = "FAILED" ]; then
  # Retry failed task
  orkes execution retry "$WORKFLOW_ID"

  # Or rerun from failed point
  orkes execution rerun "$WORKFLOW_ID"
fi
```

### 5. Terminate stuck workflows

```bash
# Find running workflows
orkes execution search --workflow my_workflow --status RUNNING

# Terminate specific execution
orkes execution terminate "$WORKFLOW_ID"
```

### 6. Create and test webhook

```bash
# Create webhook
orkes webhook create \
  --name my_webhook \
  --source-platform Custom \
  --verifier HEADER_BASED \
  --headers "Authorization:secret123" \
  --receiver-workflows my_workflow:1

# List webhooks to verify
orkes webhook list
```

### 7. Manage workflow versions

```bash
# Get latest version
orkes workflow get my_workflow

# Get specific version
orkes workflow get my_workflow 2

# Delete old version
orkes workflow delete my_workflow 1
```

### 8. Poll and process tasks

```bash
# Poll for tasks
orkes execution task-poll my_task_type --count 10 --worker-id worker1

# Update task status
orkes execution task-update \
  --workflow-id "$WORKFLOW_ID" \
  --task-ref-name my_task \
  --status COMPLETED \
  --output '{"result":"success"}'
```

### 9. Search for executions

```bash
# Find failed executions for a workflow
orkes execution search --workflow my_workflow --status FAILED --count 50

# Find executions within time range
orkes execution search --workflow my_workflow \
  --start-time-after "2025-01-01" \
  --start-time-before "2025-01-31"

# Combine filters
orkes execution search --workflow my_workflow \
  --status RUNNING \
  --start-time-after "2025-01-01 10:00:00" \
  --count 100
```

**Search flags:**
- `--workflow <name>` - Filter by workflow name
- `--status <status>` - Filter by status (COMPLETED, FAILED, RUNNING, PAUSED, TERMINATED, TIMED_OUT)
- `--count <n>` - Number of results (max 1000, default 10)
- `--start-time-after <time>` - Started after time (formats: YYYY-MM-DD HH:MM:SS, YYYY-MM-DD, or epoch milliseconds)
- `--start-time-before <time>` - Started before time (same formats)

## Error Handling

### Connection Errors
```
Error: Get "https://...": dial tcp: lookup ...: no such host
```
**Solution:** Verify `--server` URL or `CONDUCTOR_SERVER_URL`

### Authentication Errors
```
Error: 401 Unauthorized
```
**Solution:** Check authentication credentials (token, key/secret)

### Not Found Errors
```
Error: 404 Not Found
```
**Solution:** Verify resource name/ID exists

### Profile Errors
```
Error: Profile 'prod' doesn't exist (expected file: ~/.conductor-cli/config-prod.yaml)
```
**Solution:** Create profile with `--save-config=prod` or check profile name

## Configuration File Format

Location: `~/.conductor-cli/config.yaml` or `~/.conductor-cli/config-<profile>.yaml`

```yaml
server: https://conductor.example.com/api
auth-token: your-token-here
# OR
auth-key: your-key
auth-secret: your-secret
```

**File permissions:** Config files are saved with `0600` (read/write for owner only) for security.

## Best Practices for LLM Usage

1. **Always redirect stderr** when parsing output: `orkes command 2>/dev/null`
2. **Extract workflow IDs** using: `grep -oE '[a-f0-9-]{36}'`
3. **Check exit codes** for error handling: `if [ $? -eq 0 ]; then ...`
4. **Use profiles** for multi-environment operations
5. **Quote workflow names** with spaces: `orkes workflow get "my workflow"`
6. **Use --force flag** when updating workflows to overwrite
7. **Save profiles once** then use `ORKES_PROFILE` env var for cleaner commands

## Auto-Update Feature

The CLI checks for updates every 24 hours and notifies when a new version is available:

```
⚠ A new version is available: v0.0.12 (current: v0.0.11)
Run 'orkes update' to download it or update with your package manager.
```

Update to latest version:
```bash
orkes update
```

**Note:** Update notifications are written to stderr and won't interfere with command output.

## Full Documentation

For detailed human-readable documentation, see [README.md](./README.md)
