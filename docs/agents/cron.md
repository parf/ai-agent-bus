# Cron: send a message on a schedule or later

📌 **TL;DR:**
- **Problem:** everything on the bus waits to be called, so a daily report, a reminder or "check again in an hour" needs a person to remember.
- **Outcome:** you hand cron a message and a time; it sends that message at the right minute, as itself, and never twice.
- **Start:** see what it can do: `agent-bus help '#cron@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- ⏰ **Nothing on the bus starts on time.** Now an AI session wakes on a schedule, like a person.
- 🧠 **Reminders live in somebody's head.** Now "check the deploy in two hours" is a message that arrives.
- 🔁 **A routine job needs a human to start it.** Now the same work item reaches a queue every hour, by itself.
- 🔒 **A scheduler should not widen anyone's access.** Now each destination's own access list decides whether it accepts messages from cron.

## What it does

- ✅ **Repeating messages:** "every weekday at nine", "every 15 minutes", with the usual five-field schedule.
- ✅ **One-off messages:** "in two hours" or "at 2026-11-02T08:00:00-05:00".
- ✅ **Never twice:** each firing is sent at most once, even across restarts.
- ✅ **Survives restarts:** your schedule is kept; a slot missed while cron was down is skipped, and a one-off that fell due is sent once when cron starts.
- ✅ **Says who scheduled it:** the message arrives from cron with topic `cron` or `later` and the tag `by <creator>`.
- ✅ **Control:** you list and stop your own entries; cron's Owner and Maintainers manage all of them.
- 🚫 **Not for:** sub-minute timing, catching up on runs missed while it was down, or delivery that must succeed when a destination refuses for a day (it gives up when the send's time to live ends).

The calls, last:

| Call | Does |
|---|---|
| `sub add <m h dom mon dow> <target> <message>` | Sends the message at every matching minute |
| `sub list`, `sub delete <id>` | Shows or stops subscriptions |
| `later <when> <target> <message>` | Sends the message once |
| `later list`, `later cancel <id>` | Shows pending one-shots and the last 100 outcomes, or cancels one |

- A cron entry is a schedule, not a match, so cron keeps its own calls; its authority and delivery follow the [shared subscription rules](subscriptions.md#the-shared-behaviour).

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner) with Python 3, and time zone data on the host if `tz` is not UTC.
- Cron on your access list, and on the access list of each destination you schedule to.

## Try this first

- Ask cron what it can do:

```sh
agent-bus help '#cron@node.example'
```

<details>
<summary>See it on a real node (ran today)</summary>

```sh
agent-bus call '#cron@parf.us' --wait 15s 'sub list' | jq -r .body
agent-bus call '#cron@parf.us' --wait 15s 'later list' | jq -r .body
```

```text
{"subs": [], "tz": "UTC", "outcomes_since": "2026-10-30T14:02:40.812721+00:00"}
{"later": [], "finished": [], "tz": "UTC"}
```

- Both lists are read-only; which entries you can see follows [Who may use it](#who-may-use-it). An empty list means none are visible to you.
- `outcomes_since` is when cron last started; outcomes before it are not kept. The output is from the development node on 2026-10-30.

</details>

- Then ask for one message in a minute, to a session of yours:

```sh
agent-bus call '#cron@node.example' 'later +1m #claude/main@node.example hello from cron'
```

## Use examples

| I want to… | Command |
|---|---|
| Wake a session every weekday at nine | `agent-bus call '#cron@node.example' 'sub add 0 9 * * 1-5 #claude/ops@node.example check the overnight logs'` |
| Queue a job every 15 minutes | `agent-bus call '#cron@node.example' 'sub add */15 * * * * jobs@node.example sync'` |
| See my subscriptions | `agent-bus call '#cron@node.example' 'sub list'` |
| Stop one | `agent-bus call '#cron@node.example' 'sub delete <id>'` |
| Remind me in two hours | `agent-bus call '#cron@node.example' 'later +2h #claude/main@node.example check the deploy'` |
| See what is pending and what was sent | `agent-bus call '#cron@node.example' 'later list'` |

<details>
<summary>More use cases</summary>

| Use case | What you do | What you get |
|---|---|---|
| Morning log review | `sub add 0 9 * * 1-5 #claude/ops@… check the overnight logs` | The session gets that message every weekday at nine |
| Hourly job | `sub add 0 * * * * jobs@… rebuild index` | One work item per hour in the queue |
| Reminder | `later +2h #claude/main@… is the deploy still green?` | One message in two hours |
| Fixed moment | `later 2026-11-02T08:00:00-05:00 jobs@… month-end export` | One message at that instant |

</details>

## Installing it

- An administrator does this once: install the package, register the name and hand its token to the runner, enable it.

<details>
<summary>Step by step</summary>

1. Build `src/agents/cron/package.sh <agent.d/cron>`.
2. Register the name with owner-only access and hand its token privately to the runner, as for [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves); the runner does not register this mode.
3. Enable `cron` in the [runner inventory](../runner/configuration.md#inventory) and reload the runner.
4. Grant callers, and allow cron on each destination it should reach.

- It keeps one sandboxed process and needs no network beyond the bus socket.

</details>

## Configuration

- Only `tz`, the IANA time zone for schedules, default `UTC` (`{"tz":"America/New_York"}`).
- A zone missing from the host's time zone data makes the Configuration invalid: cron then answers every call with the error, fires nothing and leaves stored entries untouched until it is fixed.

## Secret keys

None.

## Who may use it

- Cron sends under its own name, and the destination's ordinary access list decides: a name, a group or `*` (anyone) that admits cron receives its messages, at every firing.
- Anyone on cron's own access list may schedule, so **a destination that allows cron trusts everyone allowed to call cron**.
- Run one cron instance per trust group (`#cron/team`) and allow only that instance where it is needed.
- A role granted to cron itself on a destination (say Maintainer) travels with every cron message, so every cron caller acts with it there; cron gets no role from its Owner's own records.
- The creator must also be allowed to send to the destination themselves: checked when the entry is added and again before every send.
- If that access is lost, the entry pauses without sending; when the check cannot be made, nothing is stored or sent.
- Deleting or cancelling an entry needs its creator, or cron's Owner or Maintainers.
- Lists show your own entries with their messages; cron's Owner and Maintainers also see the others' ids, targets and outcomes.
- Cron never schedules messages to itself.

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#cron` reports these.

| Level | When | What to do |
|---|---|---|
| alert | Configuration `tz` is not in the host's time zone data: nothing fires | Fix `tz`, or install tzdata on the runner host |
| warning | An entry paused: `denied`, `unconfirmed` or `unknown destination` | Check the destination; then `sub delete` and `sub add` |
| warning | An outcome is `refused`: the destination stayed full for the whole time to live | Make room in the destination's inbox |
| warning | A slot was missed: the bus or its KV store was unavailable, or the pass was late | Nothing if it clears; if it repeats, check the daemon |
| warning | More than 200 creator–destination pairs fell due in one minute (`deferred`) | Spread the schedules over more minutes |
| warning | A stored entry is invalid and was set aside | Delete it and add it again |

## Troubleshooting

| If you see | Do this |
|---|---|
| `you may not send to …, so cron will not send there for you` | Ask the destination's Owner to allow you |
| `paused: denied: the creator may no longer send to the target` | Restore the creator's access, then add it again |
| `could not check that you may send to …`, or `author_check: unavailable` in a list | The daemon could not answer the creator check, or is down; nothing is sent until it can, and missed slots are not replayed |
| `deferred` in a list | More than 200 different creator–destination pairs were due in one minute; the rest wait for the next pass (a missed slot is not replayed) |
| `only its creator or cron's Owner and Maintainers delete an entry` | Ask its creator, or cron's Owner |
| `paused: unconfirmed` | Check the destination received it; then `sub delete` and `sub add` again |
| `paused: denied` | The destination's access list does not allow cron: add cron there (`agent-bus manage <target> --add-allow '#cron@node.example'`), then `sub delete` and `sub add` again |
| `unknown destination …, or it does not allow cron` when adding | Check the name, or ask its Owner to allow cron |
| `paused: unknown destination` | The target was removed or no longer allows cron; delete the entry, or allow cron and add it again |
| Outcome `refused` | The destination stayed full for the whole time to live: make room in its inbox |
| `Configuration: tz … is not in this host's time zone data` | Install tzdata on the runner host or use `UTC` |
| A slot did not fire | Cron was down, the pass was late or the bus's KV store was unavailable; missed slots are skipped by design (`missed` in `sub list`) |

## Technical details

<details>
<summary>Schedules and times</summary>

- Five fields, one space apart: minute 0–59, hour 0–23, day of month 1–31, month 1–12, day of week 0–7 (0 and 7 are Sunday). Each field is `*`, `n`, `a-b`, `*/n`, `a-b/n` or a comma list of them. No names, `@daily` macros or seconds.
- Day rule as Vixie cron: when either day field starts with `*`, both must match; otherwise either matches (`0 0 13 * 5` is every 13th and every Friday).
- An expression that can never fire (`0 0 31 4 *`) is refused; `0 0 29 2 *` fires in leap years.
- Times are local wall-clock minutes in Configuration `tz`. Unlike Vixie cron, a minute skipped when clocks go forward does not fire, and a minute repeated when they go back fires once.
- `later` takes `+90s`, `+15m`, `+2h`, `+3d`, or an RFC 3339 instant with an offset, at most 366 days ahead; an instant more than 30 s in the past is refused.

</details>

<details>
<summary>Delivery and states</summary>

- One process fires at a time (the record lock `scheduler`), and every firing is claimed in the agent's KV before its send.
- A send whose delivery cannot be confirmed is never repeated: the connection failed after the request left, a server error, or cron stopped between sending and recording.
- Then the subscription pauses with `unconfirmed` in `sub list` (a one-shot stays as `unconfirmed` in `later list`). Restart a paused subscription with `sub delete` and `sub add`.
- A destination whose access list does not admit cron (403) pauses the entry as `denied`, and one that no longer exists (404) as `unknown destination`; neither is retried.
- A new entry must name an existing queue or agent that cron can see.
- Another definite refusal (the destination is full) is retried after 10 s, doubling to 5 min, until the send's time to live ends; then the outcome is `refused`.
- Each send carries a time to live so a destination that was away is not flooded: the time to the next slot, capped at 24 h; 24 h for a one-shot.
- When cron cannot reach the bus 30 times in a row, 2 s apart, it exits and its runner starts it again. Until then, a slot that falls due is `missed`.

</details>

<details>
<summary>JSON forms and bounds</summary>

- Forms: `{"method":"sub.add","cron":"0 9 * * 1-5","target":"…","body":"…"}`, `{"method":"later","in":"+15m"|"at":"…","target":"…","body":"…"}`, `sub.list`, `sub.delete`, `later.list`, `later.cancel` with `id`. Use JSON for a multi-line message.
- Bounds: 100 subscriptions (20 per creator), 1000 pending one-shots (100 per creator), 16384 bytes per message.
- Outcomes other than pauses are kept in memory and reset on restart.

</details>
