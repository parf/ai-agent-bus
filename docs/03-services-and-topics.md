# Services and topics

What lives on the bus. Both are **records** in `agent-busd`'s registry, created
and changed under the same rules.

## Service kinds

Two axes: reachable or not, and who owns the record.

| Kind | Reachable | Registered by | Health | Signs events |
|---|---|---|---|---|
| **generic** | yes (host/port, unix socket, HTTP) | a user, on behalf of something that exists — description, address, optional MCP method info | health-checker probes per hints | no (no identity) |
| **agent** | yes | itself, own identity | self-reports health/stats | if key-holding |
| **consumer** | no (pulls) | itself, own identity | none | no |
| **publisher** | not a service — an identity that emits events (`agent-bus` CLI or `curl`) | — | none | if key-holding |

- **A shell script is a service too** — one command registers it and answers
  from its stdout ([runner § script services](08-runner-role.md#script-services)).
- **Registering is just pushing a description.** "There is a MySQL service
  named `xxx` on `host:port`" is a complete registration; the thing itself need
  not know the bus exists. Same for an HTTP API, a unix socket, a cron host.
  Health hints are optional extras.
- **Identity is required** for agents, consumers and publishers — see
  [access § two parameters](02-access.md#two-parameters). Only generic records
  have none.
- Publisher/consumer are **capabilities on a principal** (`publish:<glob>`,
  `consume:<glob>`), not service objects.
- generic vs agent differ only in record owner and health mode → same record
  type with a `kind` (when models are designed).

## Personal and shared

Default is **shared**.

- **personal** — runs *as* a specific user with their account (mail-reader, a
  Claude session joined via `claude --channel`). Owner = the user; default
  audience = the user; lifecycle follows the user; usually one instance;
  typically on the user's own machine. Key: the user's (ephemeral sessions) or
  its own (recommended for anything long-running — narrower, individually
  revocable). When it calls another service, *it is the user calling* — no
  delegation ([identity § delegation](01-identity.md#delegation)).
- **shared** — runs as a service principal for many; explicit ACL; long-lived;
  health-checked.

## Service and instance

A **service** is the kind: description, declared roles, health hints, optional
MCP method info (stored **raw**, shape-checked only; docs generated from it).
The description may mark individual methods as **destructive**; the MCP face
passes the mark through to the calling agent and does nothing else with it — a
hint from the service, enforced by nobody but the receiver.

An **instance** is service + private config + a place it runs, identified as
**`unique-name@host`** — stable across restarts, and also its address; the same
`name@realm` shape users have ([identity § names](01-identity.md#names)).
Private config stays with the instance by default, optionally sealed in
`agent-busd` ([identity § sealed private config](01-identity.md#sealed-private-config)).

Instances register, heartbeat, vanish. Definitions are **live records** changed
by owners under [identity § ownership](01-identity.md#ownership), signed by the
writer when it has a key.

## Topics

A topic is registered like a service and is **first-class** in the same
registry. Two kinds — the Redis model:

| Kind | Delivery | Retention | No subscribers at publish time | Redis analogue |
|---|---|---|---|---|
| **queue** | each message to **one** consumer (competing consumers take turns) | until consumed or **TTL**; bounded; overflow per mode | fine — it waits for its TTL | list + `BRPOP` + `EXPIRE` |
| **pub/sub** | a **copy** to every current subscriber | none | dropped, a no-op | `PUBLISH` / `SUBSCRIBE` |

A topic **declares its kind, TTL, bound and overflow mode at creation**:

| Aspect | Rule |
|---|---|
| Create | any authenticated principal |
| Change / delete | owner or owner group ([identity § ownership](01-identity.md#ownership)) |
| Record | name, kind, TTL, bound, overflow, owner, description, audience |
| Visibility | registry and MCP catalog, audience-filtered ([discovery § audience](05-discovery.md#audience)) |
| Access | `publish:<glob>` / `consume:<glob>` on principals |
| Signature | signed by the writing principal **when it has a key**; a static-token write is unsigned — the token authenticated it. Nodes verify signatures where present before accepting or syncing |
| Storage | live record in `agent-busd`, snapshotted to git; messages are in memory, see [messaging § durability](04-messaging.md#durability) |
| Stats | depth, in/out rate, drops, subscriber count — on the dashboard like a service |

Inboxes are implicit queue topics created on an agent's first start and owned
by it ([messaging § inbox queues](04-messaging.md#inbox-queues)). Namespacing
follows services (`team/alerts`); local shadows upstream.

## Registry sync

Between **peer nodes at the same level**, the registry syncs through git — no
live replication protocol.

- Each node holds its registry live and writes snapshots to the shared git
  repo. **On start a node pushes its snapshot and pulls from the other known
  nodes.**
- **Newer record wins per entry, provided its writer had access.** A
  key-holding writer's record carries its signature, which a peer checks before
  taking it; a static-token record is trusted on the strength of the node that
  accepted it.
- **Upstreams are not replicated.** An upstream `agent-busd` has its own
  registry and we usually lack full access to it; chaining queries it and
  caches answers, nothing more
  ([overview § chaining](00-overview.md#chaining)).

❓ **Peer sync trusts unsigned records and has no clock authority** — a peer can
push an unsigned record for any name, and "newer wins" compares clocks that are
not synchronised. *Settled by:* owner.
