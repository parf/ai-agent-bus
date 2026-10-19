# Systemd: check and restart services

📌 **TL;DR:** Find broken Linux services, read their recent logs and restart
only the services you are allowed to manage. An administrator can make a
separate helper for each team, with a short list of permitted services.

## Why you'd want it

Notice failures sooner and shorten the time needed to fix them. A team can
check and restart its own application without getting full server access.

## What it does

Systemd is Linux's service manager. This helper asks it for service status,
failed services and scheduled jobs, and can perform permitted actions.
Only people you permit can restart services. A limited helper cannot touch
services outside its list.

## You'll need

- Linux with systemd and an installed bus CLI.
- An administrator to install the helper under the root account.
- Your [installation credentials](README.md#standalone-helpers).

## Use examples

### Ask your AI

| Say this | What it will call |
|---|---|
| “Check failed services on all nodes.” | `call --tagged systemd failed`; no action permission needed |
| “Show the last hour of web logs.” | `logs nginx.service 100 since 1h`; needs operate permission |
| “Restart the web service after I approve.” | `restart nginx.service`; needs operate permission |
| “Show tomorrow's backup timer.” | `timers` and `status backup.timer` |

### 1. An error report becomes a fix

```sh
agent-bus call --tagged systemd failed
agent-bus call '#systemd@parf.us' 'status nginx.service'
agent-bus call '#systemd@parf.us' 'logs nginx.service 100 since 1h'
# After checking the cause, with permission to restart:
agent-bus call '#systemd@parf.us' 'restart nginx.service'
agent-bus call '#systemd@parf.us' 'status nginx.service'
```

The first answer groups results by server. The last status should show the
service running. A missing reply or refusal makes the multi-agent call fail;
see [calling many agents](../user/cli.md#calling-many-agents-at-once).

### 2. Check a scheduled backup

```sh
agent-bus call '#systemd@parf.us' timers
agent-bus call '#systemd@parf.us' '{"method":"status","unit":"backup.timer"}'
```

You see scheduled jobs and the named timer's state. Use the actual timer name
from the list; no restart permission is needed to read it.

### 3. Let a developer manage only the web app

Install the dedicated instance below. Then the developer can run:

```sh
agent-bus call '#systemd/web@parf.us' list
agent-bus call '#systemd/web@parf.us' 'restart nginx.service'
```

With operate permission, the restart succeeds. Asking this instance to
restart `sshd.service`, which is outside its list, is refused.

## Installing it

1. Prepare [your bus credentials](README.md#standalone-helpers). You should
   have the bus address, your token and the TLS fingerprint in your environment.
2. Install a helper with a private initial access list:

   ```sh
   sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
     python3 src/agents/standalone/systemd/install.py install \
     --name '#systemd/web@parf.us' --user root --allow '@owner'
   ```

   The installer prints its Linux service name, `agent-bus-systemd-web.service`.
3. Limit it to the team's services:

   ```sh
   agent-bus agent-template '#systemd/web@parf.us' '{"units":"nginx.service,web-worker.service,backup.timer"}'
   ```

   The next `list` should show only those units that are loaded on the host.
4. Give the intended person permission to act:

   ```sh
   agent-bus manage '#systemd/web@parf.us' --add-allow 'ann@parf.us(operate)'
   agent-bus call '#systemd/web@parf.us' list
   ```

   Ann can now read logs and restart the listed services. Replace Ann with a
   registered person or AI session on your bus.
5. Remove your personal installation token from the shell with
   `unset AGENT_BUS_TOKEN`. The helper keeps using only its own credential.

For a whole-host administrator, install the name `#systemd@parf.us` instead.
With no `units` setting, it covers every unit on the host. To remove an instance,
run the same installer with `remove --name '#systemd/web@parf.us'` and current
installation credentials. You should see its service removed.

## Configuration

The complete sample has one setting. Remove the comment before saving it:

```jsonc
{
  "units": "nginx.service,web-worker.service,backup.timer" // exact names only
}
```

A **unit** is a service, timer or another item managed by systemd. Names need
their ending, such as `.service` or `.timer`. No wildcards or file paths work.
An empty or invalid list gives an error; it never turns into whole-host access.

### Dedicated instances

Create another named helper for each team or application. Each has its own
settings and access list. With `units` set, listing, status, actions and logs
all stay inside that list. Changes apply to the next call.

## Secret keys

None. This helper needs no outside token in Secret. The installer keeps its
own bus credential in a protected file; never put a person's token in its settings.

## Calls and permissions

The owner and maintainers can do everything; give others access with a role.
The configured units still bind everyone. These permissions come from the bus, never from the call’s text.

| Permission | What it allows |
|---|---|
| A caller on the access list, no extra role | `list [type]`, `timers`, `failed`, `status <unit>` |
| `operate` | Also `start`, `restart`, `reload`, `logs` |
| `admin` | Also `stop`, `enable`, `disable` |
| Owner or Maintainer (automatic) | All of the above, within configured units |

For example, `--add-allow 'ann@parf.us'` grants reading; adding `(operate)` grants
the middle row. Do not grant `(admin)` to someone who only needs restart.

```sh
agent-bus call '#systemd/web@parf.us' '{"method":"failed"}'
agent-bus call '#systemd/web@parf.us' '{"method":"logs","unit":"nginx.service","lines":100,"since":"1h"}'
```

### Logs

Logs can contain private information, so they require operate or admin.
Use 1–1000 lines, with 100 by default. `since` is an age such as `30m`, `1h`
or `2d`. Without a unit name, logs cover only this instance's list, or the
whole host if it has no list. Large answers say when they were shortened.

### Many nodes

Find the helpers with `agent-bus ls --tag systemd`, then use the multi-agent
call in the first example. Each server answers separately. Inspect the result
before choosing a service to restart; the helper never restarts failures by itself.

## Troubleshooting

| If you see this | Do this |
|---|---|
| “requires operate” | Ask the owner for restart/log permission on this helper |
| A unit is refused by name | Check the spelling and this instance's `units` list |
| Permission denied from systemctl | The installer should run this helper with `--user root` |
| Status is an error | An inactive or failed service can have a nonzero status; inspect the text |
| Configuration error | Fix `units`; blank entries and partial names are refused |
