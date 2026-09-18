# ⚙️ The daemon

📌 **TL;DR:** Install and operate the bus; check its listeners, state and health.

`agent-busd` is the whole bus: registry, broker, MCP server and dashboard, in
one process. Nothing else needs installing — **no Redis, no RabbitMQ, no NATS,
no database server.** 🎈

Most people never run it by hand. This page is for whoever looks after it.

## 📦 Installing it, once

Verify and unpack the release archive, then follow its standalone `INSTALL.md`.
The final installation command is:

```sh
sudo ./agent-bus-setup
```

That one command:

| | |
|---|---|
| 👥 | makes two service accounts — one for the daemon, one for runners |
| 🏠 | creates their homes, `0700`, and their runtime directories |
| 📝 | writes and enables a systemd unit |
| 🔑 | adds **you** as the first user and an administrator, using your `id_ed25519.pub` |

Look before you leap:

```sh
sudo ./agent-bus-setup -dry-run      # say what would happen, change nothing
sudo ./agent-bus-setup -print-unit   # show the unit file, change nothing
```

Worth knowing:

| Flag | |
|---|---|
| `-owner user@realm` | who this daemon belongs to. Defaults to you |
| `-user account=user@realm` | seed a local account mapping on the first current start — repeat per person |
| `-key path` | a different public key for the first user |
| `-addr` · `-exec` | listen address, and which `agent-busd` to run |

🔐 **Two accounts, on purpose.** The daemon holds credentials; the runner runs
your code. They are separate so that one cannot become the other. See
[setup § the two accounts](../09-setup.md#the-two-accounts).

## 🚪 The doors it opens

Four ways in, and they are not equal:

| Door | Where | Who gets in |
|---|---|---|
| 🥇 **your own socket** | `/run/agent-bus/user-<name>.sock` | one account. The socket **is** the credential — the kernel already knows who you are, so no token |
| 🤝 **the shared socket** | `/run/agent-bus/bus.sock` | anyone on the machine, **with a token** |
| 🌐 **loopback TCP** | `127.0.0.1:6767` | with a token. Refuses to bind anything that is not loopback ✅. Opened in a browser it sends you to the dashboard ([where it listens](../05-discovery.md#where-it-listens)) |
| 🖥️ **the dashboard** | `127.0.0.1:6780` | a browser |

The daemon **cannot** listen on a public address. That is not a setting — it
checks, and refuses. To reach it from elsewhere, tunnel over ssh. 🔒

The dashboard is plain HTTP on loopback. Give it `-cert` and `-key` and it
serves HTTPS on the address you gave it instead
([where it listens](../05-discovery.md#where-it-listens)). A running daemon
prints the scheme and address it took.

## ▶️ Running it

Under systemd, after setup:

```sh
sudo systemctl status  agent-busd
sudo systemctl restart agent-busd
journalctl -u agent-busd -f
```

By hand — for a test bus of your own, in your own session:

```sh
agent-busd -owner me@demo -user $USER=me@demo -addr 127.0.0.1:6791
```

The flags:

| Flag | |
|---|---|
| `-owner user@realm` | whose daemon this is. The owner is always an administrator |
| `-user account=user@realm` | first-current-start seed for a local account and its principal; later changes use `agent-bus-admin account` |
| `-directory realm=github` | a realm and what vouches for enrolment. `realm=/path/to/keys` for a directory of key files; public GitHub profile metadata does not require this flag |
| `-addr` · `-socket` | the loopback address and the unix socket path |
| `-token-file` | the token store; created if missing |
| `-dump-file` · `-dump-every` | where the snapshot goes, and how often. `0` turns the periodic dump off |
| `-web` | run the dashboard as a child too |

## 💾 What it keeps, and what it does not

| Kind | Where it lives | Survives a restart? |
|---|---|---|
| 📇 registry — services, topics, people | its database | ✅ yes |
| 🎟️ tokens | the token store | ✅ yes |
| 📬 queued messages, statistics | **memory**, snapshotted to the dump file | ⚠️ across a **graceful** restart, yes |
| 🔋 liveness — who is reading, who is up | memory | ❌ no, and should not — it is re-learned in a second |

💡 So: `systemctl restart` keeps the queues. A crash or `kill -9` does not.
That is a deliberate trade — bounded in-memory queues are why there is no
broker to install.

A consumer being **down is fine**. Its queue simply waits, up to the TTL and
the bound set when the name was registered.

## 📈 Is it healthy?

```sh
agent-bus status
```

```json
{"up":"6s","services":2,"queued":1,"you":"me@demo","administrator":true,"daemon_owner":true}
```

The counters worth a glance:

| | |
|---|---|
| `queued` | work waiting. Steady is fine; **only ever climbing** means something stopped reading |
| `dropped` | a bounded ring queue threw the oldest away — raise `--bound`, or read faster |
| `expired` | messages that outlived their TTL before anyone took them |
| `refused` | a call was turned away. A few is normal; a flood is somebody misconfigured. It is counted per reason, and each reason has its own status code ([refusals](../05-discovery.md#refusals)) |

And `agent-bus ls -h` for the picture at a glance — a service with `READER no`
and a rising `QUEUED` is one that has stopped.

## 👤 Letting people in

From the daemon's own account:

```sh
agent-bus-admin user add parf@myhost ~/.ssh/id_ed25519.pub
agent-bus-admin user add parf@myhost - --admin < key.pub
agent-bus-admin user list
agent-bus-admin token parf@myhost --rotate
agent-bus-admin user remove parf@myhost
```

A key added this way reaches exactly **one command and no shell**. 🔑 To give
somebody their own socket — no token at all — use:

```sh
agent-bus-admin account set <local-account> <user@realm>
sudo systemctl restart agent-busd
```

`account list` says whether the saved map still needs a restart; `account
remove` retires a mapping on the next restart. The daemon account's own socket
is implicit and cannot be edited.

## 🧬 How it is put together

Not one blob. A small supervisor with least-privilege children:

| Piece | Holds |
|---|---|
| the supervisor | one capability, **no state** |
| bus | the registry and the queues |
| auth | principal token secrets — and it is the **only** one that does |
| web, health | no secrets at all |

They talk over unix sockets and passed file descriptors, nothing shared
implicitly. And **no process the daemon starts may exec** — that is why the
runner is a separate program under a separate account, not a child.

📖 The design: [processes § supervisor](../11-processes.md).

## 🆘 When it will not start

| | |
|---|---|
| `journalctl -u agent-busd -n 50` | ✅ start here. It says why |
| refuses the address | `-addr` is not loopback. That is the check doing its job |
| the dashboard did not start | the port it was given is somebody else's, or not yours to bind — the log says which |
| a user has no socket | check `agent-bus-admin account list`; add the mapping and restart the full daemon |
| the queues are empty after a restart | it did not exit gracefully, so the dump was never written |

---

📖 New here? Start with [the command-line tools](cli.md). Putting your own
script on the bus is [running a service](runner.md); putting an AI session on
it is [Claude Code, Codex and opencode](agents.md).
