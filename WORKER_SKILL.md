# Skill Workers

A **skill** is a directory containing instructions and scripts. Registered with the server
it becomes an agent, but its tools still have to run on your machine — so the server
dispatches each tool call back to the CLI as a Conductor task, and the CLI runs it locally
and returns the result.

That local half is a worker. It shares the poll loop with
[stdio workers](WORKER_STDIO.md) and [JavaScript workers](WORKER_JS.md), and it works
with or without an agent: because a skill tool is just a Conductor task type, an ordinary
workflow can call one directly.

## Skill layout

```
myskill/
  SKILL.md            # required; frontmatter must set `name`
  scripts/
    greet.sh          # each script becomes a tool
```

```markdown
---
name: greetskill
description: Says hello
---

# Greet skill

Instructions the agent reads.
```

```bash
#!/usr/bin/env bash
echo "Hello $1"
```

Script language is chosen by extension: `.py`, `.sh`, `.js`, `.mjs`, `.ts`, `.rb`, `.go`,
`.bat`, `.cmd`. Anything else is run with `bash`.

## Serving the tools

```bash
conductor skill serve ./myskill
# Serving workers for skill greetskill. Press Ctrl-C to stop.
```

`skill serve` starts one worker per tool and blocks. Use it when the skill is being run
somewhere else — from the UI, or by another process. `conductor skill run <skill> <prompt>`
starts the workers *and* runs the agent, stopping the workers when the run ends.

## Task types

Every tool is exposed as the task type:

```
{skillName}__{tool}
```

So `greetskill` with `scripts/greet.sh` serves `greetskill__greet`. Alongside the scripts,
these built-in tools are served too:

| Tool | Task type | Purpose |
|---|---|---|
| `read_skill_file` | `{skill}__read_skill_file` | Read a file bundled with the skill |
| `list_workspace` | `{skill}__list_workspace` | List files in the workspace |
| `read_workspace_file` | `{skill}__read_workspace_file` | Read a workspace file |
| `search_workspace` | `{skill}__search_workspace` | Search the workspace |
| `git_status` | `{skill}__git_status` | Workspace git status |
| `git_diff` | `{skill}__git_diff` | Workspace git diff |

The workspace tools are only served when a workspace is enabled; `--no-workspace` disables
them.

## Tool contract

Different from stdio workers, and simpler:

**Input** — `inputParameters.command` is passed to the script as **arguments**, not on
stdin. Only that field reaches the script.

**Output** — the script's **stdout** becomes the task output, wrapped as
`{"result": "<stdout>"}`. There is no envelope to emit.

**Failure** — a non-zero exit fails the task, with stderr in the failure reason.

**Environment** — the skill root and the configured workspace roots are exported to the
script.

## Using a skill tool from a plain workflow

Nothing about this requires an agent. Point a `SIMPLE` task at the tool's task type:

```json
{
  "name": "skill_as_worker",
  "version": 1,
  "tasks": [
    {
      "name": "greetskill__greet",
      "taskReferenceName": "g",
      "type": "SIMPLE",
      "inputParameters": { "command": "${workflow.input.name}" }
    }
  ]
}
```

```bash
conductor skill serve ./myskill &
conductor workflow start --workflow skill_as_worker --input '{"name":"Miguel"}' --sync
# { "result": "Hello Miguel\n" }
```

This makes a skill the lowest-ceremony way to run a script as a Conductor worker: no
result envelope, no SDK, and no protocol to implement.

Two constraints to know before relying on it:

- **The task type is fixed** as `{skill}__{tool}`, so an existing workflow cannot adopt a
  skill tool without renaming its task.
- **Input is a single string.** Structured `inputData` does not reach the script; only
  `command` does. Use a [stdio worker](WORKER_STDIO.md) when the task needs structured
  input.

## Flags

| Flag | Applies to | Purpose |
|---|---|---|
| `--version` | run, serve | Skill version or checksum prefix |
| `--script-timeout` | run, serve | Per-script timeout in seconds (default 300) |
| `--script-output-limit` | run, serve | Max bytes captured from a script (default 10 MiB) |
| `--workspace` | run, serve | Workspace directory (default `.`) |
| `--no-workspace` | run, serve | Do not expose a workspace |
| `--filesystem name=path` | run, serve | Extra read-only root, repeatable |
| `--model` | run | Model for the agent (required for `run`) |
| `--param` | run | Skill parameter override, repeatable |

## Comparison with other worker types

| | Skill tools | [Stdio](WORKER_STDIO.md) | [JavaScript](WORKER_JS.md) |
|---|---|---|---|
| Task type | `{skill}__{tool}` | any | any |
| Input | `command` → argv | full task JSON on stdin | `$.task` |
| Output | bare stdout | `{status, output, logs, reason}` | `{status, body}` |
| Boilerplate | none | result envelope | result object |
| Structured input | no | yes | yes |
| Languages | by extension | any executable | JavaScript only |
