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
principal with an Ed25519 key. The **`agent-bus` CLI provides it** — creates
the key, registers, publishes, consumes, signs. Only *generic* records (a
description pushed by someone else) have no key of their own.

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

## Service vs instance

A service is the *kind* (code, description, declared roles, health hints,
optional MCP method info). An instance is service + **private config** + a
place it runs. Private config is kept by the instance by default (any format),
optionally sealed in AUTH/Config. Instances register, heartbeat, vanish; the
service definition is signed and rare-change.

## Service Discovery

("Service discovery" is the preferred name over "registration".)

- **The required minimum**: the daemon runs discovery (registrations) and
  hosts **in-memory queues** (bounded, non-durable) that events land in and
  consumers pull from. **No AUTH in the minimum** — AUTH is a separate
  optional service; everything else is optional too.
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
