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

Two corollaries the first draft left implicit, both from home-parf:

**Provenance is the same class of distinction.** A figure the daemon reported
and a figure the web child computed are not interchangeable, and a table that
shows them side by side without saying which is which has merged them exactly
the way declared and observed must not be merged.

**An observation is worth what its read path actually did.** The daemon reports
queue state without pruning expiry first, so *held* is not *waiting* and *at
capacity* is not *refusing* ([glyphs](glyphs.md#what-an-observation-is-worth)).
Where a value's meaning depends on when it was computed, this file says so.

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
| `Queued` | messages **held** now — the observation path does not prune, so some may already have outlived their TTL | queued | — |
| `Oldest` | age of the head of the queue, which may be a message the daemon already considers expired but has not swept | oldest held | empty: `—`. The queue is empty **now**; the daemon does not say whether it ever held anything |
| `In` | messages accepted, **cumulative across restarts** | **accepted** | — not "since start": [`Restore`](../../../src/internal/core/snapshot.go) puts these back from the snapshot. codex's correction |
| `Out` | messages handed to a reader, cumulative across restarts | **dequeued** | — never "completed". Handing a message over is not doing the work |
| `Dropped` | discarded by the overflow policy, cumulative across restarts | dropped | zero: `0`, which is a measurement |
| `Expired` | passed their TTL undelivered, cumulative across restarts | expired | zero: `0` |
| `AtBound` | at capacity **at the moment of the read** | **at capacity when observed**, red | predicting the next send at all. Not "dropped or refused now", and not "refused or drops oldest" either: the send path prunes before deciding, may hand a waiting reader the message directly, and races a concurrent consume ([glyphs](glyphs.md#what-an-observation-is-worth)) |
| `Bound` | queue capacity | unset: *uses the daemon default* — `boundOf` really does resolve one | — never copy the resolved value into a template as though the record carried it |
| `TTL` | this record's retention | unset: **no expiry**, said in those words | — **never** *uses the daemon default*. There is no retention default: `life()` returns zero, `Expires` stays zero and `prune` skips it. Rendering inheritance here invents a value that does not exist |
| `Full` | overflow policy | unset: **refuse**, the protocol's documented meaning of empty | — a normalisation, not a daemon resolution |

### Node and scope

| Field | Means | We call it |
|---|---|---|
| `Status.Up` | time since this daemon started | uptime |
| `Status.Services` | **node-wide** record count | records on this node |
| `Status.Queued`, `Waiting`, `Dropped`, `Expired` | node-wide totals | labelled node-wide, always |
| `Status.Unclean` | the last stop did not write memory down | a fact, with no acknowledgement state — rendered **only when true**. It is `omitempty`, so absent covers both a clean stop and a first start with no previous stop, and we may not claim either |
| `Status.Refused` | lifetime counts by reason, a `map[string]int` | refusals since start — **never** "climbing" *here*, which needs a window this value does not carry. Only reasons that have occurred are present, so an absent reason is `¿`, not `0`. (Activity does compute per-interval deltas, so a windowed refusal signal is buildable there.) |

**Node totals and any list never have to agree**, and every page showing both
says so: one is the node, the other is what this caller may see
([C16](review/codex.md#junk-and-misleading-content)).

### Credentials

| Field | Means | We call it |
|---|---|---|
| `Fingerprint` | names a credential without being one | fingerprint |
| `Issued` | when it was minted | issued |
| `Used` | last seen this run | last used, or *not this run* — not an expiry signal |
| `Kind: unregistered` | a credential whose name the daemon holds nothing else for | a leftover, with what it means — and **legacy-only** since 0.5.29: such a name is refused `401` on every call and is not issued a credential at all, so the daemon no longer creates these. It describes the cohort that predates the change, and must not read as a live category |

A credential is never rendered. Age is not expiry; nothing here retires for
being old.

### Directory

| Field | Means | We call it | Never |
|---|---|---|---|
| identity kind | user, record, or credential-only | a word: registered user / registered name / credential with no registered name | a colour, or a guess from a slash or prefix in the name |
| `Services` | records this identity owns | owned records | — |
| `Groups` | memberships visible to the caller | memberships, or *no memberships* | — |
| group members, withheld | the caller may not see them | **not visible to you** | an empty array shown as zero members |
| `PeopleCount`, `OtherCount` | counts over the caller-visible directory before search | labelled with that scope, **and marked as the face's own** — they are fields of `peopleView` in the web child, not daemon answers, and sit beside `Services` and `Groups`, which are | presenting a face-computed figure and a daemon-reported one as the same kind of fact. Provenance is the same class of distinction as declared-versus-observed, and this file's rule covers it |

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
