# Discord

📌 **TL;DR:** Your AI can join the conversations your team already has in Discord. It posts updates, reads recent messages and routes human replies to the right session. One bot can serve one team or several approved servers; its token stays with the helper.

## Why you'd want it

- 💬 Keep assistants and people in the same conversations, without copying messages between tools.
- 🔐 Let teams use a bot without handing its password to an assistant.

## What it does

- 📣 Post plain text, read recent messages, react, edit your bot’s messages, delete messages or start a thread.
- ↩️ Reply to a session’s post, mention the bot or DM it to reach a session; subscriptions forward selected channel messages.

## You'll need

- Your owner creates a Discord bot, enables **Message Content Intent**, invites it with permission to view channels, send messages, read history and add reactions, and registers a `#discord/<account>` helper with its runner token.
- The owner stores `DISCORD_BOT_TOKEN` in that record’s Secret, configures allowed `guilds` and a `default` channel, packages the agent and enables it in the runner. The bot holds an outbound gateway connection; no public endpoint is needed.

## Try this first

- Set `DISCORD_AGENT` to that helper’s full bus name. Ask `whoami` before choosing a channel or posting.

## Use examples

<details>
<summary>A read-only CLI run against the local fixture</summary>

This ran on 2026-10-30 against the packaged agent, a private bus and fake Discord endpoints. The identifiers below belong to that fixture. No live Discord call was made.

Check the bot and its allowed server:

```sh
agent-bus call "$DISCORD_AGENT" --wait 3s whoami | jq '.body | fromjson | .result'
```

```json
{"id":"9","username":"fixture-bot","guilds":[{"id":"1","name":"allowed"}]}
```

</details>

## Installing it

- The runner host needs Bun. The owner copies the package made by `src/agents/discord/package.sh` into `agent.d/discord`, enables its instance in the runner inventory and reloads the runner.
- The instance serves itself with its own record token. Enabling it without a registered record and runner token is refused; registration and credentials belong to the owner.

## Configuration


| Key | Meaning |
|---|---|
| `guilds` | Distinct allowed server ids. Other servers are refused. |
| `users` | Discord user id to bus person, for choosing a session by tag. |
| `tags` | Short permanent tag to bus session. |
| `route` | Default bus destination for addressed messages from listed people. |
| `channels` | Alias, `#name` or numeric id to `{id, route?, tags?, anyone?}`. A numeric key may omit `id`. |
| `default` | Channel for a post or read with no channel supplied. |

The local fixture used this Configuration:

```json
{
  "guilds": ["1"],
  "default": "alerts",
  "users": {"7": "ada@fixture"},
  "channels": {"alerts": {"id": "10"}}
}
```

An alias without `id` is refused at load. Two keys resolving to the same id are refused, naming both. Channel `route`, `tags` and `anyone` have the [shared reply-routing meanings](slack.md#configuration).

## Secret keys

- `DISCORD_BOT_TOKEN` is required in the record’s Secret, in dotenv form. It never comes from the process environment.
- `DISCORD_API_URL` and `DISCORD_GATEWAY_URL` accept only loopback fixture URLs. Leave them absent for Discord’s official endpoints.

## Who may use it


- A reply to a session’s post returns to that session. A listed person may choose a session by its `#tag`; addressed messages with no tag use the configured route. Unlisted people cannot choose sessions. A channel with `anyone: true` accepts them only for its explicit route or a reply to a mapped post.
- Each session gets a short permanent tag on its posts. Reply mappings last seven days. DMs and mentions are addressed messages; ordinary channel chatter is silent unless a subscription matches.
- `sub add`, `sub list` and `sub delete` follow [the shared subscription behaviour](subscriptions.md#the-shared-behaviour). The author must be allowed to send to both this helper and the target. A refused copy is dropped and counted with one warning; the next match stays active. To change a subscription, delete it and add it again.
- The bot’s own messages, edits and repeated gateway events do not make subscription copies.

## Troubleshooting

| Result | What to check |
|---|---|
| Missing `default` | Choose a channel explicitly or configure `default`. |
| Guild refused | Confirm the channel belongs to a configured `guilds` entry. |
| Empty message text | Enable Message Content Intent for the bot. |
| Invalid token, intent or repeated provider refusal | Fix the Secret or Discord permissions, then reload the agent to reconnect and clear its refusal state. |
| No reply route | Check `users`, channel routing and the session’s tag. |

## Technical details


- The bus allow list decides who may call the helper. Discord’s bot permissions decide what it can do there. `guilds` narrows those permissions: a channel in another server is refused, and incoming messages from that server are ignored.
- A channel is a configured alias, a `#name` in an allowed server, or a numeric id. A name must resolve exactly once. A `post` or `read` with no channel needs `default`; the helper never chooses the first channel for you.
- Posts are split at 2,000 characters, at most eight parts. The eighth ends with `… (truncated)` if needed. Posts do not ping users or roles automatically. Editing text is limited to the bot’s own messages; Discord requires manage-messages permission to delete someone else’s message.
- Read returns up to 100 messages, default 50, oldest first. Message Content Intent makes other people’s text available. Voice, stage and forum channels, uploads, embeds, components and slash commands are outside this helper’s methods.

<details>
<summary>Methods, limits and connection details</summary>

| Method | Text form |
|---|---|
| `post` | `post [<channel>] <text…>`; JSON also accepts a `reply_to` message id. |
| `read` | `read [<channel>] [N]` |
| `channels`, `channel` | `channels [filter]`, `channel <channel>` |
| `react` | `react <channel> <id> <emoji>`; Unicode emoji or custom `name:id`. |
| `edit`, `delete` | `edit <channel> <id> <text…>`, `delete <channel> <id>` |
| `thread` | `thread <channel> <id> <name>`; one thread per message. |
| `whoami` | `whoami` |
| Subscriptions | `sub add <channel> <target> [filter]`, `sub list`, `sub delete <id>` |

Every method also accepts JSON with its method name and named arguments. `help`, `help-json`, `help-config` and `help-secret` describe the declaration without showing Secret values.

- The bot uses Discord API v10 and gateway v10, with guild, message, DM and message-content intents. It heartbeats, reconnects and resumes with its session id and sequence.
- A missing heartbeat ACK reconnects; an invalid token or disallowed intent stops reconnecting and says why.

- Provider requests have a 15-second budget. A 429 waits for `retry_after` and retries once if it fits. An uncertain write is not retried.
- A 401 stops requests for that token; a 403 blocks the same request route for ten minutes, and a burst of invalid requests stops further requests. Fix access or the Secret, then reload.

`DISCORD_API_URL` and `DISCORD_GATEWAY_URL` are Secret keys for loopback test fixtures only. Production uses the official endpoints. The process environment never supplies the bot token or provider endpoints.

The runner executes the self-contained Bun package. Its shared subscription engine and inline filter worker are bundled with it; the package reads no source checkout. The local checks cover every method, routing, provider refusals, shutdown and gateway resume against fixtures.

</details>
