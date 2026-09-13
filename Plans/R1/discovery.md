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

## Health checker

Itself an agent: registered, replicated 2×, holds a `health` role on the
services it probes.

- **generic** → probe per the registered hints.
- **agent** → heartbeat; K missed → down.


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

| View | Stage | Dependency |
|---|---|---|
| **groups** and who is in them | R1 | groups themselves ([identity § groups and roles](identity.md#groups-and-roles)) |
| **health** — up, down, and how long since the last probe | R1 | the health child ([health checker](#health-checker)) |
| **load** — calls per minute and per hour, per name and for the node | R1 | the ring buffers in [stats](#stats). Inline SVG; `/metrics` ([exports](#exports)) is what a real graphing stack reads |
| **runner** — what a `runner@<host>` manages: every service it knows, which are enabled and which are up ([runner § what an instance is](runner.md#what-an-instance-is)), with the verbs on each, and a form that installs a new instance | R1 | nothing new — the runner is a service and answers like one ([runner § reaching the runner](runner.md#reaching-the-runner)). Every control **posts as the person** ([rules it is built to](../../docs/05-discovery.md#rules-it-is-built-to)), so the page drives a runner it has no authority over |
| **children** — bus, web, auth: alive, restarted how often. Not the runner, which the supervisor does not start and has nothing to report about | R1 | the supervisor reporting into the bus. Until then that answer is `agent-bus status` and the page says nothing about it |
| **origin** — which node a record came from | R1 | chained registries ([overview § chaining](federation.md#chaining)) |

A one-time sign-in code and a dashboard-scoped credential remain candidates from the R1 plan.
