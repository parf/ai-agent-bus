# Telegram agent

📌 **TL;DR:** `#telegram/<account>@<realm>` puts one Telegram bot on the bus,
built as the [Slack agent](../../R1.2/slack/README.md#slack-agent)'s twin: the
same method names, text and JSON forms, answers, subscriptions and authority
wherever Telegram allows. What differs is Telegram's: no history API, so `read`
answers what the bot has received; a declared default chat and alias table in
the instance's configuration; updates by long polling.

Status: planned 2026-10-18 for [№B.1](../TODO.md#bundled-agents); the owner's
instruction is "do it like Slack, the same methods and keywords where possible".
Built by OpenCode, reviewed by Claude B and Codex ([TODO](TODO.md#tasks)). Code
goes in `src/agents/telegram/`.

## What carries over from Slack

| Unchanged | |
|---|---|
| Process | one bun process, `serves: itself`, packaged by its own `package.sh`; exits 78 when it starts unconfigured, naming what is missing |
| Forms | a text line or the same call as JSON; the JSON destination key is `channel`, the same keyword Slack takes — a JSON call answered `{"ok": true, …}` / `{"ok": false, "error": "…"}`, a text call as text, a failure `error: <why>` |
| Upstream errors | Telegram's `ok:false` is an answer: `error: <description>`; a 429 waits `parameters.retry_after`, at most 30 s, retries once; a write is never retried otherwise, a read once on 5xx |
| Subscriptions | `sub list/add/edit/delete`, the same filters, KV storage, counters, worker-bounded regex, and authority by the destination through `GET /manages` ([Slack § subscriptions](../../R1.2/slack/README.md#subscriptions-authority-errors), [pinned behaviour](../../R1.2/slack/README.md#behaviour-pinned-by-the-review)) |
| Shared code | imported from `src/agents/slack/` (filters, filter worker, subscriptions, env file, `/manages`), never copied; the bundle makes that safe. A move to a common directory only if both agents' tests stay green |

## Where things live

| | |
|---|---|
| Bot token | the record's bus secret, the one source it reads: `TELEGRAM_BOT_TOKEN`; `TELEGRAM_API_URL`, optional, for tests. Never the process env, never `~/.codex/telegram.toml` ([agents § telegram](../agents.md#telegram)) |
| Default chat and aliases | the record's configuration, read as that record (`record.config()`): accepted as proposed (telegram-build, 2026-10-18): `{"default": "#dev", "aliases": {"#dev": "-1001234567890", "@alice": "12345678"}}` — `default` is a string, an alias with its sigil or a numeric chat id; `aliases` maps sigil-carrying keys (so `#dev` and `dev` differ, and no key collides with a numeric id) to chat ids as decimal strings, exact for any id Telegram issues. A malformed configuration is refused at start, exit 78 naming the bad key: an alias key without its sigil, a value that is not a decimal integer string, a default naming an alias that does not exist, and an unknown configuration key (accepted: it matches agent.json's strict loading and the H1 rule — refuse unknown flags, fail loudly). No configuration is none, not malformed: the agent runs |
| Subscriptions, counters, update offset, recent messages | the agent's own KV |

An unknown alias is refused, never resolved to anything else. A call with no
destination goes to the declared default; with none declared it is refused,
naming the configuration key. A numeric chat id given directly in a call is
used as is, not required to be declared in `aliases` — for the owner's
review. The script's first-configured-chat fallback does not survive.

## Methods

`<chat>` is `#alias`, `@alias` or a numeric chat id; `<id>` is a message id.

| Method | Text | Telegram | Differs from Slack |
|---|---|---|---|
| post | `post [<chat>] <text…>` | `sendMessage`, plain text | the chat may be omitted: the first word is a chat only when it starts with `#` or `@` or is an integer; JSON `reply_to` replies to a message. Answers chat and message id |
| read | `read [<chat>] [N]` | none | answers the last N messages the bot received in that chat (default 20, at most 200), oldest first; Telegram gives bots no history |
| channels | `channels [filter]` | none | the aliases and the chats the bot has seen, with id, title, type |
| channel | `channel <chat>` | `getChat` | id, title, type, description, member count |
| users | `users [filter]` | none | the `@` aliases and the senders the bot has seen |
| user | `user <@alias\|id>` | `getChatMember` / seen senders | |
| react | `react <chat> <id> <emoji>` | `setMessageReaction` | |
| edit · delete | `edit <chat> <id> <text…>` · `delete <chat> <id>` | `editMessageText` · `deleteMessage` | |
| whoami | `whoami` | `getMe` | the bot's username and id |
| sub … | as Slack | | the channel is a `<chat>` |
| help | `help`, `--help` (the ask), or anything unknown | | |

Left out: `thread` (no thread history for bots), `join` (a person adds the bot),
file upload, and formatting modes; `post` sends plain text.

## Receiving

- `getUpdates` long polling, 30 s; the offset in KV, written after each
  non-empty batch, so a restart does not forward a batch again. A 409 (another
  poller or a webhook on the token) is logged and named by `whoami`.
- Each received message is kept in a per-chat ring of 200 for `read`, in memory,
  flushed to KV once a minute, the owner's statistics rule.
- Forwarding as Slack's: topic `telegram`, tag the chat id, body
  `{chat, chat_name, user, user_name?, text, message_id, reply_to?}`. Edits and
  the bot's own posts are not forwarded.
- In groups the bot sees every message only with privacy mode off in BotFather;
  the agent's README says so.

## Reply routing

Built in [TG.9](TODO.md#tasks); the [agent's reply-routing contract](../../../src/agents/telegram/README.md#reply-routing)
owns session tags, listed users, both reply directions, the default route and refusal diagnostics.

## Per-chat routing

Proposed as [№TG.10](TODO.md#tasks); not built. Owner, 2026-10-19: a chat may have its own default route, may turn off `#tag` detection, and may let everyone in it write. Example: a friends' group and company groups, each routed to its own session.

| Per chat (Configuration `chats`, keyed by alias or chat id) | Meaning |
|---|---|
| `route` | this chat's default route, before the global one |
| `tags: false` | `#word` is ordinary text here, never a session tag |
| `anyone: true` | any member may write; the "we do not know you" answer and the `users` check are skipped in this chat |

Company channels are real Telegram channels for employees, where only admins post (owner, 2026-10-19). There the bot must be a channel admin; it then receives every post as a `channel_post`. A channel is an announcement outlet: sessions post to it, and an admin's post goes to the channel's `route`. Readers cannot reply in a channel. Comments live in the channel's linked discussion group, which is an ordinary group with its own `chats` entry, and a Reply there to a session's post routes as anywhere else.

What is forwarded from a group is what Telegram delivers to the bot. In privacy mode, the default, that is a mention of the bot, a command, and a Reply to the bot's own posts; everyday chatter between people never reaches it (owner, 2026-10-19: option a). A chat without an entry follows the global rules. A Reply to a session's post routes in every chat.

