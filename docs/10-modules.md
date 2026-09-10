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
| `directory/github` | adapter | the built-in HTTP client | |
| `balance/radius` | adapter | shells out to the standard client | |
| `sandbox/systemd`, `sandbox/bwrap`, `sandbox/unshare` | adapter | one per backend, chosen by environment | |
| `dump/parquet` | adapter | the Parquet writer and loader | |
| `vcs/git` | adapter | shells out to `git` | |
| `api`, `mcp`, `web`, `cli` | face | one entry point each, no domain logic | [discovery § faces](05-discovery.md#faces) |

Which process a module ends up in is a **runtime** boundary, not a layer:
see [processes](11-processes.md). Each process is still built from the layers
above, and a module can move between processes without changing layer.

## Languages

**Go inside, and a face may be written in something else.** A face only
translates an outside request into a core call, so it is allowed to be its own
process in another language, speaking the protocol over a socket like any
other client.

| Part | Language | Why |
|---|---|---|
| protocol, ports, core, adapters | **Go** | one static binary, no runtime; peer credentials on a unix socket, privilege-dropped children and passed fds are stdlib, not FFI ([processes](11-processes.md)) |
| `cli` face | **Go** | it ships with the daemon, and the same core is already linked |
| `mcp` face, the push adapters | **bun / TypeScript** | both already exist in that shape in V1 and are ported, not rewritten ([stages § PoC](12-stages.md#poc)) |
| client libraries | Go, PHP, Rust, JS, Python | each reimplements `protocol` and nothing below it |

Two rules hold this together:

- **No verb exists only in a face.** Every CLI or MCP operation is first a
  public core API; a face that does routing, signing or retry of its own has
  taken work that belongs inward.
- **A face in another language is a client, not a shortcut inward.** It gets
  no privilege the protocol does not give it, and it is supervised like any
  other child ([processes](11-processes.md)).

## External tools

**Do not reinvent the wheel.** Nothing here is ours if something standard
already does it. Three ways to satisfy that, in order of preference:

| | Use | When |
|---|---|---|
| 1 | **the language's built-in** | it already covers the job. **HTTP above all** — every language ships a client, so a web request is never a subprocess |
| 2 | **the system's tool**, shelled out to | a standard binary is the well-trodden path, and the call is occasional |
| 3 | **a well-known library**, in-process | neither fits and it is on the hot path |
| — | **never** our own crypto or protocol primitives | — |

### HTTP is built in

**Web requests are a first-class citizen of this design, not glue.** Generic
services are HTTP endpoints, health hints are HTTP probes, and one of the
three faces is a web server ([discovery § faces](05-discovery.md#faces)). It
is also the one thing every language already has in stock, so there is no
wheel to avoid reinventing.

So HTTP is always the built-in client — including fetching a login's public
keys, which is an ordinary GET. **The exception is a script**: a shell or
script adapter uses `curl`, because that is what a script has.

### What we do shell out to

| Job | Tool | Behind |
|---|---|---|
| generate an Ed25519 keypair | `ssh-keygen -t ed25519` | `cli` |
| sign / verify a challenge | `ssh-keygen -Y sign -n agent-bus`, `-Y verify` (SSHSIG) | `session` |
| sign / verify a bundle generation | the same | `vcs` |
| seal private config to a key | `age` | `store` |
| query a directory | `ldapsearch` | `directory/ldap` ([future](future/ldap-ad.md)) |
| push and pull the repo | `git` | `vcs/git` |
| ask for a balance | the standard RADIUS client | `balance/radius` |
| confine a child | `systemd-run` · `bwrap` · `unshare` | `sandbox/*` |
| authenticate an admin or issue a token | `sshd` with a forced command | — |

Each sits behind one port, in an adapter — so core cannot tell a subprocess
from a library, which is also what makes it testable without any of them.
What it buys: the user's existing SSH keys work with no conversion, TLS and
Kerberos and proxy and trust-store handling belong to the system, and every
one of these is a command an operator can run by hand to see what the daemon
sees.

### The hot path

Per-message AEAD and the HKDF behind a session cannot spawn a process; those
are rule 3, in-process
([access § encrypted sessions](02-access.md#encrypted-sessions)).

**On `openssl` specifically**: it fits where you would expect and mostly is
not needed, because two earlier decisions removed its usual jobs — identity is
SSH keys, so `ssh-keygen` covers generation and signing, and there is no TLS
and no PKI, so there are no certificates to make or verify. It stays the right
reach for one-off key inspection and format conversion.

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
