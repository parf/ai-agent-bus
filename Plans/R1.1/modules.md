# R1.1 module boundaries

Status: target design, not current package layout. The [built layers](../../src/MODULES.md#modules) are prerequisites.

| Extension | Proposed boundary |
|---|---|
| Sessions | Core session logic for handshake, derivation and key confirmation; [crypto](access.md#encrypted-sessions) owns its contract |
| AUTH distribution | VCS port and git adapter; [AUTH](auth.md#topology) owns distribution |

The hot-path rule and what a boundary buys are with [R1's module
boundaries](../R1.0-Release/modules.md#modules).
