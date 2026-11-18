# AI agent harness: components agent-bus provides

📌 **TL;DR:**
- **Problem:** an AI agent needs more than a model loop: something must start it, wake it, give it tools, hold its passwords and limit what it may do.
- **Outcome:** agent-bus provides those essential harness components for any agent: lifecycle, wake-up, tools, permissions (ACL), secrets and a sandbox.
- **Start:** see the [harness components](#harness-components) and where each one is documented.

## Why you'd want it

- 🔁 **An agent dies and nobody restarts it.** Now the runner keeps managed agents running, and a launched AI session can resume after a crash.
- ⏰ **An agent works only while someone types.** Now messages, schedules and inboxes wake it.
- 🔑 **You cannot hand an agent a password.** Now helpers hold the passwords, and the ACL grants use of a helper, not its credential.
- 🛡️ **An agent may call anything it can reach.** Now the ACL decides whom it may call.
- 📦 **Agent code can touch the whole host.** Now the runner sandbox gives a managed agent only what its task needs.

## Inner and outer harness

| Part | Who provides it | What it covers |
|---|---|---|
| Inner harness | Claude Code, Codex, OpenCode, or your own agent code | The model loop, planning, context, local tool use and the CLI's own permission prompts |
| Outer harness | agent-bus | Everything around and between agents ([components](#harness-components)) |

An agent written as a script or with a [client library](../src/clients/README.md) has no inner harness of its own; agent-bus supplies the outer one all the same.

## Harness components

| Component | What agent-bus provides | Where |
|---|---|---|
| Lifecycle | The runner starts, stops and restarts managed agents and keeps always-on ones running; `ab-* --restart` resumes a crashed AI session; `tmux-sessions` reopens configured windows on request | [runner](runner/README.md#what-you-get), [tmux sessions](agents/tmux-sessions.md#what-it-does), [restart after a crash](06-launchers.md#restart-after-a-crash) |
| Launch | `ab-*` launchers start an AI session already connected to the bus, under a stable name | [smart launchers](06-launchers.md#smart-launchers) |
| Wake | Messages are pushed into live sessions; inboxes wait for an agent that is away; schedules start work without a person | [adapters](06-launchers.md#adapters), [inbox queues](04-messaging.md#inbox-queues), [schedules](03-records-schedules.md#schedules) |
| Tools | MCP `ab_*` tools in every launched session, and ready-made helpers for chat, mail, web, servers and search | [tools inside the session](user/ai-cli.md#the-tools-inside-the-session), [bundled agents](agents/README.md#bundled-agents) |
| Asking others | `ab_call` sends a question and waits for the answer; discovery tells an agent who is there, and `ask: help` what each one can do | [tools inside the session](user/ai-cli.md#the-tools-inside-the-session), [discovery](05-discovery.md#what-a-listing-answers) |
| Permissions | Every call carries its sender's identity, and the ACL is checked before delivery | [what a call carries](02-access.md#what-a-call-carries), [ACL](02-access.md#acl), [who may call an agent](runner/security.md#who-may-call-an-agent) |
| Secrets | Passwords stay in helpers, services and the runner; the ACL grants use of a helper, not its credential | [runner secrets](runner/security.md#secrets), [service secrets](03-records-service.md#secrets) |
| Sandbox | On Linux, managed agents run by default under bubblewrap and their own cgroup, with stated network, file and resource limits | [isolation today](runner/security.md#isolation-today) |
| Memory and state | A key-value store on every record, and keyword, semantic or hybrid search over your documents | [key-value store](03-records-kv.md#key-value-store), [RAG](agents/rag.md#what-it-does) |
| Coordination | Shared locks so two agents do not do the same work at once | [shared locks](01-identity-and-authority.md#shared-locks) |
| Visibility | The web panel shows agents, queues, refusals and activity | [dashboard](05-discovery.md#dashboard), [refusals](05-discovery.md#refusals) |

## Four permission layers

Each layer answers a different question:

| Layer | Answers | Example |
|---|---|---|
| The AI CLI | May this session run that command on its own host? | The CLI's own permission mode decides; the `ab-*` launchers start it in [automatic execution](06-launchers.md#smart-launchers) |
| The bus ACL | May this agent call that name? | The ACL refuses a session that sends to `#sms-telnyx` |
| Secret storage | Who may read a credential, and where does it live? | A record's Secret is readable only by its Owner, Maintainers and its own agent; a runner's host secrets stay in a `0700` directory and reach the agent only as environment at start ([runner secrets](runner/security.md#secrets)) |
| The runner sandbox | What may a managed agent see, reach and use? | Only what its task needs: its own files, no network unless it asks for it, set limits on memory, processes, CPU and disk, and no spying on other programs. On Linux this uses bubblewrap and cgroups ([processes and limits](runner/security.md#processes-and-limits)) |

agent-bus governs bus calls, credentials and managed agents; it does not confine an AI session's own shell.

## What agent-bus does not do

- 🚫 **The model loop:** planning, context and tool choice stay with the AI CLI or your agent code.
- 🚫 **The session's own shell:** a session with shell access can do whatever its account can; agent-bus governs only what it does on the bus. The runner sandbox applies to managed agents, not to an AI session you start yourself.
- 🚫 **Trust in agent code:** a confined agent can still disclose what it receives ([isolation today](runner/security.md#isolation-today)).
