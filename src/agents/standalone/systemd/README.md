# Systemd agent

📌 **TL;DR:** Systemd runs a fixed-argv script through the standalone starter; caller tiers and Configuration-defined coverage are checked before execution.
Implementation and verification notes stay here; use the [user guide](../../../../docs/agents/systemd.md)
for setup, calls and settings.

## Calls

See the [calls guide](../../../../docs/agents/systemd.md#calls-and-permissions).

## Logs

See the [logs guide](../../../../docs/agents/systemd.md#logs).

## Dedicated instances

See the [dedicated instances guide](../../../../docs/agents/systemd.md#dedicated-instances).

## Install

See the [install guide](../../../../docs/agents/systemd.md#installing-it).

## Many nodes

See the [many nodes guide](../../../../docs/agents/systemd.md#many-nodes).

## Implementation

Exact unit validation, role interpretation and fixed systemctl/journalctl
vectors belong in systemd.py. No body field grants authority. Configuration
is validated at script startup; malformed units never widen coverage. The
root/nobody fixture checks actual process privilege as well as application tiers.

## Verification


The slow `agent_systemd` shard checks unconfigured services and timers, every
role tier, strict names, typed listing, failed-unit columns, logs, body forgery,
installation idempotence and removal against a private bus. Its unit controller
uses real root and nobody processes; its systemctl stand-in writes a root-owned
state file. Losing root makes every acting verb return a permission error while
read methods still work. This is an isolated privilege check, not a live systemd
deployment; it never changes host units. Mutants remove role enforcement, trust
a body role or hide a systemctl failure; each must fail its named assertion.
