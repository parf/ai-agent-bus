# Data dictionary

Draft for peer review. What each value **means**, what we **call** it, and when
it is **not shown**.

This is the layer where the audited defects actually live: almost every finding
in [W01–W17](../done/web-review.md#findings) and
[C01–C16](review/codex.md#junk-and-misleading-content) is a value that was true
being presented as something it is not. The complete list of what is rendered
today is codex's [inventory](review/current-state.md#visible-fields-by-page);
this file is the vocabulary we replace it with.

## The rule

> Declared state and observed state are never merged, and neither is health.

A record *declares* it is enabled. The daemon *observes* whether a read is
outstanding. Neither says a process is alive, and nothing on this dashboard is
allowed to imply it does.

## Fields

### Identity

| Field | Means | We call it | Not shown when |
|---|---|---|---|
| `Name` | full routing address, `user@realm` | the name, in monospace | never hidden — it disambiguates sessions |
| `Descr` | caller-supplied display text | the description, shown first | absent: the name stands alone, no empty label |
| `Kind` | generic, agent or topic | Service, Agent, Channel | — |
| `Mode` | queue or pub/sub, topics only | delivery: one at a time, or a copy each | not a topic |
| `Owner` | the principal who owns the record | owner | — |
| `Maintainers` | a group with management authority | maintainers | **unset: the label goes too** |

### State

| Field | Means | We call it | Never |
|---|---|---|---|
| `Disabled` | the owner turned delivery off | **Disabled** / **Enabled** | "Inactive" — it reads as broken. It is a decision |
| `Reading` | a read is outstanding on this inbox right now | **reader attached** / **no reader waiting** | "Serving"/"Offline". A busy process between pulls is not offline, and this is not health |
| `Proto` | a caller-supplied hint that the thing is reached another way | **external** | treating it as proof of anything, or as a healthy state |
| user `State` | active, paused or banned | active / paused / banned | conflating a user's state with a record's |

**Queue counters persist; activity history does not.** The two are easy to
conflate and we did: accepted, dequeued, dropped and expired are restored from
the snapshot, while sampled traffic history starts empty after a restart. So a
counter spanning a restart is a real cumulative total, and a graph before the
restart boundary is `¿`.

### Queue

| Field | Means | We call it | Not shown when |
|---|---|---|---|
| `Queued` | messages waiting now | queued | — |
| `Oldest` | age of the oldest waiting message | oldest waiting | empty: `—`. The queue is empty **now**; the daemon does not say whether it ever held anything |
| `In` | messages accepted, **cumulative across restarts** | **accepted** | — not "since start": [`Restore`](../../../src/internal/core/snapshot.go) puts these back from the snapshot. codex's correction |
| `Out` | messages handed to a reader, cumulative across restarts | **dequeued** | — never "completed". Handing a message over is not doing the work |
| `Dropped` | discarded by the overflow policy, cumulative across restarts | dropped | zero: `0`, which is a measurement |
| `Expired` | passed their TTL undelivered, cumulative across restarts | expired | zero: `0` |
| `AtBound` | the queue is at capacity now | **at bound**, red | claiming messages are being dropped or refused *right now*. Capacity reached is not arrival; what happens on the next send depends on the overflow policy |
| `TTL`, `Bound`, `Full` | this record's own settings | with inheritance stated: *uses the daemon default* where unset | — the daemon resolves defaults internally and the record keeps them unset; never copy a default into a template as though the record carried it |

### Node and scope

| Field | Means | We call it |
|---|---|---|
| `Status.Up` | time since this daemon started | uptime |
| `Status.Services` | **node-wide** record count | records on this node |
| `Status.Queued`, `Waiting`, `Dropped`, `Expired` | node-wide totals | labelled node-wide, always |
| `Status.Unclean` | the last stop did not write memory down | a fact, with no acknowledgement state |
| `Refusals` | lifetime counts by reason | refusals since start — **never** "climbing", which needs a window we do not have |

**Node totals and any list never have to agree**, and every page showing both
says so: one is the node, the other is what this caller may see
([C16](review/codex.md#junk-and-misleading-content)).

### Credentials

| Field | Means | We call it |
|---|---|---|
| `Fingerprint` | names a credential without being one | fingerprint |
| `Issued` | when it was minted | issued |
| `Used` | last seen this run | last used, or *not this run* — not an expiry signal |
| `Kind: unregistered` | a credential whose name the daemon holds nothing else for | a leftover, with what it means |

A credential is never rendered. Age is not expiry; nothing here retires for
being old.

### Directory

| Field | Means | We call it | Never |
|---|---|---|---|
| identity kind | user, record, or credential-only | a word: registered user / registered name / credential with no registered name | a colour, or a guess from a slash or prefix in the name |
| `Services` | records this identity owns | owned records | — |
| `Groups` | memberships visible to the caller | memberships, or *no memberships* | — |
| group members, withheld | the caller may not see them | **not visible to you** | an empty array shown as zero members |
| `PeopleCount`, `OtherCount` | counts over the caller-visible directory before search | labelled with that scope | |

## Absence

Four facts, four markers, and not a licence to mark every blank
([glyphs](glyphs.md#absence-which-is-four-different-facts)):

| | |
|---|---|
| `0` | measured zero |
| `∅` | measured, non-zero, below display precision — **not** for integer counts |
| `—` | not applicable here |
| `¿` | not measured or not observable |

Where an absence does not change a decision, the label goes with the value
instead. Stating every absence is its own wall of noise.

## Time

One explicit display timezone, full timestamps available, and each of these
labelled as what it is rather than as "date":

registration update · observation time · credential issued · credential last
used · history interval · sample time.

## Not available, and not invented

Named so nobody fills them with a green badge, a zero, or a guess:

| | |
|---|---|
| process health, liveness, execution results | not supplied by MVP data |
| latency distributions, audit history | [R1](../../R1/discovery.md#dashboard-extensions) |
| PID, runtime session id, working directory, process start | launcher-local; the web child must not read launcher homes or `/proc` |
| node label and running build | plausible narrow read addition, justified by the Overview node section — and the web executable's version is **not** the bus version |
| effective resolved queue settings | would need a narrow daemon read; until then inheritance is stated honestly |
| historical credential provenance | if no profile, record or independent evidence remains, it cannot be reconstructed. Keep it unclassified rather than guessing ([provenance](../done/web-review.md#credential-provenance)) |
