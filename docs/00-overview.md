# Agent Bus — Overview

Status: design, main ideas only · no data models yet (deferred by owner) · no code

Documents, in reading order:
1. `00-overview.md` — goal, principles, `agent-busd` and its roles, chaining, storage, trade-offs, open items
2. `01-identity-and-auth.md` — principals, key directories, groups/ACL/roles, ownership, delegation, private config
3. `02-keys-sessions-replication.md` — access-key modes, encrypted sessions, signed generations in git, SSH admin
4. `03-services-and-discovery.md` — service kinds, messaging (queues, topics, send/publish), topics as records, discovery, health, stats
5. `04-runner.md` — the runner role: adapters, self-supervision, sandboxing, in-process queue, languages

`v1-original.md` records V1 (the broker-based system in production) and the V1 → V2 mapping.
`../HANDOFF.md` is the owner's full discussion record.

## Goal

Replace V1 — an external broker plus a `user → token` map inside every service —
with **one daemon of our own, `agent-busd`**. It is the registry, the broker,
the MCP server and the dashboard in one binary. There is no external broker to
run. Parties that already know each other may also talk directly,
point-to-point and encrypted.

Model for access: **Kerberos-style** — when the AUTH role is on, it hands both
parties a shared secret once, then gets out of the way.

## Principles

- **Minimum to run: `agent-busd` alone.** Registry (services + topics),
  in-memory queues, API, MCP server, dashboard. AUTH role off.
- **Authentication is always on; the AUTH role is optional.** Every participant
  presents a token or a key; there is no anonymous access. Minimal mode: a
  token set by hand (`ENV AGENT_BUS_USER_TOKEN`), matched by the receiving
  service's local mapping file. **The token is the whole identity** — no key,
  zero AUTH calls. Turn the AUTH role on when an org wants central identities,
  groups and hourly derived keys.
- **One binary, one unit, one config dir, one CLI, one git repo.** `agent-busd`
  supervises its own roles as child processes (see below) with the same
  machinery it uses for any child.
- **Ed25519 wherever there is a key**; no passwords, no client secrets.
- **AUTH is on the hot path once** per (user, service, epoch).
- **Two kinds of data.** *AUTH data* (principals, groups, ACL/roles, admin SSH
  keys) changes a few times a week → offline-signed generations in git over SSH.
  *Registry data* (service and topic definitions, ownership) is **live** in
  `agent-busd`; **a record is signed by the principal that wrote it when that
  principal has a key** (static-token writes are unsigned — the token
  authenticated them), guarded by token and ownership, snapshotted to the same
  git repo for backup and peer sync. *Live state* (health, stats, instances, queue contents) is
  neither signed nor snapshotted.
- **Policy lives in AUTH; services only interpret**, never decide.
- **Two independent controls**: SSH decides *who may administer*; a signature
  decides *which config is real*.

## `agent-busd` and its roles

| Process | Does | Default |
|---|---|---|
| **core** | registry of services and topics · in-memory queues · API · MCP server with generated docs · runner · supervises the children below | always |
| **WEB child** | dashboards: registry browser, health, stats graphs from ring buffers; **cgroup-limited** (CPU / memory / pids) so it can never starve the bus | on, may be off |
| **AUTH child** | identities, keys, groups, ACLs, roles, sealed private configs; **alone holds `master_secret`** and the signed bundle; core talks to it over a unix socket (sshd/Postfix-style privilege separation) | **off** — `auth: on` |
| health-checker | module of discovery; probes generic services per their hints | optional |
| stats | module of discovery; in-memory ring buffers feeding WEB and exporters | optional |

Nodes with `auth: on` are the AUTH replicas (run 2+). A laptop node never holds
`master_secret`. How many children there are beyond WEB and AUTH, and what is
shared between core and children, is still open (see below).

Faces of the core: **API** (register, look up, send/publish/consume),
**MCP server** (agents ask "what can I use, and how"; `agent-busd` generates
docs and, where a service exposes them, tool descriptions — filtered to what
the calling client may see), **WEB** (humans; curl-friendly).

## Chaining — local first, upstream for the rest

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
- **An upstream has its own registry**, and we usually do not have full
  access to it: chaining is *querying* upstream, never replicating it.
  Peer nodes at the same level sync their registry through git (`03`).
- The MCP face merges levels into one catalog, tagged by origin.

## Storage

SQLite by default (single file, zero ops); MySQL/PostgreSQL optional behind one
thin store layer. The git repo (over SSH) holds signed AUTH bundles (authority)
and unsigned registry snapshots (backup). Queues and stats live in memory and are
**dumped to a Parquet file on graceful shutdown or restart**, loaded back on start;
an optional **periodic dumper** (about once a minute) makes them survive an untimely
death too, losing at most one interval.

## Known trade-offs

- **Consistency window**: revoked access may be honoured for one replica poll
  interval + one epoch. Optional `revoked_users` list in the bundle closes it
  for *new* sessions; live sessions are not torn down.
- **No forward secrecy** (decided): session keys derive from the access key +
  nonces; a leaked long-term key exposes recorded sessions.
- **Registry records are owner-signed, not admin-signed**: anyone with a
  valid token may publish a new service or topic; only ownership guards
  changes. No offline key stands behind them.
- **Queues are memory, dumped to Parquet**: a graceful restart keeps them; a crash loses what arrived since the last periodic dump (if enabled) or everything (if not). A *consumer* being down is fine — its queue holds messages until TTL or bound. Overflow: `ring` drops the oldest, `strict` refuses the send.
- GitHub keys are pinned at enrolment; a key deleted on GitHub stays valid
  until someone refreshes.
- Static tokens, pairwise keys and local files have no central revocation or audit.
- Stats are memory, dumped with the queues: a crash loses the last interval.
- `master_secret` and the offline signing key are the roots of trust; every
  AUTH replica holds `master_secret`, so a compromised replica mints keys.

## Decision log (2026-09-09)

Settled with the owner; each is written into the doc named.

- Token is the whole identity in minimal mode → `01`, `02`
- Publish new service/topic: any token; change: owner or owner group → `01`, `03`
- AUTH merged into `agent-busd` as an optional role, child process; WEB child cgroup-limited → `00`, `02`, `04`
- Delegation: A + on-behalf-of U claim → `01`
- No forward secrecy → `02`
- Consumers pull by default, may register a push address → `03`
- Overflow: drop oldest → `03` *(revised same day, see below)*
- Per-agent queue on start; topic + tag; reply-to → `03`
- `send` to known receiver, `publish` to topic, `consume` own queue → `03`
- Topic kinds queue / pub·sub; kind, TTL, bound declared at creation; topics registered like services → `03`
- Instance id = address = `unique-name@host` → `03`
- `master_secret` as out-of-band file → `02`
- Wire: JSON, optional msgpack → `02`
- MCP tool info stored raw, shape-checked → `03`
- Bundle gaps: newer generation wins; bundle repo in git over SSH; master/slave default; local node + git remote (GitHub or own SSH host) as backup → `02`
- Admin keys live in the bundle; root on the box is the break-glass → `02`
- Go first, bun/NPM later; client libs Go, PHP, Rust, JS, Python → `04`
- V1 leftovers (RAG, KV/DB gateways, writers) deferred, non-core → `v1-original.md`
- Registry between peer nodes: **git push/pull on start** with the other known nodes; **newer record wins per entry, provided the writer had access**; an **upstream keeps its own registry** we usually cannot fully read → `03`
- Audience with AUTH off: **each service defines its own `user: token` map** (the local mapping file); the token names the user, the service's map decides who sees and uses it → `03`
- Wrong key at handshake: **re-query AUTH for a fresh key once; if it still fails, alert — loud** (bus event + dashboard + log). Never silent, never an endless retry → `02`
- **Registry records are signed** by the writing principal when it has a key; **static-token principals do not sign** — the token authenticated the write, and that is enough → `03`
- GitHub `/users/<login>/keys` **does** return `created_at` and `last_used` — verified with one `curl`, 2026-09-09 → `01`
- **The address outlives the process**: a registered agent's queue accepts messages while it is down; picked up on return, bounded by TTL and size → `03`
- **Two overflow modes per topic**: `ring` (drop oldest, default) and `strict` (reject the send with an error) → `03`
- **Graceful shutdown / restart dumps in-memory state to Parquet** and reloads it; an optional periodic dumper (~1 min) covers untimely death → `00`, `03`
- **`message_id`**, unique per channel, on every message → `03`
- **Optional receipts**: `ack` (received) and `done` (processed), sent back to the sender carrying the `message_id` → `03`
- **TTL per message**, optional, within the topic's TTL; expired = dropped and counted, never delivered → `03`
- **One push adapter per agent runtime**, borrowed from V1's notifiers: Claude Code (Channels), Codex (App Server), ❓ OpenCode (Z.AI); ChatGPT pull-only → `03`, `04`

## Open

1. ❓ **Process layout** — how many child processes beyond WEB and AUTH, and
   what is shared between core and children (store, queues, sockets, memory)
   vs. isolated. *Settled by:* owner review.
