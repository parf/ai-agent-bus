# Runner: agent configuration options

Status: **partly built.** The approved file formats are read by the runner;
several options and override merging remain planned. Current operating
instructions and supported settings belong to the
[configuration guide](../../docs/runner/configuration.md).
This page retains the full target and approved format design ([№Q155](DECISIONS.md#recorded-decisions)).

📌 **TL;DR:** Every requirement in the runner's plans lands on one of four
homes: the author's `agent.json`, the host's `agents.json`, an instance's
`env`, or the bus record. Each option has one home, or an author default the
host overrides with a file of the same JSON. Four older names are replaced.

## Where an option lives

| Home | Owned by | Holds |
|---|---|---|
| `agent.d/<agent>/agent.json` | the author, with the code | what the code is and can do |
| `agent.d/<agent>/env.dist` | the author | the variables it reads ([secrets](runner.md#secrets)) |
| `runner/agents.json` | the host | the inventory, and what only the host decides: enabled, name, pool, limits, sandbox |
| `runner/<agent>[/<instance>]/agent.json` | the host | an [override](#overriding-a-shipped-file) of the author's run options |
| `runner/<agent>[/<instance>]/env` | the host | variables and secrets |
| the runner's unit | the host | which bus the runner serves, the account it runs as ([runner unit](runner-implementation.md#runner-unit)) |
| the bus record | the Owner, on the bus | ACL, record secret, `config`, `deliver_to`: never runner configuration |

**Rule:** a property of the code is the author's; a decision about this host
is the host's. Where both have a say, the author gives the default and the
host overrides it with an override file of the same JSON, as `depends` already
works ([what it comes after](runner-implementation.md#what-it-comes-after)).
An option the author never ships has no override; it is the host's alone.

## The author's options

| Option | Says | Requirement it comes from |
|---|---|---|
| `description` | one line for the record | registration |
| `version` | what is installed, for a script that cannot answer a `version` call | [version reporting](runner-implementation.md#agent-version-reporting) |
| `command` | how to start it; the only place it is written | [what an instance is](runner-implementation.md#what-an-instance-is) |
| `form` | `args`, `json`, `std`, `jsonl`, `msgpack`; decides one-message or kept | [forms](runner-implementation.md#additional-script-forms) |
| `serves` | `runner` (the default): created by the runner for its person and read on its own inbox, as itself. `itself`: links a client library, registers and reads its own inbox; the runner only starts it and restarts it when it dies | [№Q151](DECISIONS.md#recorded-decisions), settled 2026-10-16 |
| `processes` | the default count or range | [kinds of agents](runner.md#kinds-of-agents) |
| `in_flight` | how many messages one kept process may hold, echoing topic and tag; default 1 | [№Q13](DECISIONS.md#recorded-decisions), [long-lived agents](runner-implementation.md#long-lived-agents) |
| `deadline` | how long one message may take before the process is killed | [long-lived agents](runner-implementation.md#long-lived-agents) |
| `idle` | how long a process above `min` waits before `SIGTERM`; default 5 minutes | [№Q154](DECISIONS.md#recorded-decisions), settled 2026-10-16 |
| `roles` | the [roles](../../docs/03-records-agent.md#roles) it understands | roles |
| `depends` | what it comes after | [what it comes after](runner-implementation.md#what-it-comes-after) |
| `needs` | what the code needs to work: the network (and to which hosts), a memory size, a writable path beyond its work directory | [sandboxing](runner.md#sandboxing) |

Not an option, because it is derived: whether the agent is a template (an
`env.dist` variable with no default), and its lifetime (its `form`).

## The host's options

What only the host decides, one row per installed agent or instance in
`agents.json`:

| Option | Says |
|---|---|
| `enabled` | started by the runner, or installed and left alone |
| `name` | a complete name, taken whole; default the directory and this host |
| `share` | this copy joins a pool of one name across hosts |
| `limits` | memory, CPU, processes, open files, I/O weight, disk, network, for the whole agent; a `needs` above them is refused; default the built limits |
| `sandbox` | `off` for this agent only, said out loud at start; on otherwise |
| `version`, `origin`, `first-started`, `last-started` | written by the runner, never by hand |

What the host overrides of the author's, in `runner/<agent>[/<instance>]/agent.json`:

| Option | Says |
|---|---|
| `processes` | this host's count or range |
| `idle`, `deadline` | this host's times |
| `depends` | entries added, or turned off as remote |

## What `serves: itself` changes

| | `serves: runner` | `serves: itself` |
|---|---|---|
| registers the name | the runner, with `deliver_to` to itself | the agent, with its own token |
| holds a token | no | yes: a variable in its instance `env` |
| `form`, `in_flight`, `deadline` | used | ignored: the runner sees no messages |
| `processes` | count or range | count only: the runner cannot see waiting messages, so it cannot grow a pool |
| `SIGTERM`, `SIGHUP` | sent to an idle process | the same signals; `SIGTERM` only when the runner stops, since a fixed count has no process above `min` to shrink ([stop and reload](runner.md#stop-and-reload)) |
| when it dies | restarted, with backoff | the same |

## Overriding a shipped file

The rule is [overriding a shipped file](runner.md#overriding-a-shipped-file) ([№Q152](DECISIONS.md#recorded-decisions));
the details ([№Q155](DECISIONS.md#recorded-decisions)):

| | |
|---|---|
| merge | JSON Merge Patch (RFC 7386): the override has the shipped file's shape and names only what changes; nested objects merge, `null` removes a key, a list replaces the shipped list whole |
| layers | the patches apply in layer order: the agent's override to the shipped file, then the instance's to that result. A key removed with `null` falls back to the built default, not to an earlier layer |
| delivery | the runner builds the merged file and mounts it read-only over the shipped path inside the [sandbox](runner.md#sandboxing), which bubblewrap does ([sandboxing](runner-implementation.md#sandboxing)); with the sandbox off for that agent, it is refused, since there is nowhere to mount it |
| `agent.json` too | the host overrides the author's run options (`processes`, `idle`, `deadline`, `depends`) with an override of `agent.json` itself, as in [the host's options](#the-hosts-options); what the author never ships, such as `limits`, stays in `agents.json` |
| what may not be overridden | what the code is: `command`, `form`, `serves` and the rest of the author's options. An override naming one is refused at `reload`, by name; a host that wants other code forks it |
| not JSON | a shipped settings file in another format is not overridden; its agent reads variables instead |

## File formats

Approved 2026-10-16 for [№M.1](TODO.md#runner-waves). Every field is optional except `command`; an
unknown field is refused at `reload`, by name.

<details>
<summary><code>agent.d/&lt;agent&gt;/agent.json</code>, the author's</summary>

```json
{
  "description": "Fetches one mailbox",
  "version": "1.2.0",
  "command": "./fetch.sh",
  "form": "json",
  "serves": "runner",
  "processes": "0..1",
  "in_flight": 1,
  "deadline": "30s",
  "idle": "5m",
  "roles": ["admin"],
  "depends": ["#db"],
  "needs": { "network": ["imap.example.com"], "memory": "256M", "writable": ["cache"] }
}
```

`processes` is a number (`33`) or a range in a string (`"10..33"`); times are
Go durations.

</details>

<details>
<summary><code>runner/agents.json</code>, the host's, keyed by <code>&lt;agent&gt;</code> or <code>&lt;agent&gt;/&lt;instance&gt;</code></summary>

```json
{
  "mail-fetcher/work": {
    "enabled": true,
    "name": "#mail-fetcher/work@pool1",
    "share": false,
    "sandbox": "on",
    "limits": { "memory": "1G", "cpu": "200%", "processes": 64, "files": 1024, "io_weight": 100, "disk": "1G" },
    "version": "1.2.0",
    "origin": { "remote": "https://…", "commit": "abc123" },
    "first_started": "2026-10-16T21:00:00Z",
    "last_started": "2026-10-16T21:00:00Z"
  }
}
```

The runner writes `version`, `origin`, `first_started` and `last_started`.

</details>

<details>
<summary>The answer to a <code>status</code> call</summary>

```json
{
  "runner": "#runner@h",
  "version": "0.8.113",
  "agents": [
    { "name": "#mail-fetcher/work@h", "enabled": true, "state": "running",
      "processes": { "running": 1, "min": 0, "max": 1 } },
    { "name": "#broken@h", "enabled": true, "state": "refused",
      "reason": "agent.json: unknown field \"comand\"" }
  ]
}
```

`state` is `running`, `idle`, `stopped`, `disabled` or `refused`; `reason`
only with `refused`.

</details>

<details>
<summary>The env files: <code>env.dist</code>, <code>runner/&lt;agent&gt;/env</code>, an instance's <code>env</code>, a record's bus secret</summary>

Approved 2026-10-16 for [№M.2](TODO.md#runner-waves) and revised 2026-10-17 (Q171): the bus's
[env-file syntax](../../docs/03-records-service.md#env-file-syntax), one parser shared with the daemon's
check on a secret. `github.com/joho/godotenv` was approved first, but it
expands `$VAR` from earlier lines, so an unquoted `pa$word` would arrive as
`pa`; the owner chose no expansion over the library.

```sh
# a whole-line comment
IMAP_HOST=imap.example.com      # a comment after an unquoted value
export IMAP_PORT=993            # export is allowed
IMAP_FOLDER="INBOX/Work"        # double quotes: \n and \r become a newline and a CR
IMAP_PASSWORD='p@ss#word'       # single quotes: literal, # is no comment
IMAP_USER=                      # an empty value
```

`$VAR` is never expanded: a value is what is written. In `env.dist` an empty
value means required, which is what makes a template; an empty default cannot
be stated. Layers apply lowest first, `env.dist`, the record's bus secret,
`runner/<agent>/env`, the instance's `env`, and a variable still required after
the last refuses the instance at `reload`, by name.

</details>

## Replaced older names

| Older name | Collided with | Now |
|---|---|---|
| `config.json` ([what an instance is](runner-implementation.md#what-an-instance-is)) | `agent.json` on the runner page | one file, `agent.json` |
| `autostart: on · off · on-demand` ([the list](runner-implementation.md#the-list-of-what-is-installed)) | `processes` with `min` 0 is on demand | `enabled` on or off; on demand is `processes: 0..N` |
| `-N` on a managed agent | `processes` | `processes`; `-N` stays the dedicated runner's option |
| the author naming the agent (runner page) | the implementation's rule that only the directory and the host name it | the host's `name` only |
