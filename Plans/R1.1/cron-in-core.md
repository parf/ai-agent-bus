# Cron and at in the core

📌 **TL;DR:** The owner wants `#cron` and `#at` to stop being agents and become bus core: any agent keeps its own crons and ats, the web gets **Crons** and **Ats** pages like **KV** with tag filtering and counts on all three, and every agent page shows its schedules the way it shows its KV. All of today's behaviour stays, and the rate floor drops to one second. This is the approach for the owner to approve: the parity list of what the two agents do now and its core equivalent, what a schedule as a daemon-owned object is, the web, the steps by subsystem and the choices in [Questions](#questions). Nothing is built and no format is chosen.

Owner, 2026-11-06 ~14:00: "#cron and #at must be converted from agents to bus core functionality; any agent can setup own crons/at at will. Create separate doc, come with ideas and questions. Our web interfaces should have new items - crons / ats (left) - show all / filter - similar as we do for KV (please add tag filtering/counting for all 3), and show on agent pages - again - same way as we do for KV. Create README TODO Questions doc (maybe combined doc, because its mostly internal re-structure). All our existing functionality will be supported."

Owner, 2026-11-06 14:05, on three details:

- Missed runs: "yes, but we have queues, which makes this much easier".
- Rate: "no more than once/second looks reasonable; if you need more often, you surely don't need cron / at, write it yourself".
- Caps: "setup reasonable limit of entries, like 10k".

Owner, 2026-11-06 14:08, on one object instead of two: "good idea". Owner, shortly after, on existing data: "we have no crons / at now".

**Decided:** one **schedule** object, with an *at* as a one-shot schedule and the web's Crons and Ats as two views of it; the `#cron` and `#at` agents are removed when the core schedule ships, with no data to migrate and no proxy period ([one canonical interface](../../CLAUDE.md#working-rules)); the may-send check runs before every firing.

## Parity list: today and its core equivalent

`#cron` and `#at` are two runner agents on one shared engine (`src/agents/shared/scheduler.py`): entries live in the agent's own KV, one process fires at a time under a record lock, and every firing is claimed in KV before its single send attempt. Pages: [cron](../../docs/agents/cron.md#what-it-does), [at](../../docs/agents/at.md#what-it-does). The owner: "All our existing functionality will be supported", so each method, option and limit below has a core equivalent, checked row by row in step 8.

Read-only look at the live node, 2026-11-06 14:01 (`sub list`, `later list`, `list`, as the calling session): both agents hold nothing and run on UTC, which matches the owner's "we have no crons / at now". Nothing is migrated.

| # | Today | Core equivalent | Differs |
|---|---|---|---|
| 1 | `#cron` `sub add`: a recurring entry on a five-field expression | create a recurring schedule on a record | the owner of the schedule is a record, not the creator inside one agent's KV |
| 2 | `sub list` | list a record's schedules, and all the visitor may use | adds the web views |
| 3 | `sub delete <id>` | delete a schedule by id | |
| 4 | `#cron` `later`: once, after `+90s`, `+15m`, `+2h`, `+3d` or at an RFC 3339 instant with an offset | create a one-shot schedule (an *at*) with the same forms | |
| 5 | `later list`: pending and the last 100 outcomes | list one-shot schedules with their pending state and the last 100 outcomes | |
| 6 | `later cancel <id>` | cancel a one-shot by id; one whose send is in flight may still deliver once | |
| 7 | `#at` `in`: a delay `90s` `30m` `2h` `3d`, optional leading `+`, no compound | the same delay forms on any one-shot | `#cron`'s `later` and `#at`'s `in` become one |
| 8 | `#at` `at`: `HH:MM` in the zone (the next one strictly after this minute) or an instant with an offset | the same on any one-shot | `HH:MM` also works for what was `#cron`'s `later` |
| 9 | `#at` `list`, `cancel` | the same list and cancel as 5 and 6 | |
| 10 | Five fields: minute, hour, day of month, month, day of week (0 and 7 Sunday), each `*`, `n`, `a-b`, `*/n`, `a-b/n` or a comma list; no names or macros; Vixie day rule; an expression that can never fire is refused; `0 0 29 2 *` fires in leap years | the same expressions, same meaning, fire at second 0 | a seconds field is added, down to one second ([Q388](#q388)) |
| 11 | Times are local wall-clock in the zone; a skipped minute does not fire, a repeated one fires once; a one-shot `HH:MM` in a gap is refused with the nearest valid time; an instant more than 30 s in the past is refused; at most 366 days ahead; an empty message is refused | the same, per schedule | the zone is per schedule, default UTC ([Q386](#q386)) |
| 12 | The message is the rest of the line, up to 16384 bytes, unchanged | the same | |
| 13 | Arrives from `#cron` or `#at`, topic `cron`, `later` or `at`, tag `by <creator>` | arrives from the owning record by default, topic `cron` for recurring and `at` for one-shot, tag `by <creator>` | sender name ([Q380](#q380)); the topic `later` merges into `at` |
| 14 | One attempt per firing; a claimed send is never repeated; an unconfirmed send is dropped as `send_unknown`; an access refusal `admission_refused`; a vanished target `target_gone`; any other refusal `send_refused`; nothing is retried | the same outcome names and warnings | |
| 15 | The creator must be allowed to send to the target when the entry is added and before every send; lost access drops that copy and the subscription stays | the owning record's may-send check, when added and before every firing; the same drop | required in the design; the creator check becomes the owner's |
| 16 | A new entry must name an existing queue or agent the sender can see | the same | |
| 17 | Delete or cancel by the creator, or the agent's Owner and Maintainers; lists show your own entries with messages, staff see the others' without | the record's Owner, Maintainers and own Agent manage and see its schedules, as KV | [Q383](#q383) |
| 18 | Limits: 100 recurring (20 per creator), 1000 pending one-shots (100 per creator) per instance | a node-wide limit of about 10 000 and a smaller per-creator or per-record cap | [Q384](#q384) |
| 19 | A time to live on each send: to the next slot, at most 24 h; 24 h for a one-shot | the same | |
| 20 | At most 200 creator-and-target may-send questions a minute, the rest `deferred` | one in-process check per firing, so no deferral rule is needed | the core asks itself, not the daemon over the network |
| 21 | A slot missed while the instance was down is skipped and counted, a one-shot that fell due is sent once at start | a recipient down is no miss (the queue holds it); the daemon's own downtime follows [Q385](#q385) | |
| 22 | A zone missing from the host's data makes the instance refuse and fire nothing (alert) | an unknown zone is refused when the schedule is made | no instance Configuration left |
| 23 | A schedule never targets its own agent | a schedule never targets its owning record | |
| 24 | Counts of sent, dropped and missed since start; the last 100 outcomes survive a restart | the same counts and outcomes, per schedule | |
| 25 | Warnings: dropped copy, missed slot, deferred, invalid stored entry set aside | the same warning names for a dropped copy and a missed slot; an invalid stored row is set aside | |

## Ideas: a schedule as a daemon-owned object

The model is KV's ([key-value store](../../docs/03-records-kv.md#key-value-store)): a schedule is attached to a record, lives in the daemon, and is used by that record's Owner, Maintainers and own Agent. Choices still open are in [Questions](#questions).

| Topic | Idea |
|---|---|
| What it is | **One object, a schedule**, belonging to one record, that fires on a recurring expression or once; an *at* is a one-shot cron. **Crons** and **Ats** are two web views of the same object (recurring and one-shot). Every record kind that holds KV can hold schedules ([Q382](#q382)). This is decided (owner, "good idea") |
| Who creates, sees, deletes | The record's Owner, Maintainers and own Agent, as for KV. An agent manages its own schedules at will, which is the owner's ask. Nobody else sees a schedule, and the daemon says who created it |
| Who sends | The daemon delivers **as the owning record**. The destination's ordinary access list then decides whether the record may send there, at every firing, so no creator check, no shared `#cron` identity and no "trusts everyone allowed to call cron" remain ([Q380](#q380)) |
| Delivery path | A timer in the core claims each due firing in the same transaction that records it, then makes one ordinary send into the destination's queue or inbox, with the message's time to live. **The may-send check runs before every firing and is required**, as in IMAP and FileScan: if the owning record may no longer send to the target, that copy is dropped with its warning and the schedule stays. Same drop outcomes as today; a refused or unknown send is dropped, never retried |
| Persistence | Kept in the daemon's database, in backups and the restore gate. A restart repeats no claimed send; a send cut between claim and record is dropped as `send_unknown` as today. A record removed removes its schedules; a transfer keeps them; an inactive record's schedules do not fire and return with it, as for KV |
| Missed runs | A firing is a queued message, so **a recipient that is down is not a miss**: the message waits in its queue or inbox, within its time to live. Only the daemon's own downtime needs a rule: what a schedule due while the daemon was stopped does at start ([Q385](#q385)). A one-shot that fell due is sent once at start in every case |
| Rate | **At most one firing a second per schedule:** an interval can be as short as one second and no shorter. Today's five-field expressions keep meaning exactly what they mean now and fire at second 0; seconds are an addition ([Q388](#q388)). A faster need is not for a schedule: write it yourself |
| Time zones | Each schedule carries its own zone, defaulting to the node's, UTC unless set. Daylight-saving rules stay as today: a skipped minute does not fire, a repeated one fires once, `HH:MM` that falls in a gap is refused with the nearest valid time ([Q386](#q386)) |
| Caps | A reasonable limit of entries, about 10 000 on the node, with a smaller cap for one creator so a single agent cannot fill it; who a cap counts against is [Q384](#q384). Over a cap is refused by name, never dropped silently |
| Tags | Every schedule can be filtered and counted by tag in the web ([Q387](#q387)) |
| CLI, MCP, clients | Calls on the same grammar words as today, in the CLI, as MCP tools beside `ab_kv_*` and in all six client libraries, next to their KV calls. `#cron` and `#at` are removed when the core schedule ships: decided, no proxy |
| Never | A message to the owning record itself (today's rule), a firing faster than once a second, retries |

## Web

Same machinery as the [KV page and cards](../../src/web/web-face/records.md#key-value-stores); the daemon decides who may see what and the pages only ask.

| Where | What |
|---|---|
| Left navigation | **Crons** and **Ats**, beside **KV** |
| `/crons`, `/ats` | every schedule the visitor may use: search over the owning record, filters (kind of record, tag, state), counts per tag beside the filter, a table with the owning record (linked), the schedule's words, next fire time in its zone, last outcome and counts, **Open**. Empty and no-match states as KV's |
| Detail | one schedule: its message, target, zone, next five fire times, outcomes since start, **Delete** or **Cancel** behind a confirmation, **Add** for a new one with the same validation as the CLI |
| Tags on all three | **KV**, **Crons** and **Ats** gain a tag filter and a tag count: how many stores or schedules carry each tag, clickable, combinable with the existing search and kind filter ([Q387](#q387)) |
| Agent page | a **Schedules** card next to the **KV** card (non-zero counts, **Open** → that record's schedules), shown to the same people as the KV card |

## Steps by subsystem

Subsystems from [the map](../subsystems.md). ⚠️ rows get a mutant per boundary and two readers.

| Step | Letter | What | Acceptance, fixtures only | AI time |
|---|---|---|---|---|
| 1 | C ⚠️ | The schedule store: tables, caps, backup, restore gate, removal with the record | a schedule survives restart and restore; a removed record takes its schedules; a transfer keeps them; the 101st over a cap is refused by name; a stored invalid row is set aside, never fired | 1.5 h |
| 2 | H | The expression parser in Go, matching `src/agents/shared/cronexpr.py` on every expression valid today, plus the seconds addition | one shared fixture table (expression, zone, start instant, next fire times, refusals) runs against both parsers and agrees on today's expressions; the seconds rows run on the Go parser only; a mutant that changes day-rule or DST handling fails a named row | 1.5 h |
| 2b | A ⚠️ | The engine: timer, claim, delivery as the owner, the required may-send check before every firing, the daemon-downtime rule, outcomes, the one-second floor | a firing is claimed once across a restart between claim and send; a record that lost the right to send drops that copy with its warning and the schedule stays; unknown send is dropped and never repeated; a recipient down is not a miss (the message queues); a due one-shot is sent once at start; a one-second schedule fires once a second and never faster; DST skip and repeat behave as today | 2.5 h |
| 3 | E | The calls over HTTP with the KV authority rules | a stranger sees and changes nothing; Owner, Maintainer and own Agent do; every refusal of today's agents is the same word | 1 h |
| 4 | H | CLI verbs and daemon wiring | each of today's `#cron` and `#at` calls maps to one verb with the same grammar and the same answers | 1 h |
| 5 | J | MCP tools beside `ab_kv_*` | the tool contract list matches the CLI; refusals pass through unchanged | 40 min |
| 6 | K | The six client libraries, with examples and the KV-style checks | the same fixture schedule works in all six and the cross-language check stays green | 2 h |
| 7 | I | Crons and Ats pages (two views of the one object), tag filter and counts on KV, Crons and Ats, the Schedules card on agent pages | a visitor sees only schedules they may use; a tag count equals its filtered list; the card matches the KV card's visibility rule | 3 h |
| 8 | L, N | Close the parity list, then remove `#cron` and `#at` with their packages, runner entries, pages and smoke-map rows | every row of the [parity list](#parity-list-today-and-its-core-equivalent) has a core call and a fixture that passes; only then are the two agents and their docs removed, and nothing in the repo names them | 1 h |
| 9 | N, O | Current docs: new schedule page, KV page cross-links, cron and at pages replaced; plan to `done/` | links and anchors resolve; no status text in docs | 1 h |

About 15 h of agent wall-clock in all, in that order; steps 3 to 7 can run in parallel once step 2b lands, so the elapsed time is nearer 9 h. Each step is its own branch with two readers where marked. No live calls anywhere: tests use fixture daemons and fake targets.

## Questions

Owner choices, in one place with the plan (the owner: "create separate doc, come with ideas and questions"). Form: [MARKUP](../rules/MARKUP.md). Q381 was drafted and withdrawn when the owner settled that there is no data and no proxy period; its number stays unused.

<a id="q379"></a>

- ❓ Q379 Which release builds this
  - Context: it is internal re-structure of two shipped agents but touches storage and authority (⚠️) and every face; R1.1 has no open scope ([Q136](QUESTIONS.md))
  - ( ) R1.2 🌟
  - ( ) R1.1, as an exception ⭐
  - ( ) R2.0
  > 💡👾 R1.2: nothing here fixes a bug, the live node has no schedules to protect, and R1.1 is closed to new scope.

<a id="q380"></a>

- ❓ Q380 Who a scheduled message comes from
  - Context: today it arrives from `#cron` or `#at`, and a destination that allows them trusts everyone allowed to call them. Settled in the design: the may-send check runs before every firing ([delivery path](#ideas-a-schedule-as-a-daemon-owned-object)); only the sender's name is open
  - ( ) From the owning record, checked against the destination's access list at every firing 🌟
  - ( ) From one system name such as `#schedule`, with the creator checked at every firing as today ⭐
  > 💡👾 The owning record: no shared sender to over-trust. A receiver that filters on `#cron` or `#at` must change its filter.

<a id="q382"></a>

- ❓ Q382 Which records can hold schedules
  - Context: KV attaches to an agent, queue, pubsub, service, group or a user's own record
  - ( ) Every record kind that holds KV 🌟
  - ( ) Agents and users only ⭐
  > 💡👾 The same set as KV: one rule to learn, and a queue or service can keep its own periodic messages.

<a id="q383"></a>

- ❓ Q383 Who may create, see and delete a schedule on a record
  - Context: the owner: "any agent can setup own crons/at at will"; KV is used by the Owner, Maintainers and the record's own Agent
  - ( ) Exactly KV's people: Owner, Maintainers and the record's own Agent 🌟
  - ( ) The same, and anyone the record's access list admits may add one that targets only their own record ⭐
  > 💡👾 KV's people: nothing new to trust, and an agent still sets its own schedules at will.

<a id="q384"></a>

- ❓ Q384 What the cap on entries counts against
  - Context: the owner: "setup reasonable limit of entries, like 10k". Today 100 recurring (20 per creator) and 1000 one-shots (100 per creator) for one instance
  - ( ) 10 000 schedules on the node in all, and 100 on any one owning record 🌟
  - ( ) 10 000 on the node in all, and a smaller cap per creator (1000) so one agent cannot fill it ⭐ (#ab-dir)
  - ( ) 10 000 on the node and nothing per record or creator
  > 💡👾 Node and record: a runaway agent stops at its own record, and the owner's 10k stays the ceiling. The second option bounds a creator instead of a record, which matters when one agent creates schedules for many records.

<a id="q385"></a>

- ❓ Q385 What a schedule does when the daemon itself was down
  - Context: the owner: "yes, but we have queues, which makes this much easier". A firing is a queued message, so a down recipient is no longer a miss; only the daemon's downtime is. Today a missed recurring slot is skipped and a due one-shot is sent once at start
  - ( ) At start, send one message per recurring schedule for its latest missed slot, marked late, and every due one-shot once 🌟
  - ( ) Keep today's rule: skip missed recurring slots, send due one-shots once ⭐
  > 💡👾 One late message per schedule: the queue absorbs it like any other, nothing floods, and a daily report missed by a restart still arrives.

<a id="q386"></a>

- ❓ Q386 Time zones
  - Context: today one zone per instance, set in Configuration; with the core there is no instance
  - ( ) A zone on each schedule, defaulting to the node's, UTC unless the node owner sets another 🌟
  - ( ) One zone for the whole node, no zone on a schedule ⭐
  > 💡👾 Per schedule: two agents on one node can keep different wall-clock times, and the default is what an instance gets today.

<a id="q387"></a>

- ❓ Q387 What a tag means on a schedule, a store and a page
  - Context: the owner asked for tag filtering and counting on KV, Crons and Ats; a KV store has no tags of its own, a record has
  - ( ) The owning record's tags on all three pages, so one tag vocabulary; a schedule carries no tags of its own 🌟
  - ( ) The record's tags plus free tags on each schedule, filtered together ⭐
  > 💡👾 The record's tags: the same filter works on KV, which has nothing else to filter by, and nothing new has to be set on each schedule.

<a id="q388"></a>

- ❓ Q388 How seconds are written
  - Context: the owner: "no more than once/second looks reasonable". Today's five fields have no seconds; today's expressions must keep their meaning
  - ( ) An optional sixth field for seconds: five fields fire at second 0 as today, six fields add seconds 🌟
  - ( ) A separate interval form for seconds, such as every 5 seconds, beside the five fields ⭐
  > 💡👾 The sixth field: one grammar, and `*/5` in it reads like every other field. The interval form is simpler to read but is a second way to say a schedule.
