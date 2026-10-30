# Slack: your AI in your team's chat

📌 **TL;DR:**
- **Problem:** alerts land in Slack while your AI works elsewhere, so someone copies messages back and forth by hand.
- **Outcome:** your AI gets a seat in Slack: it posts and reads like a team member, receives the alerts it cares about, and people answer it in a thread.
- **Start:** ask it what it can do: `agent-bus help '#slack@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔁 **Messages are copied between Slack and the AI by hand.** Now the AI posts and reads in Slack itself.
- 🚨 **The AI misses the alerts that matter.** Now matching channel messages are forwarded to its session.
- 🙋 **People cannot answer the AI where they work.** Now a reply in the AI's thread, or a DM to the bot, reaches the right session.
- 🔑 **Slack passwords end up in the AI's hands.** Now both tokens stay on the server; the AI never sees them.

## What it does

- ✅ **Post and reply:** the AI sends a message to a channel or into a thread and gets the message's link back.
- ✅ **Read a channel or a thread:** recent messages, oldest first.
- ✅ **Forward alerts:** matching messages from a channel go to an AI session, optionally from chosen authors only.
- ✅ **Talk to a session from Slack:** a DM, a mention of the bot, or a reply in a session's thread reaches that AI session; the answer comes back in the same thread.
- ✅ **Look up people and channels:** names, ids and channel details.
- ✅ **Control:** only listed people pick sessions with tags; one channel can be opened to everyone for that channel's session only.
- 🚫 **Not for:** messages on your own phone outside work chat (use [Telegram](telegram.md)), a plain text message (use [SMS](sms.md)), or content that must not pass through Slack.

The abilities, last:

| Ability | In plain words |
|---|---|
| `post` | Send a message; answers with its link |
| `read` | Show recent channel messages |
| `sub add`, `sub list`, `sub delete` | Forward matching channel messages to an AI session |
| `users`, `user` | Look up Slack people and their ids |
| `refused` | Show who the bot refused |
| `whoami` | Name the workspace |
| Edits, reactions, channel details | Change or react to messages, inspect channels |

- The routing is shared with Telegram ([reply routing](telegram.md#reply-routing)).
- How its subscriptions differ from the shared rules is in [what each helper does today](subscriptions.md#what-each-helper-does-today).

## You'll need

- A working [person's runner](README.md#runner-helpers).
- Permission to create and install a Slack app in the workspace.
- Its bot token and app token, both passwords.
- Permission to register and manage this agent on the bus.

## Try this first

- Ask it what it can do (read-only):

```sh
agent-bus help '#slack@node.example'
```

- It lists every call and setting; see [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

<details>
<summary>See it on a real node (ran today)</summary>

```sh
agent-bus call '#slack@parf.us' --wait 15s whoami | jq -r .body
```

```text
realmologs on Realmo (https://…)
```

- It names the bot and its workspace, which shows the tokens work. The workspace address is shortened here.
- The output is from the development node on 2026-10-30; nothing was posted.

</details>

## Use examples

| I want to… | Command |
|---|---|
| Tell the team a deploy finished | `agent-bus call '#slack@node.example' 'post #dev Deploy finished; checks passed.'` |
| Read the last 10 alerts | `agent-bus call '#slack@node.example' 'read #alerts 10'` |
| Forward errors to my session | `agent-bus call '#slack@node.example' 'sub add #alerts #claude/main@node.example /error/i'` |
| Answer my AI | Reply in its Slack thread, or DM the bot `#b <your answer>` |
| See who was refused | `agent-bus call '#slack@node.example' refused` |

### 3. Forward only useful errors

- The author needs send permission to both this helper and the destination session.
- The answer gives an id; `sub list` shows it and `sub delete <id>` removes it.

<details>
<summary>Rules for these calls, and more use cases</summary>

- `read` also accepts JSON, `{"method":"read","channel":"#alerts","limit":10}`, and then answers in JSON.
- To select authors, put `from=@ann,U012AB3CDE4` before the filter; a bot is named by its bot id or @name. Names are checked before saving.
- An AI's posted question ends with its tag, such as `#b`. Your reply in that thread reaches the session only if your user id is listed; the answer comes back into the same thread.
- `refused` is for the owner and maintainers only.

| Use case | What you do | What you get |
|---|---|---|
| Deploy report | AI posts in `#dev` | The team sees it at once |
| Catch up on alerts | Ask for the last 10 messages in `#alerts` | A short list to act on |
| Errors to an AI | Forward `#alerts` messages matching "error" | The AI session sees only errors |
| Ask the team | AI posts a question; someone replies in the thread | The AI continues with the answer |
| Direct request | DM the bot `#b please check the logs` | That AI session does it |

</details>

## Installing it

- An administrator does this once: create a Slack app, store its two tokens on the bus, and install the helper on the runner.

### Create the Slack app

<details>
<summary>Step by step in Slack</summary>

Following [Slack's Socket Mode guide](https://docs.slack.dev/apis/events-api/using-socket-mode/):

1. Create an app for your workspace at [Slack's app page](https://api.slack.com/apps).
2. **OAuth & Permissions**, bot scopes: `chat:write`, `channels:history`, `groups:history`, `im:history`, `mpim:history`, `channels:read`, `groups:read`, `users:read`, `users:read.email`, `reactions:write`, `channels:join`. Add `chat:write.customize` only for [custom post names or icons](#post-appearance).
3. Turn on **Socket Mode**. Under **Basic Information → App-Level Tokens** create a token with `connections:write`: the app token, `xapp-…`.
4. **Event Subscriptions**, bot events: `message.channels`, `message.groups`, `message.im`, `message.mpim`.
5. Install to the workspace and copy the **Bot User OAuth Token**, `xoxb-…`. Reinstall after adding scopes if Slack asks.
6. Invite the bot to each channel it needs; private channels always need it.

</details>

### Connect it to the bus

<details>
<summary>Step by step on the server (runner account <code>ann-ab-runner</code>)</summary>

Paths follow [runner helpers](README.md#runner-helpers).

1. Register it and store both tokens in **Secret** without echoing them:

   ```sh
   agent-bus register '#slack@node.example' --kind agent --allow '@owner'
   read -rsp 'Slack bot token: ' BOT_TOKEN; printf '\n'
   read -rsp 'Slack app token: ' APP_TOKEN; printf '\n'
   printf 'SLACK_BOT_TOKEN=%s\nSLACK_APP_TOKEN=%s\n' "$BOT_TOKEN" "$APP_TOKEN" |
     agent-bus secret '#slack@node.example' -
   unset BOT_TOKEN APP_TOKEN
   ```

2. Package it:

   ```sh
   sudo src/agents/slack/package.sh /var/lib/agent-bus/agent.d/slack
   ```

3. Give the runner this agent's own bus credential (not a Slack token):

   ```sh
   agent-bus token '#slack@node.example' |
     sudo -u ann-ab-runner sh -c 'umask 077; cat > /var/lib/agent-bus/ann-ab-runner/tokens/%23slack@node.example'
   ```

4. Reload and check; `whoami` names the workspace.

   ```sh
   agent-bus call '#runner/ann@node.example' reload
   agent-bus call '#slack@node.example' whoami
   ```

</details>

## Configuration

- Posting, reading and forwarding need no Configuration.
- To let people talk to AI sessions from Slack, list them and choose which session gets their messages.

<details>
<summary>Keys and a full example</summary>

Find Slack user ids with `users` or `user @ann`. Remove the comments before saving in the web panel's **Configuration**:

```jsonc
{
  "users": { "U012AB3CDE4": "ann" }, // who may route: Slack user id → bus person
  "tags": { "b": "#claude/main@node.example" }, // short tag → session
  "route": "#claude/main@node.example", // default session for addressed messages
  "channels": {
    "#dev": {
      "route": "#claude/dev@node.example", // this channel's session
      "tags": false, // incoming #words stay ordinary text
      "anyone": true // unlisted members limited to this channel's session
    }
  }
}
```

```sh
agent-bus config '#slack@node.example' '{"users":{"U012AB3CDE4":"ann"},"route":"#claude/main@node.example"}'
```

- Routing starts only once `users`, `tags`, `route` or `channels` is set. Each session must exist and allow this agent.

</details>

## Secret keys

- Both tokens live in the bus **Secret**, never in Configuration.

| Key | Meaning |
|---|---|
| `SLACK_BOT_TOKEN` | Required: post, read and look up |
| `SLACK_APP_TOKEN` | Required: receive live events |
| `SLACK_API_URL` | Optional test-server address; omit in real use |

### Post appearance

- Posts can show a custom name and icon.

<details>
<summary>Technical details: <code>username</code>, <code>icon</code> and host defaults</summary>

- A JSON post may set `username` and `icon` for that message.
- Host defaults go in the runner's non-secret `runner/slack/env` ([where settings belong](../runner/configuration.md#where-settings-belong)), then reload:

```sh
SLACK_USERNAME="agent-bus@{host}"
SLACK_ICON=":robot_face:"
```

- `{host}` is the short host name; an empty name uses the bot's own. A call's values win; with no name set the default is `cli@{host}`. Needs `chat:write.customize`.

</details>

## Who may use it

- The bus allow list decides who may call the helper: `agent-bus manage '#slack@node.example' --add-allow 'bob@node.example'`.
- `users` in Configuration decides who may talk to AI sessions from Slack.
- Each author lists and deletes their own subscriptions; direct Owners and Maintainers manage all. To change one, use `sub delete` and `sub add` (there is no `sub edit`).
- The author must be allowed to send to both this helper and the destination, checked at add and before every forward ([may-send for another name](../09-daemon-api.md#checking-permission-to-send)); a refused or uncertain copy is dropped, counted and warned, and the next match stays active. An unavailable permission check sends nothing.
- Only the owner and maintainers may read `refused`.

## Warnings and alerts

Besides the [entries every runner agent gets](README.md#warnings-and-alerts), `#slack` reports these.

| Level | When | What to do |
|---|---|---|
| alert | `SLACK_BOT_TOKEN` or `SLACK_APP_TOKEN` is missing, or Slack rejects it | Store valid tokens in Secret, then reload the runner |
| alert | The live-event connection keeps failing: nothing is received | Check the app token, Socket Mode and the app's event subscriptions |
| warning | The live-event connection dropped and reconnected | Nothing; if it repeats, check the network |
| warning | Events were dropped because the forward queue was full | Narrow subscriptions or routes |
| warning | A routed message was refused, or failed after three tries | Check that the session exists and allows this agent |
| warning | A subscription copy was dropped (author refused, target gone), or a channel name did not resolve | Restore access, fix the target or invite the bot; the next match stays active |

## Troubleshooting

| If you see this | Do this |
|---|---|
| “we do not know you” | Owner adds your Slack user id to `users`; the notice comes at most once an hour |
| “not understood” | Use a known tag, reply in the session's thread, or set a route |
| Missing channel messages | Invite the bot; check scopes and event subscriptions |
| Only some events arrive | Give this agent its own app connection; another client on the same app token may receive the other events |
| `starting: loading …` as its status | It is reading a large workspace's users and channels; wait. Messages you send meanwhile are kept and answered when it is ready |
| `not configured` | Store both tokens in Secret, then reload the runner |
| Repeated send or settings failures | Check the runner journal; transient failures are tried three times, then logged and skipped |

- `agent-bus help '#slack@node.example'` lists every call.

## Technical details

<details>
<summary>How Slack differs from Telegram's routing</summary>

Order, tags, seven-day thread links, `anyone` and `drop` follow [Telegram's reply routing](telegram.md#reply-routing) and [per-chat routing](telegram.md#per-chat-routing), with these differences:

- Only DMs, mentions of the bot and mapped session threads enter routing; ordinary channel chatter gets no reply.
- Subscriptions keep their own filters, independent of `users` and routing.
- `{"handler":"drop"}` on a channel also discards its subscriptions; `post` still works.
- An unresolved channel name is logged and skipped, then looked up again after a minute; other channels keep working.

</details>
