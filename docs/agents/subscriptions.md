# Subscriptions

📌 **TL;DR:** A subscription asks a helper to forward what it sees, such as mail,
log lines or chat messages, into a queue or an agent's inbox. Every helper is
meant to share one behaviour: one copy per match, the author's send right
checked at `sub add` and before every send, a refused copy dropped and counted
while the subscription stays active. The shared engine is built; the helpers
still run their own until they move to it ([B.29](../../Plans/R1.0-Release/TODO.md#bundled-agents)).

What stays per helper is only the selector, what a subscription watches: a
mailbox for [IMAP](imap.md#what-it-does), a source for
[logwatch](logwatch.md#what-it-does), a channel for [Slack](slack.md#what-it-does)
and [Telegram](telegram.md#reply-routing). [Cron](cron.md#what-it-does) fires on
a schedule, not a match, and keeps its own calls.

## The shared behaviour

The rules every helper moves to. The shared engine implements each of them
(Python since 0.9.170, the TypeScript twin since 0.9.173); no helper runs on
it yet, so for users each is pending [B.29](../../Plans/R1.0-Release/TODO.md#bundled-agents)
until its helper moves ([today](#what-each-helper-does-today)).

| Rule | Every subscription |
|---|---|
| Calls | `sub add <target> [selector] [/regex/]`, `sub list`, `sub delete <id>`; the target is a queue or an agent |
| Authority | The author must be allowed to send to the target and to the helper itself, checked at `sub add` and again before every send |
| Delivery | Each match is one message into the target's own inbox, sent once and never retried |
| Refused send | A full, revoked or unreachable target: that copy is dropped and counted, the helper writes a warning, and the subscription stays active for the next match; no pause, retry or outbox. A copy whose acceptance is unknown counts as dropped, though it may have arrived |
| Start | From the moment it is added; nothing earlier is replayed |
| Limits | 100 subscriptions per helper instance, 20 per author; a filter of at most 200 characters, checked at `sub add`; each filter gets 50 ms per message and all of them one second, and a filter out of time drops only its own copy |
| Listing | `sub list` shows each subscription's id, target, selector, filter, author, creation time, and sent and dropped counts |
| Management | Each author lists and deletes only their own; the helper record's Owner and Maintainers list and delete all. A role reached through a forward does not count. The helper's own roles decide who may subscribe at all |
| Invalid stored state | A stored filter the grammar no longer accepts stays listed with `reason: invalid_filter`, matches nothing and warns, never becoming match-all. A stored entry that is not a subscription at all is listed only to the Owner and Maintainers, as its id with `reason: invalid_state`; it counts toward the limit and only they can delete it (engine since 0.9.171) |

## What each helper does today

Each helper still runs its own subscription code, checked at 0.9.173. A cell
says where it already follows the rule, and since which version at least;
otherwise what it does instead. Telegram shares Slack's subscription code.

| Rule | [IMAP](imap.md#what-it-does) | [Logwatch](logwatch.md#what-it-does) | [Slack](slack.md#what-it-does), [Telegram](telegram.md#reply-routing) |
|---|---|---|---|
| Calls | `sub add <target> [/regex/]` over sender and subject; queue or agent checked (0.9.149) | `sub add <target> [filter]`; target kind not checked | `sub add <channel or chat> <target> [filter]` and `sub edit`; Slack also `from=`; target kind not checked |
| Authority | at add and before every send (0.9.149); not toward the helper itself | at add and before every send (0.9.145); not toward the helper | at add, on a moved target and before every send (0.9.145); not toward the helper |
| Delivery | a durable outbox that retries | one send (0.9.29); repeats within five minutes fold into one later "×N" message | one send, never replayed (Slack 0.8.137, Telegram 0.8.218) |
| Refused send | retries, then a counted skip; a denial or an unconfirmed send pauses it; copies are held | the error is logged, nothing counted; a denial or a missing target pauses it | the last error is kept, nothing counted; a denial, a missing target or an unknown author disables it |
| Start | from the mailbox's next message at add (0.9.149) | from now; history read at start is not sent (0.9.29) | from now (Slack 0.8.137); Telegram after a restart still sends what arrived while it was down |
| Limits | 100 per instance, 20 per author, filter 200 characters, 50 ms per filter (0.9.149) | no count limit; filter 200 characters; one second per batch | no count limit; filter 200 characters; 50 ms per test, then the subscription is disabled |
| Listing | id, target, author, time, filter, sent, held and skipped copies, pause | id, target, filter, author, forwarded count, paused, last error; no time | id, channel, target, filter, author, time, forwarded count, disabled, last error |
| Management | anyone admitted lists all; the author or the Owner and Maintainers delete | anyone lists; adding and deleting need Owner or Maintainer, or management of the target | as logwatch, for add, edit and delete |
| Invalid stored state | a damaged entry is listed as disabled and only the Owner and Maintainers delete it; a bad stored filter disables intake (0.9.149) | not checked at load | not checked at load; a bad stored filter stops that event for the subscriptions after it |

<details>
<summary>Filters</summary>

A filter is a bare pattern or `/pattern/flags`, with flags `i`, `m`, `s` and `u`.
It is the regular-expression subset JavaScript and Python agree on: groups,
alternation, anchors, ASCII classes, `\d \w \s`, and quantifiers up to 100,
at most 20 of them. Lookaround, backreferences, named groups, inline flags,
nested quantified groups and non-ASCII classes are refused at `sub add`. A
filter is tested against at most 8192 bytes of text. An empty filter matches
everything the selector does. The common vectors are `src/testdata/subscriptions.json`.

</details>

<details>
<summary>Where it is built</summary>

| Part | Home |
|---|---|
| Python engine | `src/agents/shared/subscriptions.py`, filters in `subscription_filters.py` |
| TypeScript twin, for Slack and Telegram | `src/agents/shared/subscriptions.ts`, since 0.9.173 |
| Shared vectors both pass | `src/testdata/subscriptions.json` |
| Engine contract | `src/agents/shared/subscriptions.md` |
| Moving each helper | [plan](../../Plans/R1.0-Release/subscriptions.md#moving-the-agents) |

</details>
