# Bus: ask the bus about itself

📌 **TL;DR:**
- **Problem:** before using a helper you need to know the bus is alive and what you may call, without logging in to the server.
- **Outcome:** anyone with a bus account asks `#bus` whether the bus is up, which version runs and which helpers they may use; it answers only with what you are already allowed to see.
- **Start:** `agent-bus call '#bus' status` ([Try this first](#try-this-first)); nothing to install, no extra password.

## Why you'd want it

- 🩺 **Is the bus up?** Now one call says so, with counts and uptime.
- 🔢 **Which version runs after an update?** Now `version` answers.
- 🔎 **Which helpers may I use?** Now `ls` lists exactly the ones you may see.
- 🤖 **A script needs the menu.** Now `--help-json` returns it in machine-readable form.

## What it does

- ✅ **Pre-flight check for an AI:** `status` answers `ok: <records> records, <readers> readers, up <uptime>`.
- ✅ **Counts are the whole node's:** records are every live record of any kind, not only the ones you may see; readers are reads waiting for a message, so one agent can count twice and a busy one not at all.
- ✅ **Deploy check:** `version` gives the running version and build.
- ✅ **Find helpers:** `ls` lists your team's helpers with one-line descriptions.
- ✅ **Read the menu:** `help` lists the read-only calls you may use.
- ✅ **Machine-readable answers:** `status --json`, `version --json` and `ls … --json` answer with the daemon's own JSON and nothing else; an empty list is `[]`. Text stays the default.
- ✅ **Built in:** every daemon has its own `#bus`, created and protected by the daemon; nobody can replace, disable, transfer or remove it, get its token or read its queue.
- 🚫 **Not for:** checking whether a particular helper works (call that helper, or use [debug](debug.md#check-the-bus-in-a-minute)), or changing anything (its calls are read-only).

## You'll need

- A bus account; nothing else.

## Try this first

- Ask the bus how it is:

```sh
agent-bus call '#bus' status
```

<details>
<summary>See it on a real node (commands that ran today)</summary>

```sh
agent-bus call '#bus' status | jq -r .body
```

```text
ok: 125 records, 47 readers, up 6h2m48s
```

```sh
agent-bus call '#bus' version | jq -r .body
```

```text
0.9.159
build_info: parf@parf.us 2026-10-29 21:46:38
```

```sh
agent-bus call '#bus' 'ls --kind agent --tag runner' | jq -r .body | head -3
```

```text
#webhook@parf.us	agent	Receive authenticated events and post to configured webhook routes
#telegram/parf@parf.us	agent	Post and read chats; route your replies to the right session
#slack@parf.us	agent	Post and read messages; route replies; forward selected conversations
```

- The output is from the development node on 2026-10-30; the list shows only what the caller may see.

</details>

## Use examples

- **Check the bus is up:** `agent-bus call '#bus' status`
- **Read the counts as JSON:** `agent-bus call '#bus' 'status --json'`
- **See the running version:** `agent-bus call '#bus' version`
- **List my team's agents:** `agent-bus call '#bus' 'ls --kind agent --tag team'`
- **Get the machine-readable description:** `agent-bus call '#bus' --help-json`
- **See the menu:** `agent-bus help '#bus'`

## Installing it

- Nothing to install: the daemon creates `#bus` itself.

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
echo '{"realm":"parf.us"}' | agent-bus config '#bus' -
```

</details>

<details>
<summary>Validation and audit</summary>

- The whole value is one JSON object of the keys above; an unknown key, a missing realm or a value out of range refuses the save and changes nothing.
- The realm is checked as written: lower case `a-z 0-9 . _ -`, starting with a letter or digit, at most 62 characters; nothing is trimmed, lowered or cut; only the first-start default lowers the host name and drops a trailing dot.
- A key given twice refuses the save.
- Numbers are whole: entries 1–10000, days 1–3650.
- A realm change writes one audit-log line naming the Owner and the old and new realm; a realm the daemon saves from the host name at start is audited as `(daemon start)`.

</details>

## Secret keys

- **None.**

## Who may use it

- **Anyone with a bus account.**
- **Every request runs with the caller's own authority;** `#bus` never lists as the daemon Owner on someone else's behalf and offers no management calls.
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

`#bus` is the daemon itself, so its entries are the daemon's own life; the daemon Owner sees them, in addition to the daemon's error log and syslog.

| Level | When | What to do |
|---|---|---|
| alert | An impossible state: a broken rule, corrupt stored data, an answer it could not give with authority | Read the daemon's error log; restore from backup if stored data is damaged |
| alert | A write to its database failed: disk full or an I/O error | Free space or fix the disk, then restart the daemon |
| alert | A listen address could not be taken at start, or a TLS certificate expired | Free the port or fix the address; renew the certificate |
| alert | The previous run ended without a clean stop (shown at the next start) | Check the host's service log for why it stopped |
| warning | A pubsub copy dropped for a stopped or full recipient; a ring queue dropping its oldest; a forward refused | Check that recipient or raise its bound |
| warning | Free disk space under 10 %, a TLS certificate expiring within 14 days, many failed sign-ins from one address, the clock stepped back | Act before it turns into an alert |
| notice | Started or stopped; the bus setup changed; the debug log turned on or off; a backup or restore finished | — |

## Technical details

<details>
<summary>Counts, listing and asks</summary>

- `#bus` is a complete, realm-less name; it is not a runner package.
- `status` counts active records and outstanding reads, including filtered reads; they are observations, not proof that an agent is healthy; a busy or empty queue does not by itself mean the daemon is degraded.
- `ls` selects the same records as `agent-bus ls`: without options, agents being read now; `--all` everything visible; `--kind` or `--tag` filters; a name looks up that record.
- Description asks use the envelope's `ask` field: `help`, `help-json`, `help-config`, `help-secret` and `mcp`.
- `help-config` lists the [bus setup](#configuration) keys; there are no Secret keys; `mcp` returns `{"tools":[]}`.
- A plain body names a method; body text `--help` is ordinary unsupported input.

</details>
