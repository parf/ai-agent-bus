# Server-info agent

📌 **TL;DR:** Server-info runs a fixed read-only command inventory through the standalone starter; availability follows the process account.
Implementation and verification notes stay here; use the [user guide](../../../../docs/agents/server-info.md)
for setup, calls and settings.

## Calls

See the [calls guide](../../../../docs/agents/server-info.md#calls).

## Sections

See the [sections guide](../../../../docs/agents/server-info.md#calls).

## Install and remove

See the [install and remove guide](../../../../docs/agents/server-info.md#installing-it).

## Help

See the [help guide](../../../../docs/agents/server-info.md#calls).

## Implementation

server-info.py accepts exactly one known method; caller arguments never reach
a shell or command vector. Command results preserve exit status and stderr.
Health and sensors availability is determined from effective uid. agent.json
owns the record description; shared installation is not copied into this agent.

## Verification


The `agent_server_info` shard runs inside `src/smoke.sh --slow`. It checks
fixed commands and refusal before execution, privileged section availability,
real bus calls against the host network, install twice, restart, graceful
removal and removal after a killed runner. Generated units pass
`systemd-analyze verify`; tests use a recording systemd stand-in and a private
daemon, never install a unit on the development node.
