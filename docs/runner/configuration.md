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

Merge these entries into your existing inventory, preserving other entries.
An entry naming a package not installed under `agent.d/` is refused.
A complete `name` is taken as written; otherwise the directory and runner's
realm supply the default. Leave metadata written by the runner alone.
The sandbox is on by default. `"sandbox": "off"` disables it for that entry;
read [security](security.md#isolation-today) before using that override.

</details>

## Environment files

Use `KEY=value`, one per line. Values are not shell code: `$VAR` is not
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

Single quotes are literal; double quotes support `\n`, `\t`, `\"` and `\\`.
Comments may occupy a line or follow a value after whitespace.
`export KEY=value` is also accepted. Put real secrets in a private host env
file or the record's bus secret, never in `env.dist`.

</details>

## Apply and check

Call the runner's `reload`, then check its `status`. A refusal names the
agent and reason. Follow [operations](operations.md#reading-status) for
commands and [troubleshooting](operations.md#why-an-agent-was-refused) for errors.
Planned override files and additional controls remain in the
[runner plan](../../Plans/R1.0-Release/runner-config.md#where-an-option-lives).
