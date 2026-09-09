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
The description may mark individual methods as **destructive**; the MCP face
passes the mark through to the calling agent and does nothing else with it — a
hint from the service, enforced by nobody but the receiver.
An instance is service + private config + a place it runs, identified as
**`unique-name@host`** — stable across restarts, and also its address. Private
config stays with the instance by default, optionally sealed in `agent-busd`
(`01`). Instances register, heartbeat, vanish; definitions are live records
changed by owners, **signed by the writer when it has a key**.

## Messaging

- **Every registered agent has its own queue** — an implicit queue topic named
  after it, in memory in `agent-busd`, bounded, with a TTL. **The address
  outlives the process**: anyone may `send` to a registered agent that is down;
  the message waits in its queue and is picked up when the agent comes back —
  unless the TTL expires or the queue is full first. Choose both sensibly.
- **Two overflow modes**, declared per topic (inboxes included) at creation:

  | Mode | Full queue | For |
  |---|---|---|
  | **ring** (default) | drop the **oldest**, count it in stats | alerts, telemetry — newest matters most |
  | **strict** | **reject the send/publish** with an error to the producer | jobs, commands — losing one silently is worse than failing loudly |
- Every message carries **`message_id`** (unique within its channel, assigned
  by `agent-busd`), **`topic`** (conversation id, e.g. one A→B exchange) and
  **`tag`** (the sender's label for the message). A asks B three questions with
  three tags; B's answers carry the same tags, so A matches them. `message_id`
  is what dedup, "did you get it?" and the dashboard refer to.
- **Receipts, optional.** A receiver may answer a message with **`ack`**
  (received) and later **`done`** (processed). Both are ordinary messages to
  the sender's queue (or its `reply-to`), carrying the original `message_id`,
  topic and tag. Neither is required; a sender that wants them asks.
- **TTL per message, optional.** A message may carry its own TTL, shorter than
  the topic's; when it expires undelivered it is dropped from the queue and
  counted, never handed to a consumer. A question nobody should answer late
  sets one.
- **Who sent it — the receiver decides the rest.** Every delivered message
  carries the **sender principal**, verified by the bus, and the
  **on-behalf-of** principal when there is one (`01`). That is all the bus
  adds: it does not classify messages as orders or content and enforces no
  policy on the receiver. Secure delivery plus who and for whom; what to do
  with it is the receiver's call.
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
  **push** address and `agent-busd` delivers to it. Agent sessions are pushed
  through a per-runtime adapter (`04`): Claude Code, Codex, maybe OpenCode.
- **Encoding**: JSON; msgpack as an optional negotiated binary form.
- **Bodies are opaque to the bus.** A message body is encrypted for its
  receiver (`02`); `agent-busd` queues and forwards ciphertext and never holds
  plaintext. What the bus reads and shows is the envelope.

## Topics

Two kinds — the Redis model:

| Kind | Delivery | Retention | No subscribers at publish time | Redis analogue |
|---|---|---|---|---|
| **queue** | each message to **one** consumer (competing consumers take turns) | until consumed or **TTL**; bounded; overflow per mode (`ring` / `strict`) | fine — it waits for its TTL | list + `BRPOP` + `EXPIRE` |
| **pub/sub** | a **copy** to every current subscriber | none | dropped, a no-op | `PUBLISH` / `SUBSCRIBE` |

A topic **declares its kind, TTL, bound and overflow mode at creation**, and is a
**first-class record registered like a service**:

| Aspect | Rule |
|---|---|
| Create | any valid token |
| Change / delete | owner or owner group |
| Record | name, kind, TTL, bound, overflow (`ring` / `strict`), owner, description, audience |
| Visibility | registry and MCP catalog, audience-filtered |
| Access | `publish:<glob>` / `consume:<glob>` on principals |
| Signature | signed by the writing principal **when it has a key**; a **static-token write is unsigned** — the token authenticated it, nothing more is needed. Nodes verify signatures where present before accepting or syncing |
| Storage | live record in `agent-busd`, snapshotted to git for backup and peer sync; **messages** are in memory, **dumped to Parquet on graceful shutdown/restart** and reloaded on start; an optional periodic dump (~1 min) bounds the loss from a crash to one interval |
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
  **With AUTH off**, audience is **defined per service** in its local mapping
  file as `user: token` entries: the presented token names the user, and the
  service's own map decides whether that user sees and uses it. Groups exist
  only with AUTH on.
- **Faces**: **API** (register, look up, send / publish / consume), **MCP
  server** (agents ask "what can I use, and how" — generated docs for every
  known service available to the caller, plus tool descriptions where the
  service exposes tools), **WEB** (humans: registry browser, health, stats
  dashboards; curl-friendly; runs as a cgroup-limited child).
- **Registry replication is git.** Each node holds its registry live and
  writes snapshots to the shared git repo. **On start a node pushes its
  snapshot and pulls from the other known nodes** (peers at the same level).
  **Newer record wins per entry, provided its writer had access** — a
  key-holding writer's record carries its signature, which a peer checks
  before taking it; a static-token record is trusted on the strength of the
  node that accepted it. No live replication protocol.
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
  dumped to Parquet with the queues, so a graceful restart keeps the window and
  a crash loses at most the last dump interval.
- **Dashboard** (the WEB child): the **list of services and topics** with
  their descriptions, who is up, and **call counts grouped by minute or hour**
  — straight from the ring buffers, no external TSDB; audience-filtered. API
  `/stats/<dimension>/<id>`. Export: Prometheus `/metrics` first (Grafana reads
  it); OTLP / StatsD secondary.
- **Nobody reads message bodies but sender and receiver.** Bodies are
  encrypted end to end (`02`); `agent-busd` stores and forwards ciphertext and
  reads only the envelope: sender, receiver, on-behalf-of, `message_id`,
  topic, tag, size, timestamps, receipts. Once a message is consumed it is gone
  — the bus keeps counts, not content. An admin may switch a service into
  **debug mode**, which keeps a trace of that service's messages; admins only,
  and the bodies in it are still ciphertext to the bus.
