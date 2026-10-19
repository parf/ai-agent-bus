# Debug agent implementation

📌 **TL;DR:** debug.py is a dependency-free args-mode script. It reads one
body argument and prints its answer; the runner handles registration,
authority and message delivery. It never holds a bus credential.
See the [debug user guide](../../../docs/agents/debug.md).

## Implementation

hello and env are text-only commands. Help and MCP requests are recognized
through AGENT_BUS_ASK; the MCP answer declares no tools. Environment output masks private values; only bus-delivered fields and roles remain visible.
The shipped agent.json uses a zero-to-one process range: idle between calls.

## Verification

The runner's TestTheShippedAgentsLoad validates its agent.json. Script smoke
checks exercise the real runner and environment delivery; it needs no bundle.

`env` masks every value as `***` except `AGENT_BUS_*` and `AB_ROLE_*`, the bus-delivered envelope and roles. Host and Secret values are never exposed by this command.
