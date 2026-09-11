# Discovery

How participants find each other, and what a human or an agent can see.
("Service discovery" is the name; registration is one operation on it.)

- **The required minimum** is the `agent-busd` core with its registry and
  queues, AUTH role off — auth itself is still required
  ([access](02-access.md)).
- **Direct talk is allowed**: if you already know where something lives, skip
  the lookup.
- Registration carries **health hints**: HTTP endpoint + expected status, TCP
  connect, unix-socket ping, command, interval, timeout. Unix-socket services
  are first-class (`unix:/path`).

## What a listing answers

**Being in the registry and being callable are different facts.** "There is a
MySQL on `db1:3306`" is a complete registration ([services § service
kinds](03-services-and-topics.md#service-kinds)) and nothing on this bus
answers for it; a service template is registered and deliberately does not run
([services § service and template](03-services-and-topics.md#service-and-template)).
So a caller reading a listing needs two more things than a name:

| Field | Says | Absent means |
|---|---|---|
| **`protocol`** | how to call it, when that is not through the bus ([services § how to call it](03-services-and-topics.md#how-to-call-it)) | an ordinary bus service: send to the name |
| **`reading`** | a read on its inbox is outstanding *now* — something is serving it | registered, but nobody is home |
| **`queued`** | how many messages are waiting in it | none are |
| **`in`** · **`out`** | how many messages have arrived for it, and how many a reader has taken, since the daemon started | none have |

`reading`, `queued`, `in` and `out` are **live state, not registry data** — attached to an
answer on its way out and never stored, so nothing in the registry depends on
who happened to be connected ([overview § principles](00-overview.md#principles)). They are the difference between
*"this name exists"* and *"a call would reach someone"*, which is the question
a caller is actually asking.

An inbox that was drained and one nobody ever wrote to both read as empty.
`in` and `out` are what tell them apart, and they are per name: a busy bus
does not make a quiet service look busy.

**A false negative is possible and harmless**: between one long poll and the
next a live service shows `reading: false` for as long as it takes to ask
again. Nothing is lost by calling it anyway — the address outlives the process
and the queue waits ([messaging § inbox
queues](04-messaging.md#inbox-queues)) — so this is a hint for choosing, never
a gate on sending.

## Faces

| Face | For | Does |
|---|---|---|
| **API** | programs | register, look up, send / publish / consume |
| **MCP server** | agents | "what can I use, and how" — generated docs for every service available to the caller, plus tool descriptions where the service exposes them |
| **WEB** | humans | registry browser, health, stats dashboards; curl-friendly; runs as a cgroup-limited child |

The MCP face merges chained levels into one catalog, tagged by origin
([overview § chaining](00-overview.md#chaining)).

## Audience

Per service and topic: who may **see and use** it. Discovery is personalised
and MCP catalogs come pre-filtered.

Audience is answered by the two ACL layers — service first, then master; see
[identity § acl](01-identity.md#acl) for the rules. Groups exist only with the
AUTH role on.

**The daemon filters, because it holds the record.** With AUTH off there are
no mapping files to consult: the service ACL is a field on the record
([identity § acl](01-identity.md#acl)), which the daemon already has, so the
answer is worked out in core and every face — catalog, listing, send — gets
the same one. A face that filtered for itself would be a face that could be
asked a different way.

## Health checker

Itself an agent: registered, replicated 2×, holds a `health` role on the
services it probes.

- **generic** → probe per the registered hints.
- **agent** → heartbeat; K missed → down.

## Stats

Agent heartbeats carry a small metrics blob; probe results (latency, up/down)
are the stats for generic services; queue depth and rates for topics.

Kept in memory — ring buffers, last N hours, fixed resolution — and dumped with
the queues ([messaging § durability](04-messaging.md#durability)), so a
graceful restart keeps the window, and a crash loses the last interval — or all
of it, if periodic dumping is off.

## Dashboard

The WEB child shows the **list of services and topics** with their
descriptions, who is up, and **call counts grouped by minute or hour** —
straight from the ring buffers, no external TSDB, audience-filtered. API
`/stats/<dimension>/<id>`.

It shows the envelope and nothing else
([messaging § envelope](04-messaging.md#envelope)): no message bodies, here or
anywhere.

⚠️ What the MVP ships is the envelope half: a bounded feed of what the bus has
routed lately, read by a separate `agent-bus-web` process over the API. Bodies
are struck out **in the bus**, where the feed is written — so no reader has to
be trusted to leave them alone. The feed is **master's** ([identity § acl](01-identity.md#acl)):
it is a view of the node rather than of any one service, so the per-service
layer has nothing to say about it. The records beside it are filtered per
caller like any listing. Call counts by minute or hour, and the cgroup limit
the design gives the child
([processes § the processes](11-processes.md#the-processes)), are not built.

## Exports

Prometheus `/metrics` first (Grafana reads it); OTLP / StatsD secondary.

## Debug mode

An admin may switch a service into debug mode, which keeps a trace of that
service's messages. Admins only. Bodies in the trace are ciphertext — unless
the service has switched encryption off in its own config
([access § encrypted sessions](02-access.md#encrypted-sessions)), the usual
pairing while developing.
