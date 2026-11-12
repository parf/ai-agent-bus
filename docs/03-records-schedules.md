# Schedules

📌 **TL;DR:**
- **Problem:** everything on the bus waits to be called, so a daily report, a reminder or "check again in an hour" needs somebody to remember.
- **Outcome:** any agent, queue or pub/sub keeps its own **schedules**: the bus sends a message on a cron expression, after a delay or at a time, from `#bus`, even after a restart. The web shows them on **Crons** and **Ats** and on each record's page.
- **Start:** `agent-bus schedule in '#worker@node.example' 2h '#worker@node.example' check the export` ([Try this first](#try-this-first)).

## What you get

- ⏰ **Repeating messages:** "every weekday at nine", "every 15 minutes", down to once a second.
- 1️⃣ **One-off messages:** after a delay (`90s`, `30m`, `2h`, `3d`) or at a time (`09:30`, or an RFC 3339 instant).
- 🔒 **Your reach, never more:** a message goes only where the schedule's creator may send, checked at every firing.
- 🏢 **Per agent, at any scale:** each record holds up to 100 repeating and 10 000 one-off schedules; one owner's records up to 1 000 and 100 000.
- 🧾 **Outcomes kept:** sent, refused or skipped, for seven days.
- 🕛 **UTC only:** every time is stored and evaluated in UTC; the web shows it in your zone.

## Try this first

Remind an agent in two hours, then list what it holds:

```sh
agent-bus schedule in '#worker@node.example' 2h '#worker@node.example' check the export
agent-bus schedule list '#worker@node.example'
```

| I want to… | Command |
|---|---|
| Wake a session every weekday at nine | `agent-bus schedule add '#claude/ops@node.example' '0 9 * * 1-5' '#claude/ops@node.example' check the overnight logs` |
| Queue a job every 15 minutes | `agent-bus schedule add jobs@node.example '*/15 * * * *' jobs@node.example sync` |
| Send once at 18:00 UTC | `agent-bus schedule at '#worker@node.example' 18:00 '#worker@node.example' wrap up` |
| See one with its outcomes and next five times | `agent-bus schedule get <id>` |
| Fire one now, keeping its times | `agent-bus schedule run <id>` |
| Pause, resume, remove | `agent-bus schedule disable <id>` · `enable <id>` · `delete <id>` |

## Who may use them

| Who | May |
|---|---|
| The record's Owner, Maintainers and own Agent | add, list, read, edit, enable, disable, run and delete its schedules |
| Anyone else | nothing; they cannot tell a schedule exists |

- The **creator** is whoever last added, edited or enabled a schedule. A message is sent only while the creator is active, still manages the record, and may send to the target.
- The **target** is any agent or queue the creator may send to, the record itself included; never `#bus`.

## How a message arrives

- From `#bus`, topic `schedule`, tag `by <creator>`; a reply goes to the creator.
- It carries a `schedule` stamp: its id, creator and slot, plus `late_by` and `missed` when it fired late. Only the bus sets it, so a receiver can trust it ([daemon API](09-daemon-api.md#schedules)).
- Every record admits `#bus`; no access list needs to name it ([answers](02-access.md#answers)).

## Rules

| Rule | |
|---|---|
| Expressions | five cron fields (minute, hour, day of month, month, day of week; 0 and 7 are Sunday), or six with a leading second; `*`, `n`, `a-b`, `*/n`, `a-b/n` and lists; when both day fields are restricted, either matches |
| One-off times | a delay of one unit up to 366 days; `HH:MM` is the next such UTC time; an instant up to 30 s in the past fires now |
| Message | 1 to 16 384 bytes; an empty one is refused |
| Options | `--until`, `--max-runs` (a repeating schedule ends and is disabled), `--max-late` (skip a firing later than this), `--disabled` |
| Delivery | one attempt per firing, never retried; a target that is down keeps the message in its queue until its time to live: to the next slot, at least a minute, at most a day |
| Refusals | a refused firing is skipped and warned; five in a row disable the schedule |
| Bus restarts | a firing cut off by a stop is dropped, never sent twice; a one-off that fell due fires once at start; a repeating one fires one catch-up that counts the slots it missed |
| Lifetime | removed with its record, kept when the record changes owner, silent while it is inactive; carried by record export and import |

## Warnings and alerts

They go to the record's own warnings when it is an agent ([event logs](agents/bus.md#agent-logs)), and to the bus's log in every case.

| Level | When |
|---|---|
| Warning | a refused or uncertain firing; one skipped for `--max-late`; a message that expired unread; three refusals in a row; a record at 80 % of a cap |
| Alert | a schedule disabled after five refusals; the bus running more than a minute behind; a stored schedule it cannot read |
