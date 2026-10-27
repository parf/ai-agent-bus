# Telegram

📌 **TL;DR:** Get updates from your AI assistants on your phone and answer
without returning to your computer. An administrator connects a Telegram bot.
You choose the people and chats it may connect to bus sessions.

## Why you'd want it

A question about a deploy should not wait until you get back to your desk.
Telegram lets your AI ask you, receive your answer and continue the work.
It can also deliver alerts from several services to one chat.

## What it does

The helper sends Telegram messages and remembers messages the bot received.
It connects replies and short tags such as `#b` to the right AI session.
Only listed people can choose sessions freely; an open group can be limited
to one chosen session.

## You'll need

- A [person's runner](README.md#runner-helpers) and permission to manage it.
- Telegram and a bot you create with BotFather.
- The bot's token, which is its password.
- A bus session that can receive your replies.

## Try this first

See what the installed helper can do:

```sh
agent-bus help '#telegram/parf@parf.us'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do)
for its Configuration and Secret help, JSON output, and script forms
`--help-config` and `--help-secret`.

## Use examples

### Ask your AI

| Say this | What it will call |
|---|---|
| “Post to Telegram that the deploy finished.” | Telegram `post`, using the configured default chat |
| “Ask me on Telegram whether to proceed.” | Telegram `post`; your Reply returns to that AI session |
| “Show the last five messages the bot received in #dev.” | Telegram `read #dev 5` |
| “Find why my colleague cannot contact the bot.” | Telegram `refused`; requires the helper's owner or maintainer |

### 1. Receive an update

```sh
agent-bus call '#telegram/parf@parf.us' 'post Deploy finished; checks passed.'
```

The message arrives in the default chat. The answer names the chat and message
id. No destination word is needed once the default is set.

### 2. Answer an AI question from your phone

Ask your AI to send its question using `post`. In Telegram, choose **Reply**
on that message and type your answer. The same session receives it. Its next
answer appears as a Telegram reply to you. A tag such as `#b` identifies the
session; you can also send `#b please check the logs` without using Reply.

### 3. Read recent group messages

```sh
agent-bus call '#telegram/parf@parf.us' 'read #dev 5'
agent-bus call '#telegram/parf@parf.us' '{"method":"read","channel":"#dev","limit":5}'
```

You get the messages the bot received, oldest first. This is not a search of
Telegram's entire history; the bot must have seen the messages.
Each chat keeps at most 200 messages and 64 KiB of UTF-8 JSON. A failed history
write is logged and retried on a later flush; it does not hold up polling other chats.

### 4. Send group alerts to a session

If you manage the destination session:

```sh
agent-bus call '#telegram/parf@parf.us' 'sub add #dev #claude/main@parf.us /error/i'
agent-bus call '#telegram/parf@parf.us' 'sub list'
```

You get a subscription id, then see it in the list. Matching messages from
listed users go to that session. `sub delete <id>` removes the saved subscription.

## Installing it

### Create the bot

1. In Telegram, open the verified **@BotFather** account. Send `/newbot`.
   Follow its name and username questions. You should receive a bot token.
2. Open your new bot's direct chat and press **Start**. The bot can now send
   you direct messages once it is connected. You should see `/start` in the chat.
3. For a group, add the bot as a member. In BotFather, use `/setprivacy` to
   choose what it receives. Privacy on limits group messages it sees; turn
   it off if it must read ordinary conversation. Bot admins receive all group
   messages. See [Telegram's privacy explanation](https://core.telegram.org/bots/features#privacy-mode).
   BotFather should confirm your privacy choice.
4. For a channel, add the bot as a channel administrator with permission to
   post. It should appear among the channel's administrators. A linked
   discussion group is a separate group and needs its own settings.

### Connect it to the bus

1. Register the agent:

   ```sh
   agent-bus register '#telegram/parf@parf.us' --kind agent --allow '@owner'
   ```

   You should get a successful registration.
2. Store only the bot password in **Secret**:

   ```sh
   read -rsp 'Telegram bot token: ' BOT_TOKEN; printf '\n'
   printf 'TELEGRAM_BOT_TOKEN=%s\n' "$BOT_TOKEN" |
     agent-bus secret '#telegram/parf@parf.us' -
   unset BOT_TOKEN
   ```

   The token is read privately and sent through standard input, not printed
   or placed in a command argument, a file or Configuration. You should see
   a successful Secret update.
3. Build the runner package:

   ```sh
   sudo src/agents/telegram/package.sh /var/lib/agent-bus/agent.d/telegram
   ```

   You should see “packaged the Telegram agent”.
4. In the runner's `/var/lib/agent-bus/parf-ab-runner/agents.json`, add or
   update the `telegram` entry while keeping its other entries:

   ```json
   "telegram": { "name": "#telegram/parf@parf.us", "enabled": true }
   ```

   This is one entry inside the file's outer braces. It chooses the bot's bus name.
5. Give the runner this agent's own bus credential:

   ```sh
   agent-bus token '#telegram/parf@parf.us' |
     sudo -u parf-ab-runner sh -c 'umask 077; cat > /var/lib/agent-bus/parf-ab-runner/tokens/%23telegram%2Fparf@parf.us'
   ```

   Nothing is printed. This file is the bus credential, not the Telegram bot token.
6. Reload and check it:

   ```sh
   agent-bus call '#runner/parf@parf.us' reload
   agent-bus call '#telegram/parf@parf.us' whoami
   ```

   The answer should name your bot. If the runner says “not configured”,
   check Secret and reload again.
7. Send a short message to the bot in Telegram. Before you are listed, it
   answers “we do not know you”. As the owner, run:

   ```sh
   agent-bus call '#telegram/parf@parf.us' refused
   agent-bus call '#telegram/parf@parf.us' channels
   ```

   The first shows your Telegram user id; the second shows chats the bot has
   seen. Use those ids in Configuration below, then send another message.

## Configuration

Use decimal chat and user ids as **strings**. A user id identifies a person;
a group or channel has a different chat id, often negative. This commented
sample shows every routing setting; remove comments before saving it in the
web panel's **Configuration** field:

```jsonc
{
  "default": "@parf", // where a destination-less post goes
  "aliases": { "@parf": "382156160", "#dev": "-1001234567890" },
  "users": { "382156160": "parf" }, // Telegram user id → bus person name
  "tags": { "b": "#claude/main@parf.us" }, // short tag → AI session
  "route": "#claude/main@parf.us", // default session for incoming messages
  "chats": {
    "#dev": {
      "route": "#claude/dev@parf.us", // this chat's session
      "tags": false, // leave incoming #words as ordinary text
      "anyone": true // unlisted members stay limited to this chat
    }
  }
}
```

For example, after checking the ids, save a valid configuration with:

```sh
agent-bus config '#telegram/parf@parf.us' '{"default":"@parf","aliases":{"@parf":"382156160"},"users":{"382156160":"parf"},"route":"#claude/main@parf.us"}'
```

Settings take effect on the next message. The session must exist and allow
this Telegram agent to contact it. Incoming messages choose, in order:
Reply, explicit tag, matching subscriptions, then the chat route or global
route. With no match, the bot says it did not understand and lists known tags.
The outgoing `default` chat and incoming `route` session are different settings.

### Per-chat routing

Set a chat entry to `{"handler":"drop"}` to discard all its incoming messages without routing, replies, refusal records or seen history; `post` still works. `drop` is the only handler and cannot be combined with `route`, `anyone` or `tags`. Other chats are unchanged.

`chats` accepts an alias declared in `aliases` or a chat id. Its route overrides
the global one. `tags:false` leaves incoming tag words alone; Replies still work.
`anyone:true` lets unlisted people use only that chat's route or Reply to a
session post in that same chat. They cannot choose other sessions or use the
global route. An open chat with no local route gets “not understood”.

For a channel, its administrator's posts are confined to that channel's route
and its existing reply links, even
when Telegram does not name an individual sender. A session can post through
the channel alias. Its own posts are not sent back to the bus. Configure the
linked discussion group separately; readers reply there, not inside the channel.

### Reply routing

Routed session answers are split into at most eight messages of 4096 UTF-16 units;
longer answers end with “… (truncated)”. A posting failure is reported to the session.
Stored malformed routes are logged and skipped; a landed message remains successful
if saving its reply route fails.

Tags stay assigned once used. Removing a pin does not free its tag, and changing
an existing tag to another session is refused. Reply links last seven days and
survive restarts. Only the expected session's answer returns to Telegram.
Messages show the listed person's name, but do not give their bus permissions
to the bot or the receiving session.

## Secret keys

| Key | Why it is needed |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Required: Telegram's bot password |
| `TELEGRAM_API_URL` | Optional test-server address; leave it out in real use |

Aliases, people, tags and routes belong in Configuration, never Secret.

## Who may use it

Allow a bus caller with:

```sh
agent-bus manage '#telegram/parf@parf.us' --add-allow 'ann@parf.us'
```

This lets Ann use the bus calls. The separate `users` list controls who may
route incoming Telegram messages. Only the helper's owner and maintainers
may use `refused` or change any subscription; managing a destination also lets
a caller manage its subscriptions.

## Troubleshooting

| If you see this | Do this |
|---|---|
| `409` or “another getUpdates request” | Stop the other plugin or program receiving with this bot; only one receiver should use it |
| “we do not know you” | The owner uses `refused`, then adds the correct user id to `users`; the notice is limited to once per hour |
| “not understood” | Reply to a session's post, use a known tag, or set an incoming route |
| “that thread expired; use #tag” | Start a fresh message with the session's tag |
| Bot sees no group messages | Check membership and BotFather privacy settings |
| `read` is empty | Send a new message where the bot can see it; old unseen history is not downloaded |
| It can read a channel but cannot post | Give it channel-admin posting permission |
| Repeated handling failures | Ask the administrator to check the runner journal; failures retry three times before being skipped |

Use `agent-bus call '#telegram/parf@parf.us' --help` for every supported call,
including reactions, edits, deletion and subscription changes.
