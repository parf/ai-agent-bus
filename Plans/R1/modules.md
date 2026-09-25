# Future module boundaries

## Modules

Status: target design, not current package layout. The [built layers](../../docs/10-modules.md#modules) are prerequisites.

| Extension | Proposed boundary |
|---|---|
| Sessions | Core session logic for handshake, derivation and key confirmation; [crypto](access.md#encrypted-sessions) owns its contract |
| AUTH distribution | VCS port and git adapter; [AUTH](auth.md#topology) owns distribution |
| Managed runner | Core lifecycle logic assembled in a separate program; [runner](runner.md#what-the-runner-does) owns it |
| Client libraries | Go, PHP, Rust, JS and Python; shared protocol description must be approved first |

Database and dump alternatives are [unassigned storage work](../R2.0/storage.md#storage).

## The hot path

Per-message cryptography stays in process using established libraries. A subprocess
per message is outside the target budget; do not write crypto primitives.

## What this buys

A dependency changes at its adapter boundary. The current [layer rule](../../docs/10-modules.md#the-rule) applies to these additions too.

Open choices are in [questions](QUESTIONS.md#open-questions).
