# Slack

📌 **TL;DR:** Send team updates, collect alerts and answer an AI session in
Slack. An administrator connects one Slack app. You choose who may use it and
where messages should go.

## Why you'd want it

Keep alerts and the people who can act on them in one place. Your AI can send
an update or follow a conversation without someone copying messages by hand.

## What it does

The helper posts and reads Slack messages. It can forward matching channel
messages to the bus, or connect a direct message or a reply under a Slack
message to an AI session. Slack calls those grouped replies a **thread**.

Only people you list may use session tags. You can let everyone in one
channel talk to that channel's chosen session without giving them wider access.

## You'll need

- A working [person's runner](README.md#runner-helpers).
- Permission to create and install a Slack app in your workspace.
- A bot token and an app token. Each is a password; keep both private.
- Permission to register and manage this agent on the bus.

## Use examples

### Ask your AI

| Say this | What it will call |
|---|---|
| “Post in #dev that the deploy finished.” | Slack `post #dev …` |
| “Show the last 10 messages in #alerts.” | Slack `read #alerts 10` |
| “Forward #alerts errors to my bus session.” | Slack `sub add`; you must manage that destination |
| “Who was refused when they tried to contact the bot?” | Slack `refused`; only this helper's owner or maintainers |

### 1. Tell the team a deploy finished

```sh
agent-bus call '#slack@parf.us' 'post #dev Deploy finished; checks passed.'
```

You get a link to the new Slack message. Open it to check the update.

### 2. Follow an alert

```sh
agent-bus call '#slack@parf.us' 'read #alerts 10'
agent-bus call '#slack@parf.us' '{"method":"read","channel":"#alerts","limit":10}'
```

Both show the recent messages. The second form returns structured JSON data.

### 3. Forward only useful errors

If `#claude/main@parf.us` is a destination you manage:

```sh
agent-bus call '#slack@parf.us' 'sub add #alerts #claude/main@parf.us /error/i'
agent-bus call '#slack@parf.us' 'sub list'
```

The first answer gives the subscription's id. New matching messages go to that
session. Save the id; `sub delete <id>` removes it. To select authors too, use
`from=@ann,U012AB3CDE4` before the filter. A bot can be selected by its bot
id or @name. Names are checked before the subscription is saved.

### 4. Answer your AI in a thread

Ask your AI to post a question with `post #dev …`. Its message ends with a
short name such as `#b`. Reply in that Slack thread. Your listed user id lets
the reply reach that session, and its answer comes back into the same thread.
You can also DM the bot `#b please check the logs`.

## Installing it

### Create the Slack app

1. Open [Slack's app page](https://api.slack.com/apps) and create an app for
   your workspace. You should see its settings page.
2. Under **OAuth & Permissions**, add the bot permissions below. They let the
   app read the chosen conversations and perform its supported actions.

   `chat:write`, `channels:history`, `groups:history`, `im:history`,
   `mpim:history`, `channels:read`, `groups:read`, `users:read`,
   `users:read.email`, `reactions:write`, `channels:join`.
   Add `chat:write.customize` only for custom post names or icons. You should
   see the permissions in the bot scope list.
3. Turn on **Socket Mode**, Slack's live connection for events. Under **Basic
   Information → App-Level Tokens**, create a token with `connections:write`.
   You should receive an app token, normally beginning `xapp-`.
4. Turn on **Event Subscriptions**. Add bot events `message.channels`,
   `message.groups`, `message.im` and `message.mpim`. They should appear in
   the subscribed events list.
5. Install the app to the workspace. Copy its **Bot User OAuth Token**,
   normally beginning `xoxb-`. Reinstall after adding permissions if Slack asks.
   You should see the Bot User OAuth Token on the permissions page.
6. Invite the bot to each channel it needs. You should see it in the channel's
   member list. Private channels always need an invitation.

These steps follow [Slack's Socket Mode guide](https://docs.slack.dev/apis/events-api/using-socket-mode/).

### Connect it to the bus

1. Register its name:

   ```sh
   agent-bus register '#slack@parf.us' --kind agent --allow '@owner'
   ```

   You should get a successful registration.
2. Save the two passwords in **Secret**, not Configuration:

   ```sh
   read -rsp 'Slack bot token: ' BOT_TOKEN; printf '\n'
   read -rsp 'Slack app token: ' APP_TOKEN; printf '\n'
   printf 'SLACK_BOT_TOKEN=%s\nSLACK_APP_TOKEN=%s\n' "$BOT_TOKEN" "$APP_TOKEN" |
     agent-bus secret '#slack@parf.us' -
   unset BOT_TOKEN APP_TOKEN
   ```

   The command confirms the write without printing either token.
3. Build its runner package:

   ```sh
   sudo src/agents/slack/package.sh /var/lib/agent-bus/agent.d/slack
   ```

   You should see “packaged the Slack agent”.
4. Give the runner this agent's bus credential:

   ```sh
   agent-bus-token '#slack@parf.us' |
     sudo -u parf-ab-runner sh -c 'umask 077; cat > /var/lib/agent-bus/parf-ab-runner/tokens/%23slack@parf.us'
   ```

   This writes the helper's bus credential, not a Slack token. It prints nothing.
5. Reload the runner and check the bot:

   ```sh
   agent-bus call '#runner/parf@parf.us' reload
   agent-bus call '#slack@parf.us' whoami
   ```

   The runner should show Slack as running. `whoami` names the workspace.

## Configuration

No settings are needed for ordinary bus calls and subscriptions. Reply routing
starts only when you add `users`, `tags`, `route` or `channels`. Read Slack user
ids with `users` or `user @ann` before filling this in.

This full sample has comments for explanation. Remove the comments before
saving it in the web panel's **Configuration** field:

```jsonc
{
  "users": { "U012AB3CDE4": "parf" }, // Slack person id → bus person name
  "tags": { "b": "#claude/main@parf.us" }, // short name → AI session
  "route": "#claude/main@parf.us", // where an addressed message goes by default
  "channels": {
    "#dev": {
      "route": "#claude/dev@parf.us", // this channel's session
      "tags": false, // treat incoming #words as ordinary text here
      "anyone": true // unlisted members can reach only this channel's session
    }
  }
}
```

For example, this valid command enables a listed person and a default session:

```sh
agent-bus agent-template '#slack@parf.us' '{"users":{"U012AB3CDE4":"parf"},"route":"#claude/main@parf.us"}'
```

Session names must exist and allow the Slack agent to send to them. Replies
in a session thread go there first; otherwise a leading `#tag` selects a
session, followed by the channel route or the global route. Tags stay assigned
once used. Thread links last seven days. Unlisted members admitted by
`anyone:true` can use only that channel's route or its own session threads.

Only DMs, mentions of the bot and mapped session threads enter routing.
Ordinary channel chatter gets no routing reply. Subscriptions keep their own
filters and do not depend on this list of people. With no route, an addressed
message explains “not understood” and lists the known tags.

## Secret keys

| Key | Why it is needed |
|---|---|
| `SLACK_BOT_TOKEN` | Required: post, read and look up Slack information |
| `SLACK_APP_TOKEN` | Required: receive live events |
| `SLACK_API_URL` | Optional test-server address; leave it out in real use |

Passwords never belong in the Configuration sample.

### Post appearance

A JSON post may set `username` and `icon` for that message. For defaults, the
administrator can set these non-secret values in `runner/slack/env` and reload:

```sh
SLACK_USERNAME="agent-bus@{host}"
SLACK_ICON=":robot_face:"
```

`{host}` means the short host name; an empty name uses the bot's normal name.
A call's settings take priority. Without a name setting, the default is
`cli@{host}`. Slack requires `chat:write.customize` for these overrides.

## Who may use it

The bus access list chooses who may call the helper. For example:

```sh
agent-bus manage '#slack@parf.us' --add-allow 'ann@parf.us'
```

That lets Ann use its calls. Managing a subscription also requires permission
to manage its destination. Only this helper's owner and maintainers may read
`refused`; the people listed in Configuration control incoming Slack routing,
not permission to call the helper from the bus.

## Troubleshooting

| If you see this | Do this |
|---|---|
| “we do not know you” | Ask the owner to add your Slack user id to `users`; the notice appears at most once an hour |
| “not understood” | Use the right session tag, reply in its thread, or set a route |
| Missing channel messages | Invite the bot; check scopes and event subscriptions |
| Only some events arrive | Give this agent its own app connection; another client using the same app token may receive the other events |
| `not configured` | Store both tokens in Secret, then reload the runner |
| A send or setting read fails repeatedly | Check the runner journal; routing tries transient failures three times, then logs and moves on |

Use `agent-bus call '#slack@parf.us' --help` for every supported call, including
edits, reactions, channel details and subscription changes.

## Ask the agent about its settings

These calls show what the installed agent accepts. The Secret help shows key
names and instructions, never passwords or saved values.

```sh
agent-bus call '#slack@parf.us' --help
agent-bus call '#slack@parf.us' --help-config
agent-bus call '#slack@parf.us' --help-secret
agent-bus call '#slack@parf.us' --help-json
```

`--help-json` gives programs the whole description as JSON. The other three
answers are plain text.

See [method metadata](../runner/configuration.md#package-help)
for how these answers are generated.
