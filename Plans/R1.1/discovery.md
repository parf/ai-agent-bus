# Discovery extensions

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Where a member says it is

Moved to R1 with pools (Q139, 2026-10-16): [where a member says it is](../R1.0-Release/runner-implementation.md#where-a-member-says-it-is).

## Method metadata

Method metadata stayed with the release's own scope: see
[method metadata](../R1.0-Release/done/method-metadata.md#method-metadata).

## Health checker

Itself an agent: registered, replicated 2×, holds a `health` role on the
records it probes.

- **service** → probe the external thing per its registered address and protocol.
- **agent** → heartbeat; K missed → down.

A record its owner declared down or retired is not probed and not counted
against: what this reports is observed, and that is stated
([down and retired](../R1.2/records.md#down-and-retired)).


## Stats

Agent heartbeats carry a small metrics blob; probe results (latency, up/down)
are the stats for external services; queue depth and rates for the two channel kinds.

Kept in memory — ring buffers, last N hours, fixed resolution — and dumped with
the queues ([messaging § durability](../../docs/04-messaging.md#durability)), so a
graceful restart keeps the window, and a crash loses the last interval — or all
of it, if periodic dumping is off.


## Exports

Prometheus `/metrics` first (Grafana reads it); OTLP / StatsD secondary.

## Dashboard extensions

Optional for MVP; assigned to the old R1 by the owner on 2026-09-13, and R1.1's since the 2026-09-30 split. The
[required dashboard](../../docs/05-discovery.md#required-tabs) owns the baseline.
These additions do not remove its built diagnostic views.

| View | Stage | Dependency |
|---|---|---|
| **advanced groups** — delegated administration | R1.1 | [roles](../../docs/03-records-agent.md#roles); basic group and maintainer administration is [MVP](../../docs/01-identity-and-authority.md#groups); group expressions are [R1.3](../R1.3/composed-groups.md#group-expressions) |
| **health** — up, down, and how long since the last probe | R1.1 | the health child ([health checker](#health-checker)) |
| **advanced activity** — longer history, latency distributions, comparisons and export | R1.1 | [stats](#stats) and [exports](#exports); basic activity graphs are [MVP](../../docs/05-discovery.md#required-tabs) |
| **runner** — what a `#runner/<instance>@<host>` manages: every agent it knows, which are enabled and which are up ([runner § what an instance is](../R1.0-Release/runner-implementation.md#what-an-instance-is)), with the verbs on each, and a form that installs a new instance | R1.1 | nothing new — the runner is a service and answers like one ([runner § reaching the runner](../R1.0-Release/runner-implementation.md#reaching-the-runner)). Every control **posts as the person** ([rules it is built to](../../docs/05-discovery.md#rules-it-is-built-to)), so the page drives a runner it has no authority over |
| **children** — bus, web, auth: alive, restarted how often. Not the runner, which the supervisor does not start and has nothing to report about | R1.1 | the supervisor reporting into the bus. Until then that answer is `agent-bus status` and the page says nothing about it |
| **origin** — which node a record came from | R1.1 | chained registries ([federation § chaining](../R1.2/federation.md#chaining)) |
| **exchange explorer** — expanded investigation of late and unanswered calls | R1.1 | the existing MVP envelope timeline remains available |
| **operations tab** — consolidated node/child health, restart history, stuck queues and alerts | R1.1 | supervisor and health reports; existing node status and stuck-inbox views remain MVP |
| **advanced access/settings** — central ACL editor, account mapping and credential administration | R1.1 | extends required per-record controls and existing CLI administration; never renders raw credentials |

A one-time sign-in code and a dashboard-scoped credential remain candidates from the old R1 plan.
