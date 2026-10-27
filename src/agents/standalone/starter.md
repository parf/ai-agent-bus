# Standalone starter

📌 **TL;DR:** `install.py` keeps one script behind one bus name using a systemd
unit running `agent-bus run --sandbox off --algo args`. The same starter is
used by server-info and is available for the systemd agent; neither needs a
second runner or a copy of this installation logic.

Start a host-access package with the [create-agent prompt](../../../docs/agents/create-agent-prompt.md#prompt-to-paste).

## Interface

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  ./src/agents/standalone/install.py install \
  --name '#agent@host' --script /absolute/agent.py \
  --allow '@owner'
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  ./src/agents/standalone/install.py remove --name '#agent@host'
```

The script must be executable. Its absolute installation path retains a release
installation’s `current` link, so a later restart uses the selected release. Its directory must contain `agent.json` with a
nonempty string `description`; a missing or blank description refuses installation
by agent name. This description is registered on the bus and passed as `--descr`
to the unit’s `agent-bus run`. Standalone metadata is outside the runner loader. Optional `tags` is a list of
simple lowercase labels; the starter passes it to registration and `run` as `--tags`.
 `--allow` is optional: omission uses the bus's
restricted default. It is sent only at installation, so a unit restart preserves
subsequent ACL grants. `--bus-binary` chooses the installed CLI, including its `token` verb for credential issuance during installation. The systemd unit
is derived from the agent name without its host, with punctuation replaced
by hyphens: `#server-info@parf.us` becomes `agent-bus-server-info.service`, and
`#systemd/web@h` becomes `agent-bus-systemd-web.service`. It is printed on
success. A collision with another name or an existing foreign unit is refused
by unit name, including on removal; nothing is overwritten or renamed around it.
The service uses its own systemd state directory;
it has no private network, sandbox or additional privilege restrictions.

By default the starter creates a dedicated system account, `ab-<agent>`:
`ab-server-info` or `ab-systemd-web` in these examples. It has a nologin shell,
`/nonexistent` as its home, and no home directory is created. The invoking
human's account is never the default, including under `sudo`. If account
creation is refused or `useradd` is unavailable, installation fails by account
name; it never falls back to a shared account. An existing account is reused only with these home and login
restrictions; an unrestricted account with that name is refused.

`--user` explicitly selects another existing account. Root requires
`--user root`; for server-info this adds exactly the health and sensors
sections. All other sections are available to the default unprivileged account.
Removal retains the dedicated account so its UID is not reassigned while
retained logs still belong to it.

`--unit-dir`, `--config-dir`, `--systemctl`, and `--useradd` allow installation into a
staging tree or a test controller. Defaults are `/etc/systemd/system`,
`/etc/agent-bus/standalone`, `systemctl`, and `useradd`. Tests redirect all four.

## Lifecycle

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

The config directory must be an absolute path containing only letters, digits,
slashes, underscores, dots or hyphens. Its EnvironmentFile directive uses the
validated path without quotes; ExecStart arguments use systemd argument quoting.

The private, mode-0600 EnvironmentFile contains `AGENT_BUS_ADDR` (HTTP or
HTTPS), the **agent's own token**, and an optional TLS fingerprint. The
launching account's credential is used only during installation to register
the name and ask `agent-bus token` for that agent's credential; it is never
written to disk. The unit uses `agent-bus run --stay-registered`, so graceful
stops retain the agent record and its credential for restart. The script's
environment does not receive `AGENT_BUS_TOKEN`. It receives `AGENT_BUS_CONFIG`
at each start when configured; see the [jq example](../../../docs/03-records-agent.md#configuring-a-template).

Installation is serialized per name. Atomic file replacements keep the unit
and credential file complete. An unchanged installation is enabled without
restarting it; changed settings restart the unit. An existing bus record
without this starter's credential file is refused, rather than adopted.
An interrupted or failed install can be retried or removed with the same name
and directories.

Removal requires the remover's current `AGENT_BUS_ADDR` and `AGENT_BUS_TOKEN`
(and TLS fingerprint when used); it never borrows a stored credential.
It disables and stops the unit, explicitly removes any remaining agent
registration, then removes the unit and credential file and reloads systemd.
It also handles a registration left by a killed runner. If the bus refuses
removal, for example because its queue is not empty, removal fails and keeps
the files so it can be retried after resolving that condition. Repeated
successful removal is a no-op. The systemd state directory retains the
runner's logs; the small installer lock file contains no credential.

Installer-only `--script-arg` may be repeated to pass fixed arguments to the
script. Each argument is shell-quoted with the executable before systemd
ExecStart quoting; these values never come from a bus call.

An optional `roles` list in `agent.json` declares the application roles the agent understands. The starter publishes it on the record and keeps it across restarts. `owner` and `maintainer` are automatic delivered roles, not application role declarations.

## Long-lived agents

`--serves-itself` runs an executable directly instead of wrapping it in
`agent-bus run`. It receives its own `AGENT_BUS_NAME`, `AGENT_BUS_TOKEN` and
connection settings, and must serve using a client library. It reads its own
Configuration through that client; it is not given a per-call Configuration
snapshot. Registration, published tags/roles, private credential storage,
idempotent installation and removal use the same installer lifecycle.
The default per-call mode above is unchanged. Logwatch uses this mode because
its history must remain in memory between calls.

The generated `agent-bus run` command supplies `--package-dir` so the runner
answers packaged help asks directly ([package help](../../../docs/runner/configuration.md#package-help)).
