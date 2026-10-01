# R1 module boundaries

## Modules

Status: target design, not current package layout. The [built layers](../../src/MODULES.md#modules) are prerequisites.

| Extension | Proposed boundary |
|---|---|
| Managed runner | Core lifecycle logic assembled in a separate program; [runner](runner.md#what-the-runner-does) owns it |
| Client libraries | [Proposed interface](client-libraries.md#scope); shared protocol description must be approved first ([Q17](QUESTIONS.md#open-questions)) |

Sessions and AUTH distribution are [R1.1's boundaries](../R1.1/modules.md#modules), with the hot-path rule for per-message cryptography;
database and dump alternatives are [unassigned storage
work](../R2.0-Future/storage.md#storage).

## What this buys

A dependency changes at its adapter boundary. The current [layer rule](../../src/MODULES.md#the-rule) applies to these additions too.

Open choices are in [questions](QUESTIONS.md#open-questions).
