# Services, Messaging and Discovery

## Service kinds

Two axes: reachable or not, and who owns the record.

| Kind | Reachable | Registered by | Health | Signs events |
|---|---|---|---|---|
| **generic** | yes (host/port, unix socket, HTTP) | a user, on behalf of something that exists — description, address, optional MCP method info | health-checker probes per hints | no (no identity) |
| **agent** | yes | itself, own identity | self-reports health/stats | if key-holding |
| **consumer** | no (pulls) | itself, own identity | none | no |
| **publisher** | not a service — an identity that emits events (`agent-bus` CLI or `curl`) | — | none | if key-holding |

- **Registering is just pushing a description.** "There is a MySQL service
  named `xxx` on `host:port`" is a complete registration; the thing itself need
  not know the bus exists. Same for an HTTP API, a unix socket, a cron host.
  Health hints are optional extras.
- **Identity is required** for agents, consumers and publishers: a token
  (minimal mode, the token *is* the identity) or an Ed25519 key. The
  `agent-bus` CLI provides it. Only generic records have none.
- Publisher/consumer are **capabilities on a principal** (`publish:<glob>`,
  `consume:<glob>`), not service objects.
- generic vs agent differ only in record owner and health mode → same record
  type with a `kind` (when models are designed).

## Personal vs shared (default: shared)

- **personal** — runs *as* a specific user with their account (mail-reader, a
  Claude session joined via `claude --channel`). Owner = the user; default
  audience = the user; lifecycle follows the user; usually one instance;
  typically on the user's own machine. Key: the user's (ephemeral sessions) or
  its own (recommended for anything long-running — narrower, individually
  revocable). When it calls another service, *it is the user calling*.
- **shared** — runs as `svc:<name>` for many; explicit ACL; long-lived;
  health-checked.

## Service vs instance

A service is the *kind*: description, declared roles, health hints, optional
MCP method info (stored **raw**, shape-checked only; docs generated from it).
An instance is service + private config + a place it runs, identified as
**`unique-name@host`** — stable across restarts, and also its address. Private
config stays with the instance by default, optionally sealed in `agent-busd`
(`01`). Instances register, heartbeat, vanish; definitions are live records
changed by owners, each **signed by its writer**.

## Messaging

- **Every agent gets its own queue when it starts** — an implicit queue topic
  named after it, in memory in `agent-busd`, bounded; overflow **drops the
  oldest** and counts it in stats.
- Every message carries **`topic`** (conversation id, e.g. one A→B exchange)
  and **`tag`** (message id). A asks B three questions with three tags; B's
  answers carry the same tags, so A matches them.
- **Two verbs.**

  | Verb | Target | Lands in | Allowed if |
  |---|---|---|---|
  | **`send`** | a known receiver, `unique-name@host` | exactly that queue | you may talk to that principal |
  | **`publish`** | a topic | every queue holding a matching `consume:<glob>` | you hold `publish:<glob>` |

  `consume` reads your own queue in both cases. Delivery does not report who
  received a publish, but the API answers "are there subscribers on this
  topic, and who?" to anyone whose access allows the lookup.
- **Reply routing**: to the sender's queue with the same topic + tag — unless
  the request sets `reply-to: {service, topic, tag}`.
- **Consumers pull** by default (long-poll / stream); a consumer may register a
  **push** address and `agent-busd` delivers to it.
- **Encoding**: JSON; msgpack as an optional negotiated binary form.

## Topics

Two kinds — the Redis model:

| Kind | Delivery | Retention | No subscribers at publish time | Redis analogue |
|---|---|---|---|---|
| **queue** | each message to **one** consumer (competing consumers take turns) | until consumed or **TTL**; bounded, drop-oldest | fine — it waits for its TTL | list + `BRPOP` + `EXPIRE` |
| **pub/sub** | a **copy** to every current subscriber | none | dropped, a no-op | `PUBLISH` / `SUBSCRIBE` |

A topic **declares its kind, TTL and bound at creation**, and is a
**first-class record registered like a service**:

| Aspect | Rule |
|---|---|
| Create | any valid token |
| Change / delete | owner or owner group |
| Record | name, kind, TTL, bound, owner, description, audience |
| Visibility | registry and MCP catalog, audience-filtered |
| Access | `publish:<glob>` / `consume:<glob>` on principals |
| Signature | **every record is signed by the principal that wrote it** (owner or publisher); nodes verify before accepting or syncing |
| Storage | live record in `agent-busd`; messages are live state; snapshotted to git for backup and peer sync |
| Stats | depth, in/out rate, drops, subscriber count — on the dashboard like a service |

Inboxes are implicit queue topics created on an agent's first start and owned
by it. Namespacing follows services (`team/alerts`); local shadows upstream.

## Service Discovery

("Service discovery" is the name; "registration" is one operation on it.)

- **The required minimum**: the `agent-busd` core with its registry and queues.
  AUTH role off; auth still required via static token.
- Direct talk is allowed: if you already know where something lives, skip the lookup.
- Registration carries **health hints**: HTTP endpoint + expected status, TCP
  connect, unix-socket ping, command, interval, timeout. Unix-socket services
  are first-class (`unix:/path`).
- **Audience** per service and topic — users, services or org groups who may
  see and use it. Discovery is personalised; MCP catalogs come pre-filtered.
  ❓ With AUTH off the only identity is the token — filtering per token is the
  assumption (`00`).
- **Faces**: **API** (register, look up, send / publish / consume), **MCP
  server** (agents ask "what can I use, and how" — generated docs for every
  known service available to the caller, plus tool descriptions where the
  service exposes tools), **WEB** (humans: registry browser, health, stats
  dashboards; curl-friendly; runs as a cgroup-limited child).
- **Registry replication is git.** Each node holds its registry live and
  writes snapshots to the shared git repo. **On start a node pushes its
  snapshot and pulls from the other known nodes** (peers at the same level).
  **Newer record wins per entry, provided its writer had access** — every
  record carries its writer's signature, so a peer can check that before
  taking it. No live replication protocol. ❓ How token-only principals sign
  is open (`00`).
- **Upstreams are not replicated.** An upstream `agent-busd` has its own
  registry and we usually lack full access to it; chaining queries it and
  caches answers, nothing more.

## Health-checker and Stats (modules of discovery)

- **Health-checker** is itself an agent: registered, replicated 2×, holds a
  `health` role on the services it probes. generic → probe per hints; agent →
  heartbeat, K missed → down.
- **Stats**: agent heartbeats carry a small metrics blob; probe results
  (latency, up/down) are the stats for generic; queue depth and rates for
  topics. Kept in memory — ring buffers, last N hours, fixed resolution;
  restart = empty window (accepted).
- **Dashboard** (the WEB child): graphs straight from the ring buffers, no
  external TSDB. Views per service / server / user / topic — any dimension the
  metrics carry; numbers + sparklines; audience-filtered. API
  `/stats/<dimension>/<id>`. Export: Prometheus `/metrics` first (Grafana reads
  it); OTLP / StatsD secondary.
