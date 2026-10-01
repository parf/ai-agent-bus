# 🏃 Running an agent

📌 **TL;DR:** `agent-bus start` serves messages through your script — or your
agent session, or anything else that answers. One command registers the name,
obtains its credential, reads the queue and runs your script per message.
The `ab-*` launchers do the same for a Claude Code, Codex or opencode session.

The runner is `agent-bus start`, part of the ordinary [`agent-bus`](cli.md)
command. A managed runner with autostart and restart policies is
[R1 scope](../../Plans/R1.0-Release/runner.md#managed-runner).

ℹ️ An agent's name begins with `#`. `start` adds it for you: `hi@demo` and
`'#hi@demo'` publish the same agent.

## ⚡ The one-liner

```sh
agent-bus start '#hi@demo' --algo=args /full/path/to/hi.sh --descr "greets"
```

That single command does four things for you:

| | |
|---|---|
| 1️⃣ | registers `#hi@demo` so everyone can see it |
| 2️⃣ | gets the agent its own credential |
| 3️⃣ | reads its inbox until stopped. When the daemon restarts or the link drops, it waits and reconnects by itself |
| 4️⃣ | runs your script **once per message**, and sends back what it printed |

It stays in the **foreground**. Ctrl-c stops it and, with nothing queued, unregisters the name. 🛑

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

A tiny agent, end to end:

```sh
cat > ~/hi.sh <<'SH'
#!/bin/sh
echo "you said: $1"
SH
chmod +x ~/hi.sh
agent-bus start '#hi@demo' --algo=args ~/hi.sh --descr "greets"
```

```sh
agent-bus call '#hi@demo' --wait 10s "ping"
```
```
ack from #hi@demo
{"from":"#hi@demo","body":"you said: ping", …}
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
agent-bus start '#busy@demo' -5 --algo=json /full/path/to/worker.sh
```

`-5` means at most five copies of the script at once. One by default.

💡 A message is only taken from the bus when a script slot is **free**. So if
the agent dies, only the work already in flight is lost — the rest is still
queued, waiting for whatever reads that inbox next. Nothing evaporates.

## 👀 Watching and stopping it

From any terminal of yours, not just the one it is running in:

```sh
agent-bus logs '#hi@demo' --lines 50
agent-bus logs '#hi@demo' --follow
agent-bus stop '#hi@demo'
```

| | |
|---|---|
| **`stop`** | polite: it stops taking new work, **waits** for scripts still running, then exits. It does not report a stop that has not finished |
| **the logs** | outlive the run on purpose — what a run said is most wanted after it has ended |
| **only you can stop it** | a running agent leaves a note in **your own** state directory, and that is what `stop` and `logs` read. Nobody else can see it, so nobody else can touch it 🔒 |

A deliberate exit takes the **registration** with it: when nothing is waiting
in the queue, the agent unregisters itself, so a name nobody serves answers
"no such name" instead of quietly collecting messages. A **crash** or an error
exit is not a deliberate exit — the name and its queue stay, so the agent can be started
again and picks up where the queue left off. If messages were still waiting
when a stopped agent tried to leave, it stays registered for the same reason.

A daemon that goes away is not an exit at all. The runner says `daemon away`,
shows `; away` in `ps`, and keeps trying — every second at first, then every
30 s — until the daemon is back, then says `reconnected`. An answer your script
worked out meanwhile is sent once the daemon is back, while the caller still
waits. Only a refusal, such as a revoked token, ends it.

Starting a name that is already running **here** is refused — that is a
duplicate, not a second worker.

## 🧯 Common trip-ups

| Symptom | Cause |
|---|---|
| `use an absolute script path` at start | ⚠️ `start` refuses any relative path, such as `./hi.sh` or `bin/hi.sh`: the child runs in its *own* work directory. Use an absolute path |
| `exit status 127` in the logs | ⚠️ a bare command that is not on `PATH` |
| script runs but the caller times out | it printed nothing **and** exited nonzero. Check `agent-bus logs` |
| `already running` | you started this name in another terminal |
| messages pile up, nothing happens | `agent-bus ls -h --kind agent` — if `READERS` says `0`, your agent is not running |

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

Three launchers ship with agent-bus and do the whole arrangement for you:

```sh
ab-claude          # a Claude Code session, on the bus
ab-codex           # a Codex session, on the bus
ab-opencode        # an opencode session, on the bus
```

They find your socket, register the session under a name, load the bus tools
into it, and wire up message delivery so peers can talk to your **live**
session. Bun and the runtime itself must be installed.

| Environment | |
|---|---|
| `AGENT_BUS_ADDR` | skip discovery, use this address |
| `AGENT_BUS_TOKEN` | a token, when the socket is not enough |
| `AGENT_BUS_TLS_FINGERPRINT` | the pin for an `https://` address ([over HTTPS](../02-access-remote.md#over-https)) |
| `AGENT_BUS_NAME` | the bus name you want, beginning with `#`. Otherwise `#<runtime>/<session title or directory>@<host>` is derived |

👉 The interesting part is that you can then **talk to that running session**
from any terminal — see [Claude Code, Codex and opencode](agents.md).

`ab-claude --help` (and the same for the other two) says the rest. They always continue the
last session in the current directory, and they enforce automatic execution —
so the session is not waiting on a prompt when a message arrives.

⚠️ An incoming message can never change the session's permissions or mode.
That is fixed when the launcher starts, and messages are data, not orders.

📖 The design behind all of this: [runner § what the runner
does](../06-runner-role.md#what-the-runner-does).
