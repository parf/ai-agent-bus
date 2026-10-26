# The daemon on the bus

📌 **TL;DR:** Ask `#bus` about the daemon you are connected to: its status,
version and the records you may see. It runs inside the daemon and needs no
separate installation, token or service.

## Ask for help

```sh
agent-bus help '#bus'
agent-bus call '#bus' status
agent-bus call '#bus' version
agent-bus call '#bus' 'ls --all'
```

`#bus` is a complete, realm-less name, local to each daemon. The daemon creates
and protects it before serving connections. People cannot replace, disable,
transfer or remove it, obtain its token, or read its request queue. It is not a
runner package.

## What it answers

| Method | Answer |
|---|---|
| `help` | The read-only methods available to the caller |
| `status` | `ok: <records> records, <readers> readers, up <uptime>` |
| `version` | Shared version and stamped build information |
| `ls [name] [--kind kind] [--tag tag] [--all]` | Caller-visible names, kinds and descriptions, one record per line |

Status uses the same counts as the daemon's status call: records are active
records, and readers are outstanding reads, including filtered reads. They are
observations, not proof that an agent is healthy. A busy or empty queue does not
by itself make the daemon degraded or failed.

`ls` selects the same records as `agent-bus ls`: without options, agents being
read now; `--all` lists everything visible; `--kind` or `--tag` filters; a name
looks up that record. The short text format is for an AI or a person.

Every request runs with its original sender's authority. `#bus` never lists as
the daemon Owner on someone else's behalf. It offers no management methods.
Replies follow the [normal reply routing and ACL](../04-messaging.md#reply-routing).
A non-Owner private agent must allow `#bus` to receive its answers. Owner AI CLI
sessions that allow `@owner` already admit it.

## Description asks

Asks use the envelope's `ask` field: `help`, `help-json`, `help-config`,
`help-secret`, and `mcp`. `help-json` is the machine-readable description;
Configuration and Secret declarations are empty; `mcp` returns `{"tools":[]}`.
A plain body names a method. Body text `--help` is ordinary unsupported input,
not a description ask. For scripts, use `agent-bus call '#bus' --help-json`.
