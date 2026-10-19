# Standalone starter

📌 **TL;DR:** `install.py` keeps one script behind one bus name using a systemd
unit running `agent-bus run --sandbox off --algo args`. The same starter is
used by server-info and is available for the systemd agent; neither needs a
second runner or a copy of this installation logic.

## Interface

```sh
sudo --preserve-env=AGENT_BUS_ADDR,AGENT_BUS_TOKEN,AGENT_BUS_TLS_FINGERPRINT \
  ./src/agents/standalone/install.py install \
  --name '#agent@host' --user EXISTING_ACCOUNT --script /absolute/agent.py \
  --allow '@owner'
sudo ./src/agents/standalone/install.py remove --name '#agent@host'
```

The script must be executable. `--allow` is optional: omission uses the bus's
restricted default. `--bus-binary` chooses the installed CLI. The systemd unit
is `agent-bus-standalone-<first 16 hex digits of SHA-256(name)>.service`, printed
on success. The name determines one unit regardless of its user or script.
The service uses the supplied account and its own systemd state directory;
it has no private network, sandbox or additional privilege restrictions.

`--unit-dir`, `--config-dir`, and `--systemctl` allow installation into a
staging tree or a test controller. Defaults are `/etc/systemd/system`,
`/etc/agent-bus/standalone`, and `systemctl`. Tests redirect all three.

## Lifecycle

The private, mode-0600 EnvironmentFile contains `AGENT_BUS_ADDR` (HTTP or
HTTPS), `AGENT_BUS_TOKEN`, and an optional TLS fingerprint. It holds the
**launching account's token**, because a dedicated runner unregisters its
idle agent on graceful stop and must register it again on restart. Use an
account authorised to register and manage the chosen name. The dedicated
runner switches to the agent's credential and removes it from the script's
environment. No credential is written into the unit or printed by the starter.

Installation is serialized per name. Atomic file replacements keep the unit
and credential file complete. An unchanged installation is enabled without
restarting it; changed settings restart the unit. An existing bus record
without this starter's credential file is refused, rather than adopted.
An interrupted or failed install can be retried or removed with the same name
and directories.

Removal disables and stops the unit, explicitly removes any remaining agent
registration, then removes the unit and credential file and reloads systemd.
It also handles a registration left by a killed runner. If the bus refuses
removal, for example because its queue is not empty, removal fails and keeps
the files so it can be retried after resolving that condition. Repeated
successful removal is a no-op. The systemd state directory retains the
runner's logs; the small installer lock file contains no credential.
