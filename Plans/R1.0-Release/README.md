# R1

## Scope

Partly built ahead of the release on the 0.8 line — shared locks in 0.8.66, Resource records in 0.8.70 and the key-value store in 0.8.77, roles in 0.8.110, the six client libraries in 0.8.100 and record tags in 0.8.143 ([done](DONE.md)); the [runner](../../docs/runner/README.md) has managed processes, per-agent controls, host-file editing and the unconfigured state ([waves](TODO.md#runner-waves)). The release's own scope: the pieces that make one bus a
connectable, installable building block — record-defined roles, its shared
extensions (locks, key-value, Resource records, method metadata), record tags, installable distribution, the runner, and the client
libraries. The selected [bundled agents](agents.md#what-moved-and-what-r1-adds)
are also R1 work.
Identity, AUTH, encryption, observability, operations, storage
backends and the peer registry are [R1.1](../R1.1/README.md#scope); the rest of
the tools stage is [R1.2](../R1.2/README.md#scope).

| Topic | Canonical knowledge |
|---|---|
| Installable distributions | [Release artifacts](distribution.md#release-artifacts) |
| Federation moved to R1.2 | [Chaining](../R1.2/federation.md#chaining), owner decision 2026-10-17 |
| Shared locks | [Shared locks](done/locks.md#shared-locks) |
| Key-value store | [Key-value store](done/kv.md#per-record-storage) |
| MCP Resource records | [Resource records](done/resources.md#resource-records) |
| Runner | Partly built, wave by wave ([runner waves](TODO.md#runner-waves)): [current user guide](../../docs/runner/README.md). Future scope: [runner](runner.md#runner), [configuration design](runner-config.md#where-an-option-lives) and [implementation](runner-implementation.md#what-the-runner-does) |
| Runners in the web face | [User journeys and checklist](runner/web-face.md#goal) |
| Record tags | [Classification and filtering](tags.md#classification-and-filtering) |
| Agents for the runner | [What R1 adds](agents.md#what-moved-and-what-r1-adds); the shared shape stays in the [catalogue](../R1.2/services.md#rules-they-all-obey) |
| RAG service | [RAG](rag/README.md#rag-service), built from agents as blocks; steps in [its TODO](rag/TODO.md#steps) |
| Record-defined roles | [Roles](roles.md#record-defined-roles) |
| Daemon configuration | [RT22 proposal, pending approval](daemon-configuration.md#approved-plan) |
| Method metadata | [Method metadata](done/method-metadata.md#method-metadata) |
| Client libraries | [Client libraries](done/client-libraries.md#scope) |

Open choices are in [questions](QUESTIONS.md#open-questions); recorded choices
are in [decisions](DECISIONS.md#recorded-decisions). Execution prerequisites
are in [TODO](TODO.md#objective).
