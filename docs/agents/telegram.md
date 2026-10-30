# Telegram: talk to your AI from your phone

📌 **TL;DR:**
- **Problem:** your AI is working on your computer, gets stuck on a question ("deploy now?"), and waits until you are back at your desk.
- **Outcome:** it sends you the question on Telegram; you tap **Reply** and your answer goes back to the same AI conversation.
- **Start:** ask it what it can do: `agent-bus help '#telegram/ann@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- ⏳ **The AI waits for you at your desk.** Now it asks on your phone and carries on when you answer.
- 📣 **Alerts come from many services.** Now several of them can send into one Telegram chat.
- 👥 **A group wants to talk to an AI.** Now a group chat can reach one chosen AI session.
- 🔑 **The bot's password would be in the AI's hands.** Now the token stays on the server; the AI never sees it.

## What it does

- ✅ **Messages to your phone:** the AI posts an update or asks a question.
- ✅ **Answers back to the right conversation:** replying to its message, or starting yours with its tag such as `#b`, reaches that session.
- ✅ **Read a chat:** the recent messages the bot has seen there.
- ✅ **Forward alerts:** matching group messages go to an AI session.
- ✅ **Quiet a noisy chat:** tell the bot to ignore a chat and nothing from it reaches the AI.
- ✅ **Control:** only people you list may pick sessions; an open group can be limited to one session.
- 🚫 **Not for:** teams that live in Slack (use [Slack](slack.md)), a plain text message to someone without Telegram (use [SMS](sms.md)), or content that must not pass through Telegram's servers.

The abilities, last:

| Ability | In plain words |
|---|---|
| `post` | Send a message (to the default chat if none is named) |
| `read` | Show recent messages the bot has seen in a chat |
| `sub add`, `sub list`, `sub delete` | Forward matching chat messages to an AI session |
| `refused`, `channels` | Show who the bot refused and which chats it has seen |
| `whoami` | Name the bot |
| Edits, reactions, deletion | Change or react to messages |

- Slack uses the same router; this page owns the [shared routing rules](#reply-routing).
- How its subscriptions differ from the shared rules is in [what each helper does today](subscriptions.md#what-each-helper-does-today), the same as Slack.

## You'll need

- A [person's runner](README.md#runner-helpers) and permission to manage it.
- A Telegram bot made with BotFather, and its token (the bot's password).
- An AI session on the bus that may receive your replies.

## Try this first

- Ask it what it can do (read-only):

```sh
agent-bus help '#telegram/ann@node.example'
```

- It lists every call and setting; see [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

<details>
<summary>See it on a real node (ran today)</summary>

```sh
agent-bus call '#telegram/parf@parf.us' --wait 15s whoami | jq -r .body
```

```text
@parf_claude_bot (id …)
```

- It names the bot, which shows the token works. The numeric id is shortened here.
- The output is from the development node on 2026-10-30; nothing was sent.

</details>

## Use examples

| I want to… | Command |
|---|---|
| Get a message on my phone | `agent-bus call '#telegram/ann@node.example' 'post Deploy finished; checks passed.'` |
| Answer my AI | In Telegram, **Reply** to its message, or send `#b <your answer>` |
| See the last five messages in a group | `agent-bus call '#telegram/ann@node.example' 'read #dev 5'` |
| Forward group errors to a session | `agent-bus call '#telegram/ann@node.example' 'sub add #dev #claude/main@node.example /error/i'` |
| List or remove forwards | `… 'sub list'`, `… 'sub delete <id>'` |

<details>
<summary>Rules for these calls, and more use cases</summary>

- `read` also accepts JSON: `{"method":"read","channel":"#dev","limit":5}`.
- Each author lists and deletes their own subscriptions; direct Owners and Maintainers manage all. To change one, use `sub delete` and `sub add` (there is no `sub edit`). Matching messages are forwarded.
- The author must be allowed to send to both this helper and the destination, checked at add and before every forward ([may-send for another name](../09-daemon-api.md#checking-permission-to-send)); a refused or uncertain copy is dropped, counted and warned, and the next match stays active. An unavailable permission check sends nothing.
- To silence a chat, set its `chats` entry to `{"handler":"drop"}` ([per-chat routing](#per-chat-routing)).

| Use case | What you do | What you get |
|---|---|---|
| Deploy report | AI posts when a deploy ends | A message on your phone |
| "Should I proceed?" | AI asks; you tap Reply | The AI continues with your answer |
| Quick instruction | Send `#b please check the logs` | That AI session does it and answers |
| Group alerts | Forward group messages matching "error" | The AI session sees only the errors |
| Catch up on a group | Ask for the last five messages | What the bot saw there |
| Quiet a noisy chat | Tell the bot to ignore that chat | Nothing from it reaches the AI |

</details>

## Installing it

- An administrator does this once: create a bot in Telegram, store its token on the bus, install the helper on the runner, then list the people allowed to use it.

### Create the bot

<details>
<summary>Step by step in Telegram</summary>

1. Open the verified **@BotFather**, send `/newbot` and answer its questions. You receive the bot token.
2. Open the bot's direct chat and press **Start**, so it may message you.
3. For a group: add the bot. With BotFather `/setprivacy` on, it sees only some group messages; turn it off to read ordinary conversation. Bot admins see everything ([Telegram privacy](https://core.telegram.org/bots/features#privacy-mode)).
4. For a channel: make the bot a channel administrator allowed to post. A linked discussion group is a separate group with its own settings.

</details>

### Connect it to the bus

<details>
<summary>Step by step on the server (runner account <code>ann-ab-runner</code>)</summary>

Paths follow [runner helpers](README.md#runner-helpers).

1. Register it and store the token in **Secret** without echoing it:

   ```sh
   agent-bus register '#telegram/ann@node.example' --kind agent --allow '@owner'
   read -rsp 'Telegram bot token: ' BOT_TOKEN; printf '\n'
   printf 'TELEGRAM_BOT_TOKEN=%s\n' "$BOT_TOKEN" | agent-bus secret '#telegram/ann@node.example' -
   unset BOT_TOKEN
   ```

2. Package it and add an [inventory](../runner/configuration.md#inventory) entry to `/var/lib/agent-bus/ann-ab-runner/agents.json`, keeping other entries:

   ```sh
   sudo src/agents/telegram/package.sh /var/lib/agent-bus/agent.d/telegram
   ```

   ```json
   "telegram": { "name": "#telegram/ann@node.example", "enabled": true }
   ```

3. Give the runner this agent's own bus credential (not the bot token):

   ```sh
   agent-bus token '#telegram/ann@node.example' |
     sudo -u ann-ab-runner sh -c 'umask 077; cat > /var/lib/agent-bus/ann-ab-runner/tokens/%23telegram%2Fann@node.example'
   ```

4. Reload and check; `whoami` names your bot. “not configured” means the Secret is missing: fix it and reload.

   ```sh
   agent-bus call '#runner/ann@node.example' reload
   agent-bus call '#telegram/ann@node.example' whoami
   ```

5. Message the bot. It answers “we do not know you”. As owner, `refused` and `channels` show the user and chat ids it has seen; put them in Configuration.

</details>

## Configuration

- Configuration says who you are on Telegram, which chats have names, and which AI session gets incoming messages.
- Changes apply on the next message.

<details>
<summary>Keys and a full example</summary>

Chat and user ids are decimal **strings**; a group or channel id is often negative. Remove the comments before saving in the web panel's **Configuration**:

```jsonc
{
  "default": "@ann", // outgoing chat for a post without destination
  "aliases": { "@ann": "123456789", "#dev": "-1001234567890" }, // names for chats
  "users": { "123456789": "ann" }, // who may route: Telegram user id → bus person
  "tags": { "b": "#claude/main@node.example" }, // short tag → session
  "route": "#claude/main@node.example", // incoming default session (not `default`)
  "chats": { // see per-chat routing below
    "#dev": {
      "route": "#claude/dev@node.example", // this chat's session
      "tags": false, // incoming #words stay ordinary text
      "anyone": true // unlisted members limited to this chat's session
    }
  }
}
```

```sh
agent-bus config '#telegram/ann@node.example' '{"default":"@ann","aliases":{"@ann":"123456789"},"users":{"123456789":"ann"},"route":"#claude/main@node.example"}'
```

- Each session must exist and allow this agent.

</details>

### Reply routing

- Telegram and Slack share this router.
- A message goes to the session you replied to, else the one whose tag you used, else the chat's or global default.

<details>
<summary>Technical details: routing order, tags and limits</summary>

An incoming message goes to the first match:

1. **Reply** to a session's post: back to that session.
2. A leading **tag** such as `#b`: that session.
3. Matching **subscriptions**.
4. The chat's **route**, then the global `route`.

- No match: the bot says “not understood” and lists known tags.
- Only the expected session's answer returns, as a reply in the same place.
- A tag stays assigned once used: removing a pin does not free it, and moving it to another session is refused.
- Reply links last seven days and survive restarts.
- The listed person's name is shown; their bus permissions are never given to the bot or the receiving session.
- Answers split into at most eight messages of 4096 UTF-16 units; longer ones end with “… (truncated)”. A posting failure is reported to the session.
- A stored malformed route is logged and skipped; a delivered message stays successful if saving its reply route fails.
- Handling failures are tried three times, then logged and skipped.
- The “we do not know you” notice is sent at most once per hour.

</details>

### Per-chat routing

- Each chat can have its own AI session, can be opened to everyone for that session only, or can be ignored entirely.

<details>
<summary>Technical details: chat settings and channels</summary>

A `chats` entry, keyed by alias or chat id, overrides the global route for that chat.

| Setting | Effect |
|---|---|
| `route` | This chat's session |
| `tags: false` | Incoming tag words stay text; Replies still work |
| `anyone: true` | Unlisted people may use only this chat's route, or Reply to a session post in this chat; never other sessions or the global route. Without a local route they get “not understood” |
| `handler: "drop"` | Discard all incoming messages: no routing, replies, refusal records or seen history; `post` still works. The only handler; cannot combine with `route`, `anyone` or `tags` |

- Channels: an administrator's posts stay confined to that channel's route and its existing reply links, even when Telegram names no sender.
- A session may post via the channel alias; its own posts are not sent back to the bus.
- Readers reply in the linked discussion group, configured separately.

</details>

## Secret keys

- The bot token lives in the bus **Secret**, never in Configuration.

| Key | Meaning |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Required: the bot's password |
| `TELEGRAM_API_URL` | Optional test-server address; omit in real use |

## Who may use it

- The bus allow list decides who may call the helper: `agent-bus manage '#telegram/ann@node.example' --add-allow 'bob@node.example'`.
- The `users` list decides who may talk to AI sessions from Telegram.
- Only the owner and maintainers may use `refused`. Each author manages their own subscriptions; direct Owners and Maintainers manage all, and destination management alone grants no subscription rights.

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#telegram` reports these.

| Level | When | What to do |
|---|---|---|
| alert | `TELEGRAM_BOT_TOKEN` is missing, or Telegram rejects it | Store the token in Secret, then reload the runner |
| alert | Another program receives for this bot (`409`): nothing arrives here | Stop the other receiver or webhook |
| alert | Configuration is malformed at start, so it does not run | Fix the key the entry names |
| warning | A receive poll failed and a later one recovered | Nothing; if it repeats, check the network |
| warning | A routed message was refused, or failed after three tries | Check that the session exists and allows this agent |
| warning | A subscription copy was dropped (author refused, target gone) | Restore access or fix the target; the next match stays active |

## Troubleshooting

| If you see this | Do this |
|---|---|
| `409` or “another getUpdates request” | Stop the other program using this bot; only one receiver may |
| “we do not know you” | Owner runs `refused`, adds the user id to `users` |
| “not understood” | Reply to a session's post, use a known tag, or set a route |
| “that thread expired; use #tag” | Start a new message with the session's tag |
| Bot sees no group messages | Check membership and BotFather privacy |
| `read` is empty | Send a new message where the bot can see it; older history is not fetched |
| Can read a channel but not post | Give it channel-admin posting permission |
| Repeated handling failures | Administrator checks the runner journal |

- `agent-bus help '#telegram/ann@node.example'` lists every call.

## Technical details

<details>
<summary>Reading history</summary>

- `read` returns only messages the bot has seen, oldest first; it does not search Telegram's history.
- Each chat keeps at most 200 messages and 64 KiB of UTF-8 JSON.
- A failed history write is logged and retried on a later flush, without holding up other chats.

</details>
