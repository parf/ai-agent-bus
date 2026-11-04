# The bus host: install it once, know it is healthy

📌 **TL;DR:**
- **Problem:** your people, agents and AI sessions need one always-on place to meet, and somebody has to install it and know when it is unwell.
- **Outcome:** one setup command installs the bus and its web dashboard as system services; one status command shows connection and queue counters.
- **Start:** check a running bus with `agent-bus status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🧱 **Install one host service** instead of assembling separate messaging components.
- 🔑 **Use a local login.** An account mapped to a bus User has its own socket and needs no copied token.
- 🌍 **Agents live on other hosts too.** Now they connect over the network with a token, with TLS when you turn it on.
- 🩺 **"Is it up?" takes digging.** Now one command shows the counters that matter.

## What you get

- ✅ **Linux service setup:** accounts, systemd units and you as the first administrator. For macOS, use the [platform guide](../platforms/macos.md#client-and-runner).
- ✅ **Scales out:** one laptop to start; other hosts, users and teams join through the same address.
- ✅ **Four ways in:** your own socket, the shared socket, the network port and the dashboard.
- ✅ **Separate accounts:** the bus stores credentials; the dashboard forwards authenticated requests without a credential database of its own.
- ⚠️ **Crash recovery has a save window:** see [what it keeps](#what-it-keeps).
- ⚠️ **Plain HTTP off this machine:** turn TLS on before other hosts connect.
- 🚫 **It never runs your code:** scripts run in your own session or under the [runner](../runner/README.md).

## Try this first

- Ask the bus how it is doing, from any account that can reach it:

```sh
agent-bus status
```

- What each counter means: [Is it healthy?](#is-it-healthy).

## 📦 Installing it, once

- Follow the [installation guide](../07-setup.md#install) to get the programs. On Linux, its setup step is:

```sh
sudo /usr/local/bin/agent-bus-setup --owner "$USER"
```

That one command:

| | |
|---|---|
| 👥 | makes two service accounts: one for the bus, one for the dashboard |
| 🏠 | creates their homes and runtime directories |
| 📝 | writes and starts two systemd units: `agent-busd` and `agent-bus-web` |
| 🔑 | adds **you** as the first user and an administrator, from your `id_ed25519.pub` |

Look before you leap:

| To | Run |
|---|---|
| See what setup would do, changing nothing | `sudo /usr/local/bin/agent-bus-setup --dry-run` |
| See the unit file, changing nothing | `sudo /usr/local/bin/agent-bus-setup --print-unit` |
| Add sample users, agents, services, queues, pub/sub channels and groups | `sudo /usr/local/bin/agent-bus-setup --samples` |
| Take exactly those samples away | `sudo /usr/local/bin/agent-bus-setup --remove-samples` |

- Sample data: [setup § sample data](../administration.md#sample-data).
- Every setup flag: [setup flags](#setup-flags).

## Who can get in

| Way in | Where | Who |
|---|---|---|
| 🥇 **your own socket** | `/run/agent-bus/user-<account>.sock` | one account. The socket **is** the credential: no token. An agent you own, sending its token here, acts as that agent |
| 🤝 **the shared socket** | `/run/agent-bus/bus.sock` | anyone on the machine, **with a token** |
| 🌐 **the network port** | `127.0.0.1:6767` by default; any address with `--addr` | with a token, from any host that reaches it. A browser is sent on to the dashboard |
| 🖥️ **the dashboard** | `127.0.0.1:6780` | a browser |

- 🔓 **Plain HTTP is unencrypted:** use HTTPS or an SSH tunnel for remote clients. Turning on bus TLS adds HTTPS on the same port; it does not disable plain HTTP.
- 🔒 **Turn TLS on:** `sudo /usr/local/bin/agent-bus-setup --tls self-signed` makes the same port answer TLS; clients pin its fingerprint.
- 🔒 **The dashboard too:** setup's `--tls` turns TLS on for it, and plain HTTP there redirects to `https://`.
- 🚇 **No exposed port at all:** one SSH remote forward carries your own socket to another machine.
- 📖 All four ways, each with an example: [remote access](../02-access-remote.md#choosing-a-way).
- 🔐 Why two accounts: [setup § the two accounts](../administration.md#the-two-accounts).

## Running it

Under systemd, after setup:

| To | Run |
|---|---|
| See whether the bus is up | `sudo systemctl status agent-busd` |
| Restart it cleanly | `sudo systemctl restart agent-busd` |
| Follow its log | `journalctl -u agent-busd -f` |
| See whether the dashboard is up | `sudo systemctl status agent-bus-web` |

A test bus of your own, by hand, in your own session:

- Make a directory for it: `mkdir -p "$HOME/test-bus"`.
- Start it: `agent-busd --owner=me@demo --addr=127.0.0.1:6791 --socket="$HOME/test-bus/bus.sock" --db="$HOME/test-bus/agent-bus.db" --create`.
- `--create` makes the database on the first run only.
- Your own account owns this bus, so your own socket answers as its owner: no token for yourself.
- Every flag: [bus flags](#bus-flags).

## What it keeps

| Kind | Survives a restart? |
|---|---|
| 📇 agents, channels, services, people | ✅ yes |
| 🎟️ tokens | ✅ yes |
| 📬 waiting messages, statistics | ⚠️ a clean restart, yes; a crash loses what came since the last save (every minute by default) |
| 🔋 waiting readers | no; the count changes as clients reconnect and issue reads |

- 🔌 **Agents reconnect by themselves:** `agent-bus run` and the `ab-*` sessions.
- 💤 **A reader that is down is fine:** its messages wait, up to the time-to-live and size set when its name was registered.
- 💾 Save or move records while it runs: [backups](backup.md).

## Is it healthy?

- Run `agent-bus status` ([Try this first](#try-this-first)) and glance at:

| Counter | Means |
|---|---|
| `queued` | messages waiting; sustained growth means intake exceeds consumption, so inspect worker capacity and connectivity |
| `dropped` | a bounded ring inbox discarded its oldest message; inspect capacity and consumption before raising its bound |
| `expired` | messages that outlived their time-to-live before anyone took them |
| `refused` | calls turned away, counted per reason; investigate unexpected growth ([refusals](../05-discovery.md#refusals)) |

- Inspect individual agents with `agent-bus ls --table --kind=agent`. `READERS 0` means no read is waiting at that instant; a busy, disconnected or stopped worker can all produce it. A rising queue warrants checking its logs.

## Letting people in

Use the [operator commands](cli.md#-for-the-bus-operator) to add users and keys. A local account mapping gives that user a private socket without copying a token:

```sh
agent-bus-admin account set <local-account> <user[@realm]>
sudo systemctl restart agent-busd
```

Replace the angle-bracket placeholders before running the command. `account list` reports whether the saved map needs a restart; `account remove` retires a mapping on the next restart. The bus account's implicit socket cannot be edited.

## When it will not start

| Symptom | Look at |
|---|---|
| anything | ✅ `journalctl -u agent-busd -n 50` first. It says why |
| refuses the address | `--addr` is not `host:port`, or the port is taken |
| the dashboard did not start | `journalctl -u agent-bus-web`; check the reported bind, permission or installed-program error |
| a user has no socket | `agent-bus-admin account list`; add the mapping and restart the bus |
| recent messages missing after a restart | it did not stop cleanly, so what arrived since the last save was never written |

- 📖 New here? Start with [the command-line tools](cli.md).
- 🏃 Put your own script on the bus: [running an agent](dedicated-runner.md).
- 🧠 Put an AI session on it: [Claude Code, Codex and opencode](ai-cli.md).

## Processes

<details>
<summary>Services, state and execution</summary>

| Process | Holds | Runs as |
|---|---|---|
| the supervisor | the listening sockets, **no state** | `agent-busd.service` |
| the bus, its one child | the records, the waiting messages and the tokens | the same unit and account |
| the dashboard | no credential database; authenticated requests pass through it | `agent-bus-web.service`, its own account |
| a script agent | its inbox reader and your script | you, in your session (`agent-bus run`) |

- **One database:** records, users, groups, credentials and waiting messages live in one SQLite file.
- **Messages are held in memory** and saved in batches; see [restart and crash behavior](#what-it-keeps).
- **The dashboard** runs as its own systemd service ([processes § the web face](../08-processes.md#the-web-face)); the MCP face runs inside each agent session that loads it ([agents](ai-cli.md)).
- 📖 The design: [processes § the processes](../08-processes.md#the-processes).

</details>

### Setup flags

<details>
<summary>The <code>agent-bus-setup</code> flags worth knowing</summary>

| Flag | |
|---|---|
| `--owner=user` | who this bus belongs to, realm optional. Defaults to you |
| `--user=account=user[@realm]` | seed a local account mapping on the first start; repeat per person |
| `--key=path` | a different public key for the first user |
| `--addr` · `--exec` | the network address, and which `agent-busd` to run |
| `--upgrade` · `--recover` | replace the installed release with automatic rollback; finish an interrupted one ([INSTALL](../../src/INSTALL.md#from-a-release-archive)) |
| `--tls=off\|self-signed\|files` · `--tls-cert` · `--tls-key` · `--tls-chain` · `--tls-name` | TLS on the bus and dashboard ports: your own files, or more names for a generated certificate ([setup § TLS](../administration.md#tls)) |
| `--dry-run` · `--print-unit` | say what would happen, or print the unit; change nothing |
| `--samples` · `--remove-samples` | add or remove the sample data |

</details>

### Bus flags

<details>
<summary>Every <code>agent-busd</code> flag</summary>

| Flag | |
|---|---|
| `--owner=user` | whose bus this is, realm optional. The owner is always an administrator |
| `--user=account=user[@realm]` | first-start seed for a local account and its principal; later changes use `agent-bus-admin account` |
| `--directory=realm=github` | a realm and what vouches for enrolment; `realm=/path/to/keys` for a directory of key files. Public GitHub profile metadata does not need this flag |
| `--addr` · `--socket` | the network address, any interface, default `127.0.0.1:6767`, never taken from `AGENT_BUS_ADDR`; and the Unix socket path |
| `--tls-dir=dir` | `cert.pem`, `key.pem` and optional `chain.pem`: the network port answers TLS beside plain HTTP ([setup § TLS](../administration.md#tls)) |
| `--db=path` | the one SQLite database, by default `$XDG_STATE_HOME/agent-bus/agent-bus.db`, else `~/.local/state/agent-bus/agent-bus.db`. Held exclusively; a missing one refuses the start |
| `--init` · `--create` | make the database: `--init` alone and exit, `--create` then serve. Only an explicit act creates one |
| `--flush-every` | how often waiting messages and counters are saved, in one batch; `0` saves only at a clean stop |
| `--syslog` | copy warnings, errors and alerts to host syslog; off by default, on in installed units |
| `--log-dir` · `--debug-log` | where `audit.log`, `error.log` and the on-demand `debug.log` go, and whether the last starts on |
| `--ssh-keys=path` | the bus account's `authorized_keys`, where an administrator may install a user's public key through the API; empty permits host-managed keys only ([SSH admin](../administration.md#ssh-admin)) |
| `--ssh-token=path` | the forced-command program for those keys; defaults to the SSH credential wrapper beside the program |
| `--dashboard=url` | where a browser opening the API address is sent; empty serves no root page |
| `--web` | accepted and ignored; the dashboard runs as its own service |

</details>

### Environment defaults

Explicit flags override these:

| Variable | Flag | When unset |
|---|---|---|
| `AGENT_BUS_OWNER` | `--owner` | the initial owner must be supplied |
| `AGENT_BUS_SOCKET` | `--socket` | the program's default Unix socket |
| `AGENT_BUS_DB` | `--db` | `$XDG_STATE_HOME/agent-bus/agent-bus.db`, else `~/.local/state/agent-bus/agent-bus.db` |
| `AGENT_BUS_LOG_DIR` | `--log-dir` | `logs/` beside the database |
| `AGENT_BUS_DASHBOARD` | `--dashboard` | the default dashboard URL |

- `AGENT_BUS_ADDR` never sets the listening address; use `--addr`.
- `AGENT_BUS_TLS_DIR` selects installed TLS files for setup, the token helper and admin. The bus itself needs an explicit `--tls-dir` to serve TLS ([TLS setup](../administration.md#tls)).
- **Dashboard TLS by hand:** set `AGENT_BUS_WEB_TLS_DIR` to the certificate directory; its port serves HTTPS and redirects plain HTTP ([where it listens](../05-discovery.md#where-it-listens)).

<details>
<summary>Where alerts go</summary>

- **By hand in a terminal:** alerts go to `error.log` under `--log-dir` (default `logs/` beside the database); startup failures also go to stderr.
- **Without `--syslog`:** nothing is copied to the host journal or syslog.
- **Installed units** pass `--syslog` and keep the normal copy at the matching severity.
- 📖 [Logs](../constitution.md#logs).

</details>
