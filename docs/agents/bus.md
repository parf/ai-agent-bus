# Bus: ask the bus about itself

📌 **TL;DR:** `#bus` is built into the bus. Anyone with a bus account can ask
it whether the bus is up, which version runs, and which helpers they may use.
Nothing to install, no extra password. Its Configuration is the bus setup:
the bus's default realm, which only the daemon Owner changes.

## Why you'd want it

**The problem.** Before using a helper you need to know the bus is alive and
what you may call — without logging in to the server.

**The idea.** The bus answers questions about itself, and only with what you
are already allowed to see.

**When to use it**

- Before a long task: is the bus up?
- After an update: which version is running?
- When looking for a helper: what may I use?

**When not to:** checking whether a particular helper works (call that helper,
or use [debug](debug.md#check-the-bus-in-a-minute)), or changing anything (its calls are read-only).

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| AI pre-flight check | `status` | `ok: … records, … readers, up …` |
| Deploy check | `version` | The running version and build |
| Find helpers | `ls --tag team` | Your team's helpers with one-line descriptions |
| Script reads the menu | `--help-json` | Machine-readable list of calls |

## What it does

| Call | Does |
|---|---|
| `help` | Lists the read-only calls you may use |
| `status` | `ok: <records> records, <readers> readers, up <uptime>` |
| `version` | Shared version and build information |
| `ls [name] [--kind kind] [--tag tag] [--all]` | Names, kinds and descriptions you may see, one per line |

Every daemon has its own `#bus`, created and protected by the daemon. Nobody can
replace, disable, transfer or remove it, get its token or read its queue.

<details><summary>Technical details: counts, listing and asks</summary>

- `#bus` is a complete, realm-less name; it is not a runner package.
- `status` counts active records and outstanding reads, including filtered
  reads. They are observations, not proof that an agent is healthy; a busy or
  empty queue does not by itself mean the daemon is degraded.
- `ls` selects the same records as `agent-bus ls`: without options, agents being
  read now; `--all` everything visible; `--kind` or `--tag` filters; a name looks
  up that record.
- Description asks use the envelope's `ask` field: `help`, `help-json`,
  `help-config`, `help-secret` and `mcp`. `help-config` lists the
  [bus setup](#configuration) keys; there are no Secret keys; `mcp` returns
  `{"tools":[]}`. A plain body names a method; body text
  `--help` is ordinary unsupported input.

</details>

## You'll need

A bus account; nothing else.

## Try this first

```sh
agent-bus help '#bus'
agent-bus call '#bus' status
agent-bus call '#bus' version
agent-bus call '#bus' 'ls --all'
```

## Use examples

| I want to… | Command |
|---|---|
| Check the bus is up | `agent-bus call '#bus' status` |
| See the running version | `agent-bus call '#bus' version` |
| List my team's agents | `agent-bus call '#bus' 'ls --kind agent --tag team'` |
| Get the machine-readable description | `agent-bus call '#bus' --help-json` |

## Installing it

Nothing to install: the daemon creates `#bus` itself.

## Configuration

The Configuration of `#bus` is the bus setup. Only the daemon Owner reads or
saves it; an Administrator, an Agent of the Owner or anybody else is refused.
A save applies at once, with no restart, and is kept in backups.

| Key | Meaning |
|---|---|
| `realm` | Required. The bus's default realm for new automatic names. The first start saves the host name; a saved realm is never replaced by the host's |
| `notice_entries`, `notice_days` | Agent-event notices kept per agent, and for how many days; default 200 and 1 |
| `warning_entries`, `warning_days` | The same for warnings; default 200 and 7 |
| `alert_entries`, `alert_days` | The same for alerts; default 50 and 30 |

```sh
echo '{"realm":"parf.us"}' | agent-bus config '#bus' -
```

Changing the realm is not a rename: every existing name, token, ACL entry,
route and saved session keeps its old spelling and keeps working. Only new
automatic names use the new realm. `agent-bus status` shows the realm in
effect. Agent events are in work; their keys are checked and kept now.

<details><summary>Technical details: validation and audit</summary>

- The whole value is one JSON object of the keys above; an unknown key, a
  missing realm or a value out of range refuses the save and changes nothing.
- The realm is checked as written: lower case `a-z 0-9 . _ -`, starting with a
  letter or digit, at most 62 characters. Nothing is trimmed, lowered or cut;
  only the first-start default lowers the host name and drops a trailing dot.
- A key given twice refuses the save.
- Numbers are whole: entries 1–10000, days 1–3650.
- A realm change writes one audit-log line naming the Owner and the old and
  new realm; a realm the daemon saves from the host name at start is audited
  as `(daemon start)`.

</details>

## Secret keys

None.

## Who may use it

Anyone with a bus account. Every request runs with the caller's own authority;
`#bus` never lists as the daemon Owner on someone else's behalf and offers no
management calls. Its [Configuration](#configuration) is the daemon Owner's.

<details><summary>Technical details: replies to private agents</summary>

Replies follow the [normal reply routing and ACL](../04-messaging.md#reply-routing):
a non-Owner private agent must allow `#bus` to receive its answers. Owner AI CLI
sessions that allow `@owner` already admit it.

</details>

## Troubleshooting

| If you see | Do this |
|---|---|
| No answer to `call '#bus' …` from a private agent | Allow `#bus` on that agent so the reply can reach it. |
| Fewer records than expected | `ls` shows only records you may see; ask their Owner for access. |
| `--help` as body text answers unsupported | Use the ask flag: `agent-bus call '#bus' --help-json`. |
