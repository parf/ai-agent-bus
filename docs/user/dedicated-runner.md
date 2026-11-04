# Running an agent: put your script on the bus with one command

📌 **TL;DR:**
- **Problem:** a handy script on your machine helps only you; letting others or an AI call it means writing message-handling code.
- **Outcome:** `agent-bus run` gives your script a name and answers each message it runs with what the script prints; the `ab-*` launchers do the same for an AI session.
- **Start:** see the command's own guide with `agent-bus run --help` ([Try this first](#try-this-first)).

## Why you'd want it

- 🙋 **Nobody else can call your script.** Now it has a name anyone allowed can call, from any host on the bus.
- 🧑‍💻 **Message code is a chore.** Now your script reads its input and prints its answer; nothing else.
- 🔌 **Links drop and servers restart.** The reader reconnects; a pending answer can be sent after reconnection while its deadline permits.

## What you get

- ✅ **One command:** registers the name, gets its own credential, reads its inbox, runs your script once per message.
- ✅ **Two input shapes:** the message as `$1`, or the whole envelope as JSON on stdin.
- ✅ **Parallel work:** up to N copies at once with `-N`.
- ✅ **Logs and a polite stop** from any terminal of yours.
- ✅ **Optional sandbox:** see its [network and filesystem boundaries](../runner/security.md#isolation-today).
- ⚠️ **No automatic retry:** a failed or timed-out call may already have effects. Check the result before deciding whether another request is safe.
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

- Help output above is shortened (`...`).
- Then serve your script: [the one-liner](#-the-one-liner).

## ⚡ The one-liner

- `agent-bus run '#hi@demo' /full/path/to/hi.sh --descr=greets` puts `/full/path/to/hi.sh` on the bus as `#hi@demo`.
- An agent's name begins with `#`; `run` adds it, so `hi@demo` names the same agent.

That single command:

| | |
|---|---|
| 1️⃣ | registers `#hi@demo`; its ACL controls calls, and managers can also discover it |
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
- ⚙️ **Settings:** `AGENT_BUS_CONFIG` holds compact Configuration at the next start, or is unset; see [Configuration](../03-records-agent.md#configuring-a-template).
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

- A missing answer does not roll back effects. Use application-specific checks or idempotency before retrying.

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
- 🔌 **The bus going away is not an exit.** The runner says `daemon away`, shows `away` in `ps` and backs off connection retries before reporting `reconnected`.
- 📨 **An answer worked out meanwhile** is sent once the bus is back, while the caller still waits. A terminal refusal, such as a revoked token, ends reconnection.
- 🚫 **Starting a name already running here is refused:** that is a duplicate, not a second worker.

## Common trip-ups

| Symptom | Cause |
|---|---|
| `use an absolute script path` | ⚠️ `run` refuses a relative path such as `./hi.sh` or `bin/hi.sh`: the script runs in its *own* work directory |
| `exit status 127` in the logs | ⚠️ a bare command that is not on `PATH` |
| the script runs but the caller times out | it may still be running, have failed, or have lost its answer; inspect `agent-bus logs` and its effects before retrying |
| `already running here` | you started this name in another terminal |
| messages pile up, nothing happens | `agent-bus ls --table --kind=agent`: `READERS 0` means no read is waiting at that instant, including while all worker slots are busy. Runner-managed agents differ: see [managed troubleshooting](../runner/operations.md#when-a-call-does-not-come-back) |

## Sandboxing

Use `--sandbox=on` to request confinement and `--network` for network access; startup names its backend or refusal. Read the [dedicated sandbox boundaries](../runner/security.md#the-separate-agent-bus-run-sandbox) before trusting it with code.

## Who it runs as

- 🙋 **As you,** the person who typed `agent-bus run`.
- 🔐 **Not a child of the bus:** the bus never runs your code, so what holds the secrets is not what runs your script.

## Putting an AI session on the bus

Start `ab-claude`, `ab-codex` or `ab-opencode` in the project directory; the corresponding AI CLI must be installed. Use `ab-opencode1` for opencode 1.x.

See [AI CLI integration](ai-cli.md#try-this-first) for naming, messages, permissions and launcher settings.

## Run options

<details>
<summary>Options for input, access and execution</summary>

| Option | |
|---|---|
| `--algo=<args\|json>` | script input form, `args` by default |
| `-N` | run up to N messages at once, e.g. `-4` |
| `--descr=<text>` | description |
| `--allow=<names\|*\|@owner>` | who may call it, comma-separated |
| `--tags=<tags>` | Comma-separated tags; include `personal` for Personal classification |
| `--sandbox=<on\|off>` | request the [dedicated isolation rules](../runner/security.md#isolation-today) |
| `--network` | allow network inside the sandbox |
| `--package-dir=<path>` | package metadata for asks; an absolute path |
| `--roles=<roles>` | roles the script understands |
| `--stay-registered` | keep the record and credential at a clean stop |
| `--no-asks` | answer unsupported asks without starting the script |

- **From a declaration:** `agent-bus run -4 - < agent.json` reads the agent as JSON from stdin; `-N` overrides its worker count.

</details>
