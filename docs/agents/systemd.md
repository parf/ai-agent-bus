# Systemd: check and restart services

📌 **TL;DR:** Systemd is the part of Linux that starts and stops programs
running in the background (services). This helper lets you or your AI see
which services are broken, read their recent logs and restart them, without a
server login. Each team can get its own helper limited to its own services.

## Why you'd want it

**The problem.** A broken service is often fixed by a look at its log and a
restart, but that needs a server login with administrator rights, which you do
not want to hand out.

**The idea.** One helper per team on the server, with a short list of services
it may touch. Looking is open to its access list; restarting needs an extra
permission. It never restarts anything by itself.

**When to use it**

- Check every server for failed services in one call.
- Let a team or an AI restart its own application.
- Check that a scheduled job (a timer) ran.

**When not to:** installing software, editing service settings, or general
server administration.

**Possible use cases**

| Use case | What you do | What you get |
|---|---|---|
| Morning check | Ask all servers for failed services | Failures grouped by server |
| A deploy failed | Read the service's last hour of logs | The error that stopped it |
| After a [git pull](git.md#use-examples) | Restart the service | It runs the new code |
| Backup check | Ask about the backup timer | Last and next run |
| Team self-service | Give a developer their own helper | They restart their app, nothing else |

## What it does

| Ability | Calls |
|---|---|
| Look | `list`, `timers`, `failed`, `status` |
| Read logs | `logs` |
| Act | `start`, `restart`, `reload`, `stop`, `enable`, `disable` |

Who may do which is under [Who may use it](#who-may-use-it). A limited helper
refuses services outside its list.

## You'll need

- Linux with systemd and the bus on the server.
- An administrator to install it.

## Try this first

```sh
agent-bus help '#systemd/web@node.example'
```

See [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

## Use examples

| I want to… | Command |
|---|---|
| Find failures on all servers | `agent-bus call --tagged=systemd failed` |
| Check one service | `agent-bus call '#systemd@node.example' 'status nginx.service'` |
| Read its last hour of logs | `agent-bus call '#systemd@node.example' 'logs nginx.service 100 since 1h'` |
| Restart it | `agent-bus call '#systemd@node.example' 'restart nginx.service'` |
| See scheduled jobs | `agent-bus call '#systemd@node.example' timers` |
| See my team's services | `agent-bus call '#systemd/web@node.example' list` |

<details><summary>Rules for these calls</summary>

- Inspect the cause before restarting; check `status` afterwards.
- With many servers, a missing reply or refusal fails the [multi-agent call](../user/cli.md#calling-many-agents-at-once).
- Use the exact timer name from `timers`; reading it needs no extra permission.

</details>

## Installing it

An administrator installs it once per team as root, lists the team's services
and grants restart permission to the right people.

<details><summary>Step by step</summary>

With [installation credentials](README.md#standalone-helpers):

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  python3 src/agents/standalone/systemd/install.py install \
  --name '#systemd/web@node.example' --user root --allow '@owner'
agent-bus config '#systemd/web@node.example' '{"units":"nginx.service,web-worker.service,backup.timer"}'
agent-bus manage '#systemd/web@node.example' --add-allow 'ann@node.example(operate)'
unset AGENT_BUS_TOKEN
```

- The installer prints `agent-bus-systemd-web.service`; `list` then shows only listed services loaded on the host.
- A whole-host helper: install `#systemd@node.example` with no `units`.
- Remove: same installer, `remove --name '#systemd/web@node.example'`, with current credentials.

</details>

## Configuration

One setting: the list of services this helper may touch. Without it, every
service on the host.

<details><summary>Keys</summary>

| Key | Meaning |
|---|---|
| `units` | Comma-separated exact names, such as `nginx.service,backup.timer`. Omitted: every host unit |

Names need their ending (`.service`, `.timer`); no wildcards or paths. An empty or invalid list is an error, never whole-host access.

</details>

### Dedicated instances

Make one named helper per team or application, each with its own service list
and access list; listing, status, actions and logs stay inside that list.
Changes apply on the next call.

## Secret keys

None. The installer keeps the helper's own bus credential in a protected file;
never put a person's token in its settings.

## Who may use it

Permissions come from the bus, never from the call's text. Owner and
Maintainers have every permission; the service list still binds everyone.

| Permission | Allows |
|---|---|
| On the access list | `list [type]`, `timers`, `failed`, `status <unit>` (no log lines) |
| `operate` | Also `start`, `restart`, `reload`, `logs` |
| `admin` | Also `stop`, `enable`, `disable` |

Do not grant `admin` to someone who only needs restart.

### Logs

Logs can hold private data, so they need `operate` or `admin`. Without a
service name they cover this helper's list, or the whole host if it has none.

<details><summary>Technical details: logs</summary>

- 1–1000 lines, default 100. `since` is an age such as `30m`, `1h`, `2d`. Large answers say when they were shortened.
- JSON form: `{"method":"logs","unit":"nginx.service","lines":100,"since":"1h"}`.

</details>

### Many nodes

Find the helpers with `agent-bus ls --tag=systemd`; `call --tagged=systemd`
asks them all, and each server answers separately.

## Troubleshooting

| If you see | Do this |
|---|---|
| "requires operate" | Ask the owner for `operate` on this helper |
| A unit refused by name | Check spelling and this helper's `units` |
| Permission denied from systemctl | Reinstall with `--user root` |
| Status is an error | Inactive or failed units return nonzero; read the text |
| Configuration error | Fix `units`; blank entries and partial names are refused |
