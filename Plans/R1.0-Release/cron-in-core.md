# Cron and at in the core

📌 **TL;DR:**
- **Problem:** scheduling lives in two runner agents, `#cron` and `#at`, so an agent cannot own its schedules, the web cannot show them, and a firing needs a runner process up.
- **Outcome:** the daemon keeps **schedules** the way it keeps KV: attached to a record, used by its Owner, Maintainers and own Agent, fired from `#bus`, shown on **Crons** and **Ats** pages and agent pages. `#cron` and `#at` are then removed.
- **Status:** design settled with the owner (Q379–Q390 on 2026-11-06; Q409–Q414 and Q417 on 2026-11-12); R1 task [RT35](TODO.md#cron-and-at-in-the-core); nothing built. Open: [Q415, Q416, Q418](#open-questions).

## Contents

- [Owner's ask](#owners-ask)
- [Parity list: today and its core equivalent](#parity-list-today-and-its-core-equivalent)
- [The schedule](#the-schedule)
- [Firing](#firing)
- [Storage](#storage)
- [Calls](#calls)
- [Web](#web)
- [Warnings and alerts](#warnings-and-alerts)
- [Removing cron and at](#removing-cron-and-at)
- [Steps by subsystem](#steps-by-subsystem)
- [Risks](#risks)
- [Questions](#questions)

## Owner's ask

Owner, 2026-11-06 ~14:00: "#cron and #at must be converted from agents to bus core functionality; any agent can setup own crons/at at will. Create separate doc, come with ideas and questions. Our web interfaces should have new items - crons / ats (left) - show all / filter - similar as we do for KV (please add tag filtering/counting for all 3), and show on agent pages - again - same way as we do for KV. Create README TODO Questions doc (maybe combined doc, because its mostly internal re-structure). All our existing functionality will be supported."

| Topic | Owner |
|---|---|
| Missed runs | "yes, but we have queues, which makes this much easier" |
| Rate | "no more than once/second looks reasonable; if you need more often, you surely don't need cron / at, write it yourself" |
| Caps | "setup reasonable limit of entries, like 10k" |
| One object | "good idea"; "do we need two when we decided to use schedule as a more general system" |
| Existing data | "we have no crons / at now" |
| Targets | the creator's inbox or any queue or agent the creator may send to: "да конечно" |
| Empty body | refused: "empty events - путь к ошибкам" |
| Refusals | "после X событий выключал (disable) timer"; X is 5 ([Q409](#q409)) |
| Controls | "yes - and enable(default)/disable"; `until` and `max_runs` agreed |
| Max lateness | optional, off by default: "да - good idea" |
| `#bus` as sender | 2026-11-12: "bus should be always allowed to send to anyone" ([Q410](#q410)) |
| Storage | 2026-11-12: a SQLite agent with `WHERE` is available "if KV is not enough"; not needed, see [storage](#storage) |

## Parity list: today and its core equivalent

`#cron` and `#at` share one engine (`src/agents/shared/scheduler.py`, parser `src/agents/shared/cronexpr.py`): entries in the agent's KV, one process firing under a record lock, each firing claimed just before its one send. Pages: [cron](../../docs/agents/cron.md#what-it-does), [at](../../docs/agents/at.md#what-it-does). Both held nothing on the live node on 2026-11-06; nothing is migrated. Step 9 closes this list: each row gets a core call and a fixture that fails without it.

| # | Today | Core equivalent | Differs |
|---|---|---|---|
| 1 | `#cron` `sub add`: recurring, five-field expression | add a recurring schedule on a holder | held by a record, not one agent's KV |
| 2 | `sub list` | list a holder's schedules; the web lists all a visitor may use | |
| 3 | `sub delete <id>` | delete by id | |
| 4 | `#cron` `later`: once, after `+90s` `+15m` `+2h` `+3d`, or at an RFC 3339 instant with offset | add a one-shot with the same forms | |
| 5 | `later list`: pending and the last 100 outcomes | list one-shots with state and recent outcomes | retention: [Q416](#q416) |
| 6 | `later cancel <id>` | delete; a one-shot whose send is in flight may still deliver once | |
| 7 | `#at` `in`: `90s` `30m` `2h` `3d`, optional `+`, no compound | the same delay forms | `later` and `in` become one |
| 8 | `#at` `at`: `HH:MM` (next strictly after this minute) or an instant with offset | the same; `HH:MM` read as UTC | |
| 9 | `#at` `list`, `cancel` | as 5 and 6 | |
| 10 | five fields (minute, hour, day of month, month, day of week, 0 and 7 Sunday); `*`, `n`, `a-b`, `*/n`, `a-b/n`, lists; no names or macros; Vixie day rule; never-firing refused; `0 0 29 2 *` fires in leap years | the same, at second 0; an optional sixth seconds field ([Q388](#q388)) | seconds |
| 11 | local wall-clock with DST gaps and repeats; past instant over 30 s refused; at most 366 days ahead | UTC only ([Q386](#q386)); the same 30 s and 366-day limits | no DST |
| 12 | message up to 16384 bytes; `#cron` accepts an empty body, `#at` refuses it | up to 16384 bytes; empty always refused | `#cron`'s empty body refused |
| 13 | from `#cron`/`#at`, topic `cron`/`later`/`at`, tag `by <creator>` | from `#bus`, topic `schedule`, tag `by <creator>`, creator and schedule id stamped, reply route to the creator ([Q380](#q380)) | one topic; stamped fields are new |
| 14 | one attempt per firing; claimed never repeated; `send_unknown`, `admission_refused`, `target_gone`, `send_refused`; no retry | the same names and rules | |
| 15 | creator's may-send at add and before every send; the target must also admit `#cron` | creator's may-send at add and before every firing, checked with the send; `#bus` is always admitted ([Q410](#q410)); 5 refusals in a row disable ([Q409](#q409)) | no Allow entry for the sender; disable |
| 16 | target: an existing agent or queue `#cron` can see; never `#cron` itself | an existing agent or queue the creator may send to, including the creator's own; never `#bus` | the creator, not the helper, must reach it |
| 17 | anyone admitted to `#cron` schedules; each lists and deletes their own; staff see others' entries without bodies | the holder's Owner, Maintainers and own Agent, who see every schedule on it with bodies ([Q383](#q383)) | a person keeps schedules on an agent or queue they manage, such as their session ([Q417](#q417)); managers see bodies |
| 18 | per instance: 100 recurring (20 per creator), 1000 one-shots (100 per creator) | per holder: 100 recurring, 10 000 one-shots ([Q384](#q384)); node limits [Q416](#q416) | |
| 19 | time to live: to the next slot, at least 60 s, at most 24 h; 24 h for a one-shot | the same, also capped by the target queue's own limit; a catch-up gets a one-shot's | |
| 20 | at most 200 creator-and-target questions per pass, the rest `deferred` | an in-process check per firing | `deferred` gone |
| 21 | a slot missed while down is skipped, not counted; a due one-shot sent once at start | a target down is not a miss (its queue holds the message); daemon downtime follows [Q385](#q385) | catch-up added |
| 22 | a zone missing from host data stops the instance | no zones | no Configuration |
| 23 | recurring counts in memory since start; one-shot outcomes survive restart | counts and outcomes persisted per schedule | recurring counts persist |
| 24 | the roles `#cron` holds on the target travel with each message | the roles `#bus` holds on the target travel | roles of `#bus` |
| 25 | warnings: dropped copy, missed slot, deferred, invalid stored entry | [warnings and alerts](#warnings-and-alerts) | |

## The schedule

| Property | Rule |
|---|---|
| Kind | one object; **recurring** (an expression) or **one-shot** (a time); Crons and Ats are two views of it |
| Holder | an Agent, queue or pubsub ([Q382](#q382)), never a User record ([Q417](#q417)); it stores the schedule, the target is what it triggers |
| Who manages and sees | the holder's Owner, Maintainers and own Agent, KV's `resourceManages` check ([Q383](#q383)); nobody else learns a schedule exists |
| Creator | the caller that added it; fires only while the creator is active, still manages the holder and may send to the target. Who becomes creator when another manager enables or runs it is [Q415](#q415) |
| Target | an existing agent or queue the creator may send to, the holder itself included; never `#bus` |
| Tag | `by <creator>`, as today; the creator does not choose it |
| Body | required, 1 to 16384 bytes |
| Controls | enable (default), disable, run now, delete; `until` and `max_runs`; max lateness, off by default; no edit: delete and add again, as today |
| End | a one-shot leaves once its outcome is recorded; a recurring schedule past `until` or `max_runs` is disabled and kept |
| Caps | per holder: 100 recurring and 10 000 pending one-shots; over a cap refused by name; a warning at 80 % |
| Lifetime | removed with its holder, kept on owner transfer, silent while the holder is inactive; a short-lived holder answers the add with a warning ([Q389](#q389)) |
| Time | UTC everywhere; shown in the viewer's zone on the web only ([Q386](#q386)) |
| Tags for filtering | the holder's tags ([Q387](#q387)) |

## Firing

One engine goroutine in the daemon, started and stopped in `runBus` (`src/cmd/agent-busd/bus.go`) beside the minute clock and snapshot ticker.

1. **Wake:** a timer to the earliest next fire time, reset when a schedule is added, enabled, disabled or deleted, and re-evaluated at least once a minute so a wall-clock step or suspend is noticed.
2. **Pick:** read a bounded batch of due firings from the next-fire index.
3. **Claim, one firing at a time:** a transaction records the firing as claimed and advances the next fire time to the first slot later than the last claimed one. A crash leaves at most this one firing unsent; on restart it is recorded `send_unknown` and never repeated.
4. **Check and send, under one bus lock hold:** the holder is live, the creator is active and still manages the holder, the creator may send to the target, and max lateness, if set, is not exceeded; then one send from `#bus`, which every target admits ([Q410](#q410)): topic `schedule`, tag `by <creator>`, creator and schedule id stamped, reply route to the creator, row 19's time to live.
5. **Record:** the outcome (`sent` or a row 14 drop name) and counts.
6. **Refusals:** a refused firing is skipped, counted and warned; 5 in a row disable the schedule with an alert ([Q409](#q409)).

| Case | Behaviour |
|---|---|
| Target down or slow | not a miss: the message waits in its queue or inbox until its time to live |
| Daemon was stopped | a past-due one-shot fires once with `late_by`; a recurring schedule fires one catch-up with `missed: N`, then keeps its slots ([Q385](#q385)); beyond max lateness, when set, it is skipped and warned |
| Wall clock jumps forward while running | as daemon downtime: one catch-up |
| Wall clock moves back | nothing fires twice: only slots later than the last claimed one |
| Holder inactive | recurring slots pass silently; a one-shot due meanwhile fires on return as a past-due one-shot |
| Engine behind | firings due more than a minute and unsent alert the node owner; the start-up catch-up does not count |
| `run now` | one extra firing through steps 3–5; the next slot does not move |
| Unclean daemon stop | a `sent` message may be lost with its queue's unflushed minute, like any send ([durability](../../docs/04-messaging.md#durability)) |

## Storage

Schedules are daemon state, not KV values; KV is only the model for who uses them. They get their own tables in the SQLite and memory stores beside KV's, indexed by next fire time for the engine and by holder for lists and caps, so due work is one indexed query and no SQLite agent is needed.

| Concern | Rule |
|---|---|
| Interface | a schedule port beside `ports.KVStore`, SQLite and memory implementations |
| Holder removed or re-created | rows dropped in the record-commit loop that drops KV (`dropKV`) |
| Owner transfer | rows kept: the record ID is unchanged |
| Orphans and invalid rows at start | set aside and alerted, as `ReportKVOrphans`; never fired |
| Daemon backup | inside the SQLite file with KV; step 1 checks every dump and restore path that names KV |
| Record export and import (RT29) | carried, like KV: system state ([Q411](#q411)); caps re-checked; an imported schedule fires only after its creator passes the step 4 checks on the new node |
| Format | proposed to the owner at step 1, before code ([working rules](../../CLAUDE.md#working-rules)) |

## Calls

One `schedule` set of calls on every surface: **add**, **list** (one holder's, or every schedule the caller may use, like `KVStores`), **get**, **enable**, **disable**, **run**, **delete**. Time forms on add: an expression of five or six fields; `in <delay>`; `at <HH:MM>` (UTC); `at <RFC 3339 instant>`; optional `until`, `max_runs` and max lateness.

| Surface | Where | Model |
|---|---|---|
| HTTP | routes beside `/kv*` in `internal/api/server.go`; refusals through the same error table; [daemon API](../../docs/09-daemon-api.md) | `internal/api/kv.go` |
| Envelope | creator and schedule id stamped on scheduled messages | `internal/protocol/envelope.go` (D ⚠️) |
| CLI | `agent-bus schedule <verb>` | `cmd/agent-bus/kv.go` |
| MCP | tools beside `ab_kv_*` | `src/mcp/server.ts`, `tools.yaml` |
| Clients | Go, Python, TypeScript, Rust, Java, PHP, beside each KV class | each client's KV file |
| `#bus` help | the `schedule` topic and how a receiver recognises a scheduled message | `#bus` help |

Wire format, names and stamped fields are proposed to the owner at step 3, before code.

## Web

Same machinery as KV (`src/web/pages/kv.tsx`, `src/web/ui/kv.tsx`); the daemon decides who sees what.

| Where | What |
|---|---|
| Left navigation | **Crons** and **Ats** beside **KV** (`ui/frame.tsx`) |
| Crons, Ats | every schedule the visitor may use: search by holder; filters by record kind, tag, state and "has problems"; counts per tag; columns holder, expression or time, target, next fire (viewer's zone), last outcome, counts |
| Detail | message, target, creator, next five fire times, outcomes, enable/disable, run now, delete with confirmation; **Add** with the CLI's validation |
| Agent page | a **Schedules** card like the KV card, with a problem count |
| Tags on KV too | KV gains the tag filter and per-tag counts ([Q387](#q387)) |
| States | disabled by failure red, disabled by hand grey |

## Warnings and alerts

Settled in [Q390](#q390) and [Q412](#q412): every schedule warning and alert goes into the **holder agent's built-in warnings and alerts** (agent events, RT34), which its subscribers already receive; the daemon journal also records each. Daemon code cannot write a record's events today (only the agent or an admin may), so step 3 adds a daemon-side writer. Where a queue's or pubsub's schedules report is [Q418](#q418).

| Level | What |
|---|---|
| Warning | refused firing; `send_unknown`; skipped for max lateness; holder at 80 % of a cap; message expired unread in the target's queue; the same target refusing or not reading for 3 firings ([Q413](#q413)); removed with a short-lived holder |
| Alert | disabled after 5 refusals |
| Node owner, journal and alert | invalid stored schedule set aside at start; engine late over a minute; one catch-up warning per start with schedules and runs missed |

The two target-reading warnings need the queue's expiry path (`prune`, a hot path) to know a message came from a schedule; step 3 adds that only for messages stamped with a schedule id.

## Removing cron and at

Only after step 9's parity check passes:

- Delete `src/agents/cron/`, `src/agents/at/` (code, tests, `agent.json`, help goldens, pictures, changelogs), `src/agents/shared/scheduler.py` and `cronexpr.py`; their rows in `src/agents/package.sh`, `src/cmd/agent-bus-setup/package_install.go` (G ⚠️) and `src/smoke-map.json`.
- `src/cmd/agent-bus-runner/checkout_sandbox_test.go` (F ⚠️) uses cron as its fixture agent: give it another bundled agent, keeping what it checks.
- Replace `docs/agents/cron.md` and `at.md` with one schedule page; fix `docs/agents/README.md`, `subscriptions.md` and the root README.
- Live node, by the Claude session that builds it ([Q414](#q414)): disable both in the runner inventories and unregister `#cron` and `#at` at the deploy that ships schedules.

## Steps by subsystem

Letters from [the map](../subsystems.md); ⚠️ steps get a mutant per boundary and two readers. Each step is its own branch; tests use fixture daemons, no live calls.

| Step | Letter | What | Acceptance | Depends |
|---|---|---|---|---|
| 1 | C ⚠️ | Store: port, SQLite and memory tables, caps, drop with holder, orphans, dump and restore paths, record export and import | survives restart; a removed holder takes its schedules; transfer keeps them; the 101st recurring is refused by name; an invalid row is set aside and never fired; export then import on a fresh daemon carries them, firing only once the creator passes | owner reads the format |
| 2 | H | Go expression parser ported from `cronexpr.py`, with seconds | one fixture table runs against both parsers and agrees on every five-field row; seconds rows Go only; a day-rule mutant fails a named row | — |
| 3 | A ⚠️, D ⚠️ | Engine: wake, claim, the one-hold check and send from `#bus`, outcomes, disable after 5, catch-up, clock rules, one-second floor, lateness, time to live; envelope stamps; the daemon-side event writer; the schedule-stamped expiry hook | each [firing case](#firing) has a fixture: claimed once across a crash between claim and send; a creator who lost may-send, standing on the holder or activity is skipped and warned, 5 in a row disable with an alert; a target without `#bus` in its Allow still receives; a one-second schedule fires once a second, never faster, over 10 s; `missed: N` and `late_by` after a stopped daemon; a backward clock fires nothing twice; `run now` leaves the next slot; a fire at 23:59:59Z is stamped Z and lands on the expression's UTC slot | 1, 2; RT36; owner reads the wire format |
| 4 | E | HTTP calls under KV's authority; daemon API doc | with an existing schedule, a stranger who may otherwise call the daemon sees and changes nothing and gets the same answer as for a missing holder; Owner, Maintainer and own Agent can | 3 |
| 5 | H | CLI `schedule` verbs and help | each parity row 1–9 has one verb and a fixture with today's time forms and refusal words | 4 |
| 6 | J | MCP tools | the tool list matches the CLI verbs; a refusal passes through unchanged | 4 |
| 7 | K | Six clients with examples | one fixture schedule fires in all six; the cross-language check is green | 4 |
| 8 | I | Crons, Ats, detail, Schedules card, tags on KV | a visitor sees only schedules they may use; a tag count equals its filtered list; card visibility matches KV's | 4 |
| 9 | L, M, N, F ⚠️, G ⚠️ | Parity check, then removal of `#cron` and `#at` | every parity row passes; `src/` and `docs/` contain no `#cron`, `#at`, `agents/cron` or `agents/at`; the sandbox test passes on its new fixture | 5–8 |
| 10 | N, O | Docs: schedule page, `#bus` help, KV cross-links; plan to `done/` | links resolve; no status text in docs | 9 |

Steps 1 and 2 run together; 5–8 in parallel after 4. About 17 h of agent time, about 10 h elapsed.

## Risks

| Risk | Guard |
|---|---|
| `#bus` reaches every target | it sends only what a creator who may send there scheduled, checked in the same lock hold as the send |
| A manager firing on another's authority | creator standing re-checked every firing; [Q415](#q415) |
| A busy node with many one-second schedules | the per-schedule floor; the engine-late alert; [Q416](#q416) |
| A double send after a crash | one claim per firing before its send; unknown outcomes never retried |
| Parser drift | the shared fixture table until the Python parser is deleted |
| Lost schedules on holder churn | the short-lived holder warning at add; the removal warning |

## Questions

Settled with the owner; anchors kept for links. Q381 was withdrawn and stays unused.

<a id="q379"></a>
- ✅ Q379 Which release → built now; moved to R1 on 2026-11-12 (RD184)

<a id="q380"></a>
- ✅ Q380 Who a scheduled message comes from → `#bus`, topic always `schedule`, creator and schedule id stamped, a reply goes to the creator, the creator's may-send checked at every firing (owner, 2026-11-06)

<a id="q382"></a>
- ✅ Q382 Which records hold schedules → agents, queues and pubsubs

<a id="q383"></a>
- ✅ Q383 Who manages and sees them → KV's people: Owner, Maintainers, own Agent

<a id="q384"></a>
- ✅ Q384 Caps → per record, 100 recurring and 10 000 one-shots

<a id="q385"></a>
- ✅ Q385 Daemon downtime → a past-due one-shot fires once with `late_by`; a recurring one fires one catch-up with `missed: N`

<a id="q386"></a>
- ✅ Q386 Time zones → UTC only, converted for display

<a id="q387"></a>
- ✅ Q387 Tags → the holder's tags on KV, Crons and Ats

<a id="q388"></a>
- ✅ Q388 Seconds → an optional sixth field; five fields fire at second 0

<a id="q389"></a>
- ✅ Q389 Short-lived holder → the schedule goes with it; adding answers with a warning

<a id="q390"></a>
- ✅ Q390 Alerts and warnings → every item in [warnings and alerts](#warnings-and-alerts); no notices

<a id="q409"></a>
- ✅ Q409 Refusals in a row that disable a schedule → 5. Owner, 2026-11-12: "ok"

<a id="q410"></a>
- ✅ Q410 Must a target admit `#bus` → no: `#bus` is always allowed to send to anyone; the creator's may-send guards a schedule. Owner, 2026-11-12: "bus should be always allowed to send to anyone", then "its the core rule": it applies to every `#bus` send ([RD186](DECISIONS.md#recorded-decisions), [the rule](bus-admission.md#the-rule))

<a id="q411"></a>
- ✅ Q411 Do record export and import carry schedules → yes, as system state. Owner, 2026-11-12: "yes - it a system state"

<a id="q412"></a>
- ✅ Q412 Where warnings and alerts land → the agent's built-in warnings and alerts. Owner, 2026-11-12: "we have built in warning/alert system; now when schedule is a part of daemon - they should go to agent's warnings"

<a id="q413"></a>
- ✅ Q413 Unread or refused firings to one target before a warning → 3. Owner, 2026-11-12: "ok"

<a id="q417"></a>
- ✅ Q417 May a person's User record hold schedules → no: a holder only stores a schedule, and a person keeps one on an agent or queue they manage, such as their own session. Owner, 2026-11-12: "users schedule makes no sense - what will it trigger? what for"

<a id="q414"></a>
- ✅ Q414 Who removes `#cron` and `#at` from the live node → the Claude session that builds it, at the deploy that ships schedules. Owner, 2026-11-12: "you"

### Open questions

<a id="q415"></a>
- ❓ Q415 When a manager other than the creator enables or runs a schedule, whose authority does it fire on
  - Context: a Maintainer could otherwise fire the Owner's schedule at a target the Maintainer cannot reach
  - ( ) The caller becomes the creator, after its own may-send to the target 🌟
  - ( ) The creator stays; the caller must also pass may-send to the target

<a id="q416"></a>
- ❓ Q416 Node-wide limits and outcome retention
  - Context: caps are per holder only, so one owner with many queues can hold far more; 10 000 pending one-shots against 100 kept outcomes evicts most results unread
  - ( ) A cap per holder Owner across all holders, 100 000 one-shots and 1 000 recurring; outcomes kept 7 days per holder instead of the last 100 🌟
  - ( ) Per-holder caps only, last 100 outcomes, as planned


<a id="q418"></a>
- ❓ Q418 Where a queue's or pubsub's schedule warnings go
  - Context: built-in warnings exist only on Agent records ([Q412](#q412))
  - ( ) The creator's agent warnings when the creator is an agent; the daemon journal otherwise 🌟
  - ( ) Only Agents hold schedules after all; queues and pubsubs drop out of Q382
