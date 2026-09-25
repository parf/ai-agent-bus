# Install agent-bus

`agent-bus-setup` inside this archive is the single supported MVP installer.
It installs the complete release, creates the service accounts and starts the
daemon plus its loopback dashboard.

## Prerequisites

- Linux with systemd and unified cgroup v2.
- Bun at `/usr/bin/bun` for the dashboard, which runs on it under its own
  `agent-bus-web` account and unit. Without it setup reports the gap and
  installs the daemon alone.
- Root once, through `sudo`, for installation.
- A supported Claude Code, Codex or OpenCode CLI only when using that
  runtime's `ab-*` launcher, which also needs Bun. The daemon does not.

## Install

Verify the downloaded archive beside its checksum, unpack it and run setup
from the unpacked directory:

```sh
sha256sum -c agent-bus-*.tar.gz.sha256
mkdir agent-bus-install
tar -xzf agent-bus-*.tar.gz -C agent-bus-install --strip-components=1
cd agent-bus-install
sudo ./agent-bus-setup --owner "$USER@$(hostname -s)"
```

Setup validates every program and face before changing the host. It installs
the release under `/usr/local/lib/agent-bus`, commands under `/usr/local/bin`,
state under `/var/lib/agent-bus`, and `agent-busd.service` plus the
dashboard's `agent-bus-web.service` under systemd.
Re-running the same archive is safe and repairs an interrupted first install;
an existing identical release is reused. If setup reports a command collision
in `/usr/local/bin`, move the unrelated file and run setup again. It never
replaces a non-symlink there.

Check the installed node:

```sh
sudo systemctl status agent-busd agent-bus-web
agent-bus --version
curl -fsS http://127.0.0.1:6767/identity
```

The dashboard is at <http://127.0.0.1:6780/>. Setup imports the invoking
user's public SSH key when one exists; otherwise an operator can add a user
later with `agent-bus-admin user add`.

## Call a service

The restrictive ACL default applies to reply inboxes too. Grant this service
permission to answer you, then run it in one terminal:

```sh
realm=$(hostname -s)
me="$USER@$realm"
service="fresh-echo@$realm"
cat >"$HOME/fresh-echo.sh" <<'SH'
#!/bin/sh
printf 'fresh reply: %s\n' "$1"
SH
chmod +x "$HOME/fresh-echo.sh"
agent-bus register "$me" --kind agent --allow "$service"
agent-bus start "$service" --algo=args --allow "$me" "$HOME/fresh-echo.sh"
```

In another terminal:

```sh
agent-bus call "$service" --wait 5s 'installation works'
```

The answer contains `fresh reply: installation works`.

## Upgrade and recover

Verify and unpack the new archive beside the installed node, then run its setup
program in upgrade mode. Do not repeat first-install flags: ownership, account
mappings and operator configuration come from the installed node.

```sh
sha256sum -c agent-bus-*.tar.gz.sha256
mkdir agent-bus-upgrade
tar -xzf agent-bus-*.tar.gz -C agent-bus-upgrade --strip-components=1
sudo ./agent-bus-upgrade/agent-bus-setup --upgrade
```

Upgrade verifies and stages the complete new release while the old daemon is
still serving. It then stops the daemon cleanly, copies the whole daemon state
tree — snapshot, credentials and SSH authorization together — into
`/var/lib/agent-bus/backups`, atomically switches `current`, and starts the new
release. The existing daemon unit and its drop-ins are preserved byte for
byte. Setup waits for the public identity and verifies that the supervisor and
bus run from the selected release, then refreshes and restarts the dashboard's
unit on the new release before reporting success.

A failed start restores the prior release and its matching state automatically.
If setup itself is killed or the host stops while an upgrade is in progress,
run recovery from the same unpacked new archive before retrying:

```sh
sudo ./agent-bus-upgrade/agent-bus-setup --recover
sudo ./agent-bus-upgrade/agent-bus-setup --upgrade
```

Recovery refuses if the installed unit changed after the interrupted upgrade
began; inspect that operator change before choosing which configuration to keep.
Do not delete the newest backup until the upgraded node has been checked. Plain
setup reuses an identical release for first-install repair and refuses to select
a different one; release replacement always uses `--upgrade`.
