# AgentBus runner

**Turn any script into a safe, shared team tool.**

📌 **TL;DR:**
- **Problem:** a handy script stays a private hack, because sharing it means sharing its passwords and giving it free run of a server.
- **Outcome:** the runner gives the script a name, keeps its passwords apart, applies [documented isolation](security.md#isolation-today), and lets your team call it.
- **Start:** drop the script in a folder, reload, call it by name: [Getting started](getting-started.md).

## Why you want it

- 🔑 **A script needs a password.** The password stays on the server and is passed to the trusted script, rather than handed to callers.
- 🧱 **A script can touch everything.** Now its [isolation rules](security.md#isolation-today) state what it can reach, see and write.
- 🙋 **Nobody else can call it.** Now it has a name, and you choose who may use it.
- 🔁 **It dies and nobody notices.** Now you see its state and failures in one place, and restart it from a web page.
- 🧩 **One script, three accounts.** Now one package runs as several instances, each with its own settings and access.

## What you get

- ✅ **No message code:** the script reads its input and prints its answer.
- ✅ **Secrets kept apart:** passwords live outside the code and are passed only to the agent that needs them. Trusted agent code can still disclose what it receives.
- ✅ **Confinement:** [isolation today](security.md#isolation-today) defines the Linux default, network access, file visibility and the off setting.
- ✅ **Control:** start, stop, restart, enable and disable each agent, from the command line or the web page.
- ✅ **Always-on helpers:** an agent that must keep running, such as a mail reader, is restarted if it stops.

## One way to start

- Follow [Getting started](getting-started.md): a short script, a reload, one call.
- It ends with the script answering by name.
- Already have a runner? List its visible agents with `agent-bus ls --kind=agent`, then use [Operations](operations.md#try-this-first) to inspect their state.

## How it works

```mermaid
flowchart LR
    C[Caller] -->|Call an agent name| B[AgentBus]
    B --> R[Runner]
    R -->|Start with the message| S[Your script]
    S -->|Printed answer| R
    R -->|Answer as the agent| B
    B --> C
```

- **The bus** controls who may call and delivers the message.
- **The runner** starts the script and returns what it prints.
- **Installing the runner** does not make the bus run anything itself.

## Choose your starting point

| You want to… | Read |
|---|---|
| Install a script and get its first answer | [Getting started](getting-started.md) |
| Write an agent or configure several instances | [Agents](agents.md) |
| Find the right home for a setting or secret | [Configuration](configuration.md) |
| Check status, inspect errors, update or remove code | [Operations](operations.md) |
| Understand permissions and current protections | [Security](security.md) |

- **A script in your own terminal:** use the [dedicated runner](../user/dedicated-runner.md).
- **Claude Code, Codex or OpenCode sessions:** use the [session launchers](../user/ai-cli.md).
- **A program with a client library:** it runs on its own, or under the runner as an [agent that serves itself](configuration.md#agents-that-serve-themselves).

<details>
<summary>Technical details</summary>

- **Separate program:** the runner ships in the same release and uses the bus like any client.
- **Per message, bounded:** one process starts per message, with bounded parallel work.
- **Confinement:** read the [network, filesystem and limit boundaries](security.md#isolation-today) before installing code.
- **Accepted is not always active:** see [current limits](security.md#what-is-accepted-but-inert).
- **Controls:** per-agent controls and host-file editing are in [operations](operations.md#running-and-checking-the-runner).
- **Web pages:** information and management are separate pages ([the runner web pages](../../src/web/web-face/runners.md#web-face-runner-pages)).
- **Version and install:** the runner uses the repository's shared version and [installation](../07-setup.md#install).

</details>
