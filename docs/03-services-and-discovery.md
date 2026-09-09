# Services and Discovery

## Service kinds

Two axes: reachable or not, and who owns the record.

| Kind | Reachable | Registered by | Health | Signs events |
|---|---|---|---|---|
| **generic** | yes (host/port, unix socket, HTTP) | a user, on behalf of something existing (host/port, description, optional MCP method info) | health-checker polls per hints | no (no key) |
| **agent** | yes | itself, own key | self-reports health/stats | yes, as service |
| **consumer** | no (pulls) | itself, own key | none | no |
| **publisher** | not a service — an identity that signs events (`agent-bus` CLI or `curl` + key) | — | none | yes |

**Identity is required** for agents, consumers and publishers: each is a
principal identified by an Ed25519 key — or, in minimal/static mode, by a
**token, which is the whole identity**. The **`agent-bus` CLI provides it** —
creates the key or takes the token, registers, publishes, consumes, signs.
Only *generic* records (a description pushed by someone else) have no
identity of their own.

- **Registering is just pushing a description.** Anyone may tell discovery
  "there is a MySQL service named `xxx` on `host:port`" — the thing itself
  need not know the bus exists (generic kind). Same for an HTTP API, a unix
  socket, a cron job's host. Health hints are optional extras on the record.
- Publisher/consumer are **capabilities on a principal** (`publish:<topic-glob>`,
  `consume:<topic-glob>`), not service objects.
- generic vs agent differ only in record owner and health mode → same record
  type with a `kind` (when models are designed).

## Personal vs shared (default: shared)

- **personal** — runs *as* a specific user with their account/session
  (mail-reader, the Claude-session input channel — the reference personal
  service). Owner = the user; default audience = the user; lifecycle follows
  the user; usually one instance per user; typically runs on the user's own
  machine (why AUTH-optional matters).
  Key: the user's key (ephemeral sessions) or its own key (recommended for
  anything long-running — narrower access, individually revocable).
  When a personal service calls another service, it *is the user calling* —
  no separate delegation.
- **shared** — runs as `svc:<name>` for many; explicit ACL; long-lived;
  health-checked.

## Messaging — queues, topic + tag

- **Every agent gets its own queue when it starts** (in-memory in `agent-busd`,
  bounded; overflow **drops the oldest** and counts it in stats).
- Address = instance id = **`unique-name@host`**.
- Each message carries **`topic`** (conversation identifier, e.g. A→B
  exchange) and **`tag`** (unique message id). Example: A asks B three
  questions, each with its own tag; B's answers carry the same tags so A can
  match them.
- **Two delivery verbs.** **`send`** = one message to a **known receiver**
  (`unique-name@host`), lands in exactly that queue; allowed if you may talk
  to that principal. **`publish`** = one message to a **topic**; copied into
  the queue of every principal holding a matching `consume:<glob>`; the
  publisher never learns who they were; allowed if you hold `publish:<glob>`.
  `consume` reads your own queue in both cases.
- **Reply routing**: answer goes to the sender's queue with the same
  topic+tag — **unless** the request sets `reply-to: {service, topic, tag}`.
- **Delivery**: consumers **pull** (long-poll/stream) by default; a consumer
  may register a **push** address and `agent-busd` delivers to it.
- Encoding: **JSON**, optional negotiated **msgpack** for heavy payloads.
- Publishing a service definition needs a token; changing one needs owner /
  owner group (see `01`). Definitions are live records, not signed bundle data.
- MCP tool info: stored **raw**, shape-checked only; `agent-busd` generates
  docs from it.

## Service vs instance

A service is the *kind* (code, description, declared roles, health hints,
optional MCP method info). An instance is service + **private config** + a
place it runs, identified as **`unique-name@host`** (stable across restarts). Private config is kept by the instance by default (any format),
optionally sealed in AUTH/Config. Instances register, heartbeat, vanish; the
service definition is signed and rare-change.

## Service Discovery

("Service discovery" is the preferred name over "registration".)

- **The required minimum**: the daemon runs discovery (registrations) and
  hosts **in-memory queues** (bounded, non-durable) that events land in and
  consumers pull from. **No AUTH service in the minimum** — auth is still
  required (static token, `AGENT_BUS_USER_TOKEN`); the AUTH *service* is
  separate and optional, as is everything else.
- Direct talk is still allowed: if you already know where something lives, skip
  the lookup.
- Registration carries **health hints**: HTTP endpoint + expected status, TCP
  connect, unix-socket ping, command, interval, timeout. Unix-socket services
  are first-class (`unix:/path`).
- **Audience** per service — users, services or org groups (from AUTH) who may
  see and use it. Discovery is personalized; MCP tool lists come pre-filtered.
- Faces of `agent-busd`: **API** (agents/services register, look up,
  push/pull queues), **MCP server** (agents ask "what can I use, and how";
  `agent-busd` **generates docs and tool descriptions for every known service
  that is available to the calling client** — audience-filtered, so each
  client sees its own catalog) and **WEB** (humans: registry browser, health,
  fancy stats dashboards; curl-friendly).
- Same replicated generation-based core as AUTH; live state kept separate.

## Health-checker and Stats (modules of discovery)

- Health-checker is itself an agent: registered, replicated 2×, holds a
  `health` role on the services it polls. generic → probe per hints;
  agent → heartbeat, K missed → down.
- Stats: agent heartbeats carry a small metrics blob; probe results (latency,
  up/down) are the stats for generic. Kept **in memory** (ring buffers, last N
  hours, fixed resolution). Restart = empty window (accepted).
- **Dashboard** (own, built into stats): graphs rendered straight from the
  ring buffers — no external TSDB needed. Views **per service / per server /
  per user / …** (any dimension carried by the metrics), numbers + sparklines,
  audience-filtered. API: `/stats/<dimension>/<id>`.
  Export: Prometheus `/metrics` first (Grafana reads it); OTLP/StatsD secondary.
