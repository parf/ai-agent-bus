# Install agent-bus

📌 **TL;DR:**
- **Problem:** you want a working bus on a Linux host without building it or assembling accounts and services by hand.
- **Outcome:** the release archive installs the daemon, web panel and CLI; your ordinary login can use them immediately.
- **Start:** follow [From a release archive](#from-a-release-archive).

Let your AI inspect an existing installation or guide a fresh one with [AI-SETUP.md](https://github.com/parf/ai-agent-bus/blob/main/AI-SETUP.md#how-to-start).

## Why you'd want it

- 🚀 **A node in minutes.** Setup creates the accounts, tree, services and web panel for you.
- 🔁 **Keep your data when updating.** Setup upgrades an installed release with a backup and rollback.
- 🧰 **Your first agent right away.** [Try it](#try-it) runs a two-line script as an agent.

## What you get

- ✅ **A running daemon and web panel** at <http://127.0.0.1:6780/>.
- ✅ **The CLI** linked into `/usr/local/bin`.
- ✅ **Your own bus account**, reached from your ordinary local login.

## How to start

### From a release archive

- Use a Linux system with systemd, `sudo`, `tar`, `gzip` and `sha256sum`.
- Run these commands from your ordinary login, not a root shell; only setup uses `sudo`.
- Put the full archive for your CPU and its `.sha256` sidecar in an otherwise empty directory. Do not mix it with the client archive.
- Native bundled agents need Python 3; their [guides](https://github.com/parf/ai-agent-bus/blob/main/docs/agents/README.md#choose-an-agent) name other prerequisites. The release needs neither Go nor Bun.

Check and unpack that archive, then install it:

```sh
sha256sum -c agent-bus-*.tar.gz.sha256
mkdir agent-bus-install
tar -xzf agent-bus-*.tar.gz -C agent-bus-install --strip-components=1
sudo ./agent-bus-install/agent-bus-setup --owner "$USER"
```

- The plain local user name also works with the [person's runner](https://github.com/parf/ai-agent-bus/blob/main/docs/runner/setup.md#a-persons-runner). A qualified bus identity is a separate choice, not needed for this local installation.
- Setup uses your SSH public key when present. To create one first, run `ssh-keygen -t ed25519`, or add it later with `agent-bus-admin user add`.
- The installed release includes `agents/` and its shared Python helpers and client.

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

- The answer's body is `hi: ping`.
- To keep an agent running without your terminal, follow the [runner getting-started guide](https://github.com/parf/ai-agent-bus/blob/main/docs/runner/getting-started.md).

### Update an installed release

- Unpack the new archive into a new directory the same way, then run its `agent-bus-setup --upgrade` with `sudo`.
- It keeps the node's configuration and TLS, backs up the state, and rolls back if the new release fails to start.
- After an interrupted upgrade, run `--recover`, then `--upgrade` again; see [release installation details](https://github.com/parf/ai-agent-bus/blob/main/src/INSTALL.md#from-a-release-archive).

## Technical details

### More setup options

| Need | Start here |
|---|---|
| npm installation | [npm packages](https://github.com/parf/ai-agent-bus/blob/main/docs/07-setup.md#manual-installation) |
| TLS | [setup TLS](https://github.com/parf/ai-agent-bus/blob/main/docs/administration.md#tls) |
| Sample data | `sudo /usr/local/bin/agent-bus-setup --samples`; `--remove-samples` takes it away |
| Preview | `--dry-run` prints the plan; `--print-unit` prints the daemon unit |
| Native agents | Installers under `/usr/local/lib/agent-bus/current/agents/standalone/`; Slack and Telegram are bundled under `agents/slack/` and `agents/telegram/` |

### From a checkout

- Development needs git, Go with gcc, and Bun at `/usr/bin/bun`.
- Keep the checkout outside `/home`: the daemon's account cannot see a home directory.

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

### Update

Update that checkout, build it and restart its services:

```sh
cd /usr/local/src/ai-agent-bus
git pull && bun install --cwd src/mcp && src/build.sh
sudo systemctl restart agent-busd agent-bus-web
```
