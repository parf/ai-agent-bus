# Microsoft Teams agent (B.27c, deferred)

📌 **TL;DR:**
- **Problem:** your company lives in Microsoft Teams and your AI assistants cannot post there, read it, or be answered from it.
- **Outcome:** `#teams/<account>@<realm>` puts one Teams bot on the bus like the Slack helper: your assistant posts, people message or mention the bot, and the reply reaches the right session.
- **How:** a bot registered in Azure and installed in Teams; Teams sends it each message as a web request, so unlike Discord it needs a public HTTPS address.
- **Hard part:** that public address, and Microsoft's SDK churn; both are owner choices below.
- **Status:** deferred out of R1 ([why](../R1.0-Release/DECISIONS.md#recorded-decisions), RD174): R1 exposes no public HTTPS address and Teams needs one. Plan kept for when the owner decides exposure. Facts from Microsoft's Teams docs fetched 2026-10-30; unconfirmed ones are marked.

## What you get

- **Post:** an assistant tells a channel, group chat or person what it did, or asks a question.
- **Reply routing:** message the bot, @mention it, or reply in a thread, and that assistant's session gets it.
- **Forwarding:** `sub` sends matching messages to a session; every channel message only if the team's owner consents (below), else only @mentions and direct messages.
- **Safe keys:** the bot's app secret stays on the server in the record's Secret.

## One way to start (once approved)

- In Azure, register the bot and note its app id and secret; in Teams, upload the app package to a team or chat.
- Register `#teams/work`, set the app id and secret as its Secret, give a session access, call `post`.
- Expose the agent's HTTPS address to Azure (not possible in R1, see the status line).

## Proposed methods (for approval)

| Method | Text form | Teams | Differs from Slack/Telegram |
|---|---|---|---|
| `post` | `post [<conversation>] <text…>` | send activity | a conversation can only be posted to after the bot has been messaged in or added to it (Teams gives the bot a conversation reference then); JSON `reply_to` replies in the thread |
| `read` | `read [<conversation>] [N]` | none | answers the last N messages the bot received there (a ring, as Telegram); Graph history needs extra approval, see Q251 |
| `conversations` | `conversations [filter]` | none | the aliases and conversations the bot has seen: team, channel, chat, type |
| `edit`, `delete` | as Telegram | update, delete activity | the bot's own messages |
| `whoami` | `whoami` | none | the bot's app id and tenant |
| `sub …`, `help` | as Slack | | the channel is a `<conversation>` |

- **Left out:** adaptive cards, dialogs, meetings, files, proactive installs, search.
- **`<conversation>`** is an alias or the id the bot saw.

## Proposed Configuration keys (for approval)

| Key | Says |
|---|---|
| `tenants` | the Microsoft tenant ids the bot answers; any other is refused |
| `users` | Entra user id to bus person: who may pick sessions by tag |
| `tags`, `route` | as Slack |
| `conversations` | per conversation: an alias, `route`, `tags`, `anyone` |
| `default` | where a `post` with none goes; none declared means refused, naming the key |
| `listen` | the address the agent listens on |

- **Secret keys:** `TEAMS_APP_ID`, `TEAMS_APP_SECRET` (or a certificate), `TEAMS_TENANT_ID`; test-only `TEAMS_API_URL`.

## How it receives

- **Web requests from Azure:** the bot's messaging endpoint is an HTTPS URL; each activity arrives as a POST that must be authenticated against Microsoft's signing keys before anything is read.
- **Default scope:** in channels and group chats the bot receives only messages that @mention it; direct messages always.
- **All messages:** declaring `ChannelMessage.Read.Group` and `ChatMessage.Read.Chat` in the app manifest, plus the team or chat owner's consent at install, delivers every user message; messages written by other bots are never delivered.
- **Not forwarded:** the bot's own posts.

## Provider facts that matter

| Fact | Source | Status |
|---|---|---|
| In channels and group chats a bot receives only @mentions unless it declares the two RSC permissions, and the owner consents at install | [Receive all messages](https://learn.microsoft.com/en-us/microsoftteams/platform/agents-in-teams/enable-receive-all-chat-messages) | confirmed |
| Messages from other bots, including Workflows, are not delivered even with RSC | same | confirmed |
| RSC bot access may not be used to extract large amounts of customer data; Store approval needs the description to say how message data is used | same and search summary | confirmed from the page and a search summary |
| A bot works in personal chat, group chat or channel | [Bots overview](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/what-are-bots) | confirmed |
| The Bot Framework SDK is no longer maintained or serviced from 31 December 2025; its replacements are the Microsoft 365 Agents SDK and the Teams SDK (JavaScript and C# general, Python preview) | search summary of the SDK repository notice; the overview page | partly confirmed; read the repository notice and migration guide |
| The messaging endpoint must be a public HTTPS URL; the bot must be single-tenant | not found | unconfirmed; confirm in Azure Bot Service docs |
| An outbound-only receive path (like Discord's gateway) | not found | unconfirmed; none was found |
| Azure bot registration and hosting cost; licences | not read | unconfirmed |
| Reading history needs Graph application permissions that Microsoft may restrict | not read | unconfirmed |

## Test plan

- **No live Microsoft call, no sign-up.** A fixture stands in for the Bot Connector and signing keys, and posts canned activities to the agent's listener.
- **Authentication:** an activity with a bad or missing token, the wrong audience or the wrong tenant is refused before any text is read.
- **Mutants:** token check skipped, tenant not enforced, a message of another bot accepted, the secret read from the environment, the bot's own post forwarded; each fails an assertion.

## Decided

- **SDK** (Q249): the Teams SDK or the Microsoft 365 Agents SDK for TypeScript, chosen at build time by weight and licence; never a hand-written connector. #ab-dir, 2026-10-30.
- **@mentions and direct messages first** (Q251): the all-messages consent is asked per team later. #ab-dir, 2026-10-30.
- **No public HTTPS exposure in R1** (Q250): this plan waits. #ab-ceo, 2026-10-30.

## Open

- **Q252** Which tenant and Azure subscription host the bot, and who pays? See [future questions](QUESTIONS.md#open-questions).
