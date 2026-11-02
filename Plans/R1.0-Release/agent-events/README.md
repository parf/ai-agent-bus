# Agent events

📌 **TL;DR:** R1 gets per-agent logs, subscriptions and a small web view:
a few recent lines, alerts loud, warnings quieter, and expand to the full log.
The end state is one Go engine in the daemon with `#eventlog` as its front;
that migration and the daemon's own producers are post-1.0. This plan contains
no implementation. Persistence is [pending owner, Q321](../QUESTIONS.md#r1-release-decisions).
The volatile-entry option costs about 3–5¼ AI hours in total, with a
3–5-hour tree-ready delay when the three workers overlap their parts.

Owner's [Q312 decision](../DECISIONS.md#recorded-decisions) and
[falsifiable acceptance](TODO.md#r1-min) define the cut.

## One engine at the end

Recommend keeping `#eventlog` as a thin front, rather than retiring its name:
existing writers, named logs, per-instance admission and subscriptions keep
working. The daemon owns both agent logs and isolated named-log namespaces;
`#bus` offers agent-log reads and subscriptions. A named log belonging to one
`#eventlog` instance must never become visible through another instance.

R1-min does **not** move the Python code into Go. It implements the same
[log model](../../../docs/agents/eventlog.md#log-model) and verb meanings in
Go for agent records; `#eventlog` continues unchanged for named logs. The
[built agent guide](../../../docs/agents/eventlog.md#what-it-does) owns its
current behavior; [the front transition](#eventlog-front-and-existing-state)
is a separately priced step.

The inverse, a runner writing agent events into `#eventlog`, is rejected:
its ACL is per instance rather than per log, and daemon events must not depend
on a running agent.

## Levels

The [shared log model](../../../docs/agents/eventlog.md#log-model) is the
single specification for levels, ring defaults and bounds, TTL, ordering, ids,
line normalization and fixed-window repeat folding. This page and the
[named-log plan](eventlog.md#rules) link there; they do not keep another table.

Runner start/requested stop produce notices, stderr lines warnings, and an
unexpected failure alerts. Requested stop is never a failure alert. Agent
self-writes use the same severities. Agent targets resolve to the current
registry record identity, not an arbitrary named log; a replacement record
must not inherit its predecessor's private entries.

### Drift guard

`src/agents/shared/event_rings.py` already implements the model and
`src/agents/eventlog/test_rings.py` runs `src/testdata/event-rings.json`.
Add a Go adapter to the **same** vectors; do not translate them into a second
fixture set. Extend shared cases for Unicode/control stripping, TTL boundaries,
fold windows, level isolation, pull order and inclusive clean. A changed
expectation must fail both adapters when it concerns the shared model.

Separate entry-retention profiles from model rules: today's Python profile
keeps alerts; R1's memory-entry profile loses them at restart. Split the
existing combined restart case into common id-reservation checks and explicit
retention checks; do not claim that volatile alerts pass a durable-alert
expectation. Both engines run the common cases; each configured retention
profile has a named restart assertion. Go must run the durable-alert cases
before that optional storage step can land.

## R1-min surface and authority

Recommend the existing `send`/`call` path to `#bus`: every client and agent can
already call it, no new SDK method is required, and the small web view can use that same call without a new HTTP route. Use eventlog's verbs, with an agent record in the log position:

| Verb | Meaning in `#bus` |
|---|---|
| `write <agent> notice\|warning\|alert <text>` | append to the caller's own log; daemon Owner may also write |
| `read <agent> [level]` | inspect without consuming; no level means all levels |
| `list` | only agent logs the caller manages, with live counts |
| `pull <agent> [level] [count]` | oldest ids first, default 100, range 1–500; remove each once |
| `clean <agent> [level] [id]` | no id clears the selection; an id is an inclusive cutoff |
| `sub add <target> <agent> [level] [/regex/]` | starts now for one managed agent; no level means all; target is an agent or queue |
| `sub list [agent]`, `sub delete <id>` | caller-owned or source-managed subscriptions; ids are opaque, not entry ids |

Do not add an `events` alias over `read`: one set of verbs avoids drift.
Method metadata and help list only methods the caller may use. The agent
argument is a canonical registry name, including its realm; it is not subject
to eventlog's short named-log syntax. Pull/clean retain eventlog's optional
level/count/id disambiguation; use its documented structured call when selecting
a level alone. Text and structured forms have the same validation. Existing
eventlog result semantics, order, count and truncation rules are the common
reply contract; P1/P3/P4 build against that guide, not a second reply design.
No new JSON shape is introduced here. Regex uses the existing portable
subscription-filter subset and shared vectors; no broader Go-only syntax.

### Read and list completeness

Use [eventlog's existing reply semantics](../../../docs/agents/eventlog.md#technical-details),
with these cases explicit for P4:

- `read` of a never-written or fully cleaned log returns the content-free
  not-found method refusal. An authorized agent record is not itself a log.
  An active log whose entries expired or were all pulled returns empty rings,
  zero counts and no latest alert; no refusal and no invented placeholder line.
- `read` returns **all** live selected entries, newest first within each level,
  levels alert/warning/notice. Its maximum is the shared combined ring capacity:
  500 entries per answer. Counts describe all levels even for a level-only read.
  A full result is complete; there is no truncated success, cursor or hidden page.
- Select a level with `read <agent> <level>`; it is also a complete selection.
  `read` has no count or id bound. The count on `pull` removes entries, and the
  id on `clean` removes through that id: neither is a way to page a read.
  Expired, evicted or consumed entries cannot be fetched by an older id.
- `list` returns all authorized **active log summaries**, sorted by name, with
  counts but no entry bodies. Never-written/fully cleaned logs are absent;
  active empty logs remain, and no visible active logs gives an empty list.
  It does not cut or paginate. The same hard reply budget as eventlog applies:
  over-budget output is a method refusal, not partial success. Inspect a known
  agent with `read` rather than treating a count/id as a missing list-page flag.
- The preview's three-line cut is explicitly a web presentation choice, with
  expand to the complete bounded `read`; it is not a truncated API result.

### P2 write transport

P2 uses the existing `send` to `#bus`, body `write <agent> <level> <text>`,
with the target agent's credential. The answer follows the supplied `reply_to`
via the existing reply route and preserves correlation. A refused operation
answers with a body starting `error:`; content after it is fixed and safe,
not raw storage/refusal internals. HTTP acceptance of the send alone proves
only queue acceptance, not a successful log append. No different P2 body or
`callBus` transport is required by this contract.

### Who reads

- The record's Owner, Maintainers and daemon Owner may read, pull, clean and
  subscribe. Bare admission and a `read` role grant **no** agent-log data:
  stderr can carry anything. List, status and subscriptions must not leak it.

### Who writes

- **Settled A:** the runner submits each agent's observations with that
  agent's existing `s.bus` credential, not `r.bus`. The daemon accepts a write
  only when caller equals the target agent; the daemon Owner may also write.
  Agent self-writes use that same own-agent rule. There is no daemon-readable
  runner inventory today, and tags or a shared Owner are not proof of a binding.
- An agent can write a misleading line about itself, seen only by its own
  Owner and Maintainers. The daemon Owner retains the read access above.
  No verb, entry presentation or web label claims verified runner provenance;
  a self-asserted “source: runner” is only untrusted text. Agent log lines never
  change the runner's process status or its separate trusted observations.
- Only the daemon Owner reads `#bus`'s log; automatic daemon events are post-1.0.
  Named-log role behavior remains the existing `#eventlog` contract, not an
  extra reader grant for agent logs.
- Daemon-generated lines use fixed descriptions plus approved record/reason
  identifiers: never tokens, message bodies, raw refusal internals or arbitrary
  `error.Error()` strings. Runner-generated lifecycle lines follow that rule.
  Forwarded stderr remains untrusted, private data; sanitization is not a
  promise that secret text can always be recognized and removed.

### Protocol and clients

The [freeze rules](../../../docs/09-daemon-api.md#stability-and-compatibility)
say a break to a conforming stable client is a "MAJOR version" and require
clients to ignore unknown answer fields. The owner's
[pre-1.0 decision](../DECISIONS.md#recorded-decisions) now keeps `protocol` and
`protocol_min` at 1 until 1.0.0: before then, wire additions amend protocol 1
and its shared vectors; from 1.0.0, the integer rises with any wire change.

Every verb above is additive agent-method behavior over unchanged message
routes; none changes an old verb, envelope, refusal kind or client handshake.
Transport errors retain the existing HTTP refusal set; method-level errors
use eventlog's content-free descriptions through the existing `#bus` answer
convention: the body starts `error:`. They are not new HTTP kinds. A new HTTP refusal kind or broader read permission
would not meet this design; neither is proposed. Keep T-24's client vectors
passing, and add representative call/refusal integration cases through the
existing client calls; ring/ACL/subscription cases belong in their shared
engine fixtures, not six copied SDK suites.

A direct HTTP events read/write surface is an alternative, **also protocol 1
before 1.0.0**. It costs another 15–25 AI minutes for
route inventory, unknown/duplicate-field/refusal checks, documentation and
T-24 conformance cases, plus any new SDK helpers. It offers a shorter path for
high-volume writers and web reads. Defer it unless measured call-path cost
requires it; this recommendation rests on reuse, not protocol avoidance.

## Flood and persistence

**Persistence: [pending owner, Q321](../QUESTIONS.md#r1-release-decisions).**
Recommend alerts on disk in R1: daemon deploys restart it several times a day,
including the release deploy, and an agent's verb-written alert is not copied
to `error.log` or syslog; those hold daemon-generated errors. P5 adds this after
P3 for 25–40 AI minutes. The memory option below remains an alternative, not
a recorded decision.

Proposed admission defaults: ten accepted occurrences per second per agent,
burst twenty, counting folded occurrences too. Match the existing eventlog
request bound for explicit writes; runner stderr capture must also have a
bounded partial-line buffer before normalization. Ring bounds remain the
shared model. Runner forwarding has a bounded queue; overflow drops and
counts rather than blocking the child. Existing routine stderr output can
still make noisy warnings; caps do not classify it, and the all-agent producer
cleanup is outside this cut. Per-agent serialization must not hold
the registry's global lock during disk work or subscription sends. Rate-limited
entries consume no ids; expose their count without logging another event per
drop. Budget these controls with the engine/writers, not as later hardening.

| Option | Restart and retention | Work |
|---|---|---|
| All entry rings in memory, if chosen in Q321 | Every level expires by the shared TTL and disappears at daemon restart | No durable entry table; durable id high-water is still mandatory |
| P5 alerts on disk, recommended, pending Q321 | Alerts survive restart until the same TTL/size/clean/pull removes them; other entries remain volatile | New store table, migration, atomic writes/removals, reopen/corrupt-state checks and storage mutants |

"All in memory" cannot include the id allocator while retaining eventlog's
never-reused-id rule. Persist reserved high-water blocks before issuing ids;
retain the tombstone through clean. This is its own ⚠️ storage step, even
without durable alert bytes. A failed reservation returns no accepted entry;
never fall back to a restarted counter. Existing storage primitives may reduce
the work, but their integration and restart proof are not free.

### P1 storage seam for P5

P1 separates ring/model operations from storage: id-block reservation is always
durable; entry retention has a replaceable backend. The initial memory-entry
backend stores no alert bytes. P5 supplies alert load/commit/removal through
that seam, without rewriting the engine, verb parser, ACL or web contract.
Reserve ids before use, commit a durable alert before accepting/publishing it,
and commit pull/clean removal before answering. A backend failure cannot
acknowledge a half-write or roll the high-water backwards. P1 includes a fake
backend failure check; P5 proves the real table/migration/restart path. No
schema or wire layout is selected in this plan.

### Subscriptions are new daemon work

The selected R1 cut includes subscriptions; R1-min without them is an
estimate comparison and does not satisfy Q312. Reuse core's existing send path and eventlog's send-once,
drop-and-count behavior, without a retry queue or outbox. Copies are decided
before a matching entry can be taken by pull, including accepted folded
occurrences. A full/revoked/unreachable target drops that copy, increments its
counter and leaves the subscription available for the next entry.

The daemon still needs durable definitions, management, creator/source/target
ACL checks, regex matching and bounded fan-out. Check authority at creation
and delivery; management is creator or current source managers, with no new
reader role. Store identities, not credentials. Reserve capacity before
accepting a write; no network call holds the global registry lock. Failure
counters must not recursively generate subscribed events. Shared subscription
vectors already exist, but a Go adapter and storage are new work.

### Bounded dispatch and fairness

The present `ServeDaemonAgent` consumes, answers and replies on one goroutine.
Do not put synchronous subscription fan-out on that reader. **P1 adds bounded
separate dispatch lanes:** two workers for help/status/version/read/list, four
for mutations, with bounded queues (64 read/control jobs, 64 mutations in total and at most
16 waiting mutations per agent). Admission to a full lane returns a content-free busy
answer; it never starts another goroutine. Schedule mutation-ready agents
round-robin, at most one active mutation per agent. A hot agent cannot occupy
all mutation workers, and fan-out cannot borrow read/control capacity.

**P3 chooses delivery off the answer-reader path**, rather than merely waiting
for each target on the sole reader. One write's mutation worker commits and
publishes its entry, completes the bounded subscriber attempts, then answers.
Pull/clean for that agent pass the same per-agent completion barrier, so they
cannot erase an entry before its subscriber attempts finish. Read takes a
snapshot without waiting for that delivery barrier. The engine catalog lock only finds per-agent state; id I/O and delivery do
not hold that lock or the global registry mutex. No ring mutex stays held
over a send. Other agents' reads and status/help use the
reserved lane while a delivery is blocked.

Use the existing non-waiting core queue-admission/send path, not HTTP calls
back into the daemon. Cap each target attempt at one second and a write's
whole fan-out at five seconds, or its remaining caller deadline if shorter;
unattempted copies at the budget are dropped and counted once. These bounds
do not permit a timed-out send goroutine to continue later: the delivery
adapter must finish/cancel before the barrier is released. The current core
queue send refuses a full target rather than waiting for its consumer.

A readiness-barrier fixture holds one target's injected send pending, then
issues an unrelated read and status. Both must answer while that send is still
held; a named two-second assertion bounds the check, inside a longer fixture
send budget. Only then release/cancel the target and verify pull, counts and
single delivery. A serial-dispatch mutant must fail that reply assertion,
not a harness timeout. Saturate one agent's mutation queue too: another
agent still gets a worker, and control replies remain available. P1 owns the
lane/fairness proof with an injected blocking operation; P3 repeats it through
real subscription selection and delivery. This adds 15–25 AI minutes to P1
and 10–20 to P3 over the first estimate; it is required work, not polish.

## The bus's own events

Post-1.0: wire up the daemon's own start/stop, store, authentication, TLS and
other report sites. Agent logs and subscriptions do not require those hooks;
adding them now expands the producer audit and risks re-emitting internal
refusal or secret text. `error.log`, audit and syslog remain independent.
Any later engine failure uses the existing journal, not its own subscriptions.
The wider producer catalogue remains in [the TODO](TODO.md#per-agent-events).

## Web

**In R1, P4:** per agent, preview the three greatest entry ids across all
levels, newest first; style alerts louder than warnings, with expand to the
full log. Do not mistake level-prioritized read order for a global recent-line
selection. Expansion reuses `read` and its level order;
no websocket, new stream route, dashboard aggregation or visual polish.
Recheck Owner/Maintainer/daemon-Owner access on the server for both preview and
expand, render text as untrusted data, and show no verified runner source.
The limit is presentation-only: it does not change ring retention or read order.

## Later runner provenance

Option B is post-1.0: explicit daemon-side runner-to-agent writer grants,
controlled by the target's managers, plus current-binding validation and
separate producer provenance. Host-local `agents.json` is not observable by
the daemon; an Owner-set tag is not authority. This adds authority/storage
work and deployment handoff, while option A uses credentials already handed
to the runner. Do not quietly replace A with a shared-owner shortcut.

## Eventlog front and existing state

Post-1.0: keep deployed instances on the current engine until migration is
accepted. This can ship separately: agent logs have a distinct private audience,
and safe import/rollback of existing named logs adds storage and ACL work.
Do not delay the agent-log flow for this conversion.
Stop one instance's writes, preserve its work-directory state and subscription
KV, then import alerts, reserved id high-waters/tombstones, configuration and
subscription identities into an isolated daemon namespace. Validate counts,
ids and retention before one atomic cutover; no dual writer or replay of old
entries to subscribers. Notice/warning loss is the existing restart behavior.
Keep the backup for rollback; refuse corrupt/ambiguous imports without deleting
or resetting the original files. Resume the old engine only after disabling
the front, so rollback cannot deliver twice. There is no automatic deletion of
old logs. This is storage/authority work, not merely a proxy rename.

## Build split and estimate

Based on the inspected `a949ffc9` code plus the owner decisions now on local
main: Python rings already have durability and shared vectors; `#bus` currently
serves help/status/version/ls, runner stderr is local, and daemon-side
subscription management does not exist. Times are focused AI implementation
work; reviews and gates are separate. ⚠️ parts need compiling assertion-red
mutants and two readers. No production log migration is in the R1 cut.

| Part | Likely paths and subsystem letter | AI time | R1-min or later |
|---|---|---|---|
| Daemon ring engine + flood bounds, P1 | `src/internal/journal/` C ⚠️; core A ⚠️ | 25–35 min | R1 |
| Bounded dispatch and reserved read/control lane, P1 | daemon agent E; core A ⚠️; harness M | 15–25 min | R1, required for P3 fairness |
| Durable id high-water, P1 | `src/internal/ports/`, `store/`, `journal/` C ⚠️ | 15–25 min | R1, mandatory even for volatile entries |
| Runner writers + bounded stderr, P2 W2B | `src/cmd/agent-bus-runner/`, `internal/script/` F ⚠️ | 20–30 min | R1, `s.bus` per agent |
| Agent self-write, P1 | core A ⚠️; `src/internal/api/daemon_agent.go` E | 5–10 min | R1; no all-agent producer migration |
| Read API + ACL, P1 | core A ⚠️; daemon agent E; help/docs H/N | 20–30 min | R1 via verbs |
| Shared ring/vector adapters, P1 | `src/testdata/` M; eventlog/shared L; journal C ⚠️ | 10–20 min | R1 |
| Subscriptions + durable management, P3 W2C | core A ⚠️; ports/store C ⚠️; daemon agent E | 40–65 min | Selected R1 cut, including bounded fan-out |
| P5 alerts on disk, after P3 | store/ports/journal C ⚠️ | 25–40 min | Pending owner, Q321; recommended for R1 |
| `#bus` own events | `src/cmd/agent-busd/` H; core/journal A/C ⚠️ | 15–25 min | Post-1.0 |
| Three-line preview + expand, P4 W1C | `src/web/` I; core/API A ⚠️/E | 20–35 min | Selected R1 cut |
| Eventlog front and migration | eventlog/shared L; store/core C/A ⚠️ | 35–60 min | Post-1.0; existing logs stay put |
| Tests and mutants, P1/P2 | owning packages; smoke/acceptance M | 20–35 min | R1; optional-part proof included in their rows |
| Runner provenance B | core/store A/C ⚠️; runner F ⚠️ | +15–25 min | Post-1.0; outside totals |
| Direct HTTP alternative | API E/protocol D ⚠️; vectors/clients K/M; docs N | +15–25 min, SDK extras separate | Alternative, outside totals |

P1 is W2C, target 11:40–12:40; P2 is W2B, P3 is W2C after P1, P4 is W1C.
P5 follows P3 if the owner chooses disk alerts; P1 builds its storage seam now.
P2 and P4 can build against the verb contract while P1 is implemented, then
prove their integration against it. P1's counted scope is 110–180 minutes,
including its proof allocation: a one-hour slot is an aggressive target,
not evidence that storage/ACL checks take no time. An overrun must be reported,
not hidden by skipping tests. P3 follows the engine; readers and the gate
remain shared resources.

| R1 choice | Summed AI time | Tree-ready moves later by, with the split | Calendar impact |
|---|---|---|---|
| R1-min without subscriptions or web, volatile entries | 2 h 10–3 h 30 | 2 h 20–4 h, including 30–60 min review/gate | Comparison only; moves the Nov 2 release run by one session and does not meet Q312 |
| With subscriptions, without web | 2 h 50–4 h 35 | 3 h–5 h 05, including 30–60 min review/gate | Comparison only; same Nov 2 delay, but omits the owner's web requirement |
| **Selected R1 scope: subscriptions + small web; memory-entry cost if Q321 chooses it** | **3 h 10–5 h 10** | **3 h–5 h 05**, P1 then P3, P2/P4 overlapped, plus 30–60 min review/gate | With an 11:40 start and no queue, tree-ready about 14:40–16:45 on Nov 2; late starts/findings can slip to Nov 3 |
| Selected scope + P5 durable alerts, recommended, pending Q321 | 3 h 35–5 h 50 | 3 h 25–5 h 45, including its migration/restart proof | Same scope, longer Nov 2 window; alerts survive only within the shared retention limits |
| Full listed set: durable alerts, `#bus` producers, front/migration | 4 h 25–7 h 15 | About 5½–8¾ h with review/gates; migration is serial | Another full session; not the R1 cut |

These are delays from the existing release-run/Fable tree-ready point, not a
new promised release date. Lock queues and findings add time. The earlier
1½–2-hour whole-feature estimate is too small for the selected cut. The owner's
R1 scope decision stands; persistence is [pending owner, Q321](../QUESTIONS.md#r1-release-decisions),
with P5 recommended and priced separately after P3. The named-log transition and verified
runner provenance are not hidden in that estimate.
