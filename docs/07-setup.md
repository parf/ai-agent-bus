# Install AgentBus

📌 **TL;DR:** Start your AI on the target computer and ask it to [read and execute AI-SETUP.md](../AI-SETUP.md#how-to-start). It checks for an existing installation, installs only when needed and helps configure the bus and its agents.

## Why

Give your tools a shared bus on one computer or across your team's hosts, without assembling accounts and services by hand.

## What you get

Choose a client for an existing bus, or a node with its own bus and web panel. Add the agents you need from the [catalogue](agents/README.md#choose-an-agent).

## Install

1. Start your AI agent on the computer you want to use.
2. Ask: **Read and execute [AI-SETUP.md](../AI-SETUP.md#how-to-start) to install AgentBus.**
3. Answer any missing choices, such as whether to join an existing bus or run your own, and which agents to enable.

Already installed? Use the same instruction to **update AgentBus or configure the bus or any agent**. Say what you want changed; the AI reads the relevant guides and preserves existing settings.

### Manual installation

| Need | Install |
|---|---|
| Client for an existing bus | `npm install -g ai-agent-bus-client` |
| Linux node | `npm install -g ai-agent-bus`, then run its `agent-bus-setup --owner <user>` with sudo |
| Checked archive | Follow [archive installation](../src/INSTALL.md#from-a-release-archive) |
| macOS node | Use the full archive and the [macOS instructions](platforms/macos.md#the-daemon) |

The npm wrapper needs Node 18 or newer. Full Linux setup requires a **root-owned Node executable and global prefix**, with no group/other write access, including parent directories. Do not run setup with sudo through nvm or another user-owned Node installation. Client-only use needs no root.

Installing an npm package creates no service accounts or units until you explicitly run setup. npm installation trusts the registry; archive installation verifies its supplied checksums and manifest before setup.

### Check an existing installation

Print the installed version, then check the bus:

```sh
agent-bus --version
agent-bus status
```

For a Linux node, check both services:

```sh
systemctl is-active agent-busd agent-bus-web
```

### Update or configure

For a manual archive update, unpack the new archive and run its setup with `--upgrade`; after an interrupted upgrade use `--recover` first. [Back up](user/backup.md#which-backup) before replacing node state. macOS setup does not support `--upgrade`.

Enable supplied agents in the [runner inventory](runner/configuration.md#inventory) with `enabled:true`, then reload the runner. Follow each agent's guide for settings, credentials and access.

## More help

- [Install a runner](runner/setup.md#a-persons-runner).
- [Accounts, TLS, storage and backups](administration.md).
- [macOS service layout and limits](platforms/macos.md).
- [Command-line tools](user/cli.md).
