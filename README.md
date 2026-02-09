# CLI for Conductor

[Conductor](https://www.conductor-oss.org/) is the leading open-source orchestration platform allowing developers to build highly scalable distributed applications.

Check out the [official documentation for Conductor](https://conductor.io/content).

This repository provides a CLI for the Conductor Conductor Server.

## ⭐ Conductor OSS

Show support for the Conductor OSS.  Please help spread the awareness by starring Conductor repo.

[![GitHub stars](https://img.shields.io/github/stars/conductor-oss/conductor.svg?style=social&label=Star&maxAge=)](https://GitHub.com/conductor-oss/conductor/)

## Installation

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

**After tapping, future updates are simple:**
```bash
brew upgrade conductor
```

<details>
<summary><b>Migrating from old tap (conductor-tools)?</b></summary>

If you previously installed from `conductor-oss/conductor-tools`, migrate to the new tap:

```bash
brew uninstall conductor
brew untap conductor-oss/conductor-tools
brew install conductor-oss/conductor/conductor
```
</details>

### Using npm

If you have Node.js installed:

```bash
npm install -g @io-conductor/conductor-cli
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

### Custom Installation Directory

To install to a custom directory:

```bash
INSTALL_DIR=$HOME/.local/bin curl -fsSL https://raw.githubusercontent.com/conductor-oss/conductor-cli/main/install.sh | sh
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

## Command Reference

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `workflow` | `list` | `--json`, `--csv` | List all workflow definitions | `conductor workflow list` |
| `workflow` | `get` | `<name> [version]` | Get workflow definition | `conductor workflow get my_workflow` |
| `workflow` | `get_all` | | Get all workflow definitions (JSON) | `conductor workflow get_all` |
| `workflow` | `create` | `<file>`, `--force`, `--js` | Create/register a workflow | `conductor workflow create flow.json --force` |
| `workflow` | `update` | `<file>` | Update a workflow definition | `conductor workflow update flow.json` |
| `workflow` | `delete` | `<name> <version>` | Delete a workflow definition | `conductor workflow delete my_workflow 1` |
| `workflow` | `start` | `-w`, `-i`, `-f`, `--version`, `--correlation`, `--sync`, `-u` | Start workflow execution | `conductor workflow start -w my_workflow -i '{"key":"value"}'` |
| `workflow` | `search` | `-w`, `-s`, `-c`, `--start-time-after`, `--start-time-before`, `--json`, `--csv` | Search workflow executions | `conductor workflow search -w my_workflow -s FAILED` |
| `workflow` | `status` | `<workflow_id>` | Get workflow execution status | `conductor workflow status abc-123` |
| `workflow` | `get-execution` | `<workflow_id>`, `-c` | Get full execution details | `conductor workflow get-execution abc-123` |
| `workflow` | `terminate` | `<workflow_id>` | Terminate a running execution | `conductor workflow terminate abc-123` |
| `workflow` | `pause` | `<workflow_id>` | Pause a running execution | `conductor workflow pause abc-123` |
| `workflow` | `resume` | `<workflow_id>` | Resume a paused execution | `conductor workflow resume abc-123` |
| `workflow` | `restart` | `<workflow_id>`, `--use-latest` | Restart a completed workflow | `conductor workflow restart abc-123` |
| `workflow` | `retry` | `<workflow_id>`, `--resume-subworkflow-tasks` | Retry the last failed task | `conductor workflow retry abc-123` |
| `workflow` | `rerun` | `<workflow_id>`, `--task-id`, `--task-input`, `--workflow-input` | Rerun from a specific task | `conductor workflow rerun abc-123 --task-id task1` |
| `workflow` | `skip-task` | `<workflow_id> <task_ref>`, `--task-input`, `--task-output` | Skip a task | `conductor workflow skip-task abc-123 task1` |
| `workflow` | `jump` | `<workflow_id> <task_ref>`, `--task-input` | Jump to a specific task | `conductor workflow jump abc-123 task2` |
| `workflow` | `delete-execution` | `<workflow_id>`, `-a` | Delete a workflow execution | `conductor workflow delete-execution abc-123` |
| `workflow` | `update-state` | `<workflow_id>`, `--variables`, `--task-updates` | Update workflow state | `conductor workflow update-state abc-123 --variables '{"x":1}'` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `task` | `list` | `--json`, `--csv` | List all task definitions | `conductor task list` |
| `task` | `get` | `<task_type>` | Get task definition | `conductor task get my_task` |
| `task` | `get_all` | | Get all task definitions (JSON) | `conductor task get_all` |
| `task` | `create` | `<file>` | Create a task definition | `conductor task create task.json` |
| `task` | `update` | `<file>` | Update a task definition | `conductor task update task.json` |
| `task` | `delete` | `<task_type>` | Delete a task definition | `conductor task delete my_task` |
| `task` | `poll` | `<task_type>`, `--count`, `--worker-id`, `--domain`, `--timeout` | Batch poll for tasks | `conductor task poll my_task --count 5` |
| `task` | `update-execution` | `--workflow-id`, `--task-ref-name`, `--status`, `--output` | Update task by reference | `conductor task update-execution --workflow-id abc --task-ref-name t1 --status COMPLETED` |
| `task` | `signal` | `--workflow-id`, `--status`, `--output` | Signal a task (async) | `conductor task signal --workflow-id abc --status COMPLETED` |
| `task` | `signal-sync` | `--workflow-id`, `--status`, `--output` | Signal a task (sync) | `conductor task signal-sync --workflow-id abc --status COMPLETED` |

---

### Schedule Commands

Manage workflow schedules (Enterprise only).

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `schedule` | `list` | `--json`, `--csv` | List all schedules | `conductor schedule list` |
| `schedule` | `get` | `<name>` | Get schedule details | `conductor schedule get my_schedule` |
| `schedule` | `create` | `<file>` or `-n`, `-c`, `-w`, `-i`, `-p` | Create a schedule | `conductor schedule create -n daily -c "0 0 * * *" -w my_workflow` |
| `schedule` | `update` | `<file>` or flags | Update a schedule | `conductor schedule update schedule.json` |
| `schedule` | `delete` | `<name>` | Delete a schedule | `conductor schedule delete my_schedule` |
| `schedule` | `pause` | `<name>` | Pause a schedule | `conductor schedule pause my_schedule` |
| `schedule` | `resume` | `<name>` | Resume a schedule | `conductor schedule resume my_schedule` |
| `schedule` | `search` | `-c`, `-s` | Search schedule executions | `conductor schedule search -s FAILED` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `secret` | `list` | `--with-tags`, `--json`, `--csv` | List all secrets | `conductor secret list` |
| `secret` | `get` | `<key>`, `--show-value` | Get a secret | `conductor secret get db_password --show-value` |
| `secret` | `put` | `<key> [value]`, `--value` | Create/update a secret | `conductor secret put db_password mySecret` |
| `secret` | `delete` | `<key>`, `-y` | Delete a secret | `conductor secret delete db_password -y` |
| `secret` | `exists` | `<key>` | Check if secret exists | `conductor secret exists db_password` |
| `secret` | `tag-list` | `<key>`, `--json`, `--csv` | List tags for a secret | `conductor secret tag-list db_password` |
| `secret` | `tag-add` | `<key>`, `--tag` | Add tags to a secret | `conductor secret tag-add db_password --tag env:prod` |
| `secret` | `tag-delete` | `<key>`, `--tag` | Remove tags from a secret | `conductor secret tag-delete db_password --tag env:prod` |
| `secret` | `cache-clear` | `--local`, `--redis` | Clear secrets cache | `conductor secret cache-clear` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `webhook` | `list` | `--json`, `--csv` | List all webhooks | `conductor webhook list` |
| `webhook` | `get` | `<webhook_id>` | Get webhook details | `conductor webhook get my_webhook` |
| `webhook` | `create` | `<file>` or flags | Create a webhook | `conductor webhook create webhook.json` |
| `webhook` | `update` | `<webhook_id>`, `--file` | Update a webhook | `conductor webhook update id --file webhook.json` |
| `webhook` | `delete` | `<webhook_id>` | Delete a webhook | `conductor webhook delete my_webhook` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `api-gateway service` | `list` | `--json` | List all services | `conductor api-gateway service list` |
| `api-gateway service` | `get` | `<service_id>` | Get service details | `conductor api-gateway service get my-service` |
| `api-gateway service` | `create` | `<file>` or flags | Create a service | `conductor api-gateway service create service.json` |
| `api-gateway service` | `update` | `<service_id> <file>` | Update a service | `conductor api-gateway service update my-service service.json` |
| `api-gateway service` | `delete` | `<service_id>` | Delete a service | `conductor api-gateway service delete my-service` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `api-gateway auth` | `list` | `--json` | List auth configs | `conductor api-gateway auth list` |
| `api-gateway auth` | `get` | `<auth_config_id>` | Get auth config | `conductor api-gateway auth get token-based` |
| `api-gateway auth` | `create` | `<file>` or flags | Create auth config | `conductor api-gateway auth create auth.json` |
| `api-gateway auth` | `update` | `<auth_config_id> <file>` | Update auth config | `conductor api-gateway auth update token-based auth.json` |
| `api-gateway auth` | `delete` | `<auth_config_id>` | Delete auth config | `conductor api-gateway auth delete token-based` |

**Auth Create Options:**
- `--auth-config-id` - Auth config ID
- `--auth-type` - Auth type: `API_KEY` or `NONE`
- `--application-id` - Application ID
- `--api-keys` - API keys (comma-separated)

#### Route Management

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `api-gateway route` | `list` | `<service_id>`, `--json` | List routes | `conductor api-gateway route list my-service` |
| `api-gateway route` | `create` | `<service_id>` `<file>` or flags | Create a route | `conductor api-gateway route create my-service route.json` |
| `api-gateway route` | `update` | `<service_id> <path> <file>` | Update a route | `conductor api-gateway route update my-service /users route.json` |
| `api-gateway route` | `delete` | `<service_id> <method> <path>` | Delete a route | `conductor api-gateway route delete my-service GET /users` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `server` | `start` | `--port`, `--version`, `--oss`, `--orkes`, `-f` | Start local server | `conductor server start` |
| `server` | `stop` | | Stop local server | `conductor server stop` |
| `server` | `status` | | Check server status | `conductor server status` |
| `server` | `logs` | `-f`, `-n` | Show server logs | `conductor server logs -f` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `worker` | `stdio` | `--type`, `--count`, `--worker-id`, `--domain`, `--poll-timeout`, `--exec-timeout`, `--verbose` | Run stdio worker | `conductor worker stdio --type my_task python worker.py` |
| `worker` | `js` | `--type`, `--count`, `--worker-id`, `--domain`, `--timeout` | Run JavaScript worker | `conductor worker js --type my_task worker.js` |
| `worker` | `remote` | `--type`, `--count`, `--worker-id`, `--domain`, `--refresh` | Run remote worker | `conductor worker remote --type my_task` |
| `worker` | `list-remote` | `--namespace` | List remote workers | `conductor worker list-remote` |

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

| Command | Sub-command | Options | Description | Example |
|---------|-------------|---------|-------------|---------|
| `config` | `save` | `--profile` | Save configuration (interactive) | `conductor config save` |
| `config` | `list` | | List all profiles | `conductor config list` |
| `config` | `delete` | `[profile]`, `--profile`, `-y` | Delete a profile | `conductor config delete production -y` |

---

### Other Commands

| Command | Description | Example |
|---------|-------------|---------|
| `update` | Update CLI to latest version | `conductor update` |
| `whoami` | Display current user info | `conductor whoami` |
| `completion` | Generate shell completion script | `conductor completion zsh` |

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

**Non-Interactive (Legacy):**

You can still save configuration non-interactively by providing flags:

```bash
# Enterprise with auth token (default server type)
conductor --server https://developer.conductorcloud.com --auth-token your-token config save

# Enterprise with API key + secret
conductor --server https://developer.conductorcloud.com --auth-key your-key --auth-secret your-secret config save

# OSS Conductor
conductor --server http://localhost:8080/api --server-type OSS config save
```

Once saved, you can run commands without providing flags:

```bash
# After saving config, simply run:
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
