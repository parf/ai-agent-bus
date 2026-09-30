# R1

## Scope

Not started. The release's own scope: the pieces that make one bus a
connectable, installable building block — its shared extensions (locks,
key-value, Resource records, method metadata), federation, installable
distribution, the managed runner, and the client libraries. Identity, AUTH,
encryption and observability are [R1.1](../R1.1/README.md#scope); the tools
stage is [R1.2](../R1.2/README.md#scope).

| Topic | Canonical knowledge |
|---|---|
| Installable distributions | [Release artifacts](distribution.md#release-artifacts) |
| Federation | [Federation](federation.md#chaining) |
| Shared locks | [Shared locks](locks.md#shared-locks) |
| Key-value store | [Key-value store](kv.md#per-name-storage) |
| MCP Resource records | [Resource records](resources.md#resource-records) |
| Managed runner | [Managed runner](runner.md#what-the-runner-does) |
| Record-defined roles | [Roles](roles.md) |
| Method metadata | [Method metadata](method-metadata.md) |
| Client libraries (Go, PHP, Python, Rust, TS) | [Client libraries](modules.md#modules) |

Open choices are in [questions](QUESTIONS.md#open-questions); recorded choices
are in [decisions](DECISIONS.md#recorded-decisions). Execution prerequisites
are in [TODO](TODO.md#objective).
