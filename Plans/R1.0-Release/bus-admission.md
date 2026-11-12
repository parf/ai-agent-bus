# `#bus` is always admitted

📌 **TL;DR:**
- **Problem:** a target must list `#bus` in its Allow to get agent-event copies, an answer redirected by `reply-to`, and, with schedules, scheduled messages; a missed entry silently loses them.
- **Rule:** owner, 2026-11-12: "bus should be always allowed to send to anyone", "its the core rule". Every target admits `#bus`; whatever `#bus` sends on someone's behalf needs that someone's own may-send to the target.
- **Status:** decided ([RD186](DECISIONS.md#recorded-decisions)); not built; task [RT36](TODO.md#bus-is-always-admitted).

## The rule

| `#bus` sends | Today | With the rule |
|---|---|---|
| An answer on the asker's own route | admitted | unchanged |
| An answer to a `reply-to` naming another record | that record's Allow must name `#bus` | the asker's may-send to that record decides |
| An agent-event subscription copy | the target's Allow must name `#bus`, refused at `sub add` otherwise | the subscription creator's may-send decides, as today, and nothing else |
| A scheduled message | — | the schedule creator's may-send ([schedules](cron-in-core.md#firing)) |

- `#bus` never sends on its own authority to a third party: each send names the principal whose may-send was checked, in the same bus lock hold as the send.
- A Personal record's permission to list `#bus` (Q328) becomes needless; existing entries stay harmless.

## Where it lands

| Place | Change |
|---|---|
| `src/internal/core/bus.go` | `sendTarget` admits `#bus` only through one path that takes the principal and checks its may-send |
| `src/internal/core/event_subs.go` | `subTargetTakesBus` no longer refuses a target that does not name `#bus` |
| [Answers](../../docs/02-access.md#answers), [`#bus`](../../docs/agents/bus.md#what-it-does), [records](../../docs/03-records.md#personal-and-shared), [messaging](../../docs/04-messaging.md), [decisions](../../docs/decisions.md#settled) | the Allow-must-name-`#bus` sentences go; the principal rule replaces them |
