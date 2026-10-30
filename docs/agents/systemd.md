# Systemd: check and restart services

📌 **TL;DR:**
- **Problem:** a broken service is often fixed by a look at its log and a restart, but that needs a server login with administrator rights you do not want to hand out.
- **Outcome:** you or your AI see which services are broken, read their recent logs and restart them, without a login; each team gets a helper limited to its own services.
- **Start:** ask what the helper can do: `agent-bus help '#systemd/web@node.example'` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔑 **Restarting needs an admin login.** Now one helper does it, and restart is a permission you grant per person.
- 🧱 **A team could touch everything.** Now each team's helper has a short list of services and refuses the rest.
- 🔍 **You find failures by logging in.** Now one call asks every server for failed services.
- 🤖 **An AI should restart its app, not the host.** Now it gets that one permission on one list.

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

- Ask the helper what it can do:

```sh
agent-bus help '#systemd/web@node.example'
```

- More on this: [asking an agent what it can do](../user/cli.md#asking-an-agent-what-it-can-do).

## Use examples

- **Find failures on all servers:** ask every tagged helper for `failed`.
- **Check one service:** ask for `status` and its name, such as `nginx.service`.
- **Read its last hour of logs:** ask for `logs`, the service, a line count and `since 1h`.
- **Restart it (for example after a [git pull](git.md#use-examples)):** ask for `restart` and its name.
- **See scheduled jobs:** ask for `timers`.
- **See my team's services:** ask your team's helper for `list`.

<details>
<summary>Rules for these calls</summary>

- Inspect the cause before restarting; check `status` afterwards.
- With many servers, a missing reply or refusal fails the [multi-agent call](../user/cli.md#calling-many-agents-at-once).
- Use the exact timer name from `timers`; reading it needs no extra permission.

</details>

## Installing it

- An administrator installs it once per team as root.
- Then the administrator lists the team's services and grants restart permission to the right people.

<details>
<summary>Step by step</summary>

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
- Remove: the same installer, `remove --name '#systemd/web@node.example'`, with current credentials.

</details>

## Configuration

- **One setting:** the list of services this helper may touch.
- **Without it:** every service on the host.
- **Invalid list:** an error, never whole-host access.

<details>
<summary>Keys</summary>

| Key | Meaning |
|---|---|
| `units` | Comma-separated exact names, such as `nginx.service,backup.timer`. Omitted: every host unit |

Names need their ending (`.service`, `.timer`); no wildcards or paths. An empty or invalid list is an error, never whole-host access.

</details>

### Dedicated instances

- **One named helper per team or application,** each with its own service list and access list.
- **Inside the list:** listing, status, actions and logs stay there.
- **When it applies:** changes apply on the next call.

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

<details>
<summary>Technical details: logs</summary>

- 1–1000 lines, default 100. `since` is an age such as `30m`, `1h`, `2d`. Large answers say when they were shortened.
- JSON form: `{"method":"logs","unit":"nginx.service","lines":100,"since":"1h"}`.

</details>

### Many nodes

- Find the helpers: `agent-bus ls --tag=systemd`.
- Ask them all: `call --tagged=systemd`; each server answers separately.

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
