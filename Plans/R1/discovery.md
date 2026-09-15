# Discovery extensions

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

### Where a member says it is

**A service states its hostname at registration, as a field of its own.** It
became necessary the moment a name stopped carrying one: a pool's realm claims
membership rather than a location ([identity § names](../../docs/01-identity.md#names)),
which is right, and leaves *which box do I go and look at* unanswered.

| | |
|---|---|
| **stated, never observed** | the daemon cannot see it. A unix socket has no peer host at all, and a TCP peer address is the network's opinion rather than the machine's name. So the machine says |
| **a label, and never an input** | nothing routes, authorises or decides on it, because nothing checked it. It is for the person reading a listing at three in the morning |
| **one entry per member, not one per name** | a pool registers one name from several places, so what is kept is a set — refreshed by the heartbeat that already says a member is alive, and gone when that member is ([health checker](#health-checker)) |
| **not part of the identity** | the name is the identity ([identity § names](../../docs/01-identity.md#names)); this is a fact about a process serving it. Keeping the two apart is the whole reason the realm stopped carrying a hostname |

It is also where a **version** would go when a service can say one
([Plans/R1](TODO.md#todo-r1)) — same shape, same reason: per member,
stated, and the answer to *which of these four is the odd one out*.

## Method metadata

Status: proposed, not built. The MVP carries a service's method information in
its [description](../../docs/03-services-and-topics.md#service-and-template) —
one sentence, written by whoever registered it, and the only thing a listing or
the MCP catalog can show. That is enough to recognise a service and not enough
to call one: a caller cannot learn a verb's arguments from it, nothing marks the
verbs that destroy something, and a face has nothing to generate documentation
from.

R1 replaces it with something **stated**, the same shape as
[where a member says it is](#where-a-member-says-it-is): supplied by the
registrant, kept raw, and not a value the daemon interprets or routes on. What
it has to answer:

| | |
|---|---|
| **which verbs there are** | a service's callable methods, each described on its own, so a catalog lists them instead of one sentence about the whole service |
| **which of them destroy something** | the hint a face needs before it calls one on a person's behalf |
| **generated, not maintained twice** | the faces build their catalog from what the service stated, rather than from prose somebody keeps in step by hand |

The representation, and whether the daemon checks its shape at all, is release
design work after scope confirmation. The description field stays either way: a
service that states nothing keeps exactly today's behavior.

## Health checker

Itself an agent: registered, replicated 2×, holds a `health` role on the
services it probes.

- **generic** → probe per the registered hints.
- **agent** → heartbeat; K missed → down.

A record its owner declared down or retired is not probed and not counted
against: what this reports is observed, and that is stated
([down and retired](../R1.1/records.md#down-and-retired)).


## Stats

Agent heartbeats carry a small metrics blob; probe results (latency, up/down)
are the stats for generic services; queue depth and rates for topics.

Kept in memory — ring buffers, last N hours, fixed resolution — and dumped with
the queues ([messaging § durability](../../docs/04-messaging.md#durability)), so a
graceful restart keeps the window, and a crash loses the last interval — or all
of it, if periodic dumping is off.


## Exports

Prometheus `/metrics` first (Grafana reads it); OTLP / StatsD secondary.

## Dashboard extensions

Optional for MVP; assigned to R1 by the owner on 2026-09-13. The
[required dashboard](../../docs/05-discovery.md#required-tabs) owns the baseline.
These additions do not remove its built diagnostic views.

| View | Stage | Dependency |
|---|---|---|
| **advanced groups** — nested expressions and delegated administration | R1 | [groups and roles](identity.md#groups-and-roles); basic group and maintainer administration is [MVP](../../docs/01-identity.md#groups-and-maintainers) |
| **health** — up, down, and how long since the last probe | R1 | the health child ([health checker](#health-checker)) |
| **advanced activity** — longer history, latency distributions, comparisons and export | R1 | [stats](#stats) and [exports](#exports); basic activity graphs are [MVP](../../docs/05-discovery.md#required-tabs) |
| **runner** — what a `runner@<host>` manages: every service it knows, which are enabled and which are up ([runner § what an instance is](runner.md#what-an-instance-is)), with the verbs on each, and a form that installs a new instance | R1 | nothing new — the runner is a service and answers like one ([runner § reaching the runner](runner.md#reaching-the-runner)). Every control **posts as the person** ([rules it is built to](../../docs/05-discovery.md#rules-it-is-built-to)), so the page drives a runner it has no authority over |
| **children** — bus, web, auth: alive, restarted how often. Not the runner, which the supervisor does not start and has nothing to report about | R1 | the supervisor reporting into the bus. Until then that answer is `agent-bus status` and the page says nothing about it |
| **origin** — which node a record came from | R1 | chained registries ([overview § chaining](federation.md#chaining)) |
| **exchange explorer** — expanded investigation of late and unanswered calls | R1 | the existing MVP envelope timeline remains available |
| **operations tab** — consolidated node/child health, restart history, stuck queues and alerts | R1 | supervisor and health reports; existing node status and stuck-inbox views remain MVP |
| **advanced access/settings** — central ACL editor, account mapping and credential administration | R1 | extends required per-record controls and existing CLI administration; never renders raw credentials |

A one-time sign-in code and a dashboard-scoped credential remain candidates from the R1 plan.
