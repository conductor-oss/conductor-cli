# Conductor CLI

A command-line client for Conductor. It talks to remote Conductor servers, runs a
Conductor server locally for development, and manages the workflow, task and agent
resources those servers hold.

## Language

### Server

**OSS Conductor**:
The open-source Conductor distribution. Accepts anonymous requests and does not
implement the commercial resource APIs.
_Avoid_: community edition, open Conductor, vanilla Conductor

**Orkes Conductor**:
The commercial Conductor distribution. Requires authentication and serves
resources absent from OSS Conductor.
_Avoid_: Enterprise Conductor (except as the literal `Enterprise` server-type
value), Orkes Cloud, hosted Conductor

**Server type**:
Which distribution an invocation is addressing. It selects client behaviour, not
merely an address — the same command can be valid against one distribution and
refused by the other.
_Avoid_: mode, flavour, edition

**Orkes-only**:
A command or capability that exists solely on Orkes Conductor and is refused
outright by OSS Conductor.
_Avoid_: enterprise-only, premium, paid

**Local server**:
A Conductor server that the CLI downloads and runs as a background process on the
user's machine for development and testing.
_Avoid_: embedded server, dev server, test server

### Configuration

**Profile**:
A named, persisted set of server and authentication settings, selected per
invocation.
_Avoid_: config, environment, context, target

**Default profile**:
The unnamed profile read when no profile is selected.
_Avoid_: global config, base config

### Resources

**Definition**:
The registered, versioned template for a workflow or task.
_Avoid_: spec, schema, metadata, template

**Execution**:
A single running or completed instance of a definition.
_Avoid_: run, instance, job, invocation

**Agent**:
A model-backed unit of work whose turns are carried out by the server rather than
by the CLI. Consequently the *server* needs the model provider credential; the CLI
only starts and observes.
_Avoid_: assistant, bot, LLM

**Worker**:
A process that polls the server for tasks of a given type and executes them on the
machine where it runs. The counterpart to an Agent: a Worker runs locally, an Agent
runs server-side.
_Avoid_: consumer, poller, runner

### Testing

**Venue**:
The kind of server a test runs against — OSS Conductor or Orkes Conductor. It
determines which tests are *valid*, not merely where they execute: a test asserting
that anonymous access is rejected is meaningless against a server that permits it.
_Avoid_: environment, target, stage

**Tier**:
When a test runs — on every pull request, or on the nightly schedule. Tests that
cost money, depend on a third party, or are non-deterministic belong to the nightly
tier.
_Avoid_: suite, level, category, stage

**OSS-safe**:
A test that is valid against a local OSS Conductor server.
_Avoid_: local test, offline test

**Known-broken guard**:
A test that asserts the *correct* behaviour of an already-filed defect and is
skipped with that issue's reference, so the gap is executable documentation and
un-skipping it is the whole fix-verification step.
_Avoid_: expected failure, xfail, pending test
