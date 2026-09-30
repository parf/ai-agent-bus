# R1.1

## Scope

Not started. The distributed-identity half of the old R1: identity and policy,
AUTH distribution, scoped credentials and encryption, discovery and
observability, operations, and the additional storage backends. The release's
own extensions — federation, locks, the key-value store, Resource records,
method metadata, the managed runner, the client libraries and installable
distribution — stayed in [R1](../R1.0-Release/README.md#scope); the tools
stage is [R1.2](../R1.2/README.md#scope).

| Topic | Canonical knowledge |
|---|---|
| Identity extensions (sigils, delegation, ownership) | [Identity extensions](identity.md#sigils) |
| AUTH distribution | [AUTH distribution](auth.md#bundle) |
| Scoped credentials and encryption | [Scoped credentials and encryption](access.md#token-scope) |
| Encryption waves | [Encryption waves](encryption-wave.md#acceptance) |
| Discovery and observability | [Discovery and observability](discovery.md#where-a-member-says-it-is) |
| One front door (proposed) | [One front door](discovery.md#one-front-door) |
| Operations | [Operations](operations.md#reload) |
| Storage backends | [Storage backends](storage.md#backends) |

Open choices are in [questions](QUESTIONS.md#open-questions); recorded choices
are in [decisions](DECISIONS.md#recorded-decisions). Execution prerequisites
are in [TODO](TODO.md#objective).
