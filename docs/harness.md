# AI agent harness: components agent-bus provides

📌 **TL;DR:**
- **Problem:** an AI agent needs more than a model loop: something must start it, wake it, give it tools, hold its passwords and limit what it may do.
- **Outcome:** agent-bus provides those essential harness components for any agent: lifecycle, wake-up, tools, permissions (ACL), secrets and a sandbox.
- **Start:** see the [harness components](#harness-components) and where each one is documented.

## Why you'd want it

- 🔁 **An agent dies and nobody restarts it.** Now the runner and `tmux-sessions` bring it back.
- ⏰ **An agent works only while someone types.** Now messages, schedules and inboxes wake it.
- 🔑 **You cannot hand an agent a password.** Now helpers hold the passwords and the agent only calls them.
- 🛡️ **An agent may call anything it can reach.** Now the ACL decides whom it may call, and the runner sandbox decides what managed code may touch.

## Inner and outer harness

| Part | Who provides it | What it covers |
|---|---|---|
| Inner harness | Claude Code, Codex, OpenCode, or your own agent code | The model loop, planning, context, local tool use and the CLI's own permission prompts |
| Outer harness | agent-bus | Everything around and between agents: lifecycle, wake-up, tools on the bus, permissions, secrets, confinement, shared state and identity |

An agent written as a script or with a [client library](../src/clients/README.md) has no inner harness of its own; agent-bus supplies the outer one all the same.

## Harness components

| Component | What agent-bus provides | Where |
|---|---|---|
| Lifecycle | The runner starts, stops, restarts and reports managed agents; `tmux-sessions` brings AI sessions back after a crash or reboot | [runner](runner/README.md#what-you-get), [tmux sessions](agents/tmux-sessions.md#what-it-does), [restart after a crash](06-launchers.md#restart-after-a-crash) |
| Launch | `ab-*` launchers start an AI session already connected to the bus, under a stable name | [smart launchers](06-launchers.md#smart-launchers) |
| Wake | Messages are pushed into live sessions; inboxes wait for an agent that is away; schedules start work without a person | [adapters](06-launchers.md#adapters), [inbox queues](04-messaging.md#inbox-queues), [schedules](03-records-schedules.md#schedules) |
| Tools | MCP `ab_*` tools in every session, and ready-made helpers for chat, mail, web, servers and search | [tools inside the session](user/ai-cli.md#the-tools-inside-the-session), [bundled agents](agents/README.md) |
| Asking others | `ab_call` sends a question and waits for the answer; discovery tells an agent who is there and what each one can do | [tools inside the session](user/ai-cli.md#the-tools-inside-the-session), [discovery](05-discovery.md#what-a-listing-answers) |
| Permissions | Every call carries its sender's identity, and the ACL is checked before delivery | [what a call carries](02-access.md#what-a-call-carries), [ACL](02-access.md#acl), [who may call an agent](runner/security.md#who-may-call-an-agent) |
| Secrets | Passwords stay in helpers, services and the runner; the calling agent never sees them | [runner secrets](runner/security.md#secrets), [service secrets](03-records-service.md#secrets) |
| Confinement | Managed agents run in a sandbox with stated network, file and resource limits | [isolation today](runner/security.md#isolation-today) |
| Memory and state | A key-value store on every record, and keyword search over your documents | [key-value store](03-records-kv.md#key-value-store), [RAG](agents/rag.md#what-it-does) |
| Coordination | Shared locks so two agents do not do the same work at once | [shared locks](01-identity-and-authority.md#shared-locks) |
| Visibility | One page shows agents, queues, refusals and activity | [dashboard](05-discovery.md#dashboard), [refusals](05-discovery.md#refusals) |

## Two permission layers

Each layer answers a different question:

| Layer | Answers | Example |
|---|---|---|
| The AI CLI | May this session run that command on its own host? | Claude Code asks before it runs `rm` |
| agent-bus | May this agent call that name, and what may managed code reach? | The ACL refuses a session that sends to `#sms`; the SMS helper uses a password the session never sees |

agent-bus adds to the AI CLI's permissions; it does not replace them.

## What agent-bus does not do

- 🚫 **The model loop:** planning, context and tool choice stay with the AI CLI or your agent code.
- 🚫 **The session's own shell:** a session with shell access can do whatever its account can; agent-bus governs only what it does on the bus. The runner sandbox applies to managed agents, not to an AI session you start yourself.
- 🚫 **Trust in agent code:** a confined agent can still disclose what it receives ([isolation today](runner/security.md#isolation-today)).
