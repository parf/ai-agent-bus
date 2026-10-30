# Configuration: every setting and secret in its one place

📌 **TL;DR:**
- **Problem:** settings scattered through code and servers leak secrets and break on every update.
- **Outcome:** the code says how an agent runs; host files say which names run here and hold their private values; the bus keeps who may call.
- **Start:** ask an agent what it expects ([Try this first](#try-this-first)), then put each value where [the table](#where-settings-belong) says.

## Why you'd want it

- 🔄 **An update overwrites local settings.** Now host files live outside the code, so updating code never touches them.
- 🔑 **Passwords sit in the checkout.** Now they live in a private host file or the record's Secret, never in the code.
- 🎛️ **Tuning a tool means editing its code.** Now the host overrides run options such as deadlines in its own small file.
- 🧯 **A typo starts a broken agent.** Now a wrong key or a missing required value refuses that agent by name.

## What you get

- ✅ **One home per value:** code, host files or the bus record, never two of them.
- ✅ **Enable, disable, rename and instances** from one inventory file.
- ✅ **Limits per agent:** memory, CPU, processes, open files, disk I/O and disk use.
- ✅ **Safe edits:** apply everything with one `reload`; refusals name the agent and the reason.
- ⚠️ **Access is not here:** who may call stays on the bus record, edited with ordinary bus tools.

## Try this first

Ask an agent which settings it takes:

```sh
agent-bus help --no-color '#ai@parf.us' config
```

```text
base_url (string): Trusted OpenAI-compatible API root; chat/completions is appended for ask, ...; default: "https://openrouter.ai/api/v1"
model (string): Default chat model ID for ask/text and chat JSON requests without model; ...; example: "openai/gpt-4.1-mini"
```

Ask which secrets it needs; values are never shown:

```sh
agent-bus help --no-color '#ai@parf.us' secret
```

```text
OPENROUTER_API_KEY (required): Provider API key, required. Set only in runner/ai/env on the host, never bus Secret or Configuration.
```

- This ran on the development node on 2026-10-30.

## Where settings belong

- 📦 `agent.d/` is the shared package folder, normally `/var/lib/agent-bus/agent.d`.
- 🏠 `runner/` below means the folder given to the runner as `--host`, normally `/var/lib/agent-bus/<account>-ab-runner`; do not add another `runner/` under it.

| File | Holds |
|---|---|
| `agent.d/<agent>/agent.json` | Command, input form and description, from the author |
| `agent.d/<agent>/env.dist` | Non-secret defaults and the list of required variables |
| `runner/agents.json` | Optional [inventory](#inventory): enable, disable, rename or declare instances |
| `runner/<agent>/env` | Host settings shared by every instance of the agent |
| `runner/<agent>/<instance>/env` | Settings for one instance |
| `runner/<agent>[/<instance>]/agent.json` | The host's [override](#overriding-the-authors-options) of run options |
| `runner/<agent>[/<instance>]/<file>.json` | The host's [override](#overriding-a-settings-file) of another JSON file the agent ships |

- 🧩 An agent-wide env file holding every required key makes the plain agent runnable without an instance.
- 🛡️ The bus record owns its access list, Maintainers and Secret.
- 🔐 File permissions and which secret wins: [security](security.md#secrets).

## Record configuration

- ⚙️ **Configuration** holds JSON settings; **Secret** holds secret values ([configuring a template](../03-records-agent.md#configuring-a-template)).
- ✏️ The Owner and Maintainers edit both on the bus or the web pages.
- 📨 Each new process gets the settings as compact `AGENT_BUS_CONFIG`, unset when none.
- 🔁 A per-message script sees a change on its next message; a kept process after a restart or `reload`.

## Agent directories

The runner reads shared packages first, then bundled packages. The first package with a name wins, including when that package is refused; removing it lets the next `reload` fall back to the bundled copy.

| Source | Default | Starts without an inventory entry? |
|---|---|---|
| Shared packages (`--agents`) | `/var/lib/agent-bus/agent.d` | Yes |
| Bundled packages (`--bundled`) | `/usr/local/lib/agent-bus/current/agents` on a release node; the checkout's `src/agents` when running from git | No: set `enabled:true` in `agents.json` |

- Bundled manifests under `standalone/` are included; support directories without a manifest are skipped. New bundled agents appear at the next `reload`.
- Each status row's `agent_dir` names the selected package directory.
- A bundled agent without an explicit enable is `available`, with no reader or process. Explicitly disabling it shows `disabled`.

Commands, default env values and read-only author files come from the selected package. Host settings and overrides keep their paths under `runner/<agent>[/<instance>]/`.

## Package options

`agent.d/<agent>/agent.json` belongs to the author:

- ✅ `command` and `description` are required; every other key is optional.
- 🚫 An unknown key refuses the package by name.
- 📋 [agent.json.dist](agent.json.dist) shows every key with an example value.

| Key | Meaning |
|---|---|
| `command` | Shell command line; `./` paths resolve inside the package |
| `description` | Nonempty text registered on the bus; missing or blank refuses the package by name |
| `form` | How a message reaches the script: `args` or `json` (default) |
| `serves` | `runner` (default): a script per message; `itself`: kept processes that read their own inbox ([below](#agents-that-serve-themselves)) |
| `version` | Own version shown in help; bundled agents use the shared bus version instead |
| `license`, `author`, `homepage` | License (SPDX), author and documentation link shown in help |
| `body_formats`, `arguments`, `methods`, `config` | Call, method and Configuration descriptions ([package help](#package-help)) |
| `processes` | Most scripts at once, a count or `min..max`, default `0..1`; the runner enforces the maximum. An agent that serves itself takes a count |
| `deadline` | Longest run, such as `30s`; this or the caller's deadline, whichever comes first, ends it. For an agent that serves itself: how long a stop waits before killing, default `10s` |
| `tags` | Labels, joined with inventory tags and the automatic `runner` label ([tag rules](../03-records.md#tags)) |
| `roles` | Roles the agent understands, registered on its record; grants no caller any role |
| `depends` | Agents started before this one, as bus names (`#db`); see below |
| `needs.network` | A nonempty host list turns network access on; the list itself does not limit which hosts |
| `needs.memory` | Memory the agent needs, as `2G`; above the host's `limits.memory` the agent is refused by name |

<details>
<summary>How <code>depends</code> works</summary>

- It orders starts among this runner's agents only.
- It does not wait for the other agent to be ready.
- A name not installed here, or a cycle, refuses the agent at `reload`, naming it.
- A realm in a name (`#db@host`) is not checked.
- An agent still starts when what it depends on is refused or disabled.
- `status` lists agents in start order.

</details>

## Package help

- 📖 The runner answers `help`, `help-config`, `help-secret` and `help-json` from `agent.json`, without running the script.
- 🧠 Set `"help":"native"` when the script must shape its own help from live Configuration, as `#systemd` and `#git` do.
- 🔇 Set `"asks":"none"` to answer every other ask with `unsupported: <ask>`; `agent-bus run` has `--no-asks` for the same.
- 🧪 `agent-bus run --package-dir /path` loads a package the same way; the standalone starter uses it.
- ✍️ Write good descriptions with the [authoring checklist](../agents/authoring.md#checklist-writing-an-agents-description-and-help); start from the [create-agent prompt](../agents/create-agent-prompt.md#prompt-to-paste).
- 📜 The contract: [agent self-description](../constitution.md#agent-self-description).

<details>
<summary>Asks, status and tools</summary>

- With `"help":"native"`, all four help asks run through the script with the normal context, Configuration and delivered roles.
- Without it, the runner renders package metadata itself, and answers `mcp` with `{"tools":[]}`.
- Offering tools needs a package with `"help":"native"`.
- Every other ask, `status` included, reaches the script as `AGENT_BUS_ASK` with an empty body.
- An unpackaged script receives every ask except `mcp`, which is answered with `{"tools":[]}`.
- Neither policy stops the metadata answers.
- A running script handles asks before its normal flow; an unknown ask gets an ordinary unsupported reply.
- While it holds a reader, the runner answers `status` itself during start (`starting`), shutdown (`stopping`) or failure (`failed`).
- A normally running agent answers its own status; with asks off it answers unsupported.
- A refused or unconfigured agent has no reader, so its ordinary messages stay queued.
- An agent that serves itself answers help in its own process.

</details>

<details>
<summary>Describing calls, methods and Configuration</summary>

- `body_formats` lists accepted call bodies: `text`, `json` or both; default `["text"]`.
- It describes the call body, not help output or answer encoding.
- A JSON body has `method` and the named method arguments; a text-only agent does not advertise JSON.
- `methods` is keyed by method name, `config` by Configuration name.
- Each entry has nonempty `help` and an optional `type`: `string` (default), `int`, `bool`, `list`, `json`, or a list of enum choices.
- A method's `args` are named entries with `help`, `type`, `optional` and `rest` (last argument only).
- Methods may state `returns`, `role`, `destructive`, a usage line `text` and a copy-paste `example`.
- Shared arguments live in top-level `arguments`; a method argument without `help` points to one.
- Argument `default` documents a default; `json_only` keeps a field out of the text syntax but in machine help.
- Methods keep their written order.
- A `role` is one name or a list, any of which grants the method; tiers are spelled out, never inferred.
- Configuration keys may state `example` and `default`.
- Unknown fields, bad types and duplicate argument names are refused by name.
- Unknown Configuration keys and values outside an enum are warned about, not refused.
- These describe calls; the agent still checks every call itself.
- Help hides methods whose role the caller lacks; the Owner and Maintainers see every method.

</details>

<details>
<summary>What each help answer contains</summary>

- `help`, `help-config` and `help-secret` answer text.
- `help-json` answers one object: description, license, version, author, homepage, methods, asks, config, secrets, `body_formats` and `hidden_methods`.
- `hidden_methods` holds only a `count` and the sorted required role names, never method names, descriptions, arguments or examples.
- Native scope limits apply before the count; the Owner and Maintainers see zero.
- Secrets are name, help, required and has-default flags, never values.
- Optional identity fields are left out when absent.
- No host env file or stored Secret is read to build help.

</details>

## Agents that serve themselves

`"serves": "itself"` is a long-lived program that reads its own inbox through a client library, such as the [Slack agent](../../src/agents/slack/README.md). The runner keeps it running instead of starting it per message.

- 🧭 Build and check one with the [create-agent prompt](../agents/create-agent-prompt.md#prompt-to-paste).
- 🪪 Register its record yourself and hand its token once into `runner/tokens/`, as for any [existing agent](security.md#what-runs-as-what); until then it is refused by name.
- 🔁 A process that exits is restarted after 1 s, doubling to 30 s if it keeps dying.
- 🩹 An agent that reports it is [not configured](operations.md#not-configured) waits: fix its settings and `reload`.

<details>
<summary>Processes, environment, changes and stopping</summary>

| | |
|---|---|
| Processes | A fixed count (`processes` as a number), each in the agent's cgroup and sandbox |
| What it is told | `AGENT_BUS_NAME`; its own `AGENT_BUS_TOKEN`, since it talks to the bus itself; `AGENT_BUS_ADDR` with the bus socket mounted in; `AGENT_BUS_WORK`, its work folder; `AGENT_BUS_CONFIG` at start when configured; its env files. Not the bus Secret: it reads that as its record |
| A change | A changed command, package files, Configuration, env values or private env-source classification restarts it at `reload` |
| Stopping | `SIGTERM` to its processes, then its cgroup is killed after its `deadline`, 10 s by default |
| A bus restart | Only the socket is mounted, never its folder, whose account sockets would identify it as the runner. A new socket cannot appear in the mount, so each sandboxed process is stopped and started again within about 2 s of the new socket |
| `env.dist` | Only describes its keys: no defaults are injected and no requirements enforced |

</details>

## Inventory

`runner/agents.json` is a JSON object keyed by package name or `package/instance`:

- 📦 Shared packages run by default, even with no entry. Bundled packages require explicit `enabled:true` ([agent directories](#agent-directories)).
- 🔤 Each name part uses letters, digits, `_`, `.` or `-`, starts with a letter, digit or `_`, and has at most 64 characters.
- 🚫 Extra path parts, or an entry for a package not found in the agent directories, are refused.
- 📋 [agents.json.dist](agents.json.dist) shows every key with an example value.

| Key | Meaning |
|---|---|
| `enabled` | `false` keeps the entry installed but unserved; default `true` for shared packages and `false` for bundled packages. The runner's `enable` and `disable` [calls](operations.md#controlling-one-agent) write it |
| `name` | A complete bus name, taken as written; default `#<key>@<realm>` |
| `tags` | Labels added to the package's own |
| `sandbox` | `on` (default) or `off` for this entry only; read [security](security.md#isolation-today) first |
| `limits` | The whole agent's [limits](#limits) |
| `version`, `origin`, `first_started`, `last_started` | Reserved for the runner; never set by hand |

Disable a package and give an instance its own name; merge these into your existing file:

```json
{
  "hello": {"enabled": false},
  "mail/work": {"enabled": true, "name": "#work-mail@your-realm"}
}
```

## Overriding the author's options

- 🛠️ The host changes run options without touching the checkout.
- 📄 `runner/<agent>/agent.json` applies to the agent and every instance; `runner/<agent>/<instance>/agent.json` to one instance, after it.
- 🧩 Each is a JSON Merge Patch of `agent.json`; `null` removes a key, back to the built-in default.

| May override | May not |
|---|---|
| `processes`, `deadline`, `depends` | Everything else: `command`, `form`, `serves` and the rest describe what the code is |

Give the agent three processes and two minutes per run:

```json
{"processes": 3, "deadline": "2m"}
```

- 🚫 A key that may not be overridden refuses the agent at `reload`, naming the file and the key; the runner's `write` call refuses it before writing.
- 🔗 `{"depends": []}` drops a dependency that runs elsewhere; the list replaces the author's whole list.

## Overriding a settings file

- 🛠️ An agent that ships its own settings in a JSON file reads them where it always did; the host changes them without editing the package.
- 📄 `runner/<agent>/<file>.json` applies to the agent and every instance; `runner/<agent>/<instance>/<file>.json` to one instance, after it.
- 🧩 Each is a JSON Merge Patch of the shipped file: nested objects merge, `null` removes a key, a list replaces the shipped list whole.
- 🔒 The runner mounts the merged file read-only over the shipped path inside the sandbox; the file on disk stays the author's.

Turn one nested setting off and keep the rest, in `runner/hello/settings.json`:

```json
{"retry": {"enabled": false}}
```

- 🚫 An override of a file the package does not ship refuses the agent at `reload`, naming the file.
- 🛡️ An agent running with the sandbox off is refused too: there is nowhere to mount the file.
- 🔄 A changed override restarts a kept agent at `reload`; a one-message agent reads it on its next run.

## Limits

An entry's `limits` bound the whole agent, all its processes together, in its own cgroup ([isolation](security.md#isolation-today)). Each key is optional.

| Key | Means | Default |
|---|---|---|
| `memory` | Memory, as `512M` or `1G`; no swap; also the size of its `/tmp` | `512M` |
| `cpu` | A share of one CPU, as `150%` | unbounded |
| `processes` | Processes and threads at once | `64` |
| `files` | Open files, per process | `1024` |
| `io_weight` | Disk I/O weight, 1–10000 | the kernel's, 100 |
| `disk` | What its work folder may hold, measured after each run; above it the agent stops until emptied and reloaded | `1G` |

Give the work mail agent more room:

```json
{"mail/work": {"limits": {"memory": "1G", "cpu": "200%", "processes": 32}}}
```

- 🚫 A malformed limit, or a `needs.memory` above `memory`, refuses the agent by name.
- 🔁 A change applies at `reload`, to running processes too.

## Environment files

- 📝 One `KEY=value` per line, in the bus's [env-file syntax](../03-records-service.md#env-file-syntax).
- 🚫 Values are not shell code: `$VAR` is not expanded.
- ✅ In `env.dist` only a trailing `# required` comment marks a key required (the whole comment or its first word).
- 💤 An unmarked key is optional, even a bare `KEY=`; any key may have a default.
- 🔗 Layers stack; a required key must end up nonempty, and its own default counts unless a later layer empties it.
- 🩹 A missing requirement reports `needs KEY`.
- 💬 Comment lines right above a key describe it; `help-secret` shows required or optional and `has default`, never the value.
- 🔑 Real secrets go in a private host env file or the record's Secret, never in `env.dist`.
- 🔌 An agent that serves itself reads its own Secret; its `env.dist` only describes keys.

<details>
<summary>An env.dist with quoting and comments</summary>

```dotenv
# Non-secret defaults go in env.dist
HOST=mail.example.com
FOLDER="INBOX/Work"
USER= # required
OPTIONAL=
MODE=prod # required
```

</details>

## Apply and check

- 🔁 Call the runner's `reload`; a refusal names the agent and the reason.
- ⏳ It waits up to 15 seconds, or the caller's shorter deadline, for restarted kept agents to come back.
- 🟡 One not ready yet shows `starting`; check `status` before using it.
- 🌐 Every file here can also be read, and the host's ones written, over the bus ([the host's files](operations.md#the-hosts-files-over-the-bus)); a write applies at once.
- 🩺 Commands: [reading status](operations.md#reading-status); errors: [why an agent was refused](operations.md#why-an-agent-was-refused).

## Technical details

<details>
<summary>Order and precedence</summary>

- Env layers apply in order: `env.dist`, the record's Secret, `runner/<agent>/env`, then `runner/<agent>/<instance>/env`; each later one wins.
- Option overrides apply in order: the author's `agent.json`, the agent-wide override, then the instance override.
- Which secret source wins, and file permissions: [security](security.md#secrets).
- Runner-started scripts get the `env.dist` defaults and required-key checks; an agent that serves itself does not.

</details>
