# Systemd agent

📌 **TL;DR:** Systemd runs a fixed-argv script through the standalone starter; caller tiers and Configuration-defined coverage are checked before execution.
Use the [user guide](../../../../docs/agents/systemd.md)
for setup, calls and settings.

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

## Authored help

`agent.json` owns methods, typed arguments and Configuration descriptions.
`env.dist` owns environment-key comments. This package declares `"help":"native"` so
its handler applies runtime Configuration scope and delivered roles to all four
help asks, using the shared manifest helper rather than a second method list. Bundled agents omit `version` and use the bus version.
Executable-grammar checks compare the methods with generated help.
