# R1.1 module boundaries

## Modules

Status: target design, not current package layout. The [built layers](../../src/MODULES.md#modules) are prerequisites.

| Extension | Proposed boundary |
|---|---|
| Sessions | Core session logic for handshake, derivation and key confirmation; [crypto](access.md#encrypted-sessions) owns its contract |
| AUTH distribution | VCS port and git adapter; [AUTH](auth.md#topology) owns distribution |

What a boundary buys is with [R1's module boundaries](../R1.0-Release/modules.md#what-this-buys).

## The hot path

Per-message cryptography stays in process using established libraries. A subprocess
per message is outside the target budget; do not write crypto primitives.
