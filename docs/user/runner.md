# 🏃 Running a service

Your script, your agent session, or anything else that answers messages.

ℹ️ **There is no `agent-bus-runner` program today.** The runner is
`agent-bus start` — part of the ordinary [`agent-bus`](cli.md) command. A
separate managed runner, with autostart and restart policies, is planned for
R1. Everything below is what you can actually run right now.

## ⚡ The one-liner

```sh
agent-bus start hi@demo --algo=args /full/path/to/hi.sh --descr "greets"
```

That single command does four things for you:

| | |
|---|---|
| 1️⃣ | registers `hi@demo` so everyone can see it |
| 2️⃣ | gets the service its own credential |
| 3️⃣ | reads its inbox, forever |
| 4️⃣ | runs your script **once per message**, and sends back what it printed |

It stays in the **foreground**. Ctrl-c stops it. 🛑

## 📨 How your script hears the message

Two shapes — pick the one that suits you:

| | Your script gets | Good for |
|---|---|---|
| `--algo=args` | the body as **argument `$1`** | simple scripts. `hi.sh "ping"` |
| `--algo=json` | the whole envelope as **JSON on stdin** | when you care who sent it, or the topic |

Either way, a handful of environment variables come along:

```
AGENT_BUS_MESSAGE_ID  AGENT_BUS_FROM  AGENT_BUS_TO
AGENT_BUS_TOPIC       AGENT_BUS_TAG   AGENT_BUS_WORK
```

⚠️ These **describe the work; they do not authenticate anybody.** `FROM` is a
label, not proof. If your script guards something, guard it with the bus's
`--allow`, not with a string comparison on `$AGENT_BUS_FROM`.

`AGENT_BUS_WORK` is a scratch directory of your own — the script starts there.

A tiny service, end to end:

```sh
cat > ~/hi.sh <<'SH'
#!/bin/sh
echo "you said: $1"
SH
chmod +x ~/hi.sh
agent-bus start hi@demo --algo=args ~/hi.sh --descr "greets"
```

```sh
agent-bus call hi@demo --wait 10s "ping"
```
```
ack from hi@demo
{"from":"hi@demo","body":"you said: ping", …}
```
🎉

## 📤 What the caller gets back

| Your script | The caller sees |
|---|---|
| printed something, exit 0 | that output, as the answer ✅ |
| printed nothing, exit 0 | `done` — so they stop waiting |
| exited nonzero | nothing. It is logged as a failure, and **not retried** ❌ |
| the caller's deadline already passed | the script is not run at all |

There is no retry on purpose: the bus cannot know whether running your script
twice is safe. If you want a retry, the caller asks again.

## 🔢 More than one at a time

```sh
agent-bus start busy@demo -5 --algo=json /full/path/to/worker.sh
```

`-5` means at most five copies of the script at once. One by default.

💡 A message is only taken from the bus when a script slot is **free**. So if
the service dies, only the work already in flight is lost — the rest is still
queued, waiting for whatever reads that inbox next. Nothing evaporates.

## 👀 Watching and stopping it

From any terminal of yours, not just the one it is running in:

```sh
agent-bus logs hi@demo --lines 50
agent-bus logs hi@demo --follow
agent-bus stop hi@demo
```

| | |
|---|---|
| **`stop`** | polite: it stops taking new work, **waits** for scripts still running, then exits. It does not report a stop that has not finished |
| **the logs** | outlive the run on purpose — what a run said is most wanted after it has ended |
| **only you can stop it** | a running service leaves a note in **your own** state directory, and that is what `stop` and `logs` read. Nobody else can see it, so nobody else can touch it 🔒 |

`stop` leaves the **registration** in place, so the name still exists and its
queue still collects. To remove the name too: `agent-bus unregister hi@demo`.

Starting a name that is already running **here** is refused — that is a
duplicate, not a second worker.

## 🧯 Common trip-ups

| Symptom | Cause |
|---|---|
| `exit status 127` in the logs | ⚠️ **a relative script path.** The child runs in its *own* work directory, so `./hi.sh` is not where you think. Use an absolute path |
| script runs but the caller times out | it printed nothing **and** exited nonzero. Check `agent-bus logs` |
| `already running` | you started this name in another terminal |
| messages pile up, nothing happens | `agent-bus ls -h` — if `READER` says `no`, your service is not running |

## 🛡️ Sandboxing

Off unless you ask. `--sandbox on` confines the script with
`systemd-run --user`:

| | |
|---|---|
| 📁 | system read-only, your home read-only, a private `/tmp` |
| 📂 | the work directory writable; the script's own directory read-only |
| 🌐 | **no network** unless you add `--network` |
| 🔻 | no new privileges, and process/memory limits |

Asking for confinement on a machine with no working user manager **fails
loudly** rather than quietly running your script unconfined. Startup tells you
which backend it chose.

## 🤝 Who it runs as

As **you** — the person who typed `agent-bus start`. It is not a child of the
daemon, and the daemon never execs anything. That is deliberate: the thing
that holds the secrets is not the thing that runs your code.

## 🧠 Putting an AI session on the bus

Two launchers ship with agent-bus and do the whole arrangement for you:

```sh
ab-claude          # a Claude Code session, on the bus
ab-codex           # a Codex session, on the bus
```

They find your socket, register the session under a name, load the bus tools
into it, and wire up message delivery so peers can talk to your **live**
session. Bun and the runtime itself must be installed.

| Environment | |
|---|---|
| `AGENT_BUS_ADDR` | skip discovery, use this address |
| `AGENT_BUS_TOKEN` | a token, when the socket is not enough |
| `AGENT_BUS_NAME` | the bus name you want. Otherwise one is derived from the session title or the directory |

👉 The interesting part is that you can then **talk to that running session**
from any terminal — see [Claude Code and Codex](agents.md).

`ab-claude --help` / `ab-codex --help` say the rest. They always continue the
last session in the current directory, and they enforce automatic execution —
so the session is not waiting on a prompt when a message arrives.

⚠️ An incoming message can never change the session's permissions or mode.
That is fixed when the launcher starts, and messages are data, not orders.

📖 The design behind all of this: [runner § what the runner
does](../08-runner-role.md#what-the-runner-does).
