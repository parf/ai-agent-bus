# Configuring the runner's agents

📌 **TL;DR:** Code describes how an agent runs; host files decide which names
run here and supply their private settings. Apply file changes with `reload`.
Access permissions remain on the bus record.

- Update code without mixing local settings into its checkout.
- Use an instance when one package needs several configurations.
- Check [accepted but inactive settings](security.md#what-is-accepted-but-inert)
  before relying on a configuration key.

## Record configuration

Keep secrets in Secret and config in Configuration; see [configuration](../03-records-agent.md#configuring-a-template).

Owner and Maintainers edit JSON settings on the bus or web. The runner passes
compact `AGENT_BUS_CONFIG` to each new process, unset when unconfigured.
Per-message starts read the latest value; kept processes need restart or `reload`.
See [configuration and jq](../03-records-agent.md#configuring-a-template).

## Where settings belong

`agent.d/` is the shared package directory, normally `/var/lib/agent-bus/agent.d`.
The logical `runner/` prefix below means the directory passed as `--host`,
normally `/var/lib/agent-bus/<account>-ab-runner`; do not add another `runner/`
directory beneath it. An agent-wide env file that supplies all required keys
makes the plain agent runnable without an instance.

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

`agent.d/<agent>/agent.json` is the author's. Every key but `command` and `description` is
optional, and an unknown key is refused by name. A key marked *inert* is
accepted and changes nothing yet ([inert keys](security.md#what-is-accepted-but-inert)).
[agent.json.dist](agent.json.dist) holds every key with an example value.

| Key | Meaning | Today |
|---|---|---|
| `command` | Required shell command line; `./` paths resolve within the package | applied |
| `form` | How a message reaches the script: `args` or `json`, default `json`; `std`, `jsonl` and `msgpack` are known and refused as not served yet | applied |
| `serves` | `runner`, the default: a script per message; `itself`: kept processes that read their own inbox ([below](#agents-that-serve-themselves)) | applied |
| `description` | Required nonempty description registered on the bus; missing or blank refuses the package by name | applied |
| `version` | Own version shown in help; absent for bundled agents, which use the shared bus version | applied |
| `license`, `author`, `homepage` | SPDX license, author and documentation URL shown in help | applied |
| `body_formats`, `arguments`, `methods`, `config` | Strictly checked method/argument and Configuration descriptions ([package help](#package-help)) | applied |
| `processes` | Maximum concurrent scripts, a count or `min..max`, default `0..1`; only the maximum is enforced. An agent that serves itself takes a count | applied |
| `deadline` | Maximum run duration, such as `30s`; whichever comes first, this or the caller's deadline, ends the run. For an agent that serves itself, how long a stop waits before killing, default `10s` | applied |
| `idle` | How long an idle kept process waits before it is stopped, such as `5m` | inert |
| `in_flight` | Messages one kept process takes at once | inert |
| `tags` | Package labels, combined with host-inventory tags and the automatic `runner` label; [update rules](../03-records.md#tags) | applied |
| `roles` | Registered on the record as the roles the agent understands; it grants no caller any role | applied |
| `depends` | Agents this one is started after, as bus names (`#db`); a name that is nothing installed here, or a cycle, refuses the agent at `reload`, naming it. Only the order of starts, among agents of this runner; it does not wait for readiness, and a realm in a name (`#db@host`) is not checked; an agent still starts when what it depends on is refused or disabled; `status` lists agents in start order, not by key | applied |
| `needs.network` | A nonempty host list enables network access; it does not restrict access to those hosts yet | applied, coarsely |
| `needs.memory` | Memory the agent needs, as `2G`; above the host's `limits.memory` the agent is refused by name | applied |
| `needs.writable` | Paths writable beyond the work directory | inert |

## Package help

Set `"help":"native"` when the script must apply live Configuration scope or
caps to help (as systemd and git do). The runner and `agent-bus run` then execute
all four help asks through the script with the normal context, Configuration
and delivered roles; otherwise they render package metadata themselves.
A package without native help also answers `mcp` with `{"tools":[]}`. Every
remaining ask, including `status`, reaches the script as `AGENT_BUS_ASK` with an
empty body; an unpackaged script receives every ask except `mcp`, which is answered with `{"tools":[]}`. Offering tools requires a package with `help: native`. `"asks":"none"` suppresses
that execution and answers `unsupported: <ask>` instead. The equivalent run flag
is `--no-asks`. Neither policy prevents metadata answers. A running script must
dispatch asks before its normal flow; unknown asks are normal unsupported replies.

While it already owns a reader, the runner answers status for a known launch or
retirement (`starting`), shutdown (`stopping`) or failure (`failed`) itself. A
normally running agent supplies its own status; no-asks answers unsupported.
Reader-less refused/unconfigured agents have no fallback consumer: ordinary
messages must remain queued.

Start a package with the [create-agent prompt](../agents/create-agent-prompt.md#prompt-to-paste).

Write descriptions and help using the [authoring checklist](../agents/authoring.md#checklist-writing-an-agents-description-and-help): explain useful work without repeating the adjacent name.

The runner answers `help`, `help-config`, `help-secret` and `help-json` for its packaged
scripts without executing their command unless `help` is `native`. `agent-bus run --package-dir /path`
uses the same loader; the standalone starter supplies this path. A self-serving
agent answers in its own process. See [agent self-description](../constitution.md#agent-self-description).

An agent declares accepted call bodies with `body_formats`: a nonempty unique
list containing `text`, `json`, or both; omitted means `["text"]`. This describes
the call body, not help output or response encoding. A JSON body has `method`
and the named method arguments. Text-only agents must not advertise JSON.

`agent.json` carries `methods` keyed by method name and `config` keyed by
Configuration name. Each entry has nonempty `help` and optional `type`:
`string` (default), `int`, `bool`, `list`, `json`, or a nonempty string array
of enum choices. A method's `args` array has named entries with `help`, `type`,
`optional` and `rest` (only the last argument). Methods may state `returns`,
`role`, `destructive`, a text usage override `text`, and a copy-paste `example`.
Shared arguments live in top-level `arguments`, keyed by name; a method argument
without `help` references that definition. Argument `default` documents a default;
`json_only` omits a field from text syntax while preserving it in machine help.
Methods retain their authored order. A role is one name or an array of names, any of which
grants the method; authors spell out cumulative tiers, with no inferred hierarchy. Configuration keys may state `example` and `default`.
Unknown fields, invalid types and duplicate argument names are refused by name.
Unknown Configuration keys and values outside a declared enum are warned about,
not refused. These are descriptions, not argument execution or authority rules. The agent
still checks every call. Help omits methods requiring an unheld role; delivered
Owner and Maintainer roles include every method.

`help`, `help-config` and `help-secret` always answer text. `help-json`
answers one data object with description, license, version, author, homepage,
methods, asks, config and secrets, plus `body_formats` and `hidden_methods`.
The latter contains only `count` and the sorted union of required role names
for role-hidden methods, never their names, descriptions, arguments or examples.
Native runtime scope restrictions apply before this count. Owner/Maintainer
see count zero. These declarations explain availability; the agent still
enforces every call. Secrets are an array of name, help, required and has_default flags, never values. Optional identity fields are omitted when absent.
No host env file or stored bus Secret is read to generate help.

## Agents that serve themselves

Use the [create-agent prompt](../agents/create-agent-prompt.md#prompt-to-paste) to build and verify this serving mode.

`"serves": "itself"` is an agent with its own long-lived process that reads
its own inbox through a client library, such as the
[Slack agent](../../src/agents/slack/README.md). The runner keeps it running
rather than starting it per message:

| | |
|---|---|
| its record | yours: register it, then hand its token over once into `runner/tokens/`, as for any [existing agent](security.md#what-runs-as-what). The runner never registers one, and refuses it by name until it holds the token |
| its processes | a fixed count, `processes` a number, each in the agent's cgroup and sandbox; one that exits is restarted, after 1 s, doubling to 30 s for one that keeps dying, except for a [configuration failure](operations.md#not-configured) |
| what it is told | `AGENT_BUS_NAME`, `AGENT_BUS_TOKEN` — its own, by design, since it talks to the bus itself — `AGENT_BUS_ADDR`, with the bus socket mounted in; `AGENT_BUS_WORK`, its writable work directory; compact `AGENT_BUS_CONFIG` at start when configured (otherwise unset); and its env files; not the bus secret, which it reads as its record's |
| a change | changed command, installed package files, Configuration, env values or private env-source classification restart it at `reload` |
| stopping | `SIGTERM` to its own processes, then its cgroup killed after its `deadline`, 10 s by default |
An agent can report that it is [not configured](operations.md#not-configured);
fix its settings and use `reload` before it is tried again.

The [inactive settings](security.md#what-is-accepted-but-inert) reference
lists accepted keys that have no effect yet.

</details>

## Inventory

An inventory is a JSON object keyed by package name or `package/instance`.
Each part uses letters, digits, `_`, `.` or `-`, starts with a letter, digit
or `_`, and has at most 64 characters; extra path segments are refused.
If no entry names a package, its installed directory supplies an enabled
default entry. Removing its entry therefore does not disable the package.
[agents.json.dist](agents.json.dist) holds every key with an example value, including reserved runner-written metadata for reference; never set those reserved keys by hand.

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
`idle` is still [inert](security.md#what-is-accepted-but-inert). A host removes a `depends` entry that runs elsewhere with `{"depends": []}` here; the list replaces the author's whole.

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
expanded. In `env.dist` only a trailing `# required` comment marks a key as
required (the marker is the whole comment or its first word). An unmarked key
is optional, including a bare `KEY=`. Any key may have a default. After all
layers, a required key must be nonempty; its own nonempty default satisfies it
unless a later layer empties it. Missing requirements report `needs KEY`.
Comment lines directly above a key describe it. `help-secret` reports required
or optional and `has default`, never the default's value.
A self-serving agent's `env.dist` only describes its keys: it injects no defaults
and enforces no requirements. That agent reads its own bus Secret. Runner-executed
scripts keep the normal defaults and required-key checks.

<details>
<summary>Quoting and comments</summary>

```dotenv
# Non-secret defaults go in env.dist
HOST=mail.example.com
FOLDER="INBOX/Work"
USER= # required
OPTIONAL=
MODE=prod # required
```

The syntax is the bus's one env-file grammar ([env-file syntax](../03-records-service.md#env-file-syntax)).
Put real secrets in a private host env file or the record's bus secret, never
in `env.dist`.

</details>

## Apply and check

Call the runner's `reload`. It waits up to 15 seconds (or the caller's shorter deadline) for restarted kept agents to finish retiring and attach their replacement readers. A replacement that is not ready is reported as `starting`; check `status` before using it. A refusal names the
agent and reason. Every file here can also be read, and the host's written, over
the bus ([the host's files](operations.md#the-hosts-files-over-the-bus)), which
applies at once. Follow [operations](operations.md#reading-status) for
commands and [troubleshooting](operations.md#why-an-agent-was-refused) for errors.
Overrides of an agent's other shipped JSON files remain in the
[runner plan](../../Plans/R1.0-Release/runner-config.md#overriding-a-shipped-file).
