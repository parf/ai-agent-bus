# Configuring the runner's agents

📌 **TL;DR:** Code describes how an agent runs; host files decide which names
run here and supply their private settings. Apply file changes with `reload`.
Access permissions remain on the bus record.

- Update code without mixing local settings into its checkout.
- Use an instance when one package needs several configurations.
- Check [accepted but inactive settings](security.md#what-is-accepted-but-inert)
  before relying on a configuration key.

## Where settings belong

Paths below are relative to `/var/lib/agent-bus` in the standard installation.

| File | Purpose |
|---|---|
| `agent.d/<agent>/agent.json` | Command, input form and package description |
| `agent.d/<agent>/env.dist` | Non-secret defaults and required variable declarations |
| `runner/agents.json` | Optional inventory: enable, disable, rename or declare instances |
| `runner/<agent>/env` | Host settings shared by this agent's instances |
| `runner/<agent>/<instance>/env` | Settings specific to one instance |
| `runner/<agent>[/<instance>]/agent.json` | The host's [override](#overriding-the-authors-options) of the author's run options |

The bus record owns ACLs, Maintainers and its bus secret. Those are edited
with ordinary bus tools, not put into the runner's inventory.
File permissions and secret precedence are defined in
[security](security.md#secrets).

## Package options

`agent.d/<agent>/agent.json` is the author's. Every key but `command` is
optional, and an unknown key is refused by name. A key marked *inert* is
accepted and changes nothing yet ([inert keys](security.md#what-is-accepted-but-inert)).
[agent.json.dist](agent.json.dist) holds every key with an example value.

| Key | Meaning | Today |
|---|---|---|
| `command` | Required shell command line; `./` paths resolve within the package | applied |
| `form` | How a message reaches the script: `args` or `json`, default `json`; `std`, `jsonl` and `msgpack` are known and refused as not served yet | applied |
| `serves` | `runner`, the default: a script per message; `itself`: kept processes that read their own inbox ([below](#agents-that-serve-themselves)) | applied |
| `description` | Description registered on the bus | applied |
| `version` | The package's own version | inert |
| `processes` | Maximum concurrent scripts, a count or `min..max`, default `0..1`; only the maximum is enforced. An agent that serves itself takes a count | applied |
| `deadline` | Maximum run duration, such as `30s`; whichever comes first, this or the caller's deadline, ends the run. For an agent that serves itself, how long a stop waits before killing, default `10s` | applied |
| `idle` | How long an idle kept process waits before it is stopped, such as `5m` | inert |
| `in_flight` | Messages one kept process takes at once | inert |
| `tags` | Package labels, combined with host-inventory tags and the automatic `runner` label; [update rules](../03-records.md#tags) | applied |
| `roles` | Registered on the record as the roles the agent understands; it grants no caller any role | applied |
| `depends` | Agents this one needs running first | inert |
| `needs.network` | A nonempty host list enables network access; it does not restrict access to those hosts yet | applied, coarsely |
| `needs.memory` | Memory the agent needs, as `2G`; above the host's `limits.memory` the agent is refused by name | applied |
| `needs.writable` | Paths writable beyond the work directory | inert |

## Agents that serve themselves

`"serves": "itself"` is an agent with its own long-lived process that reads
its own inbox through a client library, such as the
[Slack agent](../../src/agents/slack/README.md). The runner keeps it running
rather than starting it per message:

| | |
|---|---|
| its record | yours: register it, then hand its token over once into `runner/tokens/`, as for any [existing agent](security.md#what-runs-as-what). The runner never registers one, and refuses it by name until it holds the token |
| its processes | a fixed count, `processes` a number, each in the agent's cgroup and sandbox; one that exits is restarted, after 1 s, doubling to 30 s for one that keeps dying, except for a [configuration failure](operations.md#not-configured) |
| what it is told | `AGENT_BUS_NAME`, `AGENT_BUS_TOKEN` — its own, by design, since it talks to the bus itself — `AGENT_BUS_ADDR`, with the bus socket mounted in, and its env files; not the bus secret, which it reads as its record's |
| a change | an env file or its command changed restarts it at `reload` |
| stopping | `SIGTERM` to its own processes, then its cgroup killed after its `deadline`, 10 s by default |
An agent can report that it is [not configured](operations.md#not-configured);
fix its settings and use `reload` before it is tried again.

The [inactive settings](security.md#what-is-accepted-but-inert) reference
lists accepted keys that have no effect yet.

</details>

## Inventory

An inventory is a JSON object keyed by package name or `package/instance`.
If no entry names a package, its installed directory supplies an enabled
default entry. Removing its entry therefore does not disable the package.
[agents.json.dist](agents.json.dist) holds every key with an example value.

| Key | Meaning | Today |
|---|---|---|
| `enabled` | `false` leaves the entry installed and unserved; default `true`. The runner's `enable` and `disable` [calls](operations.md#controlling-one-agent) write it | applied |
| `name` | A complete bus name, taken as written; default `#<key>@<realm>` | applied |
| `tags` | Labels added to the package's | applied |
| `sandbox` | `on`, the default, or `off` for this entry only ([security](security.md#isolation-today)) | applied |
| `limits` | The whole agent's [limits](#limits) | applied |
| `share` | This copy joins a pool of one name across hosts | inert |
| `version`, `origin`, `first_started`, `last_started` | Reserved for the runner to write; it writes none yet. Never set by hand | inert |

<details>
<summary>Disable a package or give an instance an explicit name</summary>

```json
{
  "hello": {"enabled": false},
  "mail/work": {"enabled": true, "name": "#work-mail@your-realm"}
}
```

An inventory entry may add `"tags": ["work notes"]` to package defaults.
Merge these entries into your existing inventory, preserving other entries.
An entry naming a package not installed under `agent.d/` is refused.
A complete `name` is taken as written; otherwise the directory and runner's
realm supply the default. Leave metadata written by the runner alone.
The sandbox is on by default. `"sandbox": "off"` disables it for that entry;
read [security](security.md#isolation-today) before using that override.

</details>

## Overriding the author's options

A host changes the author's run options without editing the checkout:
`runner/<agent>/agent.json` for the agent and every instance, then
`runner/<agent>/<instance>/agent.json` for one instance. Each is a JSON Merge
Patch of `agent.json`, applied in that order; `null` removes a key, so it falls
back to the built default rather than to an earlier layer.

| May override | May not |
|---|---|
| `processes`, `deadline`, `idle`, `depends` | everything else: `command`, `form`, `serves` and the rest are what the code is |

```json
{"processes": 3, "deadline": "2m"}
```

A key that may not be overridden refuses the agent at `reload`, naming the
file and the key; the runner's `write` call refuses it before writing.
`idle` and `depends` are still [inert](security.md#what-is-accepted-but-inert).

## Limits

An entry's `limits` bound the whole agent, every process of it together, in
its own cgroup ([isolation](security.md#isolation-today)). Each is optional.

| Key | Means | Default |
|---|---|---|
| `memory` | memory, as `512M` or `1G`; no swap; also the size of its `/tmp` | `512M` |
| `cpu` | a share of one CPU, as `150%` | unbounded |
| `processes` | processes and threads at once | `64` |
| `files` | open files, per process | `1024` |
| `io_weight` | disk I/O weight, 1–10000 | the kernel's, 100 |
| `disk` | what its work directory may hold, measured after each run; over it the agent is stopped until emptied and reloaded | `1G` |

```json
{"mail/work": {"limits": {"memory": "1G", "cpu": "200%", "processes": 32}}}
```

A malformed limit, or an `agent.json` whose `needs.memory` is above `memory`,
refuses the agent by name. A change applies at `reload`, to processes already
running as well.

## Environment files

Use `KEY=value`, one per line, in the bus's
[env-file syntax](../03-records-service.md#env-file-syntax). Values are not shell code: `$VAR` is not
expanded. In `env.dist` an empty value declares a requirement; it does not
declare an empty default.

<details>
<summary>Quoting and comments</summary>

```dotenv
# Non-secret defaults go in env.dist
HOST=mail.example.com
FOLDER="INBOX/Work"
USER=
```

The syntax is the bus's one env-file grammar ([env-file syntax](../03-records-service.md#env-file-syntax)).
Put real secrets in a private host env file or the record's bus secret, never
in `env.dist`.

</details>

## Apply and check

Call the runner's `reload`, then check its `status`. A refusal names the
agent and reason. Every file here can also be read, and the host's written, over
the bus ([the host's files](operations.md#the-hosts-files-over-the-bus)), which
applies at once. Follow [operations](operations.md#reading-status) for
commands and [troubleshooting](operations.md#why-an-agent-was-refused) for errors.
Overrides of an agent's other shipped JSON files remain in the
[runner plan](../../Plans/R1.0-Release/runner-config.md#overriding-a-shipped-file).
