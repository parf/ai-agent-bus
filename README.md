# agent-bus

Connect AI agents, bots and services so they can find and message each other.
One daemon gives you a registry, message queues, an MCP server and a dashboard.

![Agents Bus — connecting agents](docs/img/agent-bus.png)

🚧 **Design phase.** Nothing here is implemented yet; the commands below show the
intended shape. A first version (V1) runs in production on a message broker and is
being replaced by this design. See [Status](#status).

## What it is

You have things that should talk to each other: a Claude Code session, a Codex
session, a script that reads Slack, a bot that sends SMS, a MySQL server, a cron job.
Today each pair needs its own glue.

agent-bus gives them one meeting point. A small daemon, `agent-busd`, keeps a
**registry** of everything that joined, holds a **queue** for each of them, answers
**"what can I use?"** over MCP with generated docs, and shows who is here and how
busy on a **dashboard**. Topics are registered the same way as services and show up in the
same registry. Any participant can look up another by name and send it a message.
Anything that already has an address can be **registered by hand**: "there is a MySQL
service called `xxx` on `host:port`" is a valid registration.

There is no external broker to install. `agent-busd` is the broker, the registry and
the dashboard in one binary.

Participants know each other by **public key**, and everything they say is
**encrypted**. No passwords, no certificates to manage.

## The 60-second picture

```
                          ┌──────────────── agent-busd ────────────────┐
  claude --channel   ───→ │ registry   queues   MCP server   dashboard │ ←───  codex session
  slack-reader       ───→ │                                            │ ←───  sms-out bot
  mysql "xxx" (by hand) → │  who is here · what can they do · inboxes  │ ←───  agent-bus CLI
                          └────────────────────────────────────────────┘
```

- Every participant has an **identity** and an **address**, `unique-name@host`.
  In the smallest setup the identity is just a token; with AUTH or pairwise
  keys it is an Ed25519 key. The `agent-bus` CLI handles both and joins the bus.
- Every registered participant has its **own queue**. Send to it while it is down;
  it reads the backlog when it comes back.
- **`send`** delivers to one known receiver; **`publish`** delivers to a topic. A topic
  is either a **queue** (each message to one consumer, kept until taken or expired) or
  **pub/sub** (a copy to every current subscriber, nothing kept).
- A message carries a **topic** (which conversation) and a **tag** (which message), so
  three questions to the same service come back matched to the right question.
- **Authentication is always on, and every session is encrypted.** The smallest setup
  is one token in an environment variable, and that token is the whole identity. Central
  AUTH is an optional role of the same daemon: turn it on when you need central
  identities and groups.

## Use cases

You can build these, or something like them, in minutes once it ships.

### Watch a Slack channel and act on it

The flow that runs in production today on V1. A small **slack-reader** service
forwards posts from alert channels onto the bus. A Claude Code session started with
`claude --channel` (a Claude Code channels feature, research preview) receives them,
decides what each one is, and forwards it: noise to nowhere, "tell a human" to the
Slack/Telegram/SMS/email bots, "fix it" to a **fixer** session, "check it" to a
**reviewer** session.

```
Slack channels → slack-reader → ai-claude-watch (claude --channel)
                                   ├→ alerters  (Slack · Telegram · SMS · email)
                                   ├→ fixer     (one long-lived session, its own queue)
                                   └→ reviewer  (one long-lived session, its own queue)
```

The fixer is a single session with its own queue, so it processes events in order,
keeps the history of what it already did, and never fights itself over git. The
reviewer works the same way.

### Let sessions talk to each other

Claude Code and Codex sessions join the bus the same way as any agent. One session can
ask another for a review, hand over a task, or wait for a result, addressed by name.

### Make existing services discoverable to agents

Register a database, an HTTP API, a unix socket or a cron host by description. Agents
asking the MCP server "what can I use?" get a catalog **filtered to what they may see**,
with docs generated from the registrations and tool descriptions where a service exposes
tools. No adapter code on the service side.

### Run a personal bus on your laptop

Start `agent-busd` as yourself, set one token, done. AUTH role off, no admin, no
network exposure. Everything above works.

### Run a team or company bus

Turn on the **AUTH** role (`auth: on`, same daemon, same git repo): identities come from
GitHub (most developers already have an SSH key there; Ed25519 ones are used) or LDAP,
groups compose with `& | !`, each service declares
its own roles, and access keys rotate hourly without AUTH on the hot path. Personal,
team and company buses **chain**: local first, upstream for the rest.

### Supervise and sandbox agents

`agent-busd` is also a runner. Point it at an MCP server, an HTTP API or a plain
script; it spawns the process, restarts it with backoff, registers it, injects its key
and private config, and sandboxes it by default (`systemd-run`, `bubblewrap` or
`unshare`, chosen by environment).

### See what is going on

The dashboard lists services and topics with their descriptions, who is up, and how
many calls each takes per minute or hour — straight from memory. Message bodies are not
on it, or anywhere else on the bus: they are encrypted between sender and receiver, and
once consumed they are gone. Prometheus export for Grafana if you want history.

## What it is not

- **Not a durable queue.** Queues live in memory and are saved to a Parquet file on a
  graceful restart (optionally every minute), so a crash loses at most a minute. Each
  queue has a TTL and a size; on overflow it either drops its oldest message or refuses
  new ones — the topic chooses. If one flow needs more, give that one a WAL.
- **Not a workflow engine.** It routes messages; what to do with them is the agent's job.
- **Not forward secret.** A leaked long-term key exposes recorded sessions. Accepted for
  this design.
- **Not a public-internet gateway.** Sessions are encrypted, but the design targets your
  own hosts and laptops, not anonymous clients.

## Intended CLI shape

Illustrative only; the exact verbs are part of the design work.

```sh
# join the bus with an identity
agent-bus keygen                       # creates your Ed25519 key
export AGENT_BUS_USER_TOKEN=...        # minimal auth: one token

# describe something that already exists
agent-bus register mysql-prod --kind generic --addr host:3306

# create a topic (registered like a service: token to create, owner to change)
agent-bus topic create alerts.prod --kind pubsub
agent-bus topic create build-jobs  --kind queue --ttl 1h --bound 1000

# talk
agent-bus send     fixer@srv1 --topic deploy-42 --tag q1 "run the migration?"   # known receiver
agent-bus publish  --topic alerts.prod "disk 91% on db3"                        # whoever consumes it
agent-bus consume                                                                # read my own queue

# supervise a child under the runner
agent-bus start my-mcp-server --sandbox default
agent-bus ls · agent-bus logs my-mcp-server · agent-bus stop my-mcp-server
```

## Advanced topics

Highlights for the impatient:

- **Security model.** Ed25519 everywhere, no passwords, no client secrets. Sessions are
  encrypted point-to-point with a key both sides derive; no TLS, no PKI. Kerberos-style:
  AUTH hands out a shared secret once per hour, then gets out of the way.
- **Three ways to get a key.** *Static*: a token you set by hand, never expires, the
  minimal mode. *Pairwise*: derived from the two parties' keys, no AUTH needed.
  *Derived*: issued by AUTH, one hour, deterministic across replicas.
- **Config as code.** AUTH data is a signed bundle in a git repo over SSH. Replicas pull
  on start, newer generation wins, git history is the audit trail. Service definitions
  are live records in `agent-busd`, guarded by token and ownership, signed by whoever
  wrote them when that principal has a key.
- **Admin over SSH only.** Forced commands, no shell, every call audit-logged. Root on
  the box is the break-glass.
- **One daemon that supervises itself.** AUTH and the dashboard run as child processes
  of `agent-busd`: AUTH alone holds the master secret, the dashboard is cgroup-limited
  so it can never starve the bus.

The `docs/` files are short and meant to be read in order.

| File | Covers |
|---|---|
| [docs/00-overview.md](docs/00-overview.md) | goal, principles, `agent-busd` parts, chaining, storage, trade-offs, decisions, open questions |
| [docs/01-identity-and-auth.md](docs/01-identity-and-auth.md) | principals, GitHub/LDAP key directories, groups/ACL/roles, ownership, delegation, sealed private config |
| [docs/02-keys-sessions-replication.md](docs/02-keys-sessions-replication.md) | access-key modes (derived / pairwise / static), encrypted sessions, signed generations in git over SSH, SSH admin |
| [docs/03-services-and-discovery.md](docs/03-services-and-discovery.md) | service kinds, personal vs shared, messaging (queues, topic + tag, reply-to), discovery faces, health, stats |
| [docs/04-runner.md](docs/04-runner.md) | the runner role of `agent-busd`: adapters, identity injection, sandboxing, in-process queue, languages |
| [docs/v1-original.md](docs/v1-original.md) | V1 as built (NATS JetStream, verified from source), the PRF-36 brainstorm, how V1 is used today, V1 → V2 mapping, weaknesses V1 admits |
| [HANDOFF.md](HANDOFF.md) | the full discussion record: decisions, open questions, superseded ideas |

## Status

| Area | State |
|---|---|
| Design docs | ✅ decisions of 2026-09-09 recorded; one open item marked ❓ in the overview |
| Code | 🚧 none yet. Go first, a bun/NPM build later; client libs for Go, PHP, Rust, JS, Python |
| V1 | runs in production on a broker; its ideas and usage are recorded in `docs/v1-original.md` |

## Conventions

Small files, main ideas only, tables over prose. Symbols follow
[Glyphs](https://parf.dev/ai-skills/Glyphs.md): no glyph by default, ❓ for open questions.

## Names

- `agent-busd` — the daemon · `agent-bus` — the CLI · `ab_` — MCP tool prefix only
- `agent-bus` is the keyword everywhere else: `/etc/agent-bus/`, `~/.config/agent-bus/`,
  the `agent-bus` system user, `agent-busd.service`
