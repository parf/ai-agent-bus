# Bus: ask the bus about itself

📌 **TL;DR:**
- **Problem:** before using a helper you need to know the bus is alive and what you may call, without logging in to the server.
- **Outcome:** anyone with a bus account asks `#bus` whether the bus is up, which version runs and which helpers they may use; it answers only with what you are already allowed to see.
- **Start:** `agent-bus call '#bus' status` ([Try this first](#try-this-first)); nothing to install, no extra password.

## What it does

- **Check connectivity:** `status` reports record and reader counts and uptime.
- ✅ **Deploy check:** `version` gives the running version and build.
- ✅ **Find helpers:** `ls` lists your team's helpers with one-line descriptions.
- ✅ **Read the menu:** `help` lists the calls you may use.
- ✅ **Machine-readable answers:** `status --json`, `version --json` and `ls … --json` answer with the daemon's own JSON and nothing else; an empty list is `[]`. Text stays the default.
- ✅ **Built in:** every daemon has its own `#bus`, created and protected by the daemon; nobody can replace, disable, transfer or remove it, get its token or read its queue.
- 🚫 **Not for:** checking whether a particular helper works (call that helper, or use [debug](debug.md#check-the-bus-in-a-minute)), or managing the registry through its message methods.

## You'll need

- A bus account; nothing else.

## Try this first

- Ask the bus how it is:

```sh
agent-bus call '#bus' status
```

## Use examples

- **Read the counts as JSON:** `agent-bus call '#bus' 'status --json'`
- **See the running version:** `agent-bus call '#bus' version`
- **List my team's agents:** `agent-bus call '#bus' 'ls --kind agent --tag team'`
- **Get the machine-readable description:** `agent-bus call '#bus' --help-json`
- **See the menu:** `agent-bus help '#bus'`

## Installing it

- Nothing to install: the daemon creates `#bus` itself.

## Agent logs

An alert is on disk within about 100 ms; a clean stop flushes all.

- Private per-agent logs include runner observations, subscriptions and a preview on the web record page. Alerts, id reservations and subscription definitions survive daemon restarts; notices and warnings stay in memory.
- The runner forwards masked stderr and lifecycle observations using the agent's own credential. Alerts go first in their own lane; forwarding never blocks the child. A full or unavailable event path drops and counts the event without retrying. An agent can write a misleading line about itself, seen only by its own Owner and Maintainers; no line proves who produced it.
- `write <agent> notice|warning|alert <text>` writes the agent's own log; the daemon Owner may also write. A write answers only when `wait` is set; a reply to the caller’s own inbox also requires its Allow to admit `#bus`, following the [normal reply ACL](../04-messaging.md#reply-routing).
- `read <agent> [level]`, `list`, `pull <agent> [level] [count]` and `clean <agent> [level] [id]` use the [shared log model](eventlog.md#log-model) and [reply forms](eventlog.md#technical-details). Only the log's Owner, Maintainers and daemon Owner may read or manage it; only the daemon Owner may read `#bus`'s own log.
- `sub add <target> <agent> [level] [filter]` subscribes to new matching entries of a log you manage; the [shared filter syntax](subscriptions.md#filters) applies. The target is a queue or agent that lists `#bus` in its Allow, because `#bus` sends the copies; a Personal agent may list it ([Personal rules](../03-records.md#personal-and-shared)), and `#bus` itself is refused. There is no replay.
- The [subscription limits](subscriptions.md#the-shared-behaviour) apply separately to each agent log.
- `sub list [agent]` lists subscriptions you made or manage; `sub delete <id>` removes one by its subscription id.
- Source rights are checked at creation only. Every send checks the creator's may-send to the target and the target's Allow for `#bus`, under the [shared subscription rules](subscriptions.md#the-shared-behaviour). Deleting a source or author removes its subscriptions.
- Each matching subscription sends one copy from `#bus`, including a folded occurrence. Refused copies are dropped and counted without retry; the next entry is still attempted.
- A waiting write reports real `deliveries` counts: matched, sent and dropped. Its copies finish before pull or clean can remove the entry. A waiting write with possible subscribers needs at least three seconds left before storing.
- A damaged log is left out of `list`, still refuses its own read with `storage_invalid`, and is named in the daemon's error log at startup.

## Configuration

- **The Configuration of `#bus` is the bus setup,** and only the daemon Owner reads or saves it; an Administrator, an Agent of the Owner or anybody else is refused.
- **A save applies at once,** with no restart, and is kept in backups.
- **The web has it as "Bus setup"** in the left navigation, for the daemon Owner only.
- **Changing the realm is not a rename:** every existing name, token, ACL entry, route and saved session keeps its old spelling and keeps working; only new automatic names use the new realm.
- **New web record forms suggest the current realm.** Complete the name before `@`; change or remove the suggested realm to choose an explicit identity. A refused form keeps what you typed.
- **`agent-bus status` shows the realm in effect.**

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `realm` | Required. The bus's default realm for new automatic names. The first start saves the host name; a saved realm is never replaced by the host's |
| `notice_entries`, `notice_days` | Agent-event notices kept per agent, and for how many days; default 200 and 1 |
| `warning_entries`, `warning_days` | The same for warnings; default 200 and 7 |
| `alert_entries`, `alert_days` | The same for alerts; default 50 and 30 |

```sh
agent-bus config '#bus' '{"realm":"node.example"}'
```

</details>

<details>
<summary>Validation and audit</summary>

- The whole value is one JSON object of the keys above; an unknown key, a missing realm or a value out of range refuses the save and changes nothing.
- The realm is checked as written: lower case `a-z 0-9 . _ -`, starting with a letter or digit, at most 62 characters; nothing is trimmed, lowered or cut; only the first-start default lowers the host name and drops a trailing dot.
- A key given twice refuses the save.
- Numbers are whole: entries per ring 1–500, with a combined total of 500 or less; days 1–3650.
- A realm change writes one audit-log line naming the Owner and the old and new realm; a realm the daemon saves from the host name at start is audited as `(daemon start)`.

</details>

## Secret keys

- **None.**

## Who may use it

- **Anyone with a bus account.**
- **Every request runs with the caller's own authority;** `#bus` never lists as the daemon Owner on someone else's behalf and private-log methods check the log’s management authority.
- **Its [Configuration](#configuration)** is the daemon Owner's.

<details>
<summary>Replies to private agents</summary>

- Replies follow the [normal reply routing and ACL](../04-messaging.md#reply-routing): a non-Owner private agent must allow `#bus` to receive its answers.
- Owner AI CLI sessions that allow `@owner` already admit it.

</details>

## Troubleshooting

| If you see | Do this |
|---|---|
| No answer to `call '#bus' …` from a private agent | Allow `#bus` on that agent so the reply can reach it. |
| Fewer records than expected | `ls` shows only records you may see; ask their Owner for access. |
| `--help` as body text answers unsupported | Use the ask flag: `agent-bus call '#bus' --help-json`. |

## Warnings and alerts

Inspect the daemon's [diagnostic logs](../administration.md#logs) for startup, storage and connection failures. For an individual agent, use its [agent log](#agent-logs).

## Technical details

<details>
<summary>Counts, listing and asks</summary>

- `#bus` is a complete, realm-less name; it is not a runner package.
- `status` counts all active records node-wide, regardless of listing visibility, and outstanding reads, including filtered reads; they are observations, not proof that an agent is healthy; a busy or empty queue does not by itself mean the daemon is degraded.
- `ls` selects the same records as `agent-bus ls`: without options, agents being read now; `--all` everything visible; `--kind` or `--tag` filters; a name looks up that record.
- Description asks use the envelope's `ask` field: `help`, `help-json`, `help-config`, `help-secret` and `mcp`.
- `help-config` lists the [bus setup](#configuration) keys; there are no Secret keys; `mcp` returns `{"tools":[]}`.
- A plain body names a method; body text `--help` is ordinary unsupported input.
- A body is at most 16 KiB, checked before parsing; oversized input is refused with `invalid_request`.

</details>
