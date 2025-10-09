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

## Configuration

The CLI can be configured using command-line flags, environment variables, or a configuration file. Configuration is handled with the following precedence (highest to lowest):

1. Command-line flags
2. Environment variables
3. Configuration file

### Command-line Flags

**Authentication Options:**

You must use **one** of the following authentication methods:

1. **API Key + Secret**: Use both `--auth-key` and `--auth-secret` flags together
2. **Auth Token**: Use `--auth-token` flag (you can copy this token from the Conductor UI)

```bash
# Option 1: API Key + Secret
orkes --server http://localhost:8080/api --auth-key your-api-key --auth-secret your-api-secret workflow list

# Option 2: Auth Token (copy from UI)
orkes --server http://localhost:8080/api --auth-token your-auth-token workflow list

# Using config file
orkes --config /path/to/config.yaml workflow list
```

### Environment Variables

**Authentication Options:**

Use **one** of the following authentication methods:

```bash
# Option 1: API Key + Secret
export CONDUCTOR_SERVER_URL=http://localhost:8080/api
export CONDUCTOR_AUTH_KEY=your-api-key
export CONDUCTOR_AUTH_SECRET=your-api-secret

# Option 2: Auth Token (copy from UI)
export CONDUCTOR_SERVER_URL=http://localhost:8080/api
export CONDUCTOR_AUTH_TOKEN=your-auth-token
```

### Configuration File

Configuration files are stored in `~/.conductor-cli/` directory.

**Authentication Options:**

Use **one** of the following authentication methods:

```yaml
# Option 1: API Key + Secret
server: http://localhost:8080/api
auth-key: your-api-key
auth-secret: your-api-secret
verbose: false
```

```yaml
# Option 2: Auth Token (copy from UI)
server: http://localhost:8080/api
auth-token: your-auth-token
verbose: false
```

You can also specify a custom config file location:

```bash
orkes --config /path/to/my-config.yaml workflow list
```

### Profiles

Profiles allow you to manage multiple Conductor environments (e.g., development, staging, production) easily.

**Creating a Profile:**

Save your current flags to a named profile using the `config save` command:

```bash
# Save to default profile (~/.conductor-cli/config.yaml)
orkes --server https://dev.example.com --auth-key key123 config save

# Save to named profile (~/.conductor-cli/config-production.yaml)
orkes --server https://prod.example.com --auth-token token --profile production config save
```

**Deleting a Profile:**

Delete a configuration file using the `config delete` command:

```bash
# Delete default config (with confirmation prompt)
orkes config delete

# Delete named profile (with confirmation prompt)
orkes config delete production

# Delete without confirmation
orkes config delete production -y
```

**Using a Profile:**

Load configuration from a specific profile:

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

**Profile Error Handling:**

If you reference a profile that doesn't exist, you'll get a clear error:

```bash
orkes --profile nonexistent workflow list
# Error: Profile 'nonexistent' doesn't exist (expected file: ~/.conductor-cli/config-nonexistent.yaml)
```

## Config Management Commands

The CLI provides dedicated commands for managing configuration files:

### Save Configuration

```bash
# Save to default config file
orkes --server http://localhost:8080/api --auth-key key123 config save

# Save to a named profile (using --profile flag)
orkes --server https://prod.example.com --auth-token token --profile production config save

# Flags can be placed before or after the command
orkes config save --server http://localhost:8080/api --auth-key key123 --profile staging
```

### List Configurations

```bash
# List all configuration profiles
orkes config list
```

This shows:
- `default` - for the default `config.yaml` file
- Profile names (e.g., `production`, `staging`) - for named profiles like `config-production.yaml`

### Delete Configuration

```bash
# Delete default config (with confirmation prompt)
orkes config delete

# Delete named profile using positional argument
orkes config delete production

# Delete named profile using --profile flag
orkes config delete --profile production

# Delete without confirmation using -y flag
orkes config delete production -y
orkes config delete --profile staging -y
```

**Notes:**
- The `--profile` flag specifies which profile to save/delete
- Without `--profile`, operations affect the default `config.yaml`
- `config list` shows all available profiles in `~/.conductor-cli/` directory
- Delete operations require confirmation unless `-y` flag is used
- Both positional argument and `--profile` flag work for delete command

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
