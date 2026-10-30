# At: send one message later

📌 **TL;DR:**
- **Problem:** "check again in an hour" or "start the export at nine" needs somebody to remember and send the message at the right moment.
- **Outcome:** you hand `#at` a message with a delay or a time, and it sends it once when it falls due, even across restarts; for repeating schedules use [cron](cron.md#what-it-does), which shares the same engine.
- **Start:** ask what the helper can do: `agent-bus help '#at@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🧠 **Somebody must remember.** Now the call is kept and sent when it falls due.
- 🔁 **A restart loses reminders.** Now each call is sent at most once, even across restarts.
- 🤖 **An AI wants to look again later.** Now it schedules the message to itself.
- ⏰ **A job should start at a time of day.** Now "at 09:00" does it, once.

## What it does

- ✅ **Send once after a delay:** `90s`, `30m`, `2h` or `3d` (`in <duration> <target> <message>`).
- ✅ **Send at a time:** `HH:MM` or an RFC 3339 instant (`at <time> <target> <message>`).
- ✅ **See what is pending:** your pending calls, recent outcomes and the agent's time zone (`list`).
- ✅ **Cancel one:** a pending call (`cancel <id>`).
- ✅ **No surprises:** the answer shows the resolved instant with its offset and the agent's zone (`"at": "2026-10-29T09:30:00-04:00", "tz": "America/New_York"`).
- ✅ **Arrives as:** the message is the rest of the line, unchanged, from `#at` with topic `at` and tag `by <creator>`.
- ✅ **Use cases:** re-check a deploy in 30 minutes; start a job at nine; send a message at a fixed instant.
- 🚫 **Not for:** repeating schedules (use [cron](cron.md)), sub-minute timing, or delivery that must survive a destination refusing for more than a day.

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner) with Python 3, and time zone data on the host if `tz` is not UTC.
- `#at` on your access list, and on the access list of each destination you send to.

## Try this first

- Ask the helper what it can do:

```sh
agent-bus help '#at@node.example'
```

- Then schedule one message a minute ahead to yourself or a session: `in 1m` followed by the target and the text.
- `HH:MM` is in the instance's `tz`, which is UTC unless its Configuration says otherwise; the answer shows the resolved instant and zone.

## Use examples

- **Remind a session in 30 minutes:** ask for `in 30m`, the session's name and the text.
- **Queue a job at nine:** ask for `at 09:00`, the destination and the text.
- **See what is pending:** ask for `list`.
- **Cancel one:** ask for `cancel` and its id.

## Installing it

- An administrator does this once: install the package, register the name and hand its token to the runner, enable it.

<details>
<summary>Step by step</summary>

1. Build `src/agents/at/package.sh <agent.d/at>`.
2. Register the name with owner-only access and hand its token privately to the runner, as for [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves); the runner does not register this mode.
3. Enable `at` in the [runner inventory](../runner/configuration.md#inventory) and reload the runner.
4. Grant callers, and allow `#at` on each destination it should reach.

- It keeps one sandboxed process and needs no network beyond the bus socket.

</details>

## Configuration

- **Only `tz`:** the IANA time zone for `HH:MM`, default `UTC` (`{"tz":"America/New_York"}`).
- **A zone missing from the host's time zone data makes Configuration invalid:** `#at` then answers every call with the error, sends nothing and leaves stored calls untouched until it is fixed.

## Secret keys

- **None.**

## Who may use it

- **`#at` sends under its own name,** and the destination's ordinary access list decides: a name, a group or `*` that admits `#at` receives its messages.
- **Anyone on `#at`'s own access list may schedule,** so **a destination that allows `#at` trusts everyone allowed to call `#at`**.
- **Keep the default instance owner-only** and run one instance per trust group (`#at/team`).
- **The creator must also be allowed to send to the destination themselves:** checked when the entry is added and again before every send; if that access is lost, the entry pauses without sending; when the check cannot be made nothing is stored or sent.
- **A role granted to `#at` itself on a destination** travels with every message; it gets none from its Owner's records.
- **Cancelling needs** the call's creator or `#at`'s Owner and Maintainers.
- **Lists show your own calls with their messages;** staff see the others' without.

## Troubleshooting

| If you see | Do this |
|---|---|
| `you may not send to …, so at will not send there for you` | Ask the destination's Owner to allow you |
| `paused: denied: the creator may no longer send to the target` | Restore the creator's access, then add it again |
| `could not check that you may send to …`, or `author_check: unavailable` in a list | The daemon could not answer the creator check or is down; nothing is sent until it can, and missed slots are not replayed |
| `deferred` in a list | More than 200 different creator–destination pairs were due in one minute; the rest wait for the next pass (a missed slot is not replayed) |
| `unknown destination …, or it does not allow at` | Check the name, or ask its Owner to allow `#at` |
| `… does not exist on <date>: the clocks skip it` | Pick another time; that minute is skipped by daylight saving |
| An answer at an unexpected hour | The time is in the agent's `tz`, shown in the answer; use an instant with an offset |
| `paused: denied` | Allow `#at` on the destination, then schedule the call again |
| `paused: unconfirmed` | Check whether the destination received it before scheduling again |
| Outcome `refused` | The destination stayed full for 24 h; make room in its inbox |

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#at` reports these.

| Level | When | What to do |
|---|---|---|
| alert | Configuration `tz` is not in the host's time zone data: nothing is sent | Fix `tz`, or install tzdata on the runner host |
| warning | A call paused: `denied`, `unconfirmed` or `unknown destination` | Check the destination; its creator schedules it again |
| warning | A call ended `refused`: the destination stayed full for 24 h | Make room in the destination's inbox |
| warning | The creator check or the bus was unavailable, so a send waited | Nothing if it clears; if it repeats, check the daemon |
| warning | More than 200 creator–destination pairs fell due in one minute (`deferred`) | Spread the calls over more minutes |
| warning | A stored call is invalid and was set aside | Cancel it and schedule it again |

## Technical details

<details>
<summary>Times, delivery and states</summary>

- `in` takes one number of at least 1 and one unit (`s`, `m`, `h`, `d`), with an optional leading `+`; no compound `1h30m`; at most 366 days.
- `HH:MM` is two digits each, `00:00`–`23:59`, in Configuration `tz`; it means the earliest instant strictly after the current minute whose local time is `HH:MM` (`09:30` sent at 09:30:15 is tomorrow).
- When clocks go back, both occurrences of a repeated time count, so the next one is taken; a time skipped when clocks go forward is refused with the nearest valid time, never shifted; words such as `tomorrow` are refused.
- An RFC 3339 instant needs an offset; one more than 30 s in the past is refused.
- An empty message is refused.
- JSON forms: `{"method":"in","in":"30m","target":"…","body":"…"}`, `{"method":"at","at":"09:30",…}`, `list`, `{"method":"cancel","id":"…"}`.
- Delivery is the [cron engine's](cron.md#what-it-does): claimed in KV before the send, never sent twice.
- An unconfirmed send pauses the call as `unconfirmed`; a 403 from the target's access list pauses it as `denied`, a vanished target as `unknown destination`, neither retried.
- A full inbox is retried with backoff for 24 h from the due time, then `refused`.
- A call that fell due while `#at` was down is sent once when it starts, possibly late.
- Cancelling a call whose send is in flight may still deliver it once.
- Bounds: 1000 pending calls, 100 per creator, 16384 bytes per message.

</details>
