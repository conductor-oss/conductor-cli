# CLI for Conductor

[Conductor](https://www.conductor-oss.org/) is the leading open-source orchestration platform allowing developers to build highly scalable distributed applications. 

Check out the [official documentation for Conductor](https://orkes.io/content).

This repository provides a Java client for the Orkes Conductor Server. 

## ⭐ Conductor OSS

Show support for the Conductor OSS.  Please help spread the awareness by starring Conductor repo.

[![GitHub stars](https://img.shields.io/github/stars/conductor-oss/conductor.svg?style=social&label=Star&maxAge=)](https://GitHub.com/conductor-oss/conductor/)


## Server Commands

### Work with a server
```shell
# cdt works with the environment variables to connect to the server
export CONDUCTOR_SERVER_URL=http://server:port/api

########################################
### REQUIRED when using Orkes hosted ###
########################################
# When using an Orkes server which requires api key/secret, set the following env variables 
export CONDUCTOR_AUTH_KEY=api_key
export CONDUCTOR_AUTH_SECRET=api_key_secret

# Optionally, you can copy the auth token from the conductor UI and export it as such (useful for quick testing)
export CONDUCTOR_AUTH_TOKEN=auth_token
```


## Workflow Metadata Management

```shell
# List the workflows on the server
cdt workflow list

# Get the workflows definition - fetches the latest version
cdt workflow get <workflowname>

# add a version with a comma in the name to get the specific version
cdt workflow get <workflowname>,<verion>

# You can use quotes for workflow name if the name contains spaces, comma or special characters
cdt workflow get "<workflowname with spaces>"

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
ccli code generate -n <name> -l <language> -t <type> -b <boilerplate>
```
Example:
```shell
ccli code generate -n myapp -l javascript -t worker
```
