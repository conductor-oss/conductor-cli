# CLI for Conductor

[Conductor](https://www.conductor-oss.org/) is the leading open-source orchestration platform allowing developers to build highly scalable distributed applications.

Check out the [official documentation for Conductor](https://orkes.io/content).

This repository provides a CLI for the Orkes Conductor Server.

## ⭐ Conductor OSS

Show support for the Conductor OSS.  Please help spread the awareness by starring Conductor repo.

[![GitHub stars](https://img.shields.io/github/stars/conductor-oss/conductor.svg?style=social&label=Star&maxAge=)](https://GitHub.com/conductor-oss/conductor/)

## Installation

### Using Homebrew (macOS/Linux)

```bash
brew tap conductor-oss/conductor-tools
brew install orkes
```

Or in one line:

```bash
brew install conductor-oss/conductor-tools/orkes
```

### Using npm

If you have Node.js installed:

```bash
npm install -g @io-orkes/conductor-cli
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

- **Linux amd64**: `orkes_linux_amd64`
- **Linux arm64**: `orkes_linux_arm64`
- **macOS amd64**: `orkes_darwin_amd64`
- **macOS arm64**: `orkes_darwin_arm64`
- **Windows amd64**: `orkes_windows_amd64.exe`
- **Windows arm64**: `orkes_windows_arm64.exe`

Then make it executable and move it to your PATH:

```bash
chmod +x orkes_*
mv orkes_* /usr/local/bin/orkes
```

### Verify Installation

```bash
orkes --version
```

### Shell Completion

Enable tab completion for commands, flags, and arguments:

**Zsh (macOS default):**
```bash
# One-time setup
orkes completion zsh > $(brew --prefix)/share/zsh/site-functions/_orkes

# Restart your shell or run:
source ~/.zshrc
```

**Bash:**
```bash
# Linux
orkes completion bash > /etc/bash_completion.d/orkes

# macOS
orkes completion bash > $(brew --prefix)/etc/bash_completion.d/orkes

# Then restart your shell
```

**Fish:**
```bash
orkes completion fish > ~/.config/fish/completions/orkes.fish
```

**PowerShell:**
```powershell
orkes completion powershell | Out-String | Invoke-Expression
```

After installing, you'll get tab completion when typing `orkes <TAB>`.

## Configuration

The CLI connects to your Conductor server and can optionally persist configuration using the `config save` command.

### Server Types

The CLI supports two types of Conductor servers:

- **Enterprise (Orkes Conductor)** (default): Requires server URL and authentication credentials
- **OSS Conductor**: Open-source Conductor - requires only server URL, no authentication

Use the `--server-type` flag to specify your server type (defaults to `Enterprise`):

```bash
# Enterprise/Orkes Conductor (default)
orkes --server https://developer.orkescloud.com --auth-token your-token workflow list

# OSS Conductor
orkes --server http://localhost:8080/api --server-type OSS workflow list
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
orkes config save

# Example interaction:
# Server URL [http://localhost:8080/api]: https://developer.orkescloud.com
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
orkes config save

# Example with existing config:
# Server URL [https://developer.orkescloud.com]: ← Press Enter to keep
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
orkes --server https://developer.orkescloud.com --auth-token your-token config save

# Enterprise with API key + secret
orkes --server https://developer.orkescloud.com --auth-key your-key --auth-secret your-secret config save

# OSS Conductor
orkes --server http://localhost:8080/api --server-type OSS config save
```

Once saved, you can run commands without providing flags:

```bash
# After saving config, simply run:
orkes workflow list
```

**Note:** Server URLs can be provided with or without `/api` suffix (e.g., `http://localhost:8080` or `http://localhost:8080/api`).

### Using Profiles for Multiple Environments

Profiles allow you to manage multiple Conductor environments (e.g., development, staging, production) and easily switch between them.

**Creating Profiles:**

Use the `--profile` flag with `config save` to create named profiles. The command will run in interactive mode:

```bash
# Interactively configure development profile
orkes config save --profile dev

# Interactively configure staging profile
orkes config save --profile staging

# Interactively configure production profile
orkes config save --profile production
```

You can also use the non-interactive method with flags:

```bash
# Save Enterprise staging environment (default server type)
orkes --server https://staging.example.com --auth-token staging-token --profile staging config save

# Save Enterprise production environment
orkes --server https://prod.example.com --auth-token prod-token --profile production config save

# Save local OSS development environment
orkes --server http://localhost:8080/api --server-type OSS --profile dev config save
```

**Using Profiles:**

Switch between environments by specifying the profile:

```bash
# Using --profile flag
orkes --profile production workflow list

# Using ORKES_PROFILE environment variable
export ORKES_PROFILE=production
orkes workflow list

# Flag takes precedence over environment variable
ORKES_PROFILE=staging orkes --profile production workflow list  # Uses production
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
orkes config list
```

This shows:
- `default` - for the default `config.yaml` file
- Profile names (e.g., `production`, `staging`) - for named profiles like `config-production.yaml`

**Deleting Profiles:**

```bash
# Delete default config (with confirmation prompt)
orkes config delete

# Delete named profile
orkes config delete production

# Delete without confirmation using -y flag
orkes config delete production -y
```

**Profile Error Handling:**

If you reference a profile that doesn't exist, you'll get a clear error:

```bash
orkes --profile nonexistent workflow list
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
orkes --server http://different-server:8080/api workflow list

# Use different auth token temporarily
orkes --auth-token temporary-token workflow list

# Use OSS server type
orkes --server http://localhost:8080/api --server-type OSS workflow list
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
export ORKES_PROFILE=production
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
server: https://developer.orkescloud.com/api
auth-token: your-auth-token
verbose: false
```

```yaml
# Example config.yaml for Enterprise with API key + secret
server: https://developer.orkescloud.com/api
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
orkes --config /path/to/my-config.yaml workflow list
```

## Workflow Metadata Management

```shell
# List the workflows on the server
orkes workflow list

# Get the workflows definition - fetches the latest version
orkes workflow get <workflowname>

# or you can specify a version
orkes workflow get <workflowname> <version>

# You can use quotes for workflow name if the name contains spaces, comma or special characters
orkes workflow get "<workflow name with spaces>"

```
### Create a workflow
```shell
# Register a workflow stored in the file
orkes workflow create /path/to/workflow_definition.json --force # use --force to overwrite existing
```

## Task Workers

⚠️ **EXPERIMENTAL FEATURES**

The CLI supports two types of task workers for processing Conductor tasks:

### Generic Workers (Any Language)

Execute tasks using external programs written in **any language** (Python, Node.js, Go, Rust, shell scripts, etc.). The CLI polls for tasks and passes them to your worker via stdin/stdout.

**Best for:** Complex logic, heavy dependencies, full language ecosystem access

👉 **[Complete Generic Worker Documentation →](WORKER_EXEC.md)**

**Quick example:**
```bash
# Run a Python worker (continuous polling with parallel execution)
orkes worker exec --type greet_task python3 worker.py

# Poll multiple tasks per batch for higher throughput
orkes worker exec --type greet_task python3 worker.py --count 5
```

### JavaScript Workers (Built-in)

Execute tasks using **JavaScript** scripts with built-in utilities (HTTP, crypto, string functions). No external dependencies needed.

**Best for:** Lightweight tasks, quick scripts, HTTP integrations

👉 **[Complete JavaScript Worker Documentation →](WORKER_JS.md)**

**Quick example:**
```bash
# Run a JavaScript worker
orkes worker js --type greet_task worker.js
```

## Exit Codes

The CLI uses standard exit codes for error handling:

| Exit Code | Description |
|-----------|-------------|
| `0` | Command completed successfully |
| `1` | General error (connection failed, authentication error, resource not found, etc.) |

**Example usage in scripts:**
```bash
if orkes execution start --workflow my_workflow; then
    echo "Workflow started successfully"
else
    echo "Failed to start workflow" >&2
    exit 1
fi
```

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

## For AI Assistants & LLMs

For a concise, LLM-optimized reference with command tables, exit codes, and canonical examples, see **[CLAUDE.md](./CLAUDE.md)**.
