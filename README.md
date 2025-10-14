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

- **OSS Conductor** (default): Open-source Conductor - requires only server URL, no authentication
- **Enterprise (Orkes Conductor)**: Requires server URL and authentication credentials

Use the `--server-type` flag to specify your server type (defaults to `OSS`):

```bash
# OSS Conductor (default)
orkes --server http://localhost:8080/api --server-type OSS workflow list

# Enterprise/Orkes Conductor
orkes --server https://developer.orkescloud.com --auth-token your-token --server-type Enterprise workflow list
```

### Saving Your Configuration

The `config save` command allows you to persist your server URL and credentials (for Enterprise) so you don't have to provide them with every command. This creates a configuration file in `~/.conductor-cli/`.

**For OSS Conductor:**

```bash
# Save OSS configuration (no authentication required)
orkes --server http://localhost:8080/api --server-type OSS config save

# Since OSS is the default, you can omit --server-type
orkes --server http://localhost:8080/api config save
```

**For Enterprise/Orkes Conductor:**

You must use **one** of the following authentication methods:

1. **API Key + Secret**: Use both `--auth-key` and `--auth-secret` flags together
2. **Auth Token**: Use `--auth-token` flag (you can copy this token from the Conductor UI)

```bash
# Save Enterprise configuration with auth token
orkes --server https://developer.orkescloud.com --auth-token your-token --server-type Enterprise config save

# Save with API key + secret
orkes --server https://developer.orkescloud.com --auth-key your-key --auth-secret your-secret --server-type Enterprise config save

# Flags can be placed before or after the command
orkes config save --server https://developer.orkescloud.com --auth-token your-token --server-type Enterprise
```

Once saved, you can run commands without providing flags:

```bash
# After saving config, simply run:
orkes workflow list
```

**Additional Configuration Options:**

```bash
# Save with verbose logging enabled
orkes --server http://localhost:8080/api --verbose config save
```

**Note:** Server URLs can be provided with or without `/api` suffix (e.g., `http://localhost:8080` or `http://localhost:8080/api`).

### Using Profiles for Multiple Environments

Profiles allow you to manage multiple Conductor environments (e.g., development, staging, production) and easily switch between them.

**Creating Profiles:**

Use the `--profile` flag with `config save` to create named profiles:

```bash
# Save local OSS development environment
orkes --server http://localhost:8080/api --server-type OSS --profile dev config save

# Save Enterprise staging environment
orkes --server https://staging.example.com --auth-token staging-token --server-type Enterprise --profile staging config save

# Save Enterprise production environment
orkes --server https://prod.example.com --auth-token prod-token --server-type Enterprise --profile production config save
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

# Combine multiple overrides
orkes --server http://localhost:8080/api --auth-token token --server-type Enterprise workflow list
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

# Server type (OSS or Enterprise)
export CONDUCTOR_SERVER_TYPE=Enterprise

# Profile selection
export ORKES_PROFILE=production
```

### Configuration File Format

Configuration files use YAML format and are stored in `~/.conductor-cli/`:

```yaml
# Example config.yaml for OSS Conductor (no authentication)
server: http://localhost:8080/api
server-type: OSS
verbose: false
```

```yaml
# Example config.yaml for Enterprise with auth token
server: https://developer.orkescloud.com/api
auth-token: your-auth-token
server-type: Enterprise
verbose: false
```

```yaml
# Example config.yaml for Enterprise with API key + secret
server: https://developer.orkescloud.com/api
auth-key: your-api-key
auth-secret: your-api-secret
server-type: Enterprise
verbose: false
```

**Notes:**
- `server-type` defaults to `OSS` if not specified
- OSS Conductor doesn't require `auth-token`, `auth-key`, or `auth-secret`
- Enterprise requires one authentication method (`auth-token` OR `auth-key`+`auth-secret`)

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
