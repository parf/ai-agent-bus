# Unified subscriptions

📌 **TL;DR:** every agent's `sub add / sub list / sub delete` runs on one
shared implementation with one behaviour, instead of each agent's own
(owner, 2026-10-29: "use unified code for all subs"). Python agents import
`src/agents/shared/subscriptions.py`; the TypeScript agents (Slack, Telegram)
use a twin module held to the same language-neutral JSON test vectors, as the
env-file parser's shared cases. Tasks: [B.29](TODO.md#bundled-agents).

## One behaviour

| Rule | Every subscription |
|---|---|
| Calls | `sub add <target> [agent-specific selector] [/regex/]`, `sub list`, `sub delete <id>` |
| Authority | the author must be allowed to send to the target, checked at `sub add` and before each send ([RD141](DECISIONS.md#recorded-decisions)) |
| Delivery | each match is one message into the target's own inbox, sent once |
| Refused send | the target is full, revoked or unreachable: that message is dropped and counted, the agent writes a warning, and the subscription stays active for the next match; no pause, no retry, no outbox (owner, 2026-10-29) |
| Start | from now; no backlog replay |
| Limits | per-agent bounds on subscriptions and on filter size and run time; a filter is checked at `sub add` |
| Listing | `sub list` shows each subscription with its id, target, selector, filter, and sent and dropped counts |
| Invalid stored state | An unsupported filter stays visible to its creator and managers with `reason: invalid_filter`, inert until deletion/re-add. A damaged stored row is listed only to this record's direct Owner/Maintainers with `reason: invalid_state`, remains inert, permits only manager deletion, and still counts toward the subscription limit (owner, 2026-10-30) |

What stays agent-specific is only the selector and the match: a mailbox and
mail for `#imap`, a source and lines for `#logwatch`, a channel and messages for
Slack and Telegram, a log and level for `#eventlog`. A cron entry is a schedule,
not a match, and keeps its own calls; it uses the shared authority and delivery
rules.

## Moving the agents

| Agent | Change |
|---|---|
| `#eventlog` | built on the shared module from the start |
| `#imap` | its pause-on-refusal and held-copy retry give way to drop and warning; crash safety keeps no duplicate sends |
| `#logwatch` | its own sub code replaced |
| Slack, Telegram | the TypeScript twin replaces each agent's routing subs |
| cron, `#at` | shared authority and delivery for their sends |

Each agent's guide keeps its own `sub` examples; the common rules are stated
once in [the agents README](../../docs/agents/README.md#incoming-events-go-to-a-queue-or-an-agent).
