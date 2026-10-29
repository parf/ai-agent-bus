# Cron: send a message on a schedule or later

📌 **TL;DR:** Cron sends a message you wrote to a queue or an agent, every
weekday at nine or once in fifteen minutes. Nothing else on the bus starts on
time; with cron an AI session can be woken by a schedule like by a person. You
may only schedule messages to destinations you manage, and no firing is ever
sent twice.

## Why you'd want it

**The problem.** Every agent on the bus waits to be called. A daily report, a
reminder or "check again in an hour" needs somebody to send the message at the
right moment.

**The idea.** One helper keeps a list of scheduled messages and sends each one
at its minute, under its own name. The list survives restarts, and who may add
to it is decided by the destination, not by cron.

**When to use it**

- An AI session should start a routine job at a fixed time.
- You want a reminder or a retry delivered later, without keeping a terminal open.
- A queue should receive the same work item every hour.

**When not to:** sub-minute timing, catching up on runs missed while it was
down (it skips them), or anything that must be delivered even when the
destination refuses for a day (it gives up after its TTL).

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Morning log review | `sub add 0 9 * * 1-5 #claude/ops@… check the overnight logs` | The session gets that message every weekday at nine |
| Hourly job | `sub add 0 * * * * jobs@… rebuild index` | One work item per hour in the queue |
| Reminder | `later +2h #claude/main@… is the deploy still green?` | One message in two hours |
| Fixed moment | `later 2026-11-02T08:00:00-05:00 jobs@… month-end export` | One message at that instant |

## What it does

| Call | Does |
|---|---|
| `sub add <m h dom mon dow> <target> <message>` | Sends the message at every matching minute |
| `sub list`, `sub delete <id>` | Shows or stops subscriptions |
| `later <when> <target> <message>` | Sends the message once |
| `later list`, `later cancel <id>` | Shows pending one-shots and the last 100 outcomes, or cancels one |

The message is the rest of the line, unchanged; it arrives from cron with topic
`cron` or `later`. Each firing is sent **at most once**. A slot missed while
cron was down or slow is skipped; a one-shot that fell due while it was down is
sent once when it starts.

<details><summary>Technical details: schedules, delivery and states</summary>

- Five fields, one space apart: minute 0–59, hour 0–23, day of month 1–31, month 1–12, day of week 0–7 (0 and 7 are Sunday). Each field is `*`, `n`, `a-b`, `*/n`, `a-b/n` or a comma list of them. No names, `@daily` macros or seconds.
- Day rule as Vixie cron: when either day field starts with `*`, both must match; otherwise either matches (`0 0 13 * 5` is every 13th and every Friday). An expression that can never fire (`0 0 31 4 *`) is refused; `0 0 29 2 *` fires in leap years.
- Times are local wall-clock minutes in Configuration `tz`. Unlike Vixie cron, a minute skipped when clocks go forward does not fire, and a minute repeated when they go back fires once.
- `later` takes `+90s`, `+15m`, `+2h`, `+3d`, or an RFC 3339 instant with an offset, at most 366 days ahead; an instant more than 30 s in the past is refused.
- One process fires at a time (the record lock `scheduler`), and every firing is claimed in the agent's KV before its send. A send whose delivery cannot be confirmed — the connection failed after the request left, a server error, or cron stopped between sending and recording — is never repeated: the subscription pauses with `unconfirmed` in `sub list` (a one-shot stays as `unconfirmed` in `later list`). Restart a paused subscription with `sub delete` and `sub add`.
- A definite refusal (the destination is full, unknown or does not allow cron) is retried after 10 s, doubling to 5 min, until the send TTL ends; then the outcome is `refused`.
- Each send carries a TTL so a destination that was away is not flooded: the time to the next slot, capped at 24 h; 24 h for a one-shot.
- JSON forms: `{"method":"sub.add","cron":"0 9 * * 1-5","target":"…","body":"…"}`, `{"method":"later","in":"+15m"|"at":"…","target":"…","body":"…"}`, `sub.list`, `sub.delete`, `later.list`, `later.cancel` with `id`. Use JSON for a multi-line message.
- Bounds: 100 subscriptions (20 per creator), 1000 pending one-shots (100 per creator), 16384 bytes per message. Outcomes other than pauses are kept in memory and reset on restart.

</details>

## You'll need

- A [person's runner](../07-setup.md#a-persons-runner) with Python 3, and time zone data on the host if `tz` is not UTC.
- Management of each destination you schedule to, and cron allowed to send to it.

## Try this first

```sh
agent-bus help '#cron@node.example'
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

An administrator does this once: install the package, register the name and
hand its token to the runner, enable it.

<details><summary>Step by step</summary>

1. Build `src/agents/cron/package.sh <agent.d/cron>`.
2. Register the name with owner-only access and hand its token privately to the runner, as for [agents that serve themselves](../runner/configuration.md#agents-that-serve-themselves); the runner does not register this mode.
3. Enable `cron` in the [runner inventory](../runner/configuration.md#inventory) and reload the runner.
4. Grant callers, and allow cron on each destination it should reach.

It keeps one sandboxed process and needs no network beyond the bus socket.

</details>

## Configuration

Only `tz`, the IANA time zone for schedules, default `UTC`
(`{"tz":"America/New_York"}`). A zone missing from the host's time zone data
makes Configuration invalid: cron then answers every call with the error,
fires nothing and leaves stored entries untouched until it is fixed.

## Secret keys

None.

## Who may use it

Cron sends under its own name, so a schedule is only as safe as the right to
make one. Adding, deleting or cancelling an entry needs management of its
destination, for everybody including cron's Owner, and it is checked again at
every firing: when the creator loses that right, the subscription pauses as
`revoked` and a one-shot is dropped. A destination that allows cron trusts
every person who manages it. Lists show messages only for destinations you
manage; cron's Owner and Maintainers also see the others' ids, targets and
outcomes, and may delete them. Cron never schedules messages to itself.

## Troubleshooting

| If you see | Do this |
|---|---|
| `scheduling requires management of destination` | Ask the destination's Owner to make you a Maintainer, or pick one you manage |
| `paused: unconfirmed` | Check the destination received it; then `sub delete` and `sub add` again |
| `paused: revoked` | The creator no longer manages the destination; delete it, or have a manager add it again |
| Outcome `refused` | Allow cron on the destination, or make room in its inbox |
| `Configuration: tz … is not in this host's time zone data` | Install tzdata on the runner host or use `UTC` |
| A slot did not fire | Cron was down or the pass was late; missed slots are skipped by design (`missed` in `sub list`) |
