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

❓ **With AUTH off the daemon does not hold the services' local mapping files**,
yet it is the MCP face that filters the catalog. Who filters? *Settled by:* owner.

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

## Exports

Prometheus `/metrics` first (Grafana reads it); OTLP / StatsD secondary.

## Debug mode

An admin may switch a service into debug mode, which keeps a trace of that
service's messages. Admins only. Bodies in the trace are ciphertext — unless
the service has switched encryption off in its own config
([access § encrypted sessions](02-access.md#encrypted-sessions)), the usual
pairing while developing.
