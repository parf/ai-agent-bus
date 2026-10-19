# The Telegram agent

📌 **TL;DR:** `#telegram/<account>@<realm>` puts one Telegram bot on the bus:
post, read, channels, users, channel, user, react, edit, delete — and
subscriptions forwarding a chat's messages to a bus record. The bot token
comes from the record's bus secret and nowhere else; the destination
vocabulary (`#group`, `@user`, numeric chat ids) resolves through the
instance's configuration ([the plan](../../../Plans/R1.0-Release/telegram/README.md#telegram-agent)).

## Setup

1. **BotFather**: create the bot, take the token. In BotFather, either turn
   privacy mode **off** for the groups the bot should read everything in,
   or accept that in groups it sees only commands and replies. In channels
   it sees every post either way.
2. **The record**: register `#telegram/<account>@<realm>` (kind agent) and
   set its bus secret — an env file with the bot token and, optionally, an
   endpoint override:
   ```
   TELEGRAM_BOT_TOKEN=123456789:AA…the-token
   # optional, for tests against a fake; never needed in production:
   TELEGRAM_API_URL=http://127.0.0.1:8081
   ```
   The token comes from the bus secret and nowhere else: never the process
   env, never `telegram.toml` ([the plan](../../../Plans/R1.0-Release/telegram/README.md#telegram-agent)).
3. **The configuration** (the record's config, JSON):
   ```json
   { "default": "#dev", "aliases": { "#dev": "-1001234567890", "@alice": "12345678" } }
   ```
   - `default` — where a call with no destination goes; an alias or a
     numeric chat id. A destination-less call against an instance that
     declares none is refused, naming the key.
   - `aliases` — `#group` and `@user` names resolved to chat ids as
     decimal strings; matched exactly (`#DEV` is unknown when `#dev` is
     declared); a numeric chat id given directly in a call is used as is.
   A malformed configuration is refused at start, exit 78 naming the bad
   key ([the review](../../../Plans/R1.0-Release/telegram/README.md#where-things-live)).
4. **Run it**: the runner's agent.d — `package.sh` builds the bundle:
   ```
   src/agents/telegram/package.sh /var/lib/agent-bus/runners/<you>/agent.d/telegram
   ```
   The agent serves itself (`serves: itself`), registers as
   `#telegram/<account>@<realm>`, and starts receiving.

## Methods

The Slack agent's set and keywords wherever Telegram allows
([the plan's methods table](../../../Plans/R1.0-Release/telegram/README.md#methods)):
`post [<chat>] <text…>`, `read [<chat>] [N]`, `channels [filter]`,
`channel <chat>`, `users [filter]`, `user <@alias|id>`,
`react <chat> <id> <emoji>`, `edit <chat> <id> <text…>`,
`delete <chat> <id>`, `sub add/list/edit/delete`, `whoami`, `help` —
a text line or the same call as JSON, with the destination key `channel`
in JSON. The answers name what Telegram answered: chat and message id for
a post, Telegram's own fields for the rest.

## Receiving and forwarding

getUpdates long polling runs beside the serve loop; the offset and the
seen state (the last 200 messages per chat, the chats and senders
directories, each bounded) live in the agent's own KV, written after each
batch and fully once a minute. `read` answers the last N of a chat,
oldest first. Every stored message goes to the matching subscriptions —
topic `telegram`, tag the chat id, the README's body — through the shared
filter worker; the bot's own posts and edits are not forwarded. A filter
that cannot be evaluated in time disables its subscription with the
reason recorded, as Slack's does.

## The pinned HTTP policy

Every Telegram call runs inside the caller's deadline: a 429 waits
`parameters.retry_after` (at most 30 s) and retries the write once if the
wait and the retry still fit, a write is never retried otherwise, and a
read retries once on 5xx. A 409 — another poller or a webhook on the
token — is logged with its description and named by `whoami`.
