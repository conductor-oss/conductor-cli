# CLI for Conductor

[Conductor](https://www.conductor-oss.org/) is the leading open-source orchestration platform allowing developers to build highly scalable distributed applications. 

Check out the [official documentation for Conductor](https://orkes.io/content).

This repository provides a Java client for the Orkes Conductor Server. 

## ⭐ Conductor OSS

Show support for the Conductor OSS.  Please help spread the awareness by starring Conductor repo.

[![GitHub stars](https://img.shields.io/github/stars/conductor-oss/conductor.svg?style=social&label=Star&maxAge=)](https://GitHub.com/conductor-oss/conductor/)


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
cdt --server http://localhost:8080/api --auth-key your-api-key --auth-secret your-api-secret workflow list

# Option 2: Auth Token (copy from UI)
cdt --server http://localhost:8080/api --auth-token your-auth-token workflow list

# Using config file
cdt --config /path/to/config.yaml workflow list
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
cdt --config /path/to/my-config.yaml workflow list
```

## Workflow Metadata Management

```shell
# List the workflows on the server
cdt workflow list

# Get the workflows definition - fetches the latest version
cdt workflow get <workflowname>

# or you can specify a version
cdt workflow get <workflowname> <version>

# You can use quotes for workflow name if the name contains spaces, comma or special characters
cdt workflow get "<workflow name with spaces>"

```
### Create a workflow
```shell
# Register a workflow stored in the file
cdt workflow create /path/to/workflow_definition.json --force # use --force to overwrite existing   
```
## Code Generation
```shell
# Generate a project of type (worker/application) in a particular language from a boilerplate (default is core)
# See https://github.com/conductor-sdk/boilerplates for available boilerplates
cdt code generate -n <name> -l <language> -t <type> -b <boilerplate>
```
Example:
```shell
cdt code generate -n myapp -l javascript -t worker
```
