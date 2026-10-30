# Unified subscriptions

📌 **TL;DR:** every agent's `sub add / sub list / sub delete` runs on one
shared implementation with one behaviour, instead of each agent's own
(owner, 2026-10-29: "use unified code for all subs"). Python agents import
`src/agents/shared/subscriptions.py`; the TypeScript agents (Slack, Telegram)
use a twin module held to the same language-neutral JSON test vectors, as the
env-file parser's shared cases. Tasks: [B.29](TODO.md#bundled-agents).

## One behaviour

The settled rules, with what is built and what is pending, now live in
[current docs § the shared behaviour](../../docs/agents/subscriptions.md#the-shared-behaviour);
what remains here is moving each agent onto them.

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
once in [the shared behaviour](../../docs/agents/subscriptions.md#the-shared-behaviour).
