# Cron and at in the core

📌 **TL;DR:** The owner wants `#cron` and `#at` to stop being agents and become bus core: any agent keeps its own crons and ats, the web gets **Crons** and **Ats** pages like **KV** with tag filtering and counts on all three, and every agent page shows its schedules the way it shows its KV. All of today's behaviour stays. This is the approach for the owner to approve: what the two agents do now, what a schedule as a daemon-owned object would be, how the live node moves with nothing lost, the web, the steps by subsystem and the choices in [Questions](#questions). Nothing is built and no format is chosen.

Owner, 2026-11-06 ~14:00: "#cron and #at must be converted from agents to bus core functionality; any agent can setup own crons/at at will. Create separate doc, come with ideas and questions. Our web interfaces should have new items - crons / ats (left) - show all / filter - similar as we do for KV (please add tag filtering/counting for all 3), and show on agent pages - again - same way as we do for KV. Create README TODO Questions doc (maybe combined doc, because its mostly internal re-structure). All our existing functionality will be supported."

## Today

`#cron` and `#at` are two runner agents on one shared engine (`src/agents/shared/scheduler.py`): entries live in the agent's own KV, one process fires at a time under a record lock, and every firing is claimed in KV before its single send attempt. Pages: [cron](../../docs/agents/cron.md#what-it-does), [at](../../docs/agents/at.md#what-it-does).

| | `#cron` | `#at` |
|---|---|---|
| Calls | add, list and delete a recurring entry (`sub add`, `sub list`, `sub delete`); send once later (`later`), list pending and the last 100 outcomes (`later list`), cancel (`later cancel`) | send once after a delay (`in`), at a time (`at`), `list`, `cancel` |
| When | five-field schedule in the instance's time zone, Vixie day rule, no names, macros or seconds; a delay `+90s` `+15m` `+2h` `+3d` or an RFC 3339 instant with an offset, at most 366 days ahead | a delay `90s` `30m` `2h` `3d` (no compound), `HH:MM` in the instance's zone (the next one strictly after this minute) or an RFC 3339 instant with an offset; at most 366 days |
| Arrives as | from `#cron`, topic `cron` or `later`, tag `by <creator>` | from `#at`, topic `at`, tag `by <creator>` |
| Time zone | one per instance, Configuration `tz`, default UTC; a bad zone makes the instance refuse every call and fire nothing | the same |
| Missed | a recurring slot missed while down is skipped and counted; a one-shot that fell due is sent once at start | a call that fell due while down is sent once at start, possibly late |
| One attempt | a claimed send is never repeated; an unconfirmed one is dropped as `send_unknown`; a destination that does not admit the sender is `admission_refused`, a vanished one `target_gone`, any other refusal `send_refused`; nothing is retried | the same; the call is removed and listed as finished `dropped` |
| Who may | anyone on the agent's own access list; the creator must also be allowed to send to the target, checked when added and before every send; delete or cancel by the creator, or the agent's Owner and Maintainers | the same |
| Who sees | your own entries with their messages; the Owner and Maintainers see the others' ids, targets and outcomes | the same |
| Limits | 100 recurring (20 per creator), 1000 pending one-shots (100 per creator), 16384 bytes a message, at most 200 creator-and-target questions a minute, a time to live on each send (to the next slot, at most 24 h; 24 h for a one-shot), never a message to itself | 1000 pending (100 per creator), 16384 bytes |

- **Trust today:** a destination that allows `#cron` trusts everyone allowed to call `#cron`, so the docs advise one instance per trust group. That is the main thing the core removes.
- **Everything above stays supported:** every call, grammar word, bound, outcome name, warning and the one-attempt rule carry over. What changes is who owns an entry and who sends it.

### The live node, read only

Read at 2026-11-06 14:01 with `sub list`, `later list` and `list` as the calling session: **no recurring entries, no pending one-shots, no finished outcomes, in both `#cron@parf.us` and `#at@parf.us`; both on UTC.** Nothing to convert on this node today. The calling session is on the agents' access lists but is not shown as their Owner, so the Owner confirms the same three reads with its own credential at cutover ([migration](#migration-with-nothing-lost)); until then "empty" means "empty as that caller may see".

## Ideas: a schedule as a daemon-owned object

The model is KV's ([key-value store](../../docs/03-records-kv.md#key-value-store)): a schedule is attached to a record, lives in the daemon, and is used by that record's Owner, Maintainers and own Agent. Choices are in [Questions](#questions); this is the recommended shape.

| Topic | Idea |
|---|---|
| What it is | Two kinds of schedule, **cron** (recurring) and **at** (one-shot), each belonging to one record. Every record kind that holds KV can hold schedules ([Q382](#q382)) |
| Who creates, sees, deletes | The record's Owner, Maintainers and own Agent, as for KV. An agent manages its own schedules at will, which is the owner's ask. Nobody else sees a schedule, and the daemon says who created it |
| Who sends | The daemon delivers **as the owning record**. The destination's ordinary access list then decides whether the record may send there, at every firing, so no creator check, no shared `#cron` identity and no "trusts everyone allowed to call cron" remain ([Q380](#q380)) |
| Delivery path | A timer in the core claims each due firing in the same transaction that records it, then makes one ordinary send with the message's time to live. Same drop outcomes and warnings as today; a refused or unknown send is dropped, never retried |
| Persistence | Kept in the daemon's database, in backups and the restore gate. A restart repeats no claimed send; a send cut between claim and record is dropped as `send_unknown` as today. A record removed removes its schedules; a transfer keeps them; an inactive record's schedules do not fire and return with it, as for KV |
| Missed runs | Unchanged by default: a recurring slot missed while the daemon was down is skipped and counted, a one-shot that fell due is sent once at start ([Q385](#q385)) |
| Time zones | Each schedule carries its own zone, defaulting to the node's, UTC unless set. Daylight-saving rules stay as today: a skipped minute does not fire, a repeated one fires once, `HH:MM` that falls in a gap is refused with the nearest valid time ([Q386](#q386)) |
| Per-owner caps | The same caps, now per owning record and per creator, plus one node-wide cap so a single agent cannot fill the store. Over a cap is refused by name, never dropped silently ([Q384](#q384)) |
| Tags | Every schedule can be filtered and counted by tag in the web ([Q387](#q387)) |
| CLI, MCP, clients | Calls on the same grammar words as today, in the CLI, as MCP tools beside `ab_kv_*` and in all six client libraries, next to their KV calls. The existing `#cron` and `#at` call grammar is either kept as a front door or retired ([Q381](#q381)) |
| Never | A message to the owning record itself (today's rule), sub-minute timing, retries, catching up by default |

## Web

Same machinery as the [KV page and cards](../../src/web/web-face/records.md#key-value-stores); the daemon decides who may see what and the pages only ask.

| Where | What |
|---|---|
| Left navigation | **Crons** and **Ats**, beside **KV** |
| `/crons`, `/ats` | every schedule the visitor may use: search over the owning record, filters (kind of record, tag, state), counts per tag beside the filter, a table with the owning record (linked), the schedule's words, next fire time in its zone, last outcome and counts, **Open**. Empty and no-match states as KV's |
| Detail | one schedule: its message, target, zone, next five fire times, outcomes since start, **Delete** or **Cancel** behind a confirmation, **Add** for a new one with the same validation as the CLI |
| Tags on all three | **KV**, **Crons** and **Ats** gain a tag filter and a tag count: how many stores or schedules carry each tag, clickable, combinable with the existing search and kind filter ([Q387](#q387)) |
| Agent page | a **Schedules** card next to the **KV** card (non-zero counts, **Open** → that record's schedules), shown to the same people as the KV card |

## Migration with nothing lost

- **Before cutover:** the Owner reads `sub list`, `later list` and `list` on both agents with its own credential and saves them. On this node they are empty today (above).
- **Conversion:** each recurring entry and pending one-shot becomes a schedule owned by the record that created it, with the same schedule words, target, message and creation time, and the same zone (UTC here). An entry whose creator no longer exists, or whose target no longer admits that creator, is **not** converted silently: it is listed by name for the Owner to recreate or drop.
- **Finished outcomes** (last 100 per agent) are kept as read-only history on the converted records.
- **In flight:** the cutover happens with both agents stopped for one pass, so nothing is claimed twice; a firing due in that gap follows the missed-run rule.
- **After:** `#cron` and `#at` are retired or left as front doors by [Q381](#q381); their runner entries, packages and docs go together, with no alias left over ([one canonical interface](../../CLAUDE.md#working-rules)).

## Steps by subsystem

Subsystems from [the map](../subsystems.md). ⚠️ rows get a mutant per boundary and two readers.

| Step | Letter | What | Acceptance, fixtures only | AI time |
|---|---|---|---|---|
| 1 | C ⚠️ | The schedule store: tables, caps, backup, restore gate, removal with the record | a schedule survives restart and restore; a removed record takes its schedules; a transfer keeps them; the 101st over a cap is refused by name; a stored invalid row is set aside, never fired | 1.5 h |
| 2 | A ⚠️ | The engine: timer, claim, delivery as the owner, authority, missed-run rule, outcomes | a firing is claimed once across a restart between claim and send; a record that lost the right to send drops that copy with its warning and the schedule stays; unknown send is dropped and never repeated; missed recurring slots are skipped, a due one-shot is sent once; DST skip and repeat behave as today | 2.5 h |
| 3 | E | The calls over HTTP with the KV authority rules | a stranger sees and changes nothing; Owner, Maintainer and own Agent do; every refusal of today's agents is the same word | 1 h |
| 4 | H | CLI verbs and daemon wiring; the front doors decided by Q381 | each of today's `#cron` and `#at` calls maps to one verb with the same grammar and the same answers | 1 h |
| 5 | J | MCP tools beside `ab_kv_*` | the tool contract list matches the CLI; refusals pass through unchanged | 40 min |
| 6 | K | The six client libraries, with examples and the KV-style checks | the same fixture schedule works in all six and the cross-language check stays green | 2 h |
| 7 | I | Crons and Ats pages, tag filter and counts on KV, Crons and Ats, the Schedules card on agent pages | a visitor sees only schedules they may use; a tag count equals its filtered list; the card matches the KV card's visibility rule | 3 h |
| 8 | L | Retire or reduce `#cron` and `#at`; migration of the live node; their pages and package lists | after cutover no `#cron` or `#at` process runs unless Q381 keeps them; the converted node lists the same schedules as before; the three reads match | 1.5 h |
| 9 | N, O | Current docs: new schedule page, KV page cross-links, cron and at pages replaced; plan to `done/` | links and anchors resolve; no status text in docs | 1 h |

About 14 h of agent wall-clock in all, in that order; steps 3 to 7 can run in parallel once step 2 lands, so the elapsed time is nearer 8 h. Each step is its own branch with two readers where marked. No live calls anywhere: tests use fixture daemons and fake targets.

## Questions

Owner choices, in one place with the plan (the owner: "create separate doc, come with ideas and questions"). Form: [MARKUP](../rules/MARKUP.md).

<a id="q379"></a>

- ❓ Q379 Which release builds this
  - Context: it is internal re-structure of two shipped agents but touches storage and authority (⚠️) and every face; R1.1 has no open scope ([Q136](QUESTIONS.md))
  - ( ) R1.2 🌟
  - ( ) R1.1, as an exception ⭐
  - ( ) R2.0
  > 💡👾 R1.2: nothing here fixes a bug, the live node has no schedules to protect, and R1.1 is closed to new scope.

<a id="q380"></a>

- ❓ Q380 Who a scheduled message comes from
  - Context: today it arrives from `#cron` or `#at`, and a destination that allows them trusts everyone allowed to call them
  - ( ) From the owning record, checked against the destination's access list at every firing 🌟
  - ( ) From one system name such as `#schedule`, with the creator checked at every send as today ⭐
  > 💡👾 The owning record: no shared sender to over-trust, and the creator check disappears. A receiver that filters on `#cron` or `#at` must change its filter.

<a id="q381"></a>

- ❓ Q381 What happens to the names `#cron` and `#at`
  - Context: the owner says the agents become core; existing callers use `agent-bus call '#cron@…' 'sub add …'`
  - ( ) Retire them: the same grammar becomes CLI verbs, MCP tools and client calls, and the two agents are removed with their docs 🌟
  - ( ) Keep both names as thin front doors in the daemon that accept today's calls and create schedules owned by the caller ⭐
  > 💡👾 Retire: one canonical interface, and no prerelease callers need a bridge. Choose the front doors only if you want today's scripts to keep running untouched.

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

- ❓ Q384 The caps
  - Context: today 100 recurring (20 per creator) and 1000 one-shots (100 per creator) for the whole of one instance
  - ( ) Per owning record: 100 recurring and 1000 one-shots, 20 and 100 per creator, plus one node-wide cap of 10 000 of each kind 🌟
  - ( ) Today's numbers per owning record, with no node-wide cap ⭐
  > 💡👾 Per record with a node cap: one busy agent cannot fill the store, and the numbers an agent knows today still hold for it.

<a id="q385"></a>

- ❓ Q385 Missed runs after a downtime
  - Context: today a recurring slot is skipped and a due one-shot is sent once at start; the page says catching up is not offered
  - ( ) Keep it: skip recurring, send a due one-shot once 🌟
  - ( ) Keep it, and let a schedule ask to catch up once for its latest missed slot ⭐
  > 💡👾 Keep it: it is the settled behaviour, and "all existing functionality will be supported" does not ask for more.

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

- ❓ Q388 How the live node cuts over
  - Context: both agents hold nothing today ([the live node](#the-live-node-read-only)), and an entry created in the gap would be lost or claimed twice
  - ( ) Stop both agents for one pass, convert what the Owner's own reads show, start the core engine 🌟
  - ( ) Run both together for a day, the agents firing only what they hold ⭐
  > 💡👾 Stop for one pass: with an empty node it is a restart, and two firers on one schedule are exactly what the claim rule forbids.
