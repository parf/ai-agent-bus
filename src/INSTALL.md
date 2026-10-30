# Install from a checkout

📌 **TL;DR:**
- **Problem:** you want a working bus on a Linux host without hand-building accounts, units and a database.
- **Outcome:** clone it under `/usr/local/src`, build it, run setup once and link the programs with `git-install.sh`; after that, an update is a pull, a build and a restart.
- **Start:** follow [How to start](#how-to-start).

## Why you'd want it

- 🚀 **A node in minutes.** Setup creates the accounts, tree, services and web panel for you.
- 🔁 **Updates are small.** Pull, build, restart.
- 🧰 **Your first agent right away.** The [Try it](#try-it) step runs a two-line script as an agent.

## What you get

- ✅ **A running daemon and the web panel** at <http://127.0.0.1:6780/>.
- ✅ **The CLI** (`agent-bus`) linked into `/usr/local/bin`.
- ✅ **Your SSH key as the first user's key**, so you can reach the bus from your own account.

## How to start

For a release installation, use the [npm packages](../docs/07-setup.md#npm-package). The commands below are for developing from a checkout.

Prerequisites:

- Linux with systemd, and `sudo`
- git, Go with gcc, and Bun at `/usr/bin/bun`
- Python 3 for native bundled agents; see the [agent guides](../docs/agents/README.md#choose-an-agent) for their other tools

### Install

```sh
sudo mkdir -p /usr/local/src/ai-agent-bus && sudo chown "$USER" /usr/local/src/ai-agent-bus
git clone https://github.com/parf/ai-agent-bus /usr/local/src/ai-agent-bus
cd /usr/local/src/ai-agent-bus
bun install --cwd src/mcp
src/build.sh
sudo src/agent-bus-setup --exec "$PWD/src/agent-busd"
sudo src/git-install.sh --no-build
```

- The checkout must live outside `/home`: the daemon runs as its own account and cannot see a home directory.

### Check

```sh
agent-bus status
systemctl is-active agent-busd agent-bus-web
```

### Try it

Turn a two-line script into an agent:

```sh
printf '#!/bin/sh\necho "hi: $1"\n' > ~/hi.sh && chmod +x ~/hi.sh
agent-bus run hi ~/hi.sh
```

In another terminal, call it:

```sh
agent-bus call '#hi' --wait 10s ping
```

```text
{"message_id":"…","from":"#hi","to":"you","topic":"call","tag":"…","body":"hi: ping",…}
```

### Update

```sh
cd /usr/local/src/ai-agent-bus
git pull && bun install --cwd src/mcp && src/build.sh
sudo systemctl restart agent-busd agent-bus-web
```

## Technical details

### More setup options

| | |
|---|---|
| SSH key | Setup makes your `~/.ssh/id_ed25519.pub` the first user's key. Without one, run `ssh-keygen -t ed25519` first, or add it later with `agent-bus-admin user add` |
| TLS | `sudo src/agent-bus-setup --exec "$PWD/src/agent-busd" --tls self-signed`, or `--tls files --tls-cert … --tls-key …` ([setup § TLS](../docs/07-setup.md#tls)) |
| Sample data | `sudo /usr/local/bin/agent-bus-setup --samples` adds sample users, agents, services, queues, pubsub channels and groups; `--remove-samples` takes them away |
| Preview | `--dry-run` says what setup would do; `--print-unit` prints the daemon unit. Neither changes anything |

### From a release archive

```sh
sha256sum -c agent-bus-*.tar.gz.sha256
mkdir agent-bus-install
tar -xzf agent-bus-*.tar.gz -C agent-bus-install --strip-components=1
sudo ./agent-bus-install/agent-bus-setup --owner "$USER@$(hostname -s)"
```

- The installed release includes `agents/` and its shared Python helpers and client.
- Native agents need Python 3. Their installers live under `/usr/local/lib/agent-bus/current/agents/standalone/`; Slack and Telegram are already bundled under `agents/slack/` and `agents/telegram/`.
- To upgrade, unpack the new archive the same way and run its `agent-bus-setup --upgrade`. It keeps the node's configuration and TLS, backs up the state, and rolls back by itself if the new release fails to start.
- After an interrupted upgrade, run `--recover`, then `--upgrade` again ([setup § install](../docs/07-setup.md#install)).
