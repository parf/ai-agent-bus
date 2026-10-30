# Subscriptions: what a helper sees, sent to an inbox

📌 **TL;DR:**
- **Problem:** new mail, a log error or a chat message matters only if someone notices it, and watching each source by hand does not scale.
- **Outcome:** you ask a helper to forward every match to a queue or an agent's inbox: one copy per match, only while you may send there.
- **Start:** `agent-bus call '#logwatch@node.example' 'sub add #claude/oncall@node.example ERROR'` ([how to use it](#how-to-use-it)).

## Why you'd want it

- 👀 **Nobody watches every source.** Now each match arrives in an inbox you or your AI already read.
- 🔀 **Each source would need its own rules.** Now IMAP, logwatch, Slack and Telegram share one set.
- 🔒 **A forward can outlive the right to send.** Now the right is checked when you add it and again before every copy.
- 🧯 **One bad target should not stop the rest.** Now a refused copy is dropped and counted, and the subscription keeps working.

## What you get

- ✅ **One copy per match,** sent once into the target's own inbox and never retried.
- ✅ **From now on:** a new subscription starts at the moment you add it; nothing earlier is replayed.
- ✅ **Counts you can read:** `sub list` shows how many copies each subscription sent and dropped.
- ✅ **Your own list:** each author sees and deletes their own; the helper's Owner and Maintainers manage all.
- ✅ **Safe filters:** a filter is a small regular expression that every helper reads the same way, checked when you add it.
- 🚫 **Not for:** schedules (that is [cron](cron.md#what-it-does)), replaying history, or editing a subscription in place: delete it and add it again.

Each helper decides only what a subscription watches:

| Helper | Watches |
|---|---|
| [IMAP](imap.md#what-it-does) | new mail in one mailbox, matched on sender and subject |
| [Logwatch](logwatch.md#what-it-does) | new lines from its log sources |
| [Slack](slack.md#what-it-does) | messages in one channel, optionally from chosen authors |
| [Telegram](telegram.md#reply-routing) | messages in one chat |

## How to use it

| I want to… | Command |
|---|---|
| Get new log errors in my inbox | `agent-bus call '#logwatch@node.example' 'sub add #claude/oncall@node.example ERROR'` |
| Get future invoices into a queue | `agent-bus call '#imap/work@node.example' 'sub add #invoices@node.example /invoice/i'` |
| Forward Slack errors to my session | `agent-bus call '#slack@node.example' 'sub add #alerts #claude/main@node.example /error/i'` |
| Forward a Telegram group's errors | `agent-bus call '#telegram/ann@node.example' 'sub add #dev #claude/main@node.example /error/i'` |
| See mine and their counts | `agent-bus call '#logwatch@node.example' 'sub list'` |
| Stop one | `agent-bus call '#logwatch@node.example' 'sub delete <id from sub list>'` |

- You must be allowed to send to the target and to the helper itself.
- The target is a queue or an agent; anything else is refused at `sub add`.
- The answer to `sub add` gives the id that `sub list` shows and `sub delete` takes.
- To change a subscription, delete it and add it again; there is no `sub edit`.

## Technical details

### The shared behaviour

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
| Invalid stored state | A stored filter the grammar no longer accepts stays listed with `reason: invalid_filter`, matches nothing and warns, never becoming match-all. A stored entry that is not a subscription at all is listed only to the Owner and Maintainers, as its id with `reason: invalid_state`; it counts toward the limit and only they can delete it |

### What each helper does today

Every helper follows the shared behaviour and differs only where this table
says. Telegram works like Slack.

| Rule | [IMAP](imap.md#what-it-does) | [Logwatch](logwatch.md#what-it-does) | [Slack](slack.md#what-it-does), [Telegram](telegram.md#reply-routing) |
|---|---|---|---|
| Calls | `sub add <target> [/regex/]`, matched on sender and subject | `sub add <target> [filter]` | `sub add <channel or chat> <target> [filter]`; Slack also takes `from=` authors before the filter |
| Delivery | each copy is tagged with its subscription id; the mail body is fetched once for all of them | every matching arrival is sent, repeated lines included | one copy per match |
| Start | from the mailbox's next message at `sub add` | from now; history read at start is not sent | from now; Telegram after a restart still sends what arrived while it was down |
| Listing | shows the first UID in place of a selector | the shared row | adds the channel, and the reason for an invalid filter |

<details>
<summary>Filters</summary>

- A filter is a bare pattern or `/pattern/flags`, with flags `i`, `m`, `s` and `u`.
- It is the regular-expression subset JavaScript and Python agree on: groups, alternation, anchors, ASCII classes, `\d \w \s`, and quantifiers up to 100, at most 20 of them.
- Lookaround, backreferences, named groups, inline flags, nested quantified groups and non-ASCII classes are refused at `sub add`.
- A filter is tested against at most 8192 bytes of text. An empty filter matches everything the selector does.
- The common vectors are `src/testdata/subscriptions.json`.

</details>

<details>
<summary>Where it is built</summary>

| Part | Home |
|---|---|
| Python engine | `src/agents/shared/subscriptions.py`, filters in `subscription_filters.py` |
| TypeScript twin, for Slack and Telegram | `src/agents/shared/subscriptions.ts` |
| Shared vectors both pass | `src/testdata/subscriptions.json` |
| Engine contract | `src/agents/shared/subscriptions.md` |

</details>
