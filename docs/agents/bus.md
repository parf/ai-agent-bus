# Bus: ask the bus about itself

📌 **TL;DR:** `#bus` is built into the bus. Anyone with a bus account can ask
it whether the bus is up, which version runs, and which helpers they may use.
Nothing to install, no extra password.

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
or use [debug](debug.md#check-the-bus-in-a-minute)), or changing anything (it is read-only).

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
  `help-config`, `help-secret` and `mcp`. Configuration and Secret declarations
  are empty; `mcp` returns `{"tools":[]}`. A plain body names a method; body text
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

None.

## Secret keys

None.

## Who may use it

Anyone with a bus account. Every request runs with the caller's own authority;
`#bus` never lists as the daemon Owner on someone else's behalf and offers no
management calls.

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
