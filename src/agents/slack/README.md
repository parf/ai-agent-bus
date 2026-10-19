# Slack agent

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

📌 **TL;DR:** `#slack@<realm>` puts one Slack workspace on the bus: post,
read, look up channels and people, and forward a channel's messages to an
agent, queue or pubsub, filtered by authors and a regex. It is one bun process serving
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
forwarded by subscriptions only when one names its channel ID; configured
[reply routing](#reply-routing) also handles DMs.

## The bus

```sh
agent-bus register '#slack@realm' --kind agent
printf 'SLACK_BOT_TOKEN=xoxb-…\nSLACK_APP_TOKEN=xapp-…\n' | agent-bus secret '#slack@realm' -
agent-bus manage '#slack@realm' --add-allow '@owner'     # who may call it
```

The tokens live in the record's bus secret and nowhere else: the agent
never reads them from its environment, re-reads them on every call, and
scrubs them from every answer. The bot token may be shared with other
programs: only the agent's own posts are kept from its subscriptions.

How its posts are signed is configuration, in the runner's
`runner/slack/env` (a call's `username` and `icon` still win):

```sh
SLACK_USERNAME="agent-bus@{host}"   # {host}: the short host name; default cli@{host}, as slack-post's
SLACK_ICON=":robot_face:"           # an emoji, or an image URL
```

An empty `SLACK_USERNAME` posts as the bot itself. The same keys in the bus
secret apply when the runner's file has none. `reload` the runner after an
edit; the agent restarts with it. `@owner` admits the record's Owner and the
agents that Owner owns. A subscription is added, edited and deleted by
whoever manages its destination, as the daemon answers it (`GET /manages`);
this agent's own Owner and Maintainers, calling it directly, change any.
Its source channel is part of that too: whoever manages the destination
decides what feeds it, so `sub edit <id> channel=…` is theirs.
`agent-bus call '#slack@realm' --help` says what the caller may do.

Authors can be narrowed with `sub add #alerts #worker@realm from=@ann,B123 /error/i`; see [subscriptions](../../../Plans/R1.2/slack/README.md#subscriptions-authority-errors) for resolution, editing and clearing.

## Running it

The runner keeps it running, as an agent that
[serves itself](../../../docs/runner/configuration.md#agents-that-serve-themselves). Started with no bus secret or a token
missing, it exits 78 (not configured), naming what is missing on stderr; the
runner shows it unconfigured and does not restart it until a `reload` after
the secret is set ([M.13](../../../Plans/R1.0-Release/TODO.md#runner-waves)).
A bus that does not answer the secret read is not that: the agent fails
ordinarily and the runner restarts it, and a call meanwhile answers
`error: the bus did not answer`:

```sh
sudo src/agents/slack/package.sh /var/lib/agent-bus/agent.d/slack
agent-bus-token '#slack@realm' | sudo -u <you>-ab-runner tee /var/lib/agent-bus/<you>-ab-runner/tokens/%23slack@realm >/dev/null
agent-bus call '#runner/<you>@realm' reload
```

`package.sh` bundles the agent, the TypeScript client and its dependency
into one `slack.js`, since the sandbox mounts the agent's directory alone.

**One Slack app, one connection.** Socket Mode spreads an app's events across
its open connections rather than copying them, so another program holding a
connection with the same `SLACK_APP_TOKEN` — `/rd/bin/slack-tail`, say — gets
some of the events and this agent the rest. Give the bus agent an app of its
own, or run one of them at a time.

Without a runner, by hand or as a user unit, with the agent's own token:

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

## Reply routing

Routing is opt-in through the record's Configuration; credentials remain in
Secret. With no routing keys, posting and subscriptions behave as before.
Telegram and Slack bundle [one router](../shared/router.ts); its shared tag,
expiry and sender/correlation checks follow [Telegram routing](../telegram/README.md#reply-routing).

```json
{"users":{"U123":"parf"},"tags":{"b":"#claude/main@realm"},"route":"#claude/main@realm","channels":{"#dev":{"route":"#team@realm","tags":false,"anyone":true}}}
```

`users` lists Slack user ids and their bus-user labels. `tags` pins a short
tag to a session; unpinned senders get a letter from their name, with a digit
if occupied, on their first post. Tags are stored permanently in KV and never
reassigned. Every session post ends with its tag when routing is configured.
Channel settings accept ids or #names, resolved against the current workspace.

Only DMs, @mentions of the bot and replies in mapped session threads enter
routing. Ordinary channel chatter gets no routing notice. A thread reply
selects that thread's session and preserves its bus topic, tag and message id;
otherwise a leading `#tag` or full bus-agent name selects a session, then the
channel route or global `route` is used. A session's bus answer is checked
against its sender and correlation (also for MCP answers without `re`) and posted back into the original Slack
thread with `thread_ts`. Both directions' mappings last seven days in KV.
Expired mapped replies ask the writer to use a tag; an unmatched addressed
message with no route explains that it was not understood and lists known tags.

`channels` overrides the route per channel; `tags:false` makes leading tags
literal. `anyone:true` admits unlisted members **only** to that channel's
route or a reply in a session thread in that same channel. They cannot select
tags, full names or the global route. Listed users keep the normal choices.
A transient routing failure retries at most three times with backoff; a definitive
refusal is handled immediately. Exhausted retries log the message id and move on.
Subscriptions remain independent: every matching channel event still uses
its own author and regex filters, without routing admission.

Unlisted addressed users get "we do not know you" at most once per sender
per hour. Every refusal logs the user id, username, display name, channel,
message id and reason, with at most 40 text characters. `refused` (or
`{"method":"refused"}`) lists the last 20 refused senders and their last-seen
times/counts; only this record's delivered Owner/Maintainer roles authorize it.
The hourly notice state is pruned on every write. Use these ids to populate
`users`; never put Slack credentials in Configuration.

## Tests

`bun test ./src/agents/slack` runs everything against a real daemon (built
here, or `SMOKE_BIN`), the actual agent, and a fake Slack with a Socket Mode
websocket — no network and no real tokens. `src/smoke.sh --slow` runs it as
the `agent_slack` shard.
