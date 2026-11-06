# Subscriptions: what a helper sees, sent to an inbox

📌 **TL;DR:**
- **Problem:** new mail, a log error or a chat message matters only if someone notices it, and watching each source by hand does not scale.
- **Outcome:** you ask a helper to forward every match to a queue or an agent's inbox: one copy per match, only while you may send there.
- **Start:** choose a helper below, inspect its help, then add a forward to an existing inbox you may use ([how to use it](#how-to-use-it)).

## What you get

- New matches arrive at your chosen inbox; `sub list` reports sent and dropped copies.
- No history replay, automatic delivery retries or in-place edits. To change a subscription, delete it and add it again. Schedules use [cron](cron.md#what-it-does).

Each helper decides only what a subscription watches:

| Helper | Watches |
|---|---|
| [IMAP](imap.md#what-it-does) | new mail in one mailbox, matched on sender and subject |
| [Logwatch](logwatch.md#what-it-does) | new lines from its log sources |
| [Slack](slack.md#what-it-does) | messages in one channel, optionally from chosen authors |
| [Telegram](telegram.md#reply-routing) | messages in one chat |

## How to use it

Enable the chosen helper using its guide below. Replace example helper and target names with registered names; chat/channel selectors must exist in the connected service.

| I want to… | Command |
|---|---|
| Get new log errors in my inbox | `agent-bus call '#logwatch@node.example' 'sub add #claude/oncall@node.example ERROR'` |
| Get future invoices into a queue | `agent-bus call '#imap/work@node.example' 'sub add invoices@node.example /invoice/i'` |
| Forward Slack errors to my session | `agent-bus call '#slack@node.example' 'sub add #alerts #claude/main@node.example /error/i'` |
| Forward a Telegram group's errors | `agent-bus call '#telegram/ann@node.example' 'sub add #dev #claude/main@node.example /error/i'` |
| See mine and their counts | `agent-bus call '#logwatch@node.example' 'sub list'` |
| Stop one | `agent-bus call '#logwatch@node.example' 'sub delete <id from sub list>'` |

- You must be allowed to send to the target and to the helper itself.
- The target is a queue or an agent; anything else is refused at `sub add`.
- The answer to `sub add` gives the id that `sub list` shows and `sub delete` takes.

## Technical details

### The shared behaviour

| Rule | Every subscription |
|---|---|
| Calls | `sub add <target> [selector] [/regex/]`, `sub list`, `sub delete <id>`; the target is a queue or an agent |
| Authority | The author must be allowed to send to the target and to the helper itself, both checked at `sub add`; the target is checked again before every send, the helper as [the next paragraph](#source-check) says |
| Delivery | Each match is one message into the target's own inbox, sent once and never retried |
| Refused send | A full, revoked or unreachable target: that copy is dropped and counted, the helper writes a warning, and the subscription stays active for the next match; no pause, retry or outbox. A copy whose acceptance is unknown counts as dropped, though it may have arrived |
| Start | From the moment it is added; nothing earlier is replayed |
| Limits | 100 subscriptions per helper instance, or per agent log for `#bus`; 20 per author within that same scope; a filter of at most 200 characters, checked at `sub add`; each filter gets 50 ms per message and all of them one second, and a filter out of time drops only its own copy |
| Listing | `sub list` shows each subscription's id, target, selector, filter, author, creation time, and sent and dropped counts |
| Management | Each author lists and deletes only their own; the helper record's Owner and Maintainers list and delete all. A role reached through a forward does not count. The helper's own roles decide who may subscribe at all |
| Invalid stored state | A stored filter the grammar no longer accepts stays listed with `reason: invalid_filter`, matches nothing and warns, never becoming match-all. A stored entry that is not a subscription at all is listed only to the Owner and Maintainers, as its id with `reason: invalid_state`; it counts toward the limit and only they can delete it |

<a id="source-check"></a>
The author's reach to the helper itself, the source, is checked again before each send only where removing it must stop the copies at once:

| Source checked | Helpers |
|---|---|
| At `sub add` only | [`#bus`](bus.md#agent-logs), [`#eventlog`](eventlog.md), [`#logwatch`](logwatch.md#what-it-does) |
| At `sub add` and before every send | [`#imap`](imap.md#what-it-does): its source is a mailbox, and an author whose reach to it is removed must stop receiving its mail; [`#slack`](slack.md#what-it-does) and [`#telegram`](telegram.md#reply-routing), whose shared TypeScript helper asks before every send |

`#eventlog` also checks the application role when the subscription is created. Every helper checks that the target admits the forwarding sender before each send.

### What each helper does today

Every helper follows the shared behaviour and differs only where this table
says. Telegram works like Slack.

| Rule | [IMAP](imap.md#what-it-does) | [Logwatch](logwatch.md#what-it-does) | [Slack](slack.md#what-it-does), [Telegram](telegram.md#reply-routing) |
|---|---|---|---|
| Calls | `sub add <target> [/regex/]`, matched on sender and subject | `sub add <target> [filter]` | `sub add <channel or chat> <target> [filter]`; Slack also takes `from=` authors before the filter |
| Delivery | each copy is tagged with its subscription id; the mail body is fetched once for all of them | every matching arrival is sent, repeated lines included | one copy per match |
| Start | from the mailbox's next message at `sub add` | from now; history read at start is not sent | from now; Telegram after a restart still sends what arrived while it was down |
| Listing | shows the first UID in place of a selector | the shared row | adds the channel, and the reason for an invalid filter |

<a id="filters"></a>
<details>
<summary>Filters</summary>

- A filter is a bare pattern or `/pattern/flags`, with flags `i`, `m`, `s` and `u`.
- A `#bus` text `sub add` trims trailing white space from the filter; write `/error /` to keep a trailing space.
- In a `#bus` text `sub add`, bare `notice`, `warning` and `alert` select a level; use `/alert/` to filter for the word `alert`, and likewise for the other level words.
- It is the regular-expression subset JavaScript and Python agree on: groups, alternation, anchors, ASCII classes, `\d \w \s`, and quantifiers up to 100, at most 20 of them.
- Lookaround, backreferences, named groups, inline flags, nested quantified groups and non-ASCII classes are refused at `sub add`.
- A filter is tested against at most 8192 bytes of text. An empty filter matches everything the selector does.

</details>
