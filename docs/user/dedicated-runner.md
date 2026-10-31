# Running an agent: put your script on the bus with one command

📌 **TL;DR:**
- **Problem:** a handy script on your machine helps only you; letting others or an AI call it means writing message-handling code.
- **Outcome:** `agent-bus run` gives your script a name and answers each message it runs with what the script prints; the `ab-*` launchers do the same for an AI session.
- **Start:** see the command's own guide with `agent-bus run --help` ([Try this first](#try-this-first)).

## Why you'd want it

- 🙋 **Nobody else can call your script.** Now it has a name anyone allowed can call, from any host on the bus.
- 🧑‍💻 **Message code is a chore.** Now your script reads its input and prints its answer; nothing else.
- 🔌 **Links drop and servers restart.** Now it waits and reconnects by itself, and answers once the bus is back.
- 🧠 **Your AI session is stuck in one terminal.** Now peers can message the live Claude Code, Codex or opencode session.

## What you get

- ✅ **One command:** registers the name, gets its own credential, reads its inbox, runs your script once per message.
- ✅ **Two input shapes:** the message as `$1`, or the whole envelope as JSON on stdin.
- ✅ **Parallel work:** up to N copies at once with `-N`.
- ✅ **Logs and a polite stop** from any terminal of yours.
- ✅ **Optional sandbox:** see its [network and filesystem boundaries](../runner/security.md#isolation-today).
- ⚠️ **No retry:** a failed script is logged, not run again; the caller asks again if it wants.
- 🚫 **Runs as you, in the foreground:** for installed scripts that keep running without you, use the [runner](../runner/README.md).

## Try this first

- Read the command's own guide; it changes nothing:

```sh
agent-bus run --help
```

```text
USAGE
  agent-bus run <name> <script>
  agent-bus run [-N] -

DESCRIPTION
  <name> is the agent to register; <script> is one quoted shell command. The
  default input form is args. A lone - reads the agent declaration as JSON from
  stdin; -N overrides its parallel worker count (default 1). Confinement is off
  unless --sandbox=on; a confined script has no network unless --network is
  given. ...

OPTIONS
  --algo=<args|json>        Script input form (default args)
  -N                        Run up to N messages concurrently (e.g. -4)
  --descr=<text>            Description
  --allow=<names|*|@owner>  Comma-separated access terms (comma-separated, * or @owner)
  ...
```

- The output is shortened (`...`) and is from the development node on 2026-10-30.
- Then serve your script: [the one-liner](#-the-one-liner).

## ⚡ The one-liner

- `agent-bus run '#hi@demo' /full/path/to/hi.sh --descr=greets` puts `/full/path/to/hi.sh` on the bus as `#hi@demo`.
- An agent's name begins with `#`; `run` adds it, so `hi@demo` names the same agent.

That single command:

| | |
|---|---|
| 1️⃣ | registers `#hi@demo` so everyone can see it |
| 2️⃣ | gets the agent its own credential |
| 3️⃣ | reads its inbox until stopped; when the bus restarts or the link drops, it waits and reconnects |
| 4️⃣ | runs your script **once per message**, and sends back what it printed |

- 🖥️ **It stays in the foreground.** Ctrl-c stops it and, with nothing waiting, unregisters the name.
- 🧪 **A tiny agent:** a `hi.sh` that runs `echo "you said: $1"`, made executable; a caller sending `ping` gets `you said: ping`.

## How your script hears the message

| Shape | Your script gets | Good for |
|---|---|---|
| Default (`--algo=args`) | the body as **argument `$1`** | simple scripts |
| `--algo=json` | the whole envelope as **JSON on stdin** | when you care who sent it, or the topic |

- 🧾 **Either way, the envelope is in the environment:** `AGENT_BUS_MESSAGE_ID`, `AGENT_BUS_FROM`, `AGENT_BUS_TO`, `AGENT_BUS_TOPIC`, `AGENT_BUS_TAG`, `AGENT_BUS_WORK`, `AGENT_BUS_ASK`.
- 🎫 **Roles:** one `AB_ROLE_<NAME>=1` per role granted to this caller.
- ⚙️ **Settings:** `AGENT_BUS_CONFIG` holds compact Configuration at the next start, or is unset; see the [`jq` example](../03-records-agent.md#configuring-a-template).
- 📂 **A scratch directory of its own:** `AGENT_BUS_WORK`; the script starts there.
- ⚠️ **`FROM` is a label, not proof.** The bus enforces `--allow`; for single methods check the granted `AB_ROLE_<NAME>=1`, such as `AB_ROLE_OPERATE=1`, never a string match on `$AGENT_BUS_FROM` ([roles](../03-records-agent.md#roles)).

<details>
<summary>Asks: <code>help</code>, <code>status</code>, <code>mcp</code> and friends</summary>

- **Every ask not answered by package metadata** reaches the script as `AGENT_BUS_ASK` with an empty body: `help`, `help-config`, `help-secret`, `help-json`, `mcp` or `status`.
- **With no ask,** the variable is empty.
- **`--no-asks`** (package `"asks":"none"`) answers unsupported without starting the script.
- **An unpackaged script has no MCP tools,** so `mcp` is answered `{"tools":[]}`; offering tools needs a package with `help: native`.
- 📖 Native help and observed runner status: [package help](../runner/configuration.md#package-help).

</details>

## What the caller gets back

| Your script | The caller sees |
|---|---|
| printed something, exit 0 | that output, as the answer ✅ |
| printed nothing, exit 0 | `done`, so they stop waiting |
| exited nonzero | nothing. It is logged as a failure and **not retried** ❌ |
| the caller's deadline already passed | the script is not run at all |

- 🔁 **No retry, on purpose:** the bus cannot know whether running your script twice is safe. The caller asks again if it wants.

## More than one at a time

- `agent-bus run '#busy@demo' -5 --algo=json /full/path/to/worker.sh` runs at most five copies at once; one by default.
- 📥 **A message is taken only when a slot is free.** If the agent dies, only work already in flight is lost; the rest waits for the next reader.

## 👀 Watching and stopping it

From any terminal of yours, not only the one it runs in:

| To | Run |
|---|---|
| Read its last 50 lines | `agent-bus logs '#hi@demo' --lines=50` |
| Follow its output | `agent-bus logs '#hi@demo' --follow` |
| Stop it politely | `agent-bus stop '#hi@demo'` |

- 🛑 **`stop` is polite:** it takes no new work, **waits** for running scripts, then exits. It never reports a stop that has not finished.
- 📜 **Logs outlive the run:** what a run said is most wanted after it ended.
- 🔒 **Only you can stop it:** the note `stop` and `logs` read sits in **your own** state directory.
- 🧹 **A deliberate exit unregisters the name** when nothing is waiting, so an unserved name answers "no such name" instead of collecting messages.
- 📌 **`--stay-registered` keeps the name** and credential after a clean stop, for an outside supervisor.
- 💥 **A crash or an error exit keeps the name** and its waiting messages, so a restart picks up where it left off; so does a stop with messages still waiting.
- 🔌 **The bus going away is not an exit.** The runner says `daemon away`, shows `away` in `ps`, retries every second at first, then every 30 s, and says `reconnected` when back.
- 📨 **An answer worked out meanwhile** is sent once the bus is back, while the caller still waits. Only a refusal, such as a revoked token, ends the run.
- 🚫 **Starting a name already running here is refused:** that is a duplicate, not a second worker.

## Common trip-ups

| Symptom | Cause |
|---|---|
| `use an absolute script path` | ⚠️ `run` refuses a relative path such as `./hi.sh` or `bin/hi.sh`: the script runs in its *own* work directory |
| `exit status 127` in the logs | ⚠️ a bare command that is not on `PATH` |
| the script runs but the caller times out | it printed nothing **and** exited nonzero. Check `agent-bus logs` |
| `already running here` | you started this name in another terminal |
| messages pile up, nothing happens | `agent-bus ls --table --kind=agent`: `READERS` `0` means nothing reads it. Runner-managed agents differ: see [managed troubleshooting](../runner/operations.md#when-a-call-does-not-come-back) |

## Sandboxing

- 🔓 **Optional confinement:** the [isolation reference](../runner/security.md#isolation-today) defines the dedicated runner's network and filesystem reach.
- Use `--sandbox=on` to request it and `--network` for network access; startup names its backend or refusal.

## Who it runs as

- 🙋 **As you,** the person who typed `agent-bus run`.
- 🔐 **Not a child of the bus:** the bus never runs your code, so what holds the secrets is not what runs your script.

## Putting an AI session on the bus

Four launchers ship with agent-bus:

| Launcher | Starts |
|---|---|
| `ab-claude` | a Claude Code session, on the bus |
| `ab-codex` | a Codex session, on the bus |
| `ab-opencode` | an opencode session, on the bus |
| `ab-opencode1` | the same, on a host still running opencode 1.x |

- 🔎 **They do the wiring:** find your socket, register the session under a name, load the bus tools, and deliver messages to the **live** session.
- 💬 **Talk to it from any terminal:** see [Claude Code, Codex and opencode](ai-cli.md).
- ▶️ **They continue** the last session in the current directory when there is one, and always start in automatic execution, so the session is not waiting on a prompt when a message arrives.
- ⚠️ **A message cannot change the session's permissions or mode:** those are fixed at launch, and messages are data, not orders.
- 📦 The AI tool itself must be installed. An installed release runs the launchers as a built program; run from a source checkout, they also need Bun. `ab-claude --help` (and the others) says the rest.

| Environment | |
|---|---|
| `AGENT_BUS_ADDR` | skip discovery, use this address |
| `AGENT_BUS_TOKEN` | a token, when the socket is not enough |
| `AGENT_BUS_TLS_FINGERPRINT` | the pin for an `https://` address ([over HTTPS](../02-access-remote.md#over-https)) |
| `AGENT_BUS_NAME` | the bus name you want, beginning with `#`; otherwise `#<runtime>/<session title or directory>@<host>` |

## Technical details

<details>
<summary>Every <code>agent-bus run</code> option</summary>

| Option | |
|---|---|
| `--algo=<args\|json>` | script input form, `args` by default |
| `-N` | run up to N messages at once, e.g. `-4` |
| `--descr=<text>` | description |
| `--allow=<names\|*\|@owner>` | who may call it, comma-separated |
| `--personal` | a personal agent |
| `--tags=<tags>` | comma-separated tags |
| `--sandbox=<on\|off>` | request the [dedicated isolation rules](../runner/security.md#isolation-today) |
| `--network` | allow network inside the sandbox |
| `--package-dir=<path>` | package metadata for asks; an absolute path |
| `--roles=<roles>` | roles the script understands |
| `--stay-registered` | keep the record and credential at a clean stop |
| `--no-asks` | answer unsupported asks without starting the script |

- **From a declaration:** `agent-bus run -4 - < agent.json` reads the agent as JSON from stdin; `-N` overrides its worker count.

</details>

<details>
<summary>Where this sits</summary>

- **Part of the ordinary [`agent-bus`](cli.md) tool;** installed scripts managed by the separate `agent-bus-runner` program are in the [runner guide](../runner/README.md).
- **Outside the bus:** the dedicated runner is your own process; nothing the bus runs may start user programs.
- 📖 The design: [runner § what the runner does](../06-dedicated-runner.md#what-it-does).

</details>
