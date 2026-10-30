# Discord agent (B.27a)

📌 **TL;DR:**
- **Problem:** your team is in a Discord server and your AI assistants cannot post there, read it, or be answered from it.
- **Outcome:** `#discord/<account>@<realm>` puts one Discord bot on the bus like the Slack helper: your assistant posts and reads, people reply or mention the bot, and the reply reaches the right session.
- **How:** a bot account kept connected to Discord's live gateway by the runner; no public address needed.
- **Status:** proposal for the owner's approval, no code. Facts are from Discord's developer docs, fetched 2026-10-30; unconfirmed ones are marked.

## What you get

- **Post:** an assistant tells a channel what it did, or asks a question.
- **Read:** an assistant catches up on a channel's recent messages (Discord keeps history; Telegram does not).
- **Reply routing:** answer an assistant's post in Discord, mention the bot, or DM it, and that assistant's session gets it.
- **Forwarding:** `sub` sends matching channel messages to a session.
- **Safe keys:** the bot token stays on the server in the record's Secret; the assistant never sees it.

## One way to start (once approved)

- Create a Discord application with a bot, copy its bot token.
- Add the bot to your server with the authorize link (scope `bot`, the permissions below).
- Register `#discord/team`, set the token as its Secret, give a session access, call `post`.

## Proposed methods (for approval)

| Method | Text form | Discord call | Differs from Slack/Telegram |
|---|---|---|---|
| `post` | `post [<channel>] <text…>` | create message | text over 2,000 characters is split, at most eight parts, the last ending "… (truncated)"; JSON `reply_to` is a message reference; answers channel and message id |
| `read` | `read [<channel>] [N]` | get channel messages | N 1 to 100, default 50, oldest first as Slack; needs read-history permission; content is empty for others' messages without the privileged intent below |
| `channels` | `channels [filter]` | guild channels | aliases from Configuration plus channels in the bot's servers |
| `channel` | `channel <channel>` | get channel | id, name, type, topic |
| `users`, `user` | as Slack | guild members | needs the members intent (privileged); proposed left out, see Q243 |
| `react` | `react <channel> <id> <emoji>` | create reaction | emoji URL-encoded |
| `edit`, `delete` | as Telegram | edit and delete message | only the bot's own messages unless it holds manage-messages |
| `thread` | `thread <channel> <id> <name>` | start thread from message | optional; one thread per message |
| `whoami` | `whoami` | current user | the bot's name, id and servers |
| `sub …`, `help` | as Slack | | the channel is a `<channel>` |

- **Left out:** voice, stage and forum channels, file upload, embeds and components, slash commands; `post` sends plain text.
- **`<channel>`** is an alias, `#name` in a configured server, or a numeric channel id.

## Proposed Configuration keys (for approval)

| Key | Says |
|---|---|
| `guilds` | the server ids the bot may act in; anything else is ignored and refused |
| `users` | Discord user id to bus person: who may pick sessions by tag |
| `tags` | short tag to session, as Slack |
| `route` | default session for addressed messages |
| `channels` | per channel: `route`, `tags`, `anyone`, as Slack, and an alias for the id |
| `default` | the channel a `post` with none goes to; none declared means it is refused, naming the key |

- **Secret key:** `DISCORD_BOT_TOKEN`; optional `DISCORD_API_URL` and `DISCORD_GATEWAY_URL` for the test fixture only.

## How it receives

- **Gateway connection** (persistent WebSocket): hello, heartbeat, identify, ready; on a lost connection resume with the session id and sequence; heartbeat ACK missing means reconnect.
- **Intents:** guild messages and direct messages (standard), and **message content** (privileged) so the bot can read what people write; without it the bot gets empty content except its own messages, DMs to it and messages that mention it.
- **Privileged intents** are switched on in the Developer Portal; apps in 100 or more servers need verification and approval, and apps with over 10,000 users a review. One team's bot is far below that.
- **Not forwarded:** the bot's own posts and edits.
- **Why not an HTTP endpoint:** interactions need a public address; the gateway needs only outbound access, as Slack's socket mode.

## Provider facts that matter

| Fact | Source | Status |
|---|---|---|
| Bot accounts authenticate with a bot token from the application | [OAuth2](https://docs.discord.com/developers/topics/oauth2) | confirmed |
| A bot is added to a server by the authorize link with `scope=bot` and a permissions integer; the person adding needs the right to add bots | same | confirmed |
| Automating a normal user account ("self-bot") is forbidden and can end the account; only bot accounts are planned | [Discord support](https://support.discord.com/hc/articles/115002192352) | from a search snippet; read the page before relying on exact wording |
| Message content is up to 2,000 characters; embeds up to 10 | [Message](https://docs.discord.com/developers/resources/message) | confirmed |
| Get messages: 1 to 100, default 50, newest first; no read-history permission means none returned | same | confirmed |
| Global limit 50 requests per second plus per-route limits; a 429 carries `retry_after` seconds; 10,000 invalid (401, 403, 429) requests per 10 minutes per address restricts the address | [Rate limits](https://docs.discord.com/developers/topics/rate-limits) | confirmed |
| Gateway: 120 events per connection per minute; identify 1,000 per 24 hours | [Gateway](https://docs.discord.com/developers/topics/gateway) | confirmed |
| Cost of the bot API | none found | unconfirmed; no price appeared in the developer docs read |
| A DM channel is opened by a call for the user id; whether the bot must share a server with the user | not read | unconfirmed; confirm in the channel and user resources before building DMs |
| The exact permissions integer for the authorize link | not computed | unconfirmed; derive from send, read history, add reactions, manage nothing |

## Test plan

- **No live Discord call, no sign-up.** A fixture server answers the REST calls and speaks a minimal gateway (hello, ack, ready, a message, resume).
- **Per method:** the request the fixture saw, the answer, a 429 waiting `retry_after` once, a failure answered `error: …`.
- **Routing:** reply, tag, subscription, route, and "not understood", as the shared router's checks.
- **Mutants:** the token read from the environment, a message of another server acted on, the bot's own post forwarded, content over 2,000 characters sent whole, an invalid-request loop not stopped; each fails an assertion.

## Questions for the owner (drafted, not filed)

- **Q241** Gateway or interactions? Proposed: gateway (outbound only), no public address.
- **Q242** Rely on the privileged message-content intent so the bot reads ordinary channel chatter? Proposed yes, switched on in the portal; the alternative reads only mentions and DMs.
- **Q243** Include `users` and `user` (needs the privileged members intent)? Proposed: leave out; ids come from `whoami` and the reply router.
- **Q244** Who creates and owns the Discord application and bot, and for which server? Needed before any live test; the owner's decision, no sign-up was made.
