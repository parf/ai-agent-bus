# Agents your team can use

📌 **TL;DR:** These ready-made helpers let people and their AI assistants send
messages, check servers and work with code. An administrator installs a helper
once and chooses who may use it. People then ask for help through the bus.

## What the bus gives your team

- Find broken services before customers tell you.
- Collect alerts from several servers in one place.
- Answer an AI assistant from your phone.
- Check and update an application without opening an SSH session.
- Give each person access to the job they need, rather than the whole server.

## Choose an agent

An **agent** is a small helper with a name, such as `#systemd@parf.us`.
The **runner** keeps some helpers running. Others have their own Linux service.
A **token** is a password for a bot or a bus account; keep it private.

| Agent | What it helps with | How it runs | What it needs | Who sets it up / who uses it |
|---|---|---|---|---|
| [Slack](slack.md) | Send updates, collect alerts and talk to AI sessions in Slack | Runner | Slack bot and app tokens; a Slack app | Admin installs; people and AIs use it |
| [Telegram](telegram.md) | Get updates and answer AI sessions from your phone | Runner | A Telegram bot token | Admin installs; listed people and AIs use it |
| [Systemd](systemd.md) | Check Linux services, read logs and restart permitted services | Own Linux service | Root account to manage services | Server admin installs; permitted operators and AIs use it |
| [Dedicated systemd instances](systemd.md#dedicated-instances) | Give a team access to only its own services | Own Linux service per instance | A list of allowed services | Admin limits it; that team and its AIs use it |
| [Git](git.md) | Check code changes and update a chosen application | Own Linux service | An account that can read the code; keys if needed | Admin installs; permitted developers and AIs use it |
| [Server-info](server-info.md) | See a server's network, disks, memory and other facts | Own Linux service | Linux tools; normally no root account | Admin installs; permitted people and AIs read it |
| [Debug](debug.md) | Check that a runner can answer and see its settings | Runner, once per question | Python 3 | Admin installs; trusted maintainers use it |

## Which agent for what

Use **server-info** for “what is happening on this machine?” Use **systemd**
for “which service failed, and may I restart it?” Use **git** for “what code
changed?” and an approved code update. Slack and Telegram carry the
conversation and alerts. Debug helps the administrator check the setup.

## Ask your AI

Your Claude Code, Codex or OpenCode session must be connected to the bus.
It can only use the helpers and actions it has permission to use.

| Say this | What the AI can call |
|---|---|
| “Check failed services on all nodes.” | `call --tagged systemd failed` |
| “Show the logs for the broken web service, then ask me before restarting it.” | Systemd `logs`, then `restart` with restart permission |
| “Post to Telegram that the deploy finished.” | Telegram `post` |
| “What changed in the ab repository?” | Git `log` and `diff`; no write permission needed |
| “Forward errors from Slack's #alerts channel to me.” | Slack `sub add`, when you manage the destination |
| “Check memory and disk space before we update the app.” | Server-info `memory` and `disks`, then permitted Git calls |

## Before you install

The pages use the server name `parf.us` and account `parf` as examples.
Change those to your server and account names. Run commands from the checkout
root, for example `/usr/local/src/ai-agent-bus`. The bus and CLI must already
be [installed](../07-setup.md#install).

### Runner helpers

Slack, Telegram and debug need [a person's runner](../07-setup.md#a-persons-runner).
Here that is `#runner/parf@parf.us`, running as `parf-ab-runner`.
Its shared code directory is `/var/lib/agent-bus/agent.d`; its private settings
are in `/var/lib/agent-bus/parf-ab-runner`. Installing helper code needs sudo.

### Standalone helpers

Systemd, git and server-info have their own Linux services. Their installers
need sudo and your current bus credentials. A credential proves who you are.
Get the fingerprint and your token from the bus server:

| Where you run the command | Fingerprint | Your token |
|---|---|---|
| Another computer | `ssh agent-busd@<node> token --fingerprint` | `ssh agent-busd@<node> token` |
| The bus server itself | `agent-bus-token --fingerprint` | `agent-bus-token` |

For HTTPS, set the address and fingerprint, then enter your token:

```sh
export AGENT_BUS_ADDR=https://bus.example.com:6767
export AGENT_BUS_TLS_FINGERPRINT='sha256:…' # paste the fingerprint
read -rsp 'Your bus token: ' AGENT_BUS_TOKEN; printf '\n'
export AGENT_BUS_TOKEN
```

On the bus server itself, a helper can instead use the plain HTTP address the
bus listens on, such as `http://127.0.0.1:6767` when it listens on loopback.
On this development node, that address is `http://100.121.175.32:6767`.
No HTTPS or fingerprint is needed for that local connection; the token is
still required. The standalone installer accepts HTTP(S) addresses, not Unix
socket paths. See [remote access](../02-access-remote.md#over-https) for details.

The installers store only the helper's own bus credential. Your personal
credential is used for installation, not saved. After installation, run
`unset AGENT_BUS_TOKEN`. Removing a helper needs your current credentials
again; see the [installer details](../../src/agents/standalone/starter.md#lifecycle).

JSON is a way to write settings using braces, names and values. A comment in
a sample explains a value; remove comments before saving real JSON. Root is
the Linux administrator account.

## Settings and passwords

**Configuration** holds ordinary settings: destinations, people and allowed
services. **Secret** holds outside passwords such as Slack and Telegram tokens.
Never put a bot token in Configuration. Each agent page shows the needed keys
and how to send a token into Secret without putting it in a command argument.

There are two safe places for a password. **Secret** is kept by the bus, so the
people and helpers allowed to read it can use it from any server. The
**runner's files** on one server never leave that server; the bus never sees
them. Use the second for a password that must stay on one machine
([two places for secrets](../runner/security.md#secrets)).

For [systemd](systemd.md#calls-and-permissions) and [Git](git.md#roles-and-boundaries), the owner and maintainers can do everything; give others access with a role. Configured unit and repository limits still apply to everyone.
