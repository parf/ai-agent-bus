# Systemd: check and restart services

📌 **TL;DR:**
- **Problem:** a broken service is often fixed by a look at its log and a restart, but that needs a server login with administrator rights you do not want to hand out.
- **Outcome:** you or your AI see which services are broken, read their recent logs and restart them, without a login; each team gets a helper limited to its own services.
- **Start:** ask what the helper can do: `agent-bus help '#systemd/web@node.example'` ([Try this first](#try-this-first)).

## What it does

- ℹ️ **Systemd** is the part of Linux that starts and stops programs running in the background (services).
- ✅ **Look:** list services, scheduled timers, failures and one service's state (`list`, `timers`, `failed`, `status`).
- ✅ **Read logs:** the last lines of a service, for people with the right permission (`logs`).
- ✅ **Act:** start, restart or reload a service; stop, enable or disable with a higher permission (`start`, `restart`, `reload`, `stop`, `enable`, `disable`).
- ✅ **Many servers:** ask every tagged helper at once; each server answers separately.
- 🧱 **Limited helpers refuse services outside their list;** who may do what is under [Who may use it](#who-may-use-it).
- 🚫 **Not for:** installing software, editing service settings or general server administration.

## You'll need

- Linux with systemd and the bus on the server.
- An administrator to install it.

## Try this first

Use the complete name of your [enabled instance](#installing-it) in the commands below.

- Ask the helper what it can do:

```sh
agent-bus help '#systemd/web@node.example'
```

## Use examples

- **Find failures on all servers:** ask every tagged helper for `failed`.
- **Check one service:** ask for `status` and its name, such as `nginx.service`.
- **Read its last hour of logs:** ask for `logs`, the service, a line count and `since 1h`.
- **Restart it (for example after a [git pull](git.md#use-examples)):** ask for `restart` and its name.
- **See scheduled jobs:** ask for `timers`.
- **See my team's services:** ask your team's helper for `list`.

## Installing it

- An administrator installs it once per team as root.
- Then the administrator lists the team's services and grants restart permission to the right people.

<details>
<summary>Step by step</summary>

With [installation credentials](README.md#standalone-helpers):

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  python3 /usr/local/lib/agent-bus/current/agents/standalone/systemd/install.py install \
  --name '#systemd/web@node.example' --user root --allow '@owner'
agent-bus config '#systemd/web@node.example' '{"units":"nginx.service,web-worker.service,backup.timer"}'
agent-bus manage '#systemd/web@node.example' --add-allow 'ann@node.example(operate)'
unset AGENT_BUS_TOKEN
```

- The installer prints `agent-bus-systemd-web.service`; `list` then shows only listed services loaded on the host.
- A whole-host helper: install `#systemd@node.example` with no `units`.
- Remove: the same installer, `remove --name '#systemd/web@node.example'`, with current credentials.

</details>

## Configuration

`units` limits the helper to comma-separated exact service names, for example `nginx.service,backup.timer`. Omitted: every host unit. Empty or invalid values are refused; wildcards and paths are not allowed.

Changes apply on the next call.

### Dedicated instances

Use a named instance per team or application, with its own service list and access list. Listing, status, actions and logs stay within that instance's `units` list.

## Secret keys

- **None.**
- The installer keeps the helper's own bus credential in a protected file; never put a person's token in its settings.

## Who may use it

- **Permissions come from the bus,** never from the call's text.
- **Owner and Maintainers** have every permission; the service list still binds everyone.

| Permission | Allows |
|---|---|
| On the access list | `list [type]`, `timers`, `failed`, `status <unit>` (no log lines) |
| `operate` | Also `start`, `restart`, `reload`, `logs` |
| `admin` | Also `stop`, `enable`, `disable` |

- Do not grant `admin` to someone who only needs restart.

### Logs

- **Private data:** logs can hold it, so they need `operate` or `admin`.
- **Scope:** without a service name they cover this helper's list, or the whole host if it has none.

### Many nodes

- Find the helpers: `agent-bus ls --tag=systemd`.
- Ask them all: `agent-bus call --tagged=systemd list`; each server answers separately.

## Troubleshooting

| If you see | Do this |
|---|---|
| "requires operate" | Ask the owner for `operate` on this helper |
| A unit refused by name | Check spelling and this helper's `units` |
| Permission denied from systemctl | Reinstall with `--user root` |
| Status is an error | Inactive or failed units return nonzero; read the text |
| Configuration error | Fix `units`; blank entries and partial names are refused |

## Warnings and alerts

`#systemd` has its own Linux service, so the [runner entries](README.md#warnings-and-alerts) do not apply; it reports these itself.

| Level | When | What to do |
|---|---|---|
| alert | `systemctl` refuses an action with permission denied | Reinstall with `--user root` |
| alert | `units` Configuration is invalid: every call is refused | Fix `units` |
| alert | `systemctl` or `journalctl` is missing, or the service manager does not answer | Check systemd on the host |
| warning | A `systemctl` call timed out after 30 seconds | Check the unit; it may still be changing state |

## Technical details

<details>
<summary>How it runs</summary>

- It runs `systemctl` and `journalctl` as its service account, root for a helper that restarts services.
- A `systemctl` call that takes longer than 30 seconds times out; the unit may still be changing state.

</details>

<details>
<summary>Rules for these calls</summary>

- Inspect the cause before restarting; check `status` afterwards.
- With many servers, a missing reply or refusal fails the [multi-agent call](../user/cli.md#calling-many-agents-at-once).
- Use the exact timer name from `timers`; reading it needs no extra permission.

</details>

<details>
<summary>Logs</summary>

- 1–1000 lines, default 100. `since` is an age such as `30m`, `1h`, `2d`. Large answers say when they were shortened.
- JSON form: `{"method":"logs","unit":"nginx.service","lines":100,"since":"1h"}`.

</details>
