# R1 module boundaries

## Modules

Status: target design, not current package layout. The [built layers](../../src/MODULES.md#modules) are prerequisites.

| Extension | Proposed boundary |
|---|---|
| Runner | Core lifecycle logic assembled in a separate program; [runner](runner-implementation.md#what-the-runner-does) owns it |
| Client libraries | Built, six languages: [interface](done/client-libraries.md#scope); the accepted specification approach is [protocol conformance](#protocol-conformance) (№Q17) |

Sessions and AUTH distribution are [R1.1's boundaries](../R1.1/modules.md#modules), with the hot-path rule for per-message cryptography;
database and dump alternatives are [unassigned storage
work](../R2.0-Future/storage.md#storage).

## What this buys

A dependency changes at its adapter boundary. The current [layer rule](../../src/MODULES.md#the-rule) applies to these additions too.

Open choices are in [questions](QUESTIONS.md#open-questions).

## Protocol conformance

Owner-confirmed 2026-10-17, [№RD90](DECISIONS.md#recorded-decisions): specify
the supported protocol through shared executable request/answer cases, starting
with the calls the six existing clients already implement. Each language must
reproduce the same meaning; native types and execution models remain its own.
Explanatory prose can accompany those cases. A schema or generator is deferred,
not approved by this decision. No new wire shape is defined here.

The libraries and their existing checks are built; the common conformance-first
specification is unfinished [№RT8](TODO.md#objective).
