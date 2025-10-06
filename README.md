# CLI for Conductor

[Conductor](https://www.conductor-oss.org/) is the leading open-source orchestration platform allowing developers to build highly scalable distributed applications.

Check out the [official documentation for Conductor](https://orkes.io/content).

This repository provides a CLI for the Orkes Conductor Server.

## ⭐ Conductor OSS

Show support for the Conductor OSS.  Please help spread the awareness by starring Conductor repo.

[![GitHub stars](https://img.shields.io/github/stars/conductor-oss/conductor.svg?style=social&label=Star&maxAge=)](https://GitHub.com/conductor-oss/conductor/)

## Installation

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

Create a `.conductor-cli.yaml` file in your home directory (`$HOME/.conductor-cli.yaml`).

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
