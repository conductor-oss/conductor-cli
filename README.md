# CLI for Conductor

[Conductor](https://www.conductor-oss.org/) is the leading open-source orchestration platform allowing developers to build highly scalable distributed applications.

Check out the [official documentation for Conductor](https://conductor.io/content).

This repository provides a CLI for the Conductor Conductor Server.

## ⭐ Conductor OSS

Show support for the Conductor OSS.  Please help spread the awareness by starring Conductor repo.

[![GitHub stars](https://img.shields.io/github/stars/conductor-oss/conductor.svg?style=social&label=Star&maxAge=)](https://GitHub.com/conductor-oss/conductor/)


## Installation

### Using npm

If you have Node.js installed:

```bash
npm install -g @conductor-oss/conductor-cli
```

This will automatically download and install the appropriate binary for your platform.

### Quick Install (macOS/Linux)

Install the latest version using curl:

```bash
curl -fsSL https://raw.githubusercontent.com/conductor-oss/conductor-cli/main/install.sh | sh
```

This will automatically:
- Detect your OS and architecture
- Download the latest release
- Install to `/usr/local/bin`
- Verify the installation

_Custom Installation Directory:_

```bash
INSTALL_DIR=$HOME/.local/bin curl -fsSL https://raw.githubusercontent.com/conductor-oss/conductor-cli/main/install.sh | sh
```

### Quick Install (Windows)

**PowerShell (one-liner):**

```powershell
irm https://raw.githubusercontent.com/conductor-oss/conductor-cli/main/install.ps1 | iex
```

**Command Prompt (cmd):**

```cmd
powershell -Command "irm https://raw.githubusercontent.com/conductor-oss/conductor-cli/main/install.ps1 | iex"
```

After installation, restart your terminal and verify:
```
conductor --version
```


### Using Homebrew (macOS/Linux)

**First time installation:**
```bash
# Add the Conductor tap (one-time setup)
brew tap conductor-oss/conductor

# Install the CLI
brew install conductor
```

Or install directly in one line:
```bash
brew install conductor-oss/conductor/conductor
```

### Manual Installation

Download the appropriate binary for your platform from the [releases page](https://github.com/conductor-oss/conductor-cli/releases):

- **Linux amd64**: `conductor_linux_amd64`
- **Linux arm64**: `conductor_linux_arm64`
- **macOS amd64**: `conductor_darwin_amd64`
- **macOS arm64**: `conductor_darwin_arm64`
- **Windows amd64**: `conductor_windows_amd64.exe`
- **Windows arm64**: `conductor_windows_arm64.exe`

Then make it executable and move it to your PATH:

```bash
chmod +x conductor_*
mv conductor_* /usr/local/bin/conductor
```

### Verify Installation

```bash
conductor --version
```

### Shell Completion

Enable tab completion for commands, flags, and arguments:

**Zsh (macOS default):**
```bash
# One-time setup
conductor completion zsh > $(brew --prefix)/share/zsh/site-functions/_conductor

# Restart your shell or run:
source ~/.zshrc
```

**Bash:**
```bash
# Linux
conductor completion bash > /etc/bash_completion.d/conductor

# macOS
conductor completion bash > $(brew --prefix)/etc/bash_completion.d/conductor

# Then restart your shell
```

**Fish:**
```bash
conductor completion fish > ~/.config/fish/completions/conductor.fish
```

**PowerShell:**
```powershell
conductor completion powershell | Out-String | Invoke-Expression
```

After installing, you'll get tab completion when typing `conductor <TAB>`.

---

# Usage Guide
<!-- TOC -->
* [CLI for Conductor](#cli-for-conductor)
  * [⭐ Conductor OSS](#-conductor-oss)
  * [Installation](#installation)
    * [Using npm](#using-npm)
    * [Quick Install (macOS/Linux)](#quick-install--macoslinux-)
    * [Quick Install (Windows)](#quick-install--windows-)
    * [Using Homebrew (macOS/Linux)](#using-homebrew--macoslinux-)
    * [Manual Installation](#manual-installation)
    * [Verify Installation](#verify-installation)
    * [Shell Completion](#shell-completion)
* [Usage Guide](#usage-guide)
    * [Global Flags](#global-flags)
    * [Workflow Commands](#workflow-commands)
    * [Task Commands](#task-commands)
    * [Schedule Commands](#schedule-commands)
    * [Secret Commands](#secret-commands)
    * [Webhook Commands](#webhook-commands)
    * [API Gateway Commands](#api-gateway-commands)
      * [Service Management](#service-management)
      * [Auth Configuration Management](#auth-configuration-management)
      * [Route Management](#route-management)
    * [Server Commands](#server-commands)
    * [Worker Commands](#worker-commands)
    * [Config Commands](#config-commands)
    * [Other Commands](#other-commands)
  * [Configuration](#configuration)
    * [Server Types](#server-types)
    * [Saving Your Configuration](#saving-your-configuration)
    * [Using Profiles for Multiple Environments](#using-profiles-for-multiple-environments)
    * [Configuration Precedence](#configuration-precedence)
    * [Command-line Flags](#command-line-flags)
    * [Environment Variables](#environment-variables)
      * [Disabling Colored Output](#disabling-colored-output)
    * [Configuration File Format](#configuration-file-format)
  * [Workers](#workers)
    * [Stdio Workers](#stdio-workers)
    * [JavaScript Workers (Built-in)](#javascript-workers--built-in-)
    * [Remote Workers (Registry-based)](#remote-workers--registry-based-)
  * [Exit Codes](#exit-codes)
  * [Error Handling](#error-handling)
    * [Connection Errors](#connection-errors)
    * [Authentication Errors](#authentication-errors)
    * [Resource Not Found](#resource-not-found)
    * [Profile Not Found](#profile-not-found)
  * [For AI Assistants & LLMs](#for-ai-assistants--llms)
<!-- TOC -->

### Global Flags

These flags can be used with any command:

| Flag | Description |
|------|-------------|
| `--server <url>` | Conductor server URL (or set `CONDUCTOR_SERVER_URL`) |
| `--auth-token <token>` | Auth token for authentication (or set `CONDUCTOR_AUTH_TOKEN`) |
| `--auth-key <key>` | API key for authentication (or set `CONDUCTOR_AUTH_KEY`) |
| `--auth-secret <secret>` | API secret for authentication (or set `CONDUCTOR_AUTH_SECRET`) |
| `--server-type <type>` | Server type: `OSS` or `Enterprise` (default: `OSS`) |
| `--profile <name>` | Use a specific configuration profile |
| `--config <path>` | Path to config file (default: `~/.conductor-cli/config.yaml`) |
| `-v, --verbose` | Print verbose logs |
| `-y, --yes` | Confirm yes to prompts |
| `-h, --help` | Help for any command |
| `--version` | Show CLI version |

---

### Workflow Commands

Manage workflow definitions and executions.

```
conductor workflow <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list` | List all workflow definitions (`--json`, `--csv`) |
| `get <name> [version]` | Get workflow definition |
| `get_all` | Get all workflow definitions (JSON) |
| `create <file>` | Create/register a workflow (`--force` to overwrite, `--js` for JavaScript) |
| `update <file>` | Update a workflow definition |
| `delete <name> <version>` | Delete a workflow definition |
| `start` | Start workflow execution (see options below) |
| `search` | Search workflow executions (see options below) |
| `status <workflow_id>` | Get workflow execution status |
| `get-execution <workflow_id>` | Get full execution details (`-c` for complete) |
| `terminate <workflow_id>` | Terminate a running execution |
| `pause <workflow_id>` | Pause a running execution |
| `resume <workflow_id>` | Resume a paused execution |
| `restart <workflow_id>` | Restart a completed workflow (`--use-latest`) |
| `retry <workflow_id>` | Retry the last failed task (`--resume-subworkflow-tasks`) |
| `rerun <workflow_id>` | Rerun from a specific task (`--task-id`, `--task-input`, `--workflow-input`) |
| `skip-task <workflow_id> <task_ref>` | Skip a task (`--task-input`, `--task-output`) |
| `jump <workflow_id> <task_ref>` | Jump to a specific task (`--task-input`) |
| `delete-execution <workflow_id>` | Delete a workflow execution (`-a` to archive) |
| `update-state <workflow_id>` | Update workflow state (`--variables`, `--task-updates`) |

**Workflow Start Options:**
- `-w, --workflow` - Workflow name (required)
- `-i, --input` - Input JSON string
- `-f, --file` - Input JSON file
- `--version` - Workflow version (optional)
- `--correlation` - Correlation ID
- `--sync` - Execute synchronously
- `-u, --wait-until` - Wait until task completes (with `--sync`)

**Workflow Search Options:**
- `-w, --workflow` - Filter by workflow name
- `-s, --status` - Filter by status: `COMPLETED`, `FAILED`, `PAUSED`, `RUNNING`, `TERMINATED`, `TIMED_OUT`
- `-c, --count` - Number of results (default: 10, max: 1000)
- `--start-time-after` - Filter by start time (format: `YYYY-MM-DD HH:MM:SS`, `YYYY-MM-DD`, or epoch ms)
- `--start-time-before` - Filter by start time

---

### Task Commands

Manage task definitions and executions.

```
conductor task <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list` | List all task definitions (`--json`, `--csv`) |
| `get <task_type>` | Get task definition |
| `get_all` | Get all task definitions (JSON) |
| `create <file>` | Create a task definition |
| `update <file>` | Update a task definition |
| `delete <task_type>` | Delete a task definition |
| `poll <task_type>` | Batch poll for tasks (`--count`, `--worker-id`, `--domain`, `--timeout`) |
| `update-execution` | Update task by reference (`--workflow-id`, `--task-ref-name`, `--status`, `--output`) |
| `signal` | Signal a task async (`--workflow-id`, `--status`, `--output`) |
| `signal-sync` | Signal a task sync (`--workflow-id`, `--status`, `--output`) |

---

### Schedule Commands

Manage workflow schedules (Enterprise only).

```
conductor schedule <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list` | List all schedules (`--json`, `--csv`) |
| `get <name>` | Get schedule details |
| `create` | Create a schedule (see options below) |
| `update <file>` | Update a schedule |
| `delete <name>` | Delete a schedule |
| `pause <name>` | Pause a schedule |
| `resume <name>` | Resume a schedule |
| `search` | Search schedule executions (`-c`, `-s`) |

**Schedule Create Options:**
- `-n, --name` - Schedule name (required)
- `-c, --cron` - Cron expression (required)
- `-w, --workflow` - Workflow to start (required)
- `-i, --input` - Workflow input as JSON
- `-p, --paused` - Create in paused state
- `--version` - Workflow version

---

### Secret Commands

Manage secrets (Enterprise only).

```
conductor secret <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list` | List all secrets (`--with-tags`, `--json`, `--csv`) |
| `get <key>` | Get a secret (`--show-value` to display value) |
| `put <key> [value]` | Create/update a secret (`--value` flag alternative) |
| `delete <key>` | Delete a secret (`-y` to skip confirmation) |
| `exists <key>` | Check if secret exists |
| `tag-list <key>` | List tags for a secret (`--json`, `--csv`) |
| `tag-add <key>` | Add tags to a secret (`--tag key:value`, repeatable) |
| `tag-delete <key>` | Remove tags from a secret (`--tag key:value`, repeatable) |
| `cache-clear` | Clear secrets cache (`--local`, `--redis`) |

**Secret Put Methods:**
```bash
# As argument
conductor secret put my_secret "secret_value"

# Via flag
conductor secret put my_secret --value "secret_value"

# From stdin
echo "secret_value" | conductor secret put my_secret

# From file
cat secret.txt | conductor secret put my_secret
```

---

### Webhook Commands

Manage webhooks (Enterprise only).

```
conductor webhook <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list` | List all webhooks (`--json`, `--csv`) |
| `get <webhook_id>` | Get webhook details |
| `create` | Create a webhook (from file or flags) |
| `update <webhook_id>` | Update a webhook (`--file`) |
| `delete <webhook_id>` | Delete a webhook |

**Webhook Create Options:**
- `--name` - Webhook name
- `--source-platform` - Source platform (e.g., `Custom`, `GitHub`, `Slack`)
- `--verifier` - Verifier type (e.g., `HEADER_BASED`)
- `--headers` - Headers as `key:value` pairs
- `--receiver-workflows` - Receiver workflows as `workflow:version` pairs

---

### API Gateway Commands

Manage API Gateway services, routes, and authentication (Enterprise only).

#### Service Management

```
conductor api-gateway service <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list` | List all services (`--json`) |
| `get <service_id>` | Get service details |
| `create` | Create a service (from file or flags, see options below) |
| `update <service_id> <file>` | Update a service |
| `delete <service_id>` | Delete a service |

**Service Create Options:**
- `--service-id` - Service ID
- `--name` - Display name
- `--path` - Base path
- `--description` - Description
- `--enabled` - Enable service (default: true)
- `--mcp-enabled` - Enable MCP
- `--auth-config-id` - Auth config ID
- `--cors-allowed-origins` - CORS origins (comma-separated)
- `--cors-allowed-methods` - CORS methods (comma-separated)
- `--cors-allowed-headers` - CORS headers (comma-separated)

#### Auth Configuration Management

```
conductor api-gateway auth <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list` | List auth configs (`--json`) |
| `get <auth_config_id>` | Get auth config |
| `create` | Create auth config (from file or flags, see options below) |
| `update <auth_config_id> <file>` | Update auth config |
| `delete <auth_config_id>` | Delete auth config |

**Auth Create Options:**
- `--auth-config-id` - Auth config ID
- `--auth-type` - Auth type: `API_KEY` or `NONE`
- `--application-id` - Application ID
- `--api-keys` - API keys (comma-separated)

#### Route Management

```
conductor api-gateway route <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `list <service_id>` | List routes (`--json`) |
| `create <service_id>` | Create a route (from file or flags, see options below) |
| `update <service_id> <path> <file>` | Update a route |
| `delete <service_id> <method> <path>` | Delete a route |

**Route Create Options:**
- `--http-method` - HTTP method: `GET`, `POST`, `PUT`, `DELETE`, etc.
- `--path` - Route path
- `--workflow-name` - Workflow to execute
- `--workflow-version` - Workflow version
- `--execution-mode` - `SYNC` or `ASYNC`
- `--description` - Route description
- `--request-metadata-as-input` - Include request metadata in input
- `--workflow-metadata-in-output` - Include workflow metadata in output

---

### Server Commands

Manage a local Conductor server for development.

```
conductor server <command> [flags]
```

| Command | Description |
|---------|-------------|
| `start` | Start local server (`--port`, `--version`, `--oss`, `--orkes`, `-f`) |
| `stop` | Stop local server |
| `status` | Check server status |
| `logs` | Show server logs (`-f` to follow, `-n` for line count) |

**Server Start Options:**
- `--port` - Port to run on (default: 8080)
- `--version` - Server version (default: `latest`)
- `--oss` - Use OSS Conductor (default)
- `--orkes` - Use Orkes Conductor (coming soon)
- `-f, --foreground` - Run in foreground

**Examples:**
```bash
# Start with defaults
conductor server start

# Start specific version on custom port
conductor server start --version 3.21.23 --port 9090

# Run in foreground
conductor server start -f

# View logs
conductor server logs -f -n 100
```

---

### Worker Commands

Run task workers (Experimental).

```
conductor worker <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `stdio <program> [args...]` | Run stdio worker (`--type`, `--count`, `--worker-id`, `--domain`, `--poll-timeout`, `--exec-timeout`, `--verbose`) |
| `js <file>` | Run JavaScript worker (`--type`, `--count`, `--worker-id`, `--domain`, `--timeout`) |
| `remote` | Run remote worker (`--type`, `--count`, `--worker-id`, `--domain`, `--refresh`) |
| `list-remote` | List remote workers (`--namespace`) |

**Worker Options:**
- `--type` - Task type to poll for (required)
- `--count` - Number of tasks per batch (default: 1)
- `--worker-id` - Worker identifier
- `--domain` - Task domain
- `--poll-timeout` - Poll timeout in ms (default: 100)
- `--exec-timeout` - Execution timeout in seconds
- `--verbose` - Print task and result JSON
- `--refresh` - Force re-download remote worker

---

### Config Commands

Manage CLI configuration.

```
conductor config <command> [arguments] [flags]
```

| Command | Description |
|---------|-------------|
| `save` | Save configuration interactively (`--profile` for named profile) |
| `list` | List all profiles |
| `delete [profile]` | Delete a profile (`-y` to skip confirmation) |

---

### Other Commands

| Command | Description |
|---------|-------------|
| `update` | Update CLI to latest version |
| `whoami` | Display current user info |
| `completion <shell>` | Generate shell completion script (`bash`, `zsh`, `fish`, `powershell`) |

---

## Configuration

The CLI connects to your Conductor server and can optionally persist configuration using the `config save` command.

### Server Types

The CLI supports two types of Conductor servers:

- **Enterprise (Conductor Conductor)** (default): Requires server URL and authentication credentials
- **OSS Conductor**: Open-source Conductor - requires only server URL, no authentication

Use the `--server-type` flag to specify your server type (defaults to `Enterprise`):

```bash
# Enterprise/Conductor Conductor (default)
conductor --server https://developer.conductorcloud.com --auth-token your-token workflow list

# OSS Conductor
conductor --server http://localhost:8080/api --server-type OSS workflow list
```

### Saving Your Configuration

The `config save` command provides an **interactive setup** that guides you through configuring your Conductor connection. It prompts you for:
- Server URL
- Server type (OSS or Enterprise)
- Authentication method (API Key + Secret or Auth Token for Enterprise)

If a configuration already exists, you can press Enter to keep existing values (credentials are masked as `****`).

**Interactive Configuration:**

```bash
# Run interactive configuration
conductor config save

# Example interaction:
# Server URL [http://localhost:8080/api]: https://developer.conductorcloud.com
# Server type (OSS/Enterprise) [Enterprise]: ← Press Enter to keep
#
# Authentication method:
#   1. API Key + Secret
#   2. Auth Token
# Choose [1]: 2
# Auth Token []: your-token-here
# ✓ Configuration saved to ~/.conductor-cli/config.yaml
```

**Updating Existing Configuration:**

When a configuration file already exists, the interactive prompts show your current values. Press Enter to keep them:

```bash
conductor config save

# Example with existing config:
# Server URL [https://developer.conductorcloud.com]: ← Press Enter to keep
# Server type (OSS/Enterprise) [Enterprise]: ← Press Enter to keep
#
# Authentication method:
#   1. API Key + Secret
#   2. Auth Token
# Choose [2]: ← Press Enter to keep
# Auth Token [****]: ← Press Enter to keep or enter new token
```

Once saved, you can run commands without providing flags:

```bash
conductor workflow list
```

**Note:** Server URLs can be provided with or without `/api` suffix (e.g., `http://localhost:8080` or `http://localhost:8080/api`).

### Using Profiles for Multiple Environments

Profiles allow you to manage multiple Conductor environments (e.g., development, staging, production) and easily switch between them.

**Creating Profiles:**

Use the `--profile` flag with `config save` to create named profiles. The command will run in interactive mode:

```bash
# Interactively configure development profile
conductor config save --profile dev

# Interactively configure staging profile
conductor config save --profile staging

# Interactively configure production profile
conductor config save --profile production
```

You can also use the non-interactive method with flags:

```bash
# Save Enterprise staging environment (default server type)
conductor --server https://staging.example.com --auth-token staging-token --profile staging config save

# Save Enterprise production environment
conductor --server https://prod.example.com --auth-token prod-token --profile production config save

# Save local OSS development environment
conductor --server http://localhost:8080/api --server-type OSS --profile dev config save
```

**Using Profiles:**

Switch between environments by specifying the profile:

```bash
# Using --profile flag
conductor --profile production workflow list

# Using CONDUCTOR_PROFILE environment variable
export CONDUCTOR_PROFILE=production
conductor workflow list

# Flag takes precedence over environment variable
CONDUCTOR_PROFILE=staging conductor --profile production workflow list  # Uses production
```

**Profile File Structure:**

```
~/.conductor-cli/
├── config.yaml              # Default profile
├── config-production.yaml   # Production profile
├── config-staging.yaml      # Staging profile
└── config-dev.yaml          # Development profile
```

**Listing Profiles:**

```bash
# List all configuration profiles
conductor config list
```

This shows:
- `default` - for the default `config.yaml` file
- Profile names (e.g., `production`, `staging`) - for named profiles like `config-production.yaml`

**Deleting Profiles:**

```bash
# Delete default config (with confirmation prompt)
conductor config delete

# Delete named profile
conductor config delete production

# Delete without confirmation using -y flag
conductor config delete production -y
```

**Profile Error Handling:**

If you reference a profile that doesn't exist, you'll get a clear error:

```bash
conductor --profile nonexistent workflow list
# Error: Profile 'nonexistent' doesn't exist (expected file: ~/.conductor-cli/config-nonexistent.yaml)
```

### Configuration Precedence

The CLI can be configured using command-line flags, environment variables, or a configuration file. Configuration is handled with the following precedence (highest to lowest):

1. Command-line flags
2. Environment variables
3. Configuration file

### Command-line Flags

You can override saved configuration by providing flags directly:

```bash
# Override server URL for a single command
conductor --server http://different-server:8080/api workflow list

# Use different auth token temporarily
conductor --auth-token temporary-token workflow list

# Use OSS server type
conductor --server http://localhost:8080/api --server-type OSS workflow list
```

### Environment Variables

Set these environment variables to configure the CLI without flags:

```bash
# Server and authentication
export CONDUCTOR_SERVER_URL=http://localhost:8080/api
export CONDUCTOR_AUTH_TOKEN=your-auth-token

# Or using API key + secret
export CONDUCTOR_SERVER_URL=http://localhost:8080/api
export CONDUCTOR_AUTH_KEY=your-api-key
export CONDUCTOR_AUTH_SECRET=your-api-secret

# Server type (OSS or Enterprise, defaults to Enterprise)
export CONDUCTOR_SERVER_TYPE=OSS

# Profile selection
export CONDUCTOR_PROFILE=production
```

#### Disabling Colored Output

If you want to disable color output for any reason (CI/CD, etc), you can use:

```bash
export NO_COLOR=1
```

Any non-null value in the NO_COLOR variable will disable colored output.

### Configuration File Format

Configuration files use YAML format and are stored in `~/.conductor-cli/`:

```yaml
# Example config.yaml for Enterprise with auth token (default)
server: https://developer.conductorcloud.com/api
auth-token: your-auth-token
verbose: false
```

```yaml
# Example config.yaml for Enterprise with API key + secret
server: https://developer.conductorcloud.com/api
auth-key: your-api-key
auth-secret: your-api-secret
verbose: false
```

```yaml
# Example config.yaml for OSS Conductor (no authentication)
server: http://localhost:8080/api
server-type: OSS
verbose: false
```

**Notes:**
- `server-type` defaults to `Enterprise` if not specified
- Enterprise requires one authentication method (`auth-token` OR `auth-key`+`auth-secret`)
- OSS Conductor doesn't require `auth-token`, `auth-key`, or `auth-secret`

You can also specify a custom config file location:

```bash
conductor --config /path/to/my-config.yaml workflow list
```

---

## Workers

⚠️ **EXPERIMENTAL FEATURES**

The CLI supports two types of workers for processing Conductor tasks:

### Stdio Workers

Execute tasks using external programs written in **any language** (Python, Node.js, Go, Rust, shell scripts, etc.). 

The CLI polls for tasks and passes them to your worker via stdin/stdout.

**Best for:** Complex logic, heavy dependencies, full language ecosystem access

👉 **[Complete Stdio Worker Documentation →](WORKER_STDIO.md)**

**Quick example:**
```bash
# Run a Python worker (continuous polling with parallel execution)
conductor worker stdio --type greet_task python3 worker.py

# Poll multiple tasks per batch for higher throughput
conductor worker stdio --type greet_task python3 worker.py --count 5
```

### JavaScript Workers (Built-in)

Execute tasks using **JavaScript** scripts with built-in utilities (HTTP, crypto, string functions). No external dependencies needed.

**Best for:** Prototyping, Lightweight tasks, quick scripts, HTTP integrations

👉 **[Complete JavaScript Worker Documentation →](WORKER_JS.md)**

**Quick example:**
```bash
# Run a JavaScript worker
conductor worker js --type greet_task worker.js
```

### Remote Workers (Registry-based)

⚠️ **EXPERIMENTAL** - Download and execute workers directly from your Conductor Conductor instance without managing local files.

Remote workers are stored in the Conductor Conductor job-runner registry and can be generated using the AI Assistant in your Conductor instance. Once created, workers are automatically downloaded, cached locally, and executed with all dependencies installed.

**Best for:** Team collaboration, centralized worker management, zero local setup

**Key features:**
- **Zero configuration**: No manual worker setup or file management
- **Automatic dependencies**: Python workers get a virtual environment with all dependencies installed automatically (including `conductor-python` SDK)
- **Smart caching**: Workers are cached locally after first download for fast startup
- **Multi-language support**: JavaScript (Node.js) and Python workers supported
- **Version control**: Workers are versioned and can be updated centrally

**Quick examples:**
```bash
# List available workers in your Conductor instance
conductor worker list-remote

# Run a remote worker (downloads and caches automatically)
conductor worker remote --type greet_task

# Force refresh to get latest version
conductor worker remote --type greet_task --refresh

# Run with batch processing for higher throughput
conductor worker remote --type greet_task --count 10
```

**How it works:**
1. Create workers using the AI Assistant in your Conductor Conductor instance
2. Workers are stored in the job-runner registry with all metadata and dependencies
3. CLI downloads worker code on first run and sets up the environment automatically
4. Subsequent runs use the cached worker for instant startup
5. Python workers get an isolated virtual environment with dependencies installed
6. Workers authenticate automatically using your CLI configuration

**Note:** Remote workers must exist in your Conductor Conductor instance. Currently, workers are generated by the AI Assistant feature in Conductor Conductor.

---

## Exit Codes

The CLI uses standard exit codes for error handling:

| Exit Code | Description |
|-----------|-------------|
| `0` | Command completed successfully |
| `1` | General error (connection failed, authentication error, resource not found, etc.) |

**Example usage in scripts:**
```bash
if conductor execution start --workflow my_workflow; then
    echo "Workflow started successfully"
else
    echo "Failed to start workflow" >&2
    exit 1
fi
```

---

## Error Handling

Common errors and solutions:

### Connection Errors
```
Error: Get "https://...": dial tcp: no such host
```
**Solution:** Verify your `--server` URL or `CONDUCTOR_SERVER_URL` environment variable

### Authentication Errors
```
Error: 401 Unauthorized
```
**Solution:** Check your authentication credentials (`--auth-token` or `--auth-key`/`--auth-secret`)

### Resource Not Found
```
Error: 404 Not Found
```
**Solution:** Verify the resource name or ID exists on the server

### Profile Not Found
```
Error: Profile 'prod' doesn't exist (expected file: ~/.conductor-cli/config-prod.yaml)
```
**Solution:** Create the profile using `--save-config=prod` or verify the profile name

---

## For AI Assistants & LLMs

For a concise, LLM-optimized reference with command tables, exit codes, and canonical examples, see **[CLAUDE.md](./CLAUDE.md)**.
