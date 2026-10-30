# Messaging agents

📌 **TL;DR:**
- **Problem:** your team talks in Discord, Google Chat, Teams, WhatsApp or Messenger, and your AI assistants cannot reach those chats or answer in them.
- **Outcome:** one helper per provider, shaped like the [Slack](../../../docs/agents/slack.md#what-it-does) and [Telegram](../../../docs/agents/telegram.md#what-it-does) helpers: your assistant posts and reads, people reply to the right session, matching messages are forwarded.
- **Status:** plans for the owner's approval, no code ([B.27](../TODO.md#bundled-agents)). Each plan proposes methods and Configuration keys; nothing is built or approved until the owner says.

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
| B.27a | Discord | [discord.md](discord.md) |
| B.27b | Google Chat | planned next |
| B.27c | Microsoft Teams | planned next |
| B.27d | WhatsApp | planned next |
| B.27e | Facebook Messenger | planned next |

## How each plan is written

- **Why and outcome first**, then the proposal, then provider facts with a source.
- **A fact not confirmed is marked "unconfirmed"** and names what would confirm it; no sign-up was made and nothing was sent live.
- **Owner choices are drafted as questions** from Q241; they are filed in the questions file only after the team-1 RT17 questions land.
