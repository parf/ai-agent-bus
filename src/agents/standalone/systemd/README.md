# Systemd agent

📌 **TL;DR:** B.2 inspects and manages every unit on its host through fixed
systemctl commands. It runs under its own root unit; delivered caller roles
control actions, and exact unit-name validation prevents argument injection.

## Calls

Text accepts the methods below. JSON uses an exact `method` key plus only the
listed arguments. JSON answers contain `stdout`, `stderr`, `status` and `method`;
failures include `error`. Text returns output or `ok`; failures begin with
`error:`. Commands have a 30-second bound.

| Call | Arguments and answer |
|---|---|
| `list [type]` | All loaded units, including inactive ones. Optional type: service, timer, socket, mount, path, target, slice, scope, automount, swap or device. JSON: `{"method":"list","type":"timer"}`; type is optional |
| `timers` | Timer inventory with next and last runs. JSON: `{"method":"timers"}` |
| `failed` | Failed units with unit, load, active, sub and description columns. JSON: `{"method":"failed"}`; the answer also includes a `units` array with those fields |
| `status <unit>` | Unit status. JSON: `{"method":"status","unit":"backup.timer"}` |
| `start`, `restart`, `reload`, `stop`, `enable`, `disable` | One required unit; returns command output or `ok`. JSON uses method and unit, as for status |
| `logs [<unit>] [N] [since <age>]` | Journal entries; [filters below](#logs) |
| `help` | The caller's available methods, arguments, answers and examples; identical to `agent-bus call <name> --help` |

| Delivered role | Methods added |
|---|---|
| none | list, timers, failed, status |
| operate | start, restart, reload, logs |
| admin | stop, enable, disable |

Only delivered `AB_ROLE_OPERATE` and `AB_ROLE_ADMIN` grant these tiers.
Body fields cannot grant a role. There is no installed unit allowlist: every
unit type above is supported. Names must be exact, fully qualified names;
options, paths, globs, escapes and extra arguments are refused before systemctl
runs. An inactive unit's nonzero status is returned as an error.

## Logs

Logs require operate (or admin): journal entries can contain secrets.
Text accepts `logs [<unit>] [N] [since <age>]`, for example
`logs nginx.service 200 since 1h` or `logs 50`.
JSON accepts `{"method":"logs","unit":"backup.timer","lines":200,"since":"1h"}`,
with unit, lines and since optional. Without a unit, it reads the whole journal;
with a unit, it reads that exact unit's journal.

Lines is an integer from 1 to 1000, default 100. Since is a positive relative
age matching `^[1-9][0-9]{0,4}[mhd]$` (minutes, hours, days). Names and filters
are validated before fixed journalctl arguments are built. Returned log content
is capped at 256 KiB with a JSON `truncated` flag or a text truncation notice.

## Install

With the caller's administrative bus credentials available to the installer:

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN python3 src/agents/standalone/systemd/install.py install \
  --name '#systemd@host' --user root \
  --allow '#operator@host(operate),#administrator@host(admin)'
```

No unit-set file is used. `agent.json` supplies the description and `systemd`
tag, retained across restarts. Use `remove --name '#systemd@host'` with current
administrative credentials to remove the unit, token file and record.
The [shared starter](../starter.md#standalone-starter) owns credentials and lifecycle.

## Many nodes

After an error report, inspect status and recent logs across nodes, then restart
the failing units; the same calls support a restart after an autopull deploy.
With CLI 0.9.4 or later, [tagged calls](../../../../docs/user/cli.md#calling-many-agents-at-once)
provide discovery and triage:

```sh
agent-bus ls --tag systemd
agent-bus call --tagged systemd failed
agent-bus call '#systemd@node' 'status worker.service'
agent-bus call '#systemd@node' 'logs worker.service 100 since 1h'
# After inspecting the failure, with operate granted:
agent-bus call '#systemd@node' 'restart worker.service'
```

## Verification

The slow `agent_systemd` shard checks unconfigured services and timers, every
role tier, strict names, typed listing, failed-unit columns, logs, body forgery,
installation idempotence and removal against a private bus. Its unit controller
uses real root and nobody processes; its systemctl stand-in writes a root-owned
state file. Losing root makes every acting verb return a permission error while
read methods still work. This is an isolated privilege check, not a live systemd
deployment; it never changes host units. Mutants remove role enforcement, trust
a body role or hide a systemctl failure; each must fail its named assertion.
