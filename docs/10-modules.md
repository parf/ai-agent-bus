# Layers and modules

How the code is organised. Not a package listing — the rule that decides where
a thing goes, and the boundaries that have to hold if we want to replace one
piece without touching the rest.

Concrete interfaces are deferred with the data models; this document fixes the
*boundaries*, which is what has to be right first.

## The rule

**Dependencies point inward. Only the outermost layer touches the outside
world.**

| Layer | Contains | May depend on | Touches the outside world |
|---|---|---|---|
| **protocol** | the envelope, encoding, handshake framing, name and address parsing | nothing | no |
| **ports** | the interfaces core needs — a store, a directory, a balance, a sandbox, a clock | protocol | no |
| **core** | the domain: registry, queues, sessions, ACL resolution, accounting, supervision | protocol, ports | no |
| **adapters** | one implementation per port | protocol, ports | **yes — and nothing else does** |
| **faces** | API, MCP, WEB, CLI — translate a request from outside into a core call | protocol, core | yes (inbound only) |

Consequences worth stating, because they are the point:

- **A dependency is swappable exactly when it sits behind a port.** SQLite →
  PostgreSQL is one new adapter and no change anywhere else. That is the shape
  every external dependency gets.
- **Core never imports an adapter.** If it needs one, it needs a port instead.
- **Only adapters know a protocol exists.** No `database/sql`, no HTTP client,
  no `exec.Command` outside that layer.
- **protocol is what the client libraries reimplement.** Go, PHP, Rust, JS and
  Python each need it and nothing below it, so it stays free of Go-isms and of
  every other layer.

## Modules

| Module | Layer | Owns | Doc |
|---|---|---|---|
| `protocol` | protocol | envelope, JSON/msgpack encoding, `user@realm` and `name@host` parsing | [messaging § envelope](04-messaging.md#envelope) |
| `registry` | core | service and topic records, ownership, audience | [services and topics](03-services-and-topics.md) |
| `queues` | core | inboxes, topics, TTL, overflow, delivery | [messaging](04-messaging.md) |
| `identity` | core | principals, groups, roles, the two ACL layers | [identity](01-identity.md) |
| `session` | core | handshake, key derivation, AEAD, key confirmation | [access § encrypted sessions](02-access.md#encrypted-sessions) |
| `tokens` | core | issue, persist, current + previous, expiry policy | [access § token lifetime](02-access.md#token-lifetime) |
| `runner` | core | supervision, restart policy, child lifecycle | [runner role](08-runner-role.md) |
| `billing` | core | usage accounting, allow/deny | [billing role](07-billing-role.md) |
| `store` | **port** | everything that persists in the database | [setup § storage](09-setup.md#storage) |
| `directory` | **port** | fetch a name and public keys for a login | [identity § registration](01-identity.md#registration) |
| `balance` | **port** | may this principal call this service; record usage | [billing role](07-billing-role.md) |
| `sandbox` | **port** | confine a child process | [runner § sandboxing](08-runner-role.md#sandboxing) |
| `dump` | **port** | snapshot and reload in-memory state | [messaging § durability](04-messaging.md#durability) |
| `vcs` | **port** | push and pull the git repo | [AUTH role § topology](06-auth-role.md#topology) |
| `store/sqlite`, `store/postgres` | adapter | the one place SQL is written | |
| `directory/github` | adapter | shells out to `curl` | |
| `balance/radius` | adapter | shells out to the standard client | |
| `sandbox/systemd`, `sandbox/bwrap`, `sandbox/unshare` | adapter | one per backend, chosen by environment | |
| `dump/parquet` | adapter | the Parquet writer and loader | |
| `vcs/git` | adapter | shells out to `git` | |
| `api`, `mcp`, `web`, `cli` | face | one entry point each, no domain logic | [discovery § faces](05-discovery.md#faces) |

Which process a module ends up in is a **runtime** boundary, not a layer:
see [processes](11-processes.md). Each process is still built from the layers
above, and a module can move between processes without changing layer.

## Where thin glue lands

[Thin glue](00-overview.md#principles) — shell out to the standard client
rather than link a library — is an **adapter-layer rule**. `curl`, `ldapsearch`,
`git` and the RADIUS client are each one adapter behind one port. Core cannot
tell whether a port is backed by a subprocess, a library or a stub, which is
also what makes the whole thing testable without any of them.

## What this buys

| Want to | Touch |
|---|---|
| move from SQLite to PostgreSQL | one adapter |
| add LDAP/AD ([future](future/ldap-ad.md)) | one adapter behind the existing `directory` port |
| replace Parquet dumps | one adapter |
| add a face (say gRPC) | one face; no core change |
| write the PHP client | `protocol` only |

❓ **How `protocol` is specified for five languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.
