# Messaging agents

📌 **TL;DR:**
- **Problem:** your team talks in Discord, Google Chat, Teams, WhatsApp or Messenger, and your AI assistants cannot reach those chats or answer in them.
- **Outcome:** one helper per provider, shaped like the [Slack](../../../docs/agents/slack.md#what-it-does) and [Telegram](../../../docs/agents/telegram.md#what-it-does) helpers: your assistant posts and reads, people reply to the right session, matching messages are forwarded.
- **Status:** Discord moved to [R1.1](../../R1.1/discord.md#discord-agent-b27a): its code stays in the tree, but R1 does not ship it. Google Chat remains planned for R1 ([B.27](../TODO.md#bundled-agents)). Teams, WhatsApp and Messenger need a public HTTPS address, which R1 does not expose, so their plans wait in [R2.0](../../R2.0-Future/README.md#topics).

## The shared shape

- **One instance per account:** `#<provider>/<account>@<realm>`, one bot or account per instance, its own access list.
- **The provider's token is the record's Secret:** never the process environment, never a Configuration key.
- **Methods follow Slack and Telegram where the provider allows:** `post`, `read`, `channels`, `channel`, `users`, `user`, `react`, `edit`, `delete`, `whoami`, `sub …`, `help`; each plan says what differs.
- **Replies are routed to the asking session** by the [shared reply router](../../../docs/agents/telegram.md#reply-routing): reply to a session's post, else a leading `#tag`, else matching subscriptions, else the chat's or global route.
- **`sub` runs on the shared engine** ([unified subscriptions](../subscriptions.md#one-behaviour)): the author's may-send checked at add and before each send; a refused send is dropped with one warning, never paused.
- **Upstream errors are answers:** `error: <why>`; a rate limit waits the provider's stated time once; a write is never retried otherwise.
- **Serves itself:** one bun process kept by the runner, packaged by its own `package.sh`, exiting 78 when unconfigured and naming what is missing.
- **No live provider call in any check:** tests use a fixture server standing in for the provider's API.

## Plans

| Row | Provider | Plan |
|---|---|---|
| B.27a | Discord | moved to R1.1: [discord.md](../../R1.1/discord.md#discord-agent-b27a) |
| B.27b | Google Chat | [google-chat.md](google-chat.md) |
| B.27c | Microsoft Teams | deferred: [teams.md](../../R2.0-Future/teams.md) |
| B.27d | WhatsApp | deferred: [whatsapp.md](../../R2.0-Future/whatsapp.md) |
| B.27e | Facebook Messenger | deferred: [messenger.md](../../R2.0-Future/messenger.md) |

## How each plan is written

- **Why and outcome first**, then the proposal, then provider facts with a source.
- **A fact not confirmed is marked "unconfirmed"** and names what would confirm it; no sign-up was made and nothing was sent live.
- **Choices are recorded as decisions or open questions:** [R1 questions](../QUESTIONS.md#messaging-agents) and [decisions](../DECISIONS.md#recorded-decisions).
