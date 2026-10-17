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

The bus record owns ACLs, Maintainers and its bus secret. Those are edited
with ordinary bus tools, not put into the runner's inventory.
File permissions and secret precedence are defined in
[security](security.md#secrets).

## Package options available today

Use only what your agent needs. Unknown keys are refused; some known keys
are accepted for future features without applying them.

<details>
<summary>Supported package settings</summary>

| Key | Meaning |
|---|---|
| `command` | Required shell command line; `./` paths resolve within the package |
| `form` | `args` or `json`; omitted means `json` |
| `description` | Description registered on the bus |
| `processes` | Maximum concurrent scripts; default `0..1`. A count or `min..max` is accepted, but only the maximum is enforced today |
| `deadline` | Maximum run duration, such as `30s`; whichever comes first, this limit or the caller's deadline, ends the run |
| `tags` | Package labels, combined with host-inventory tags and the automatic `runner` label; [update rules](../03-records.md#tags) |
| `roles` | Registered on the record as the roles the agent understands; it grants no caller any role |
| `needs.network` | A nonempty host list enables network access today; it does not restrict access to those hosts yet |

Other forms and `serves: "itself"` are refused as unsupported.
The [inactive settings](security.md#what-is-accepted-but-inert) reference
lists accepted keys that have no effect yet.

</details>

## Inventory

An inventory is a JSON object keyed by package name or `package/instance`.
If no entry names a package, its installed directory supplies an enabled
default entry. Removing its entry therefore does not disable the package.

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

Quotes are removed and keep the whitespace inside; in double quotes `\n` and
`\r` become a newline and a carriage return, and a quoted value may span lines.
An unquoted value is trimmed and ends at a `#`, which begins a comment.
`export KEY=value` is also accepted. Put real secrets in a private host env
file or the record's bus secret, never in `env.dist`.

</details>

## Apply and check

Call the runner's `reload`, then check its `status`. A refusal names the
agent and reason. Follow [operations](operations.md#reading-status) for
commands and [troubleshooting](operations.md#why-an-agent-was-refused) for errors.
Planned override files and additional controls remain in the
[runner plan](../../Plans/R1.0-Release/runner-config.md#where-an-option-lives).
