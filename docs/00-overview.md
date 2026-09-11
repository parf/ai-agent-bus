# Overview

Status: design, main ideas only · no data models yet (deferred by owner) · the PoC
is built in [`src/`](../src/); everything past it is design — [stages](12-stages.md)

Start here, then read in order. Each document **owns** its subject: values —
paths, commands, field lists, mode names — are stated once, where they belong,
and linked from everywhere else.

| # | Doc | Owns |
|---|---|---|
| 00 | this file | goal · principles · the roles table · chaining · trade-offs |
| 01 | [identity](01-identity.md) | principals · `user@realm` · registration · groups, roles, ACL · delegation · ownership · sealed config |
| 02 | [access](02-access.md) | the two parameters · tokens · the local socket · key modes · encrypted sessions |
| 03 | [services and topics](03-services-and-topics.md) | service kinds · personal/shared · templates and services · topic records · registry sync |
| 04 | [messaging](04-messaging.md) | queues · message fields · receipts · TTL · verbs · overflow · durability · the envelope |
| 05 | [discovery](05-discovery.md) | faces · audience · health · stats · dashboard · debug mode |
| 06 | [AUTH role](06-auth-role.md) | the bundle · signed generations · topology · `master_secret` · SSH admin |
| 08 | [runner role](08-runner-role.md) | supervision · adapters · sandboxing · in-process queue |
| 09 | [setup](09-setup.md) | install · `agent-bus setup` · local users · storage · reload |
| 10 | [modules](10-modules.md) | layers · module boundaries · which dependency is swappable · languages · external tools |
| 11 | [processes](11-processes.md) | the supervisor and its children · privilege per process · what is shared |
| 12 | [stages](12-stages.md) | PoC, MVP, Release 1 — what gets built when, and what counts as done |

07 was the billing role; it is deferred and lives in
[future/billing.md](future/billing.md). Numbers are stable, so the gap stays.

Also: [glossary](glossary.md) — every name and term, one line each ·
[decisions](decisions.md) — what is settled, open and superseded ·
[future/](future/) — designed but deferred.

## Goal

Replace V1 — an external broker plus a `user → token` map inside every service
— with **one daemon of our own, `agent-busd`**. It is the registry, the broker,
the MCP server and the dashboard in one binary. There is no external broker to
run. Parties that already know each other may also talk directly,
point-to-point and encrypted.

Model for access: **Kerberos-style** — when the AUTH role is on, it hands both
parties a shared secret once, then gets out of the way.

## Principles

Claims only; the mechanism lives in the doc each one links to.

- **Minimum to run: `agent-busd` alone** — registry, in-memory queues, API,
  MCP server, dashboard, with AUTH off.
- **Authentication is always on; the AUTH role is optional.** There is no
  anonymous access, and a call carries only ever two parameters
  ([access § two parameters](02-access.md#two-parameters)). Locally you handle
  neither ([access § local socket](02-access.md#local-socket)), which is how
  one daemon serves many users on a host and knows which is calling.
- **Names are `user@realm`**, one syntax with three sources of authority
  ([identity § names](01-identity.md#names)). The name is the identity;
  provider ids are only a check.
- **Registration is a record you state**; a provider is an alternative to
  typing it and is not needed afterwards
  ([identity § registration](01-identity.md#registration)).
- **ACL is service first, then master**
  ([identity § acl](01-identity.md#acl)).
- **A supervisor with least-privilege children**, systemd-style: the
  supervisor does almost nothing and holds nothing, each child does one task
  with the narrowest privilege for it, and nothing is shared implicitly
  ([processes](11-processes.md)).
- **Modular by layer.** Dependencies point inward and only adapters touch the
  outside world, so anything external — the database, a directory, the sandbox
  backend — sits behind a port and is replaced without touching the rest
  ([modules](10-modules.md)).
- **One binary, one unit, one config dir, one CLI, one git repo** — many
  processes out of that one binary ([processes](11-processes.md)).
- **Ed25519 wherever there is a key**; no passwords, no client secrets, no
  TLS/PKI — between bus citizens. A browser is the exception, and only the
  dashboard faces one
  ([discovery § where it listens](05-discovery.md#where-it-listens)).
- **Do not reinvent the wheel.** Prefer the language's built-in, then the
  system's tool (`ssh-keygen`, `ldapsearch`, `git`, `age`, `systemd-run`,
  `sshd`), then a well-known library — and never our own crypto or protocol
  primitives. **HTTP is always built in**: web requests are a first-class part
  of this design and every language ships a client, so they are never a
  subprocess — unless the caller is a script, which has `curl`. The system
  owns TLS, Kerberos, proxies and trust stores; we own an invocation and a
  field map ([modules § external tools](10-modules.md#external-tools)).
- **Public services are a goal, not an exception.** Open enrolment for anyone
  with a provider key; closed services queue newcomers for approval. Encrypted
  sessions without TLS or PKI are what make this safe to expose. A paid public
  API platform is designed and deferred ([future/billing.md](future/billing.md)).
- **Bodies are end-to-end encrypted** from Release 1: only sender and receiver
  read them, and the bus sees the envelope
  ([messaging § envelope](04-messaging.md#envelope)). The MVP's key mode cannot
  carry that claim, so it is not made there
  ([access § encrypted sessions](02-access.md#encrypted-sessions)) — the bus
  still reads nothing but envelopes, it is simply trusted not to.
- **AUTH is on the hot path once** per (user, service, epoch).
- **Two kinds of data.** *AUTH data* changes a few times a week → offline-signed
  generations in git ([AUTH role § bundle](06-auth-role.md#bundle)). *Registry
  data* is live in `agent-busd`, snapshotted to git
  ([identity § ownership](01-identity.md#ownership)). *Live state* — health,
  stats, running services, queue contents — is neither signed nor snapshotted, but it
  is *durable*: queues and stats dump to Parquet, and tokens are saved so a
  reloaded queue can still be decrypted
  ([access § token lifetime](02-access.md#token-lifetime)).
- **Policy lives in AUTH; services only interpret**, never decide.
- **Two independent controls**: SSH decides *who may administer*; a signature
  decides *which config is real*.

## Roles

`agent-busd` is a **supervisor plus small single-task children**, on the
systemd model: the supervisor holds no secrets and almost no responsibility,
and every child gets the narrowest privilege its task needs.

| Always | Optional |
|---|---|
| supervisor · bus · runner | web (on by default) · auth (`auth: on`) · health · billing when it ships |

Who may do what, and what is shared between them, is
[processes and privileges](11-processes.md). Faces of the bus are API, MCP and
WEB — see [discovery § faces](05-discovery.md#faces).

## Chaining

`agent-busd` accepts an **upstream** `agent-busd` (which may have its own).
Resolution: local file → local node → upstream → …; first hit wins, unresolved
falls through. Applies to identities, ACL/roles, service and topic lookups.

- Scopes stay put: personal on the laptop, team on the team node, company
  upstream. Nothing is pushed up; definitions never leak upward (queries do).
- Each hop pins its upstream's signing key; AUTH answers arrive as signed
  generations — a middle hop can fail to forward, not forge.
- Local shadows upstream by design; writes warn when they shadow.
- Namespaced ids (`team/ci`, `company/mail`, `team/alerts`) avoid collisions.

❓ **A namespace and a service template both want the `/`.** A name holds at
most one, and it already means *template* / *instance*
([identity § names](01-identity.md#names)), so `team/ci@host` parses as
template `team`. Either a chaining namespace *is* the template part, or
chaining needs a separator of its own. *Settled by:* the owner, when chaining
is designed.
- Upstream answers are cached under the usual epoch/gen rules; unreachable
  upstream = "cached or unresolved", never a wrong answer.
- `master_secret` does not span levels → cross-level access uses pairwise keys
  or keys issued by the upstream itself.
- **Chaining queries upstream, never replicates it.** Peers at the same level
  sync through git
  ([services § registry sync](03-services-and-topics.md#registry-sync)).
- The MCP face merges levels into one catalog, tagged by origin.

## Trade offs

- **Consistency window**: revoked access may be honoured for one replica poll
  interval + one epoch ([AUTH role § consistency window](06-auth-role.md#consistency-window)).
- **No forward secrecy** (decided): a leaked long-term key exposes recorded
  sessions.
- **Registry records are owner-signed, not admin-signed**: anyone
  authenticated may publish a service or topic; only ownership guards changes.
  No offline key stands behind them.
- **Queues are memory**: a graceful restart keeps them, a crash loses what
  arrived since the last dump — or everything, if the periodic dumper is off
  ([messaging § durability](04-messaging.md#durability)).
- **Provider records are pinned at enrolment** and re-checked only on request:
  a key deleted upstream stays valid until someone asks.
- **Static tokens, pairwise keys and local files** have no central revocation
  or audit — and a local token does not expire by default, so it is revoked by
  deleting it, not by waiting.
- **Stats are memory**, dumped with the queues: a crash loses the last interval.
- **`master_secret` and the offline signing key are the roots of trust**; every
  AUTH replica holds `master_secret`, so a compromised replica mints keys.
