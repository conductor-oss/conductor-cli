"""Module-level Agent variables are what `agentspan.cli.discover` scans for.

Kept deliberately minimal: these agents are only ever discovered and deployed by
the E2E suite, never executed, so the model is nominal.
"""

from agentspan.agents import Agent

e2e_fixture_greeter = Agent(
    name="e2e_fixture_greeter",
    model="anthropic/claude-haiku-4-5-20251001",
    instructions="You greet the user in exactly one short word.",
    max_turns=2,
)

e2e_fixture_echo = Agent(
    name="e2e_fixture_echo",
    model="anthropic/claude-haiku-4-5-20251001",
    instructions="You echo the user's input verbatim.",
    max_turns=2,
)
