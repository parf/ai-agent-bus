# Cron: send a message on a schedule or later

📌 **TL;DR:**
- **Problem:** everything on the bus waits to be called, so a daily report, a reminder or "check again in an hour" needs a person to remember.
- **Outcome:** you hand cron a message and a time; it attempts each due send once under its own name. Missed recurring slots are skipped, and uncertain or refused sends are not retried.
- **Start:** see what it can do: `agent-bus help '#cron@node.example'` ([Try this first](#try-this-first)).

## What it does

- ✅ **Repeating messages:** "every weekday at nine", "every 15 minutes", with the usual five-field schedule.
- ✅ **One-off messages:** a delay such as "in two hours", or an RFC 3339 instant with an offset.
- ✅ **One attempt per firing:** restarts do not repeat a claimed send; this does not guarantee receipt or processing.
- ✅ **Survives restarts:** your schedule is kept; a slot missed while cron was down is skipped, and a one-off that fell due is sent once when cron starts.
- ✅ **Says who scheduled it:** the message arrives from cron with topic `cron` or `later` and the tag `by <creator>`.
- ✅ **Control:** you list and stop your own entries; cron's Owner and Maintainers manage all of them.
- 🚫 **Not for:** sub-minute timing, catching up on runs missed while it was down, or delivery that needs retries after a destination refuses (each copy is one attempt: a refused one is dropped, never retried).

| Call | Does |
|---|---|
| `sub add <m h dom mon dow> <target> <message>` | Sends the message at every matching minute |
| `sub list`, `sub delete <id>` | Shows or stops subscriptions |
| `later <when> <target> <message>` | Sends the message once |
| `later list`, `later cancel <id>` | Shows pending one-shots and the last 100 outcomes, or cancels one |

- A cron entry is a schedule, not a match, so cron keeps its own calls; its authority and delivery follow the [shared subscription rules](subscriptions.md#the-shared-behaviour).

## You'll need

- A [person's runner](../runner/setup.md#a-persons-runner) with Python 3, and time zone data on the host if `tz` is not UTC.
- Permission to call cron, and destinations that admit both you and the cron instance.

## Try this first

Use the complete name of your [enabled instance](#installing-it) in the commands below.

- Ask cron what it can do:

```sh
agent-bus help '#cron@node.example'
```

- Replace `#claude/main@node.example` with an existing session of yours that admits cron, then request a message in a minute:

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

## Installing it

1. Use the `cron` package included with the bus installation.
2. The owner registers the instance with owner-only access and hands its bus token privately to the runner, as for [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves).
3. Set `enabled:true` for `cron` in the [runner inventory](../runner/configuration.md#inventory), preserving other entries, then reload the runner.
4. Grant callers and allow `#cron` on each destination it should reach.

It needs no network beyond the bus socket.

## Configuration

`tz` sets the IANA time zone for schedules; default: `UTC`. Example: `{"tz":"America/New_York"}`.

- A zone missing from the host's time zone data makes the Configuration invalid: cron then answers every call with the error, fires nothing and leaves stored entries untouched until it is fixed.

## Secret keys

None.

## Who may use it

- Cron sends under its own name, and the destination's ordinary access list decides: a name, a group or `*` (anyone) that admits cron receives its messages, at every firing.
- Anyone on cron's own access list may schedule, so **a destination that allows cron trusts everyone allowed to call cron**.
- Run one cron instance per trust group (`#cron/team`) and allow only that instance where it is needed.
- A role granted to cron itself on a destination (say Maintainer) travels with every cron message, so every cron caller acts with it there; cron gets no role from its Owner's own records.
- The creator must also be allowed to send to the destination themselves: checked when the entry is added and again before every send.
- If that access is lost, that copy is dropped with a warning and the subscription stays active for its next slot; when the check cannot be made, nothing is stored or sent.
- Deleting or cancelling an entry needs its creator, or cron's Owner or Maintainers.
- Lists show your own entries with their messages; cron's Owner and Maintainers also see the others' ids, targets and outcomes.
- `later list` retains recent outcomes only since the last start, shown by `outcomes_since`.
- Cron never schedules messages to itself.

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#cron` reports these.

| Level | When | What to do |
|---|---|---|
| alert | Configuration `tz` is not in the host's time zone data: nothing fires | Fix `tz`, or install tzdata on the runner host |
| warning | A copy was dropped: `admission_refused`, `target_gone`, `send_refused` or `send_unknown` (its acceptance is unknown, so it may have arrived) | Check the destination; the subscription stays active, and a one-shot is gone |
| warning | A slot was missed: the bus or its KV store was unavailable, or the pass was late | Nothing if it clears; if it repeats, check the daemon |
| warning | More than 200 creator–destination pairs fell due in one minute (`deferred`) | Spread the schedules over more minutes |
| warning | A stored entry is invalid and was set aside | Delete it and add it again |

## Troubleshooting

| If you see | Do this |
|---|---|
| `you may not send to …, so cron will not send there for you` | Ask the destination's Owner to allow you |
| `admission_refused` for an entry whose creator lost access | Restore the creator's access; the next slot is delivered |
| `could not check that you may send to …`, or `author_check: unavailable` in a list | The daemon could not answer the creator check, or is down; nothing is sent until it can, and missed slots are not replayed |
| `deferred` in a list | More than 200 different creator–destination pairs were due in one minute; the rest wait for the next pass (a missed slot is not replayed) |
| `only its creator or cron's Owner and Maintainers delete an entry` | Ask its creator, or cron's Owner |
| `send_unknown` | The delivery could not be confirmed, so that copy counts as dropped; check whether the destination received it. The subscription stays active |
| `admission_refused` | The destination's access list does not allow cron: add cron there (`agent-bus manage <target> --add-allow '#cron@node.example'`); the next slot is delivered |
| `unknown destination …, or it does not allow cron` when adding | Check the name, or ask its Owner to allow cron |
| `target_gone` | The target was removed or no longer allows cron; delete the entry, or allow cron again |
| `send_refused` | The destination refused that copy, for example because its inbox was full; make room in it. Nothing is retried |
| `sub edit …` | Not supported: delete it and add it again |
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
- Then that copy is dropped with one warning, counted in `dropped` in `sub list`, and the subscription stays active for its next slot; a one-shot is dropped and removed, and `later list` shows it as finished `dropped`.
- A destination whose access list does not admit cron (403) drops the copy as `admission_refused`, and one that no longer exists (404) as `target_gone`; neither is retried.
- A new entry must name an existing queue or agent that cron can see.
- Any other definite refusal (the destination is full) drops the copy as `send_refused`. Nothing is retried, held or paused.
- An older stored paused subscription becomes active when cron starts, and a paused one-shot is dropped. A stored id that is not 12 hex moves once, to the first 12 hex of the SHA-256 of its old key (of that digest again while the id is taken), and the new entry is written before the old one is removed.
- Each send carries a time to live so a destination that was away is not flooded: the time to the next slot, capped at 24 h; 24 h for a one-shot.
- When cron cannot reach the bus 30 times in a row, 2 s apart, it exits and its runner starts it again. Until then, a slot that falls due is `missed`.

</details>

<details>
<summary>JSON forms and bounds</summary>

- Forms: `{"method":"sub.add","cron":"0 9 * * 1-5","target":"…","body":"…"}`, `{"method":"later","in":"+15m"|"at":"…","target":"…","body":"…"}`, `sub.list`, `sub.delete`, `later.list`, `later.cancel` with `id`. Use JSON for a multi-line message.
- Bounds: 100 subscriptions (20 per creator), 1000 pending one-shots (100 per creator), 16384 bytes per message.
- Outcomes and the `sent` and `dropped` counts are kept in memory and reset on restart; a one-shot's finished record is kept.

</details>
