# Overview

Status: design, main ideas only · no data models yet (deferred by owner) · no code

Start here, then read in order. Each document **owns** its subject: values —
paths, commands, field lists, mode names — are stated once, where they belong,
and linked from everywhere else.

| # | Doc | Owns |
|---|---|---|
| 00 | this file | goal · principles · the roles table · chaining · trade-offs |
| 01 | [identity](01-identity.md) | principals · `user@realm` · registration · groups, roles, ACL · delegation · ownership · sealed config |
| 02 | [access](02-access.md) | the two parameters · tokens · the local socket · key modes · encrypted sessions |
| 03 | [services and topics](03-services-and-topics.md) | service kinds · personal/shared · instances · topic records · registry sync |
| 04 | [messaging](04-messaging.md) | queues · message fields · receipts · TTL · verbs · overflow · durability · the envelope |
| 05 | [discovery](05-discovery.md) | faces · audience · health · stats · dashboard · debug mode |
| 06 | [AUTH role](06-auth-role.md) | the bundle · signed generations · topology · `master_secret` · SSH admin |
| 07 | [billing role](07-billing-role.md) | price · RADIUS · usage · denial · the pay service |
| 08 | [runner role](08-runner-role.md) | supervision · adapters · sandboxing · in-process queue |
| 09 | [setup](09-setup.md) | install · `agent-bus setup` · local users · storage · reload |
| 10 | [modules](10-modules.md) | layers · module boundaries · which dependency is swappable |

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
- **Modular by layer.** Dependencies point inward and only adapters touch the
  outside world, so anything external — the database, a directory, the sandbox
  backend — sits behind a port and is replaced without touching the rest
  ([modules](10-modules.md)).
- **One binary, one unit, one config dir, one CLI, one git repo.** `agent-busd`
  supervises its own roles as child processes with the same machinery it uses
  for any child ([runner § supervises itself](08-runner-role.md#supervises-itself)).
- **Ed25519 wherever there is a key**; no passwords, no client secrets, no
  TLS/PKI.
- **Glue to external systems stays as thin as possible.** Where a standard
  command-line client exists, shell out to it rather than linking a library or
  reimplementing a protocol: `ldapsearch`, `curl`, `ssh`. The system owns TLS,
  Kerberos, proxies and trust stores; we own an invocation and a field map.
  Every such call happens at enrolment or an explicit re-check — never on the
  runtime path — so a thin, occasionally-slow path costs nothing.
- **Public services are a goal, not an exception.** Open enrolment for anyone
  with a provider key; closed services queue newcomers for approval. Encrypted
  sessions without TLS or PKI are what make this safe to expose. With billing
  on it is a paid public API platform ([billing role](07-billing-role.md)).
- **Bodies are end-to-end encrypted**: only sender and receiver read them; the
  bus sees the envelope ([messaging § envelope](04-messaging.md#envelope)).
- **AUTH is on the hot path once** per (user, service, epoch).
- **Two kinds of data.** *AUTH data* changes a few times a week → offline-signed
  generations in git ([AUTH role § bundle](06-auth-role.md#bundle)). *Registry
  data* is live in `agent-busd`, snapshotted to git
  ([identity § ownership](01-identity.md#ownership)). *Live state* — health,
  stats, instances, queue contents — is neither signed nor snapshotted, but it
  is *durable*: queues and stats dump to Parquet, and tokens are saved so a
  reloaded queue can still be decrypted
  ([access § token lifetime](02-access.md#token-lifetime)).
- **Policy lives in AUTH; services only interpret**, never decide.
- **Two independent controls**: SSH decides *who may administer*; a signature
  decides *which config is real*.

## Roles

| Process | Does | Default |
|---|---|---|
| **core** | registry of services and topics · in-memory queues · API · MCP server · runner · supervises the children below | always |
| **WEB child** | the dashboard ([discovery § dashboard](05-discovery.md#dashboard)); **cgroup-limited** (CPU / memory / pids) so it can never starve the bus | on, may be off |
| **AUTH child** | identities, keys, groups, ACLs, roles, sealed private configs; **alone holds `master_secret`** ([AUTH role](06-auth-role.md)) | **off** — `auth: on` |
| health-checker | module of discovery ([discovery § health checker](05-discovery.md#health-checker)) | optional |
| stats | module of discovery ([discovery § stats](05-discovery.md#stats)) | optional |
| **billing** | balance, usage and denial ([billing role](07-billing-role.md)); the bus never touches money | **off** — `billing: on` |

Faces of the core are API, MCP and WEB — see
[discovery § faces](05-discovery.md#faces).

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
