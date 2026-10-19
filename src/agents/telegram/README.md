# The Telegram agent

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

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
   sudo src/agents/telegram/package.sh /var/lib/agent-bus/agent.d/telegram
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

getUpdates long polling runs beside the serve loop (30 s, with a 40 s HTTP
timeout); an empty poll is normal and clears any previous poll error; the offset and the
seen state (the last 200 messages per chat, the chats and senders
directories, each bounded) live in the agent's own KV, written after each
batch and fully once a minute when batches arrive. Startup restores that
history and retries unavailable KV reads; a failed store write keeps its batch
dirty and its polling offset unchanged. A re-delivery is neither added nor
forwarded twice. `read` answers the last N of a chat,
oldest first. Every stored message goes to the matching subscriptions —
topic `telegram`, tag the chat id, the README's body — through the shared
filter worker; the bot's own posts and edits are not forwarded. A filter
that cannot be evaluated in time disables its subscription with the
reason recorded, as Slack's does.

Incoming messages are marked seen only after handling succeeds or is definitively
refused. Transient handler failures retry with poll backoff, at most three times;
exhaustion logs the message id and moves on.

A stored `seen:<chat>`, `chats` or `users` value that is not a JSON array is logged by key and starts empty; transient KV read failures still retry.

## Reply routing

Every session's `post` ends with its permanent short tag (`#b`, `#c2`).
Configuration may pin `"tags": {"b": "#claude/ab-dvp-b@parf.us"}`. An
unpinned sender gets the first letter in its name, followed by a number when
taken (`b`, `b1`, `b2`). All pinned tags are reserved before automatic assignment.
Tags are stored in KV, never expire and cannot be reassigned to another session;
a conflicting later pin is refused. A removed pin does not free its tag.

Configuration `"users": {"382156160": "parf"}` lists allowed Telegram user
IDs and their bus-user labels. A listed user's Telegram Reply goes to the
sender of the posted message, preserving its bus topic and tag and setting
`re` to that original bus message. `#b text` selects the tagged session;
`#full/session@host text` selects that bus name. Otherwise matching subscriptions
run first, followed by optional Configuration `"route": "#main/session@host"`.
With no match and no route, the bot replies that it did not understand where
to send the text and lists its known tags. A Reply whose mapping has expired
instead answers `that thread expired; use #tag`.

Routed messages come from this Telegram agent's own bus identity, with
`[parf (TG)]` in the body; the label grants no user authority. Each carries a
reply route: when the selected session answers, its text is posted in the
writer's Telegram chat as a Reply to their message. Both directions' mappings
are retained in the agent's KV for seven days and survive process restarts.
Only the mapped session with matching topic and tag may answer; an explicit
`re` must also match its stored bus ID. MCP answers without `re` use that
session/topic/tag correlation.
Other replies are dropped and logged. The agent holds only its own bus token.
The TypeScript client's opt-in [reply callback](../../clients/typescript/README.md#typescript-client)
receives these answers without a competing inbox reader.

Unlisted Telegram users receive `we do not know you` at most once per sender
per hour (the limit survives restart). Expired notice entries are pruned on
every routing-state write; none of their message
reaches the bus. Each refusal logs the user ID, @username, display name, chat
ID/title, message ID and reason, with at most the first 40 text characters.
The last 20 refused senders (ID, username, name, chat, last seen and count) are
kept in KV. `refused` (or JSON `{"method":"refused"}`) lists them, but only
for a delivered Owner or Maintainer role on this record; other callers are
refused. Add an employee's ID to Configuration `users` to admit them.

## Per-chat routing

Optional Configuration `chats` maps an alias or decimal chat ID to settings:
`{"chats":{"#friends":{"route":"#friends/session@host","tags":false,"anyone":true}}}`.
An alias must already exist in `aliases`; duplicate entries resolving to the
same chat are refused, as are unknown settings or non-boolean switches. Changed
aliases and settings take effect on the next incoming message.

- `route` overrides the global route for that chat, after Reply, tag and subscription matching.
- `tags:false` makes incoming `#word` and full bus names ordinary text in that chat. A Telegram Reply still routes to its mapped session; outgoing session posts still carry their short tag.
- `anyone:true` admits unlisted participants in that chat without an unknown-user notice. They are labelled `Telegram user <id>`, never a mapped bus person, and may reach only that chat's route or Reply to a session post in that same chat. They cannot select session tags/full names, use subscriptions or fall back to the global route. With no chat route they receive a not-understood answer. Other chats retain the listed-user rule.

A chat without an entry follows the global rules. For a Telegram channel,
make the bot a channel admin: it receives `channel_post` announcements and
routes them by the same order, including an explicit session tag unless
`tags:false`. A configured channel route admits its admin-authored posts even
when Telegram does not expose an individual sender; those are labelled
`channel <chat id>`. Sessions can post to a channel through its alias or ID;
known bot publications are not routed back into the bus. Readers cannot
reply inside the channel. Its linked discussion group is an ordinary group
with its own `chats` entry, and Telegram Replies there route normally.

Group routing sees only what Telegram delivers. With BotFather privacy mode
on, that is bot mentions, commands and Replies to the bot's posts, not ordinary
conversation between other people. Turn privacy mode off only when the bot
should receive every group message. No per-chat settings are enabled merely
by installing this feature.

## The pinned HTTP policy

Every Telegram call runs inside the caller's deadline: a 429 waits
`parameters.retry_after` (at most 30 s) and retries the write once if the
wait and the retry still fit, a write is never retried otherwise, and a
read retries once on 5xx. A 409 — another poller or a webhook on the
token — is logged with its description and named by `whoami`.
