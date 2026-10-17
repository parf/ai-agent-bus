# Slack agent

📌 **TL;DR:** `#slack@<realm>` puts one Slack workspace on the bus: post,
read, look up channels and people, and forward a channel's messages to an
agent, queue or pubsub, filtered by a regex. It is one bun process serving
itself; its design and contract are in
[the plan](../../../Plans/R1.2/slack/README.md#slack-agent).

## The Slack app

Create an app for the workspace, then:

| Setting | Value |
|---|---|
| Socket Mode | on; an app-level token with `connections:write` (the `xapp-…` token) |
| Bot token scopes | `chat:write`, `channels:history`, `groups:history`, `im:history`, `mpim:history`, `channels:read`, `groups:read`, `users:read`, `users:read.email`, `reactions:write`, `channels:join`; `chat:write.customize` for `username` and `icon` |
| Event Subscriptions | on, with the bot events `message.channels`, `message.groups`, `message.im`, `message.mpim` |
| Install | to the workspace; the `xoxb-…` bot token |

The bot must be a member of every channel it reads or forwards: `join` adds
it to a public channel; a private one needs an invite from Slack. A DM is
forwarded only when a subscription names its channel ID.

## The bus

```sh
agent-bus register '#slack@realm' --kind agent
printf 'SLACK_BOT_TOKEN=xoxb-…\nSLACK_APP_TOKEN=xapp-…\n' | agent-bus secret '#slack@realm' -
agent-bus manage '#slack@realm' --add-allow '@owner'     # who may call it
```

The tokens live in the record's bus secret and nowhere else: the agent
never reads them from its environment, re-reads them on every call, and
scrubs them from every answer. `@owner` admits the record's Owner and the
agents that Owner owns. Changing subscriptions needs the Owner or a
Maintainer, calling the agent directly.

## Running it

Until the managed runner starts agents that serve themselves (runner wave
M.9), start it by hand or as a user unit, with the agent's own token:

```sh
AGENT_BUS_NAME='#slack@realm' AGENT_BUS_TOKEN="$(agent-bus-token '#slack@realm')" \
  bun src/agents/slack/slack.ts
```

```ini
# ~/.config/systemd/user/agent-bus-slack.service
[Unit]
Description=agent-bus Slack agent
[Service]
Environment=AGENT_BUS_NAME=#slack@realm
EnvironmentFile=%h/.config/agent-bus/slack.env   # AGENT_BUS_TOKEN=…, mode 600
ExecStart=/usr/bin/bun /usr/local/src/ai-agent-bus/src/agents/slack/slack.ts
Restart=on-failure
[Install]
WantedBy=default.target
```

`SIGTERM` stops it: the socket and its timers end, the forwarding queue
drains for up to 5 s, and the counters are flushed.

## Calling it

```sh
agent-bus call '#slack@realm' 'post #dev deploy done'
agent-bus call '#slack@realm' 'read #dev 20'
agent-bus call '#slack@realm' 'sub add #alerts #oncall@realm /error/i'
agent-bus call '#slack@realm' 'sub list'
agent-bus call '#slack@realm' '{"method":"channels","member_only":true}'
```

`help` lists every method; a JSON call is answered in JSON.

## Tests

`bun test ./src/agents/slack` runs everything against a real daemon (built
here, or `SMOKE_BIN`), the actual agent, and a fake Slack with a Socket Mode
websocket — no network and no real tokens. `src/smoke.sh --slow` runs it as
the `agent_slack` shard.
