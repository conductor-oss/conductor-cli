# Conductor CLI - AI Assistant Reference

> Optimized reference for LLMs and AI assistants using the Conductor CLI.

## Quick Overview

Conductor CLI (`conductor`) is a command-line tool for managing Netflix Conductor workflows, executions, tasks, webhooks, and schedules. It connects to Conductor server instances for workflow orchestration.

It also runs a local Conductor server for development (`conductor server start`), runs task workers
(`conductor worker`), and manages AI agents (`conductor agent`, `conductor deploy`).

## Installation

```bash
# Homebrew (macOS/Linux)
brew install conductor-oss/conductor/conductor

# Manual download from: https://github.com/conductor-oss/conductor-cli/releases
```

**Never suggest a bare `brew install conductor`.** An unrelated `conductor` cask exists in
`homebrew/cask` (Conductor.app from conductor.build), and Homebrew resolves unqualified names
against core/cask before third-party taps — so the bare form installs the wrong package and
leaves no `conductor` binary on `PATH`. Tapping first does not help. Always use the
fully-qualified `conductor-oss/conductor/conductor`.

**If the cask is already installed**, installing the formula is not enough: Homebrew prints
`conductor cask is installed, skipping link` and creates no symlink, so `conductor --version` still
fails. Fix with `brew link conductor` (keeps both) or `brew uninstall --cask conductor` before
installing (CLI only). Diagnose with `which conductor` and `brew info --cask conductor`.

## Authentication

**Three methods.** Command-line flags override individual settings. Below them, credentials come
from a single source — the environment or a config file, never both at once (see
[Profile Management](#profile-management)):

| Method | Command-line Flags | Environment Variables |
|--------|-------------------|----------------------|
| **Auth Token** (recommended) | `--auth-token <token>` | `CONDUCTOR_AUTH_TOKEN` |
| **API Key + Secret** | `--auth-key <key> --auth-secret <secret>` | `CONDUCTOR_AUTH_KEY`, `CONDUCTOR_AUTH_SECRET` |
| **Config File** | `--config <path>` | N/A |

**Server URL:** `--server <url>` or `CONDUCTOR_SERVER_URL` (default: `http://localhost:8080/api`)

**Server type:** `--server-type <OSS|Enterprise>` or `CONDUCTOR_SERVER_TYPE` (default: `OSS`)

**Note:** OSS Conductor accepts anonymous requests, so authentication is optional when talking to a local server.

**Global flags** (available on every command):

| Flag | Description |
|------|-------------|
| `--server`, `--server-type` | Target server URL and type |
| `--auth-token`, `--auth-key`, `--auth-secret` | Authentication credentials |
| `--config <path>` | Config file path (overrides profile-based loading) |
| `--profile <name>` | Load `config-<name>.yaml` |
| `--verbose`, `-v` | Print verbose logs |
| `--yes`, `-y` | Auto-confirm destructive operations |

**Token Types:**
- **JWT tokens with `exp` claim**: Automatically cached and refreshed before expiry (5-minute buffer)
- **Long-lived tokens without `exp` claim**: Cached indefinitely, never trigger refresh attempts
- **Expired tokens**: CLI validates token expiry and provides helpful error messages with guidance to run `conductor config save`

## Profile Management

Manage multiple environments (dev, staging, prod) using profiles.

| Operation | Command | Result |
|-----------|---------|--------|
| **Save named profile** | `conductor config save --profile prod` | Creates `~/.conductor-cli/config-prod.yaml` |
| **Save default** | `conductor config save` | Prompts `Profile name (empty for default):`; Enter creates `config.yaml` |
| **Save default (no prompt)** | `conductor config save --profile default` | Creates `config.yaml` |
| **Use profile (flag)** | `conductor --profile prod workflow list` | Loads `config-prod.yaml` |
| **Use profile (env)** | `CONDUCTOR_PROFILE=prod conductor workflow list` | Loads `config-prod.yaml` |
| **Inspect what is active** | `conductor config show` | Prints each value and where it came from |

**Precedence:** `--profile` flag > `CONDUCTOR_PROFILE` env var > default config

**Profile directory:** `~/.conductor-cli/`
- `config.yaml` - default configuration
- `config-<name>.yaml` - named profiles

**`default` is an alias for the default configuration, not a profile of its own.** An empty
profile name and `--profile default` both mean `~/.conductor-cli/config.yaml`, for `save`,
`delete` and profile selection alike. The CLI never creates `config-default.yaml`; if one exists
from an older build it is ignored, and `config list`/`config show` warn that it is unused.

**Precedence.** Rank 1 wins over rank 2, rank 2 over rank 3, and so on.

| Rank | Source | Example |
|------|--------|---------|
| 1 | Command-line value flags | `--server`, `--auth-token` |
| 2 | File chosen by `--config` or `--profile` | `--profile prod` reads `config-prod.yaml` |
| 3 | Environment variables | `CONDUCTOR_SERVER_URL` |
| 4 | Default config file | `~/.conductor-cli/config.yaml` |
| 5 | Built-in defaults | `server-type: OSS` |

Ranks 2-4 are winner-takes-all: only the highest one present is used, and it supplies *every*
setting. The ranks below it are not read. Rank 1 is different — it applies per setting, so
`--server` overrides only the server URL.

`--config` and `--profile` are not rank 1. They carry no settings; they only choose the file in
rank 2.

Setting one variable therefore switches the whole configuration onto the environment: if
`CONDUCTOR_SERVER_URL` is set and `config.yaml` holds an auth token, that token is *not* used.
`CONDUCTOR_PROFILE` does not count — it selects a file rather than carrying a setting.

Run `conductor config show` to see the active source and where each value came from.

## Command Reference

Commands are organized into three help groups:
- **Conductor Management** — `workflow`, `task`, `schedule`, `webhook`, `secret`, `api-gateway`, `agent`
- **CLI Configuration** — `config`, `whoami`, `update`, `completion`
- **Development** — `server`, `deploy`, `doctor`, `worker`

### Server Commands

Run a local single-node Conductor server for development and testing. The server JAR is
downloaded automatically on first run (~600 MB) into `~/.conductor-cli/server/` and runs as a
background process on port 8080.

**Requirement:** Java 21 or higher on `PATH`.

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| `server start` | Start a local Conductor server | None | `--port`, `--foreground`/`-f`, `--version`, `--oss`, `--orkes` | `conductor server start --port 9090` |
| `server stop` | Stop the running server | None | | `conductor server stop` |
| `server status` | Check whether the server is running | None | | `conductor server status` |
| `server logs` | Show server logs | None | `--follow`/`-f`, `--lines`/`-n` | `conductor server logs -f -n 200` |
| `server update` | Re-download the server JAR | None | `--version`, `--oss`, `--orkes` | `conductor server update` |

**Flags:**
- `--port` - Port to run the server on (default: 8080)
- `--foreground`, `-f` - Run in the foreground instead of daemonizing
- `--version` - Server version to download and run (default: `latest`, e.g. `3.21.23`)
- `--oss` - Use the open-source Conductor server (default)
- `--orkes` - Use the Orkes Conductor server (coming soon)
- `--follow`, `-f` - Follow log output like `tail -f` (logs command)
- `--lines`, `-n` - Number of lines to show (logs command, default: 50)

**Notes:**
- This is a single-node dev server, not a cluster.
- The server must be stopped before running `server update`.
- Docker is a faster alternative: `docker run -p 8080:8080 conductoross/conductor:latest`

### Workflow Commands

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| **Definition Management** | | | | |
| `workflow list` | List all workflows | None | `--json`, `--csv` | `conductor workflow list` |
| `workflow get <name>` | Get workflow definition | workflow name | | `conductor workflow get my_workflow` |
| `workflow get <name> <version>` | Get specific version | name, version | | `conductor workflow get my_workflow 2` |
| `workflow get-all` | Get all workflow definitions | None | | `conductor workflow get-all` |
| `workflow create <file>` | Create/register workflow | JSON file path | `--force` | `conductor workflow create workflow.json --force` |
| `workflow update <file>` | Update workflow | JSON file path | | `conductor workflow update workflow.json` |
| `workflow delete <name> <version>` | Delete workflow definition | name, version | | `conductor workflow delete my_workflow 1` |
| **Execution Management** | | | | |
| `workflow start --workflow <name>` | Start workflow async | None | `--input`, `--file`, `--version`, `--correlation`, `--sync` | `conductor workflow start --workflow my_workflow` |
| `workflow start --sync` | Start and wait for completion | None | `--workflow`, `--input`, `--file`, `--wait-until` | `conductor workflow start --workflow my_workflow --sync` |
| `workflow status <id>` | Get execution status | workflow ID | | `conductor workflow status abc-123` |
| `workflow get-execution <id>` | Get full execution details | workflow ID | `--complete` | `conductor workflow get-execution abc-123` |
| `workflow search` | Search executions | None | `--workflow`, `--status`, `--count`, `--start-time-after`, `--start-time-before`, `--json` | `conductor workflow search --workflow my_workflow --status FAILED` |
| `workflow terminate <id>` | Terminate execution | workflow ID | | `conductor workflow terminate abc-123` |
| `workflow pause <id>` | Pause execution | workflow ID | | `conductor workflow pause abc-123` |
| `workflow resume <id>` | Resume paused execution | workflow ID | | `conductor workflow resume abc-123` |
| `workflow delete-execution <id>` | Delete execution | workflow ID | `--archive` | `conductor workflow delete-execution abc-123` |
| `workflow restart <id>` | Restart completed workflow | workflow ID | `--use-latest` | `conductor workflow restart abc-123` |
| `workflow retry <id>` | Retry last failed task | workflow ID | `--resume-subworkflow-tasks` | `conductor workflow retry abc-123` |
| `workflow rerun <id>` | Rerun from failed task | workflow ID | `--task-id`, `--correlation-id`, `--task-input`, `--workflow-input` | `conductor workflow rerun abc-123` |
| `workflow skip-task <id> <ref>` | Skip a task | workflow ID, task ref | `--task-input`, `--task-output` | `conductor workflow skip-task abc-123 task1` |
| `workflow jump <id> <ref>` | Jump to task | workflow ID, task ref | `--task-input` | `conductor workflow jump abc-123 task2` |
| `workflow update-state <id>` | Update workflow state | workflow ID | `--request-id`, `--wait-until-task-ref`, `--variables`, `--task-updates` | `conductor workflow update-state abc-123 --variables '{"key":"value"}'` |

**Flags:**
- `--force` - Overwrite existing workflow when creating
- `--json` - Output complete JSON instead of table (applies to list command)
- `--csv` - Output CSV instead of table (mutually exclusive with `--json`)
- `--sync` - Execute synchronously and wait for completion (for start command)
- `--complete` - Include complete details (for get-execution command)

**Alias:** `get-all` also accepts the legacy form `get_all`.

**Table Output (workflow list):**
Columns: NAME, VERSION, DESCRIPTION

**Status values:** `RUNNING`, `COMPLETED`, `FAILED`, `TERMINATED`, `TIMED_OUT`, `PAUSED`

### Task Commands

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| **Definition Management** | | | | |
| `task list` | List all task definitions | None | `--json` | `conductor task list` |
| `task get <task_type>` | Get task definition | task type | | `conductor task get my_task` |
| `task create <file>` | Create task definition | JSON file | | `conductor task create task.json` |
| `task update <file>` | Update task definition | JSON file | | `conductor task update task.json` |
| `task delete <task_type>` | Delete task definition | task type | | `conductor task delete my_task` |
| **Execution Management** | | | | |
| `task poll <type>` | Batch poll for tasks | task type | `--count`, `--worker-id`, `--domain`, `--timeout` | `conductor task poll my_task --count 5` |
| `task update-execution` | Update task by ref name | None | `--workflow-id`, `--task-ref-name`, `--status`, `--output`, `--worker-id` | `conductor task update-execution --workflow-id abc --task-ref-name task1 --status COMPLETED` |
| `task signal` | Signal task async | None | `--workflow-id`, `--status`, `--output` | `conductor task signal --workflow-id abc --status COMPLETED` |
| `task signal-sync` | Signal task sync | None | `--workflow-id`, `--status`, `--output` | `conductor task signal-sync --workflow-id abc --status COMPLETED` |

**Flags:**
- `--json` - Output complete JSON instead of table (applies to list command)

**Table Output (task list):**
Columns: NAME, EXECUTABLE, DESCRIPTION, OWNER, TIMEOUT POLICY, TIMEOUT (s), RETRY COUNT, RESPONSE TIMEOUT (s)

### Config Commands

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| `config save` | Interactively save configuration | None | `--profile` | `conductor config save` or `conductor config save --profile production` |
| `config list` | List all configuration profiles | None | None | `conductor config list` |
| `config show` | Show the effective config and each value's source | None | `--json`, `--show-secrets` | `conductor config show` |
| `config delete [profile]` | Delete configuration file | None | `--profile`, `-y` | `conductor config delete production` or `conductor config delete --profile production -y` |

**Notes:**
- `config save`: Interactive prompts for server URL, server type, and authentication method. Press Enter to keep existing values. `--profile <name>` writes `config-<name>.yaml`; an empty name at the prompt, or `--profile default`, writes the default `config.yaml`.
- `config list`: Shows all profiles. Default config shown as "default", named profiles show as profile name only.
- `config show`: Prints `KEY`, `VALUE` and `SOURCE` for every setting, where source is the flag, the environment variable, the config file, or `default`. Secrets are masked unless `--show-secrets` is passed.
- `config delete`: Profile can be specified as positional arg or via `--profile` flag. Use `default` to delete the default `config.yaml`. Use `-y` to skip confirmation prompt.

**Table Output (config show):**
Columns: KEY, VALUE, SOURCE

### Webhook Commands

> **Note:** Webhook commands are only available with Orkes Conductor (Enterprise).

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| `webhook list` | List webhooks | None | `--json` | `conductor webhook list` |
| `webhook get <id>` | Get webhook details | webhook ID | | `conductor webhook get webhook-id` |
| `webhook create` | Create webhook | name, source-platform, verifier | | `conductor webhook create --name hook1 --source-platform Custom --verifier HEADER_BASED` |
| `webhook update <id>` | Update webhook | webhook ID, file | | `conductor webhook update id --file webhook.json` |
| `webhook delete <id>` | Delete webhook | webhook ID | | `conductor webhook delete webhook-id` |

**Flags:**
- `--json` - Output complete JSON instead of table (applies to list command)

**Table Output (webhook list):**
Columns: NAME, WEBHOOK ID, WORKFLOWS, URL

### Schedule Commands

> **Note:** Schedule commands work against both OSS Conductor and Orkes Conductor. The OSS server must include the `scheduler` module — the default jar from `conductor-oss/conductor` (used by `conductor server start`) ships it. Custom OSS builds that omit the module will return 404 with a hint message.

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| `schedule list` | List schedules | None | `--json` | `conductor schedule list` |
| `schedule get <name>` | Get schedule details | schedule name | | `conductor schedule get my_schedule` |
| `schedule create <file>` | Create schedule | JSON file | | `conductor schedule create schedule.json` |
| `schedule delete <name>` | Delete schedule | schedule name | | `conductor schedule delete my_schedule` |
| `schedule pause <name>` | Pause schedule | schedule name | | `conductor schedule pause my_schedule` |
| `schedule resume <name>` | Resume schedule | schedule name | | `conductor schedule resume my_schedule` |

**Flags:**
- `--json` - Output complete JSON instead of table (applies to list command)

**Table Output (schedule list):**
Columns: NAME, WORKFLOW, STATUS, CREATED TIME

### Secret Commands

> **Note:** Secret commands are only available with Orkes Conductor (Enterprise).

Secret management for storing and managing sensitive configuration values like API keys, passwords, and tokens.

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| **Secret Management** | | | | |
| `secret list` | List all secrets | None | `--with-tags`, `--json` | `conductor secret list` |
| `secret get <key>` | Get secret value | secret key | `--show-value` | `conductor secret get db_password` |
| `secret put <key> [value]` | Create/update secret | secret key | `--value` | `conductor secret put db_password mySecret` |
| `secret delete <key>` | Delete secret | secret key | | `conductor secret delete db_password` |
| `secret exists <key>` | Check if secret exists | secret key | | `conductor secret exists db_password` |
| **Tag Management** | | | | |
| `secret tag-list <key>` | List tags for secret | secret key | `--json` | `conductor secret tag-list db_password` |
| `secret tag-add <key>` | Add tags to secret | secret key | `--tag` (repeatable) | `conductor secret tag-add db_password --tag env:prod` |
| `secret tag-delete <key>` | Delete tags from secret | secret key | `--tag` (repeatable) | `conductor secret tag-delete db_password --tag env:prod` |
| **Cache Management** | | | | |
| `secret cache-clear` | Clear secrets cache | None | `--local`, `--redis` | `conductor secret cache-clear --local` |

**Flags:**
- `--with-tags` - Include tags in list output (applies to list command)
- `--json` - Output complete JSON instead of table (applies to list and tag-list commands)
- `--show-value` - Display actual secret value (applies to get command, otherwise shows "Secret exists" message)
- `--value` - Provide secret value via flag instead of argument (applies to put command)
- `--tag` - Tag in key:value format, repeatable (applies to tag-add and tag-delete commands)
- `--local` - Clear local cache only (applies to cache-clear command)
- `--redis` - Clear Redis cache only (applies to cache-clear command)
- If neither `--local` nor `--redis` is specified for cache-clear, both caches are cleared

**Table Output (secret list):**
- Default: Column: KEY
- With `--with-tags`: Columns: KEY, TAGS

**Table Output (secret tag-list):**
Columns: KEY, VALUE, TYPE

**Security Notes:**
- Secret values are NOT displayed by default in `get` command for security
- Use `--show-value` flag explicitly to display secret values
- Delete operations require confirmation unless `--yes` flag is used

**Input Methods (secret put):**
```bash
# Method 1: Value as argument
conductor secret put my_secret "secret_value"

# Method 2: Value via flag
conductor secret put my_secret --value "secret_value"

# Method 3: Value from stdin
echo "secret_value" | conductor secret put my_secret

# Method 4: Value from file
cat secret.txt | conductor secret put my_secret
```

### API Gateway Commands

> **Note:** API Gateway commands are only available with Orkes Conductor (Enterprise).

API Gateway allows exposing Conductor workflows as REST APIs with authentication, CORS configuration, and route management.

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| **Service Management** | | | | |
| `api-gateway service list` | List all services | None | `--complete` | `conductor api-gateway service list` |
| `api-gateway service get <id>` | Get service details | service ID | | `conductor api-gateway service get my-service` |
| `api-gateway service create [file]` | Create service | None (file optional) | `--service-id`, `--name`, `--path`, `--description`, `--enabled`, `--mcp-enabled`, `--auth-config-id`, `--cors-allowed-origins`, `--cors-allowed-methods`, `--cors-allowed-headers` | `conductor api-gateway service create service.json` |
| `api-gateway service update <id> <file>` | Update service | service ID, JSON file | | `conductor api-gateway service update my-service service.json` |
| `api-gateway service delete <id>` | Delete service | service ID | | `conductor api-gateway service delete my-service` |
| **Auth Configuration Management** | | | | |
| `api-gateway auth list` | List auth configs | None | `--complete` | `conductor api-gateway auth list` |
| `api-gateway auth get <id>` | Get auth config | auth config ID | | `conductor api-gateway auth get token-based` |
| `api-gateway auth create [file]` | Create auth config | None (file optional) | `--auth-config-id`, `--auth-type`, `--application-id`, `--api-keys` | `conductor api-gateway auth create auth.json` |
| `api-gateway auth update <id> <file>` | Update auth config | auth config ID, JSON file | | `conductor api-gateway auth update token-based auth.json` |
| `api-gateway auth delete <id>` | Delete auth config | auth config ID | | `conductor api-gateway auth delete token-based` |
| **Route Management** | | | | |
| `api-gateway route list <service_id>` | List routes for service | service ID | `--complete` | `conductor api-gateway route list my-service` |
| `api-gateway route create <service_id> [file]` | Create route | service ID (file optional) | `--http-method`, `--path`, `--workflow-name`, `--workflow-version`, `--execution-mode`, `--description`, `--request-metadata-as-input`, `--workflow-metadata-in-output`, `--wait-until-tasks` | `conductor api-gateway route create my-service route.json` |
| `api-gateway route update <service_id> <path> <file>` | Update route | service ID, route path, JSON file | | `conductor api-gateway route update my-service /users route.json` |
| `api-gateway route delete <service_id> <method> <path>` | Delete route | service ID, HTTP method, route path | | `conductor api-gateway route delete my-service GET /users` |

**Service Create Flags:**
- `--service-id` - Service ID (required when not using file)
- `--name` - Display name of the service
- `--path` - Base path for the service (required when not using file)
- `--description` - Description of the service
- `--enabled` - Enable the service (default: true)
- `--mcp-enabled` - Enable MCP for the service (default: false)
- `--auth-config-id` - Authentication configuration ID
- `--cors-allowed-origins` - CORS allowed origins (repeatable, comma-separated)
- `--cors-allowed-methods` - CORS allowed methods (repeatable, comma-separated)
- `--cors-allowed-headers` - CORS allowed headers (repeatable, comma-separated)

**Auth Config Create Flags:**
- `--auth-config-id` - Authentication configuration ID (required when not using file)
- `--auth-type` - Authentication type: API_KEY or NONE (required when not using file)
- `--application-id` - Application ID
- `--api-keys` - API keys (repeatable, comma-separated)

**Route Create Flags:**
- `--http-method` - HTTP method: GET, POST, PUT, DELETE, etc. (required when not using file)
- `--path` - Route path (required when not using file)
- `--workflow-name` - Workflow name to map to this route (required when not using file)
- `--workflow-version` - Workflow version (optional, uses latest if not specified)
- `--execution-mode` - Workflow execution mode: SYNC or ASYNC (default: SYNC). Note: When using JSON files, use full enum values: SYNCHRONOUS or ASYNCHRONOUS
- `--description` - Route description
- `--request-metadata-as-input` - Pass request metadata as workflow input
- `--workflow-metadata-in-output` - Include workflow metadata in output
- `--wait-until-tasks` - Comma-separated task reference names to wait for

**Table Output (service list):**
Columns: ID, NAME, PATH, ENABLED, AUTH CONFIG, ROUTES

**Table Output (auth list):**
Columns: ID, AUTH TYPE, APPLICATION ID, API KEYS

**Table Output (route list):**
Columns: METHOD, PATH, WORKFLOW, VERSION, EXECUTION MODE, DESCRIPTION

### Agent Commands

Define, run, and observe AI agents. Agent configs are YAML or JSON files.

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| **Definition Management** | | | | |
| `agent init <name>` | Create a starter agent config file | agent name | `--model`, `--strategy`/`-s`, `--format`/`-f` | `conductor agent init triage -s handoff` |
| `agent compile <config-file>` | Compile a config and show its execution plan | config file | | `conductor agent compile triage.yaml` |
| `agent list` | List registered agents | None | `--json`, `--csv` | `conductor agent list` |
| `agent get <name>` | Get an agent definition | agent name | `--version` | `conductor agent get triage --version 2` |
| `agent delete <name>` | Delete an agent definition | agent name | `--version` | `conductor agent delete triage` |
| **Execution** | | | | |
| `agent run [prompt]` | Start an agent and stream its output | prompt | `--name`, `--config`, `--session`, `--no-stream` | `conductor agent run --name triage "check order 123"` |
| `agent stream <execution-id>` | Stream events from a running execution | execution ID | `--last-event-id` | `conductor agent stream exec-abc` |
| `agent status <execution-id>` | Get detailed status of an execution | execution ID | | `conductor agent status exec-abc` |
| `agent execution` | Search agent execution history | None | `--name`, `--status`, `--since`, `--window` | `conductor agent execution --status FAILED --since 1d` |
| `agent respond <execution-id>` | Respond to a human-in-the-loop task | execution ID | `--approve`, `--deny`, `--reason`, `--message`/`-m` | `conductor agent respond exec-abc --approve` |
| `agent prune` | Delete or archive old execution records | None | `--older-than`, `--archive`, `--dry-run` | `conductor agent prune --older-than 30 --dry-run` |

**Flags:**
- `--name` / `--config` - `agent run` requires exactly one: a registered agent name, or a local config file path. **Note:** on `agent run`, `--config` means the *agent* config file and shadows the global `--config` (CLI config path); use `--profile` to select CLI configuration here.
- `--session` - Session ID for conversation continuity across runs
- `--no-stream` - Start the agent and print the execution ID without streaming
- `--strategy`, `-s` - Multi-agent strategy for `init` (`handoff`, `sequential`, `parallel`, ...)
- `--format`, `-f` - Output format for `init`: `yaml` (default) or `json`
- `--since` - Relative time window, e.g. `30m`, `1h`, `1d`
- `--window` - Absolute-style window, e.g. `now-1h`, `now-7d`
- `--older-than` - Delete executions older than N days (prune command)

**Streamed event types:** `thinking`, `tool`, `result`, `handoff`, `message`, `waiting`, `guardrail` (PASS/FAIL), `error`, `done`

**Table Output (agent list):**
Columns: NAME, VERSION, TYPE, DESCRIPTION

**Table Output (agent execution):**
Columns: ID, AGENT, STATUS, START_TIME, DURATION

### Worker Commands

Run task workers that poll Conductor and execute work locally.

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| `worker js <js_file>` | Run a JavaScript worker (EXPERIMENTAL) | JS file | `--type` (required), `--count`, `--worker-id`, `--domain`, `--poll-timeout` | `conductor worker js worker.js --type my_task` |
| `worker stdio <command> [args...]` | Poll tasks and execute a command via stdin/stdout | command | `--type` (required), `--count`, `--worker-id`, `--domain`, `--poll-timeout`, `--exec-timeout`, `--verbose` | `conductor worker stdio ./handler.sh --type my_task` |
| `worker remote` | Run a worker from the job-runner registry (EXPERIMENTAL, Orkes only) | None | `--type` (required), `--count`, `--worker-id`, `--domain`, `--poll-timeout`, `--exec-timeout`, `--refresh` | `conductor worker remote --type my_task` |
| `worker list-remote` | List workers in the job-runner registry (EXPERIMENTAL, Orkes only) | None | `--namespace` | `conductor worker list-remote` |

**Flags:**
- `--type` - Task type to poll for (required)
- `--count` - Number of tasks to poll in each batch (default: 1)
- `--worker-id` - Worker ID reported to the server
- `--domain` - Task domain
- `--poll-timeout` - Server-side long-poll wait in milliseconds (default: 100)
- `--exec-timeout` - Per-task execution timeout in seconds. `stdio` and `remote` only — a
  JavaScript worker runs in-process with no interrupt, so there is nothing to time out.
  Default 0 (no timeout) for `stdio`, 100 for `remote`.
- `--timeout` - Deprecated hidden alias for `--poll-timeout`
- `--verbose` - Print task and result JSON to stdout (`stdio` command)
- `--refresh` - Force refresh the worker from the registry, ignoring cache
- `--namespace` - Registry namespace to list workers from (default: `default`)

Both flavours share one poll loop; they differ only in how user code runs and in the result
shape it returns:

| Flavour | Worker returns | Failure carries |
|---------|----------------|-----------------|
| `stdio` | `{"status","output","logs","reason"}` on stdout | `reasonForIncompletion` + logs |
| `js` | `{status, body}` from the script; `$.task` holds the task | `output.error` |

Workers exit on Ctrl-C/SIGTERM once the in-flight batch finishes — a running task is left
to complete and report its real result rather than being killed, which would report a
failure the worker inflicted on itself and consume one of the task's retries. A second
signal exits immediately. Child processes receive `TASK_TYPE`, `TASK_ID`, `WORKFLOW_ID`, `EXECUTION_ID`,
`POLL_DOMAIN`, and the CLI's own `CONDUCTOR_SERVER_URL` and credentials.

See [WORKER_JS.md](./WORKER_JS.md) and [WORKER_STDIO.md](./WORKER_STDIO.md) for the worker
protocols.

### Development Commands

| Command | Description | Required Args | Optional Flags | Example |
|---------|-------------|---------------|----------------|---------|
| `deploy` | Deploy agents from your project to the server | None | `--agents`/`-a`, `--language`/`-l`, `--package`/`-p`, `--json` | `conductor deploy --language python` |
| `doctor` | Check runtime and AI provider configuration | None | | `conductor doctor` |
| `whoami` | Display information about the current user | None | | `conductor whoami` |

**`deploy` flags:**
- `--agents`, `-a` - Comma-separated agent names to deploy (default: all discovered)
- `--language`, `-l` - Project language: `python` or `typescript` (auto-detected if omitted)
- `--package`, `-p` - Package or path to scan for agents
- `--json` - Output results as JSON

**Notes:**
- `deploy` discovers agents defined as module-level variables in your project. Python projects need a Python interpreter on `PATH` or the `PYTHON` environment variable set.
- `doctor` reports Java and Python availability, the configured server URL and auth state, and which AI provider API keys are set in the environment.
- `whoami` prints the server URL and decoded JWT claims. With no auth configured it reports `Authentication: none (OSS Conductor)`.

### Other Commands

| Command | Description | Example |
|---------|-------------|---------|
| `update` | Update CLI to latest version | `conductor update` |
| `completion <shell>` | Generate a shell completion script | `conductor completion zsh` |
| `--version` | Show CLI version | `conductor --version` |
| `--help` | Show help | `conductor --help` or `conductor workflow --help` |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error (connection failed, authentication failed, not found, etc.) |

## Output Format

- **Default:** Formatted tables for list commands, human-readable text for other commands
- **Table format:** Tab-separated columns with headers (for `list` commands)
- **JSON format:** Available via `--json` flag for all `list` commands
- **CSV format:** Available via `--csv` flag. `--json` and `--csv` are mutually exclusive.
- **Workflow ID extraction:** UUIDs in format `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` (36 characters with hyphens)
- **Status output:** Single line with status value (e.g., `RUNNING`, `COMPLETED`)

**List Commands with Table/JSON Output:**
- `workflow list` - Table with NAME, VERSION, DESCRIPTION (or `--json` for complete data)
- `task list` - Table with NAME, EXECUTABLE, DESCRIPTION, OWNER, TIMEOUT POLICY, TIMEOUT (s), RETRY COUNT, RESPONSE TIMEOUT (s) (or `--json`)
- `schedule list` - Table with NAME, WORKFLOW, STATUS, CREATED TIME (or `--json`)
- `webhook list` - Table with NAME, WEBHOOK ID, WORKFLOWS, URL (or `--json`)
- `secret list` - Table with KEY, or KEY and TAGS with `--with-tags` (or `--json`)
- `agent list` - Table with NAME, VERSION, TYPE, DESCRIPTION (or `--json`/`--csv`)

**Important:** To parse output reliably, redirect stderr to `/dev/null` to suppress update notifications and warnings:
```bash
conductor workflow list 2>/dev/null
conductor task list --json 2>/dev/null
WORKFLOW_ID=$(conductor workflow start --workflow my_workflow 2>/dev/null | grep -oE '[a-f0-9-]{36}')
```

## Input Format

### Workflow Input Data

Workflows can accept input data in two ways:

**1. Inline JSON (--input flag):**
```bash
conductor workflow start --workflow my_workflow --input '{"key":"value","count":42}'
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
conductor workflow start --workflow my_workflow --file input.json
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

See [Conductor documentation](https://conductor.io/content) for complete workflow definition schema.

## Common Patterns

### 0. Spin up a local server and run a workflow

```bash
# Start a local OSS Conductor server (downloads the JAR on first run, needs Java 21)
conductor server start

# Confirm it is up — the default server URL already points at http://localhost:8080/api
conductor server status

# Register and run a workflow; no auth needed against OSS
conductor workflow create workflow.json --force
conductor workflow start --workflow my_workflow --sync

# Tail logs if something goes wrong, then shut down
conductor server logs -f
conductor server stop
```

### 1. Deploy workflow to production

```bash
# Save production profile (interactive prompts for URL, server type, auth)
conductor config save --profile production

# Deploy workflow
conductor --profile production workflow create workflow.json --force
```

### 2. Start and monitor execution

```bash
# Start workflow and capture ID
WORKFLOW_ID=$(conductor workflow start --workflow my_workflow 2>/dev/null | grep -oE '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}')

# Start with input data
WORKFLOW_ID=$(conductor workflow start --workflow my_workflow --input '{"orderId":"12345","customerId":"cust_001"}' 2>/dev/null | grep -oE '[a-f0-9-]{36}')

# Start with input from file
WORKFLOW_ID=$(conductor workflow start --workflow my_workflow --file input.json 2>/dev/null | grep -oE '[a-f0-9-]{36}')

# Check status
conductor workflow status "$WORKFLOW_ID"

# Get full details
conductor workflow get-execution "$WORKFLOW_ID"
```

### 3. Multi-environment workflow

```bash
# Deploy to dev
CONDUCTOR_PROFILE=dev conductor workflow create workflow.json --force

# Test in dev
CONDUCTOR_PROFILE=dev conductor workflow start --workflow my_workflow

# Deploy to prod after testing
CONDUCTOR_PROFILE=prod conductor workflow create workflow.json --force
```

### 4. Handle workflow failure

```bash
# Check status
STATUS=$(conductor workflow status "$WORKFLOW_ID" 2>/dev/null)

if [ "$STATUS" = "FAILED" ]; then
  # Retry failed task
  conductor workflow retry "$WORKFLOW_ID"

  # Or rerun from failed point
  conductor workflow rerun "$WORKFLOW_ID"
fi
```

### 5. Terminate stuck workflows

```bash
# Find running workflows
conductor workflow search --workflow my_workflow --status RUNNING

# Terminate specific execution
conductor workflow terminate "$WORKFLOW_ID"
```

### 6. Create and test webhook

```bash
# Create webhook
conductor webhook create \
  --name my_webhook \
  --source-platform Custom \
  --verifier HEADER_BASED \
  --headers "Authorization:secret123" \
  --receiver-workflows my_workflow:1

# List webhooks to verify
conductor webhook list
```

### 7. Manage workflow versions

```bash
# Get latest version
conductor workflow get my_workflow

# Get specific version
conductor workflow get my_workflow 2

# Delete old version
conductor workflow delete my_workflow 1
```

### 8. Poll and process tasks

```bash
# Poll for tasks
conductor task poll my_task_type --count 10 --worker-id worker1

# Update task status
conductor task update-execution \
  --workflow-id "$WORKFLOW_ID" \
  --task-ref-name my_task \
  --status COMPLETED \
  --output '{"result":"success"}'
```

### 9. Search for executions

```bash
# Find failed executions for a workflow
conductor workflow search --workflow my_workflow --status FAILED --count 50

# Find executions within time range
conductor workflow search --workflow my_workflow \
  --start-time-after "2025-01-01" \
  --start-time-before "2025-01-31"

# Combine filters
conductor workflow search --workflow my_workflow \
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

### 10. Manage secrets

```bash
# Create a secret from command line
conductor secret put db_password mySecretPassword123

# Create a secret from environment variable
conductor secret put api_key --value "$MY_API_KEY"

# Create a secret from file (without exposing value in command history)
cat secret.txt | conductor secret put encryption_key

# List all secrets (keys only)
conductor secret list

# List secrets with tags
conductor secret list --with-tags

# Get secret value (requires explicit flag for security)
conductor secret get db_password --show-value

# Check if secret exists
conductor secret exists db_password

# Add tags to organize secrets
conductor secret tag-add db_password --tag env:prod --tag team:backend --tag type:database

# List tags for a secret
conductor secret tag-list db_password

# Delete specific tags
conductor secret tag-delete db_password --tag env:prod

# Delete a secret (requires confirmation)
conductor secret delete old_api_key

# Delete without confirmation
conductor secret delete old_api_key -y

# Clear caches after secret rotation
conductor secret cache-clear --local
conductor secret cache-clear --redis

# Clear both caches at once
conductor secret cache-clear
```

### 11. Create and manage API Gateway services

```bash
# Create service from JSON file
conductor api-gateway service create service.json

# Create service using flags
conductor api-gateway service create \
  --service-id my-api \
  --name "My API Service" \
  --path "/api/v1" \
  --description "API for accessing workflows" \
  --enabled \
  --auth-config-id token-based \
  --cors-allowed-origins "https://example.com" \
  --cors-allowed-methods "GET,POST,PUT,DELETE" \
  --cors-allowed-headers "*"

# List all services
conductor api-gateway service list

# Get service details
conductor api-gateway service get my-api
```

**Example service JSON:**
```json
{
  "id": "my-api",
  "name": "My API Service",
  "path": "/api/v1",
  "description": "API for accessing workflows",
  "enabled": true,
  "mcpEnabled": true,
  "authConfigId": "token-based",
  "corsConfig": {
    "accessControlAllowOrigin": ["https://example.com"],
    "accessControlAllowMethods": ["GET", "POST", "PUT", "DELETE"],
    "accessControlAllowHeaders": ["*"]
  }
}
```

### 12. Set up API Gateway authentication

```bash
# Create auth config from file
conductor api-gateway auth create auth-config.json

# Create auth config using flags
conductor api-gateway auth create \
  --auth-config-id "token-based" \
  --auth-type "API_KEY" \
  --application-id "my-app-id" \
  --api-keys "key1,key2,key3"

# List auth configs
conductor api-gateway auth list

# Get specific auth config
conductor api-gateway auth get token-based
```

**Example auth config JSON:**
```json
{
  "id": "token-based",
  "authenticationType": "API_KEY",
  "applicationId": "my-app-id",
  "apiKeys": ["key1", "key2"]
}
```

### 13. Create API Gateway routes for workflows

```bash
# Create a route from JSON
conductor api-gateway route create my-api route.json

# Create a route using flags
conductor api-gateway route create my-service \
  --http-method "GET" \
  --path "/users/{userId}" \
  --description "Get user by ID" \
  --workflow-name "get_user_workflow" \
  --workflow-version 1 \
  --execution-mode "SYNC"

# Create async route with metadata
conductor api-gateway route create my-service \
  --http-method "POST" \
  --path "/orders" \
  --description "Create order" \
  --workflow-name "create_order_workflow" \
  --execution-mode "ASYNC" \
  --request-metadata-as-input \
  --workflow-metadata-in-output

# List routes for a service
conductor api-gateway route list my-api

# Delete a route
conductor api-gateway route delete my-api GET /users
```

**Example route JSON:**
```json
{
  "path": "/users/{userId}",
  "httpMethod": "GET",
  "description": "Get user by ID",
  "workflowExecutionMode": "SYNCHRONOUS",
  "mappedWorkflow": {
    "name": "get_user_workflow",
    "version": 1
  }
}
```

### 14. Build and run an agent

```bash
# Check that a model provider API key is configured
conductor doctor

# Scaffold a config, inspect the plan, then run it
conductor agent init triage
conductor agent compile triage.yaml
conductor agent run --config triage.yaml "summarize open incidents"

# Run a registered agent and keep conversation context across calls
conductor agent run --name triage --session sess-001 "what changed since yesterday?"

# Start without streaming, then attach to the stream later
EXEC_ID=$(conductor agent run --name triage "long task" --no-stream 2>/dev/null | grep -oE '[a-f0-9-]{36}')
conductor agent stream "$EXEC_ID"

# Approve a human-in-the-loop step
conductor agent respond "$EXEC_ID" --approve --reason "verified manually"
```

### 15. Run a task worker

```bash
# Execute any command per task over stdin/stdout
conductor worker stdio ./handler.sh --type my_task --count 5 --verbose

# Or run a JavaScript worker
conductor worker js worker.js --type my_task --worker-id worker1
```

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
**Solution:** Create profile with `conductor config save --profile prod` or check profile name

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

1. **Always redirect stderr** when parsing output: `conductor command 2>/dev/null`
2. **Extract workflow IDs** using: `grep -oE '[a-f0-9-]{36}'`
3. **Check exit codes** for error handling: `if [ $? -eq 0 ]; then ...`
4. **Use profiles** for multi-environment operations
5. **Quote workflow names** with spaces: `conductor workflow get "my workflow"`
6. **Use --force flag** when updating workflows to overwrite
7. **Save profiles once** then use `CONDUCTOR_PROFILE` env var for cleaner commands

## Auto-Update Feature

The CLI checks for updates every 24 hours and notifies when a new version is available:

```
⚠ A new version is available: v0.0.12 (current: v0.0.11)
Run 'conductor update' to download it or update with your package manager.
```

Update to latest version:
```bash
conductor update
```

**Note:** Update notifications are written to stderr and won't interfere with command output.

## Full Documentation

For detailed human-readable documentation, see [README.md](./README.md)
