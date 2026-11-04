# Overview

📌 **TL;DR:**
- **Problem:** AI agents and the services they use cannot reach each other without per-pair wiring, shared passwords and lost messages.
- **Outcome:** each one gets a name, an inbox and a permission list on one bus.
- **Start:** see who you are on the bus: `agent-bus status` ([Try this first](#try-this-first)).

## Why you'd want it

- 🔌 **Every pair of tools needs its own wiring.** Now each one has a name, and anyone allowed can send to it.
- 🔑 **Sharing a password to share a capability.** Now the bus checks who is calling before it delivers anything.
- 📭 **A message is lost when the reader is away.** Now a name owns its inbox, and work waits for whoever serves it next.
- 🧭 **Nobody knows what exists.** Now one catalog lists the agents, services and resources you may see.

## What you get

- ✅ **Names and permissions:** every agent, person and service is a named record with its own access list.
- ✅ **Messages that wait:** each name has an inbox; replies and receipts are ordinary messages.
- ✅ **One place to look:** a catalog, a dashboard and the command line show the same bus.
- ✅ **Helpers on top:** bundled agents add mail, chat, databases and more ([bundled agent guides](agents/README.md)).
- 🚫 **Not for:** hiding message bodies from the host that runs the bus ([trust boundary](02-access.md#trust-boundary)).

## Try this first

Check your connection to the bus:

```sh
agent-bus status
```

`you` shows your identity; `realm` identifies the bus's namespace.

## Guides

| Topic | Find |
|---|---|
| [Constitution](constitution.md#project-constitution) | Names, authority and delivery guarantees |
| [Identity and authority](01-identity-and-authority.md#identities) | Names, users, positions, groups, shared locks and record lifecycle |
| [Access](02-access.md#what-a-call-carries) | Authentication, credentials, ACLs, sockets and the trust boundary |
| ↳ [Remote access](02-access-remote.md#choosing-a-way) | Reaching the bus from another host: HTTP on a trusted network, HTTPS, and your own or the shared socket forwarded over SSH |
| [Records](03-records.md#record-kinds) | Record kinds and their fields |
| ↳ [Agents](03-records-agent.md#what-an-agent-is) | The 👾 kind: name, credential, inbox, fields, agent templates, configuration and how one is started |
| ↳ [Channels](03-records-channel.md#the-two-channel-kinds) | The 📮 and 📣 kinds: delivery, retention and what publish stamps |
| ↳ [Services](03-records-service.md#what-a-service-is) | The external 📡 case: address, protocol, secrets and what it has no queue for |
| ↳ [Resources](03-records-resource.md#what-a-resource-is) | The 📚 kind: the card, its source, listing and reading through MCP |
| ↳ [Key-value store](03-records-kv.md#key-value-store) | Every record's durable string, int and JSON values: verbs, atomic JSON operations, limits and coordination patterns |
| [Messaging](04-messaging.md#inbox-queues) | Delivery, receipts, deadlines, TTL, overflow and durability |
| [Discovery](05-discovery.md#faces) | Catalog, listing, dashboard, administration and what a refusal answers |
| [Dedicated runner](06-dedicated-runner.md#script-agents) | `agent-bus run`: one script behind one agent, optionally sandboxed |
| [Session launchers](06-launchers.md) | Push adapters and the `ab-*` runtime launchers |
| [Runner](runner/README.md) | Configure, run and monitor installed agents |
| [Setup](07-setup.md#install) | Install, update or configure with your AI; manual installation fallback |
| ↳ [Backup and restore](administration.md#backup-and-restore) | Encrypted record exports and imports, and the stopped-daemon backup |
| [Processes](08-processes.md#the-processes) | Supervisor, bus and web; privileges, listeners and process titles |
| [Daemon API](09-daemon-api.md#how-a-call-is-made) | The HTTP routes, grouped; each one's meaning stays with its topic |
| [Writing an agent](10-writing-an-agent.md#3-ways-to-run-agents) | Execution choices and the minimum, suggested and optional agent-code checklist; contracts stay at their linked owning sections |
| [Create an agent](agents/create-agent-prompt.md#prompt-to-paste) | Copyable AI prompt for implementing, packaging and verifying a new agent |
| [Bundled agent guides](agents/README.md) | Ready-made helpers: choosing, installing and using them |
| [User guides](user/README.md) | What to type: the command-line tools, running an agent, the daemon, and agent sessions; the *why* stays in the topics above |
| [Glossary](glossary.md#names) | Current vocabulary |

## How it works

One daemon provides the registry and inboxes; no external broker is required. The CLI, MCP tools and web panel use that same bus. Record state and queue checkpoints are stored in one SQLite database.
