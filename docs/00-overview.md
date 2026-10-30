# Overview

📌 **TL;DR:**
- **Problem:** AI agents and the services they use cannot reach each other without per-pair wiring, shared passwords and lost messages.
- **Outcome:** each one gets a name, an inbox and a permission list on one bus; this documentation covers current behaviour on the 0.9 line, and anything not built is in its release plan.
- **Start:** see who you are on the bus: `agent-bus status` ([Try this first](#try-this-first)). Each value below is owned by one section the others link to; stage status stays in the [plan index](../Plans/README.md#stages).

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
- 🚫 **Not for:** hiding message bodies from the host that runs the bus ([trade offs](#trade-offs)).

## Try this first

- See your own name and the bus's state (read-only):

```sh
agent-bus status | jq '{realm, you}'
```

<details>
<summary>See it on a real node (ran today)</summary>

```json
{"realm":"parf.us","you":"#ab-w1b"}
```

- The realm is the bus's own name, and `you` is the name your credential carries.
- The output is from the development node on 2026-10-30.

</details>

## Technical details

Everything below is the reference. Each section owns its values; other pages link here.

## Document ownership

Values have one owning section; other pages link to it. A `↳` topic belongs to the one above it.

| Topic | Owns |
|---|---|
| [Constitution](constitution.md#project-constitution) | The model every face implements; open gaps are the [constitution conformance](../Plans/done/R0.8-MVP/TODO.md#constitution-conformance) rows |
| [Identity and authority](01-identity-and-authority.md#identities) | Names, users, positions, groups, shared locks and record lifecycle |
| [Access](02-access.md#what-a-call-carries) | Authentication, credentials, ACLs, sockets and the trust boundary |
| ↳ [Remote access](02-access-remote.md#choosing-a-way) | Reaching the bus from another host: HTTP on a trusted network, HTTPS, and your own or the shared socket forwarded over SSH |
| [Records](03-records.md#record-kinds) | Record kinds, common and kind-specific fields, and Personal |
| ↳ [Agents](03-records-agent.md#what-an-agent-is) | The 👾 kind: name, credential, inbox, fields, agent templates, configuration and how one is started |
| ↳ [Channels](03-records-channel.md#the-two-channel-kinds) | The 📮 and 📣 kinds: delivery, retention and what publish stamps |
| ↳ [Services](03-records-service.md#what-a-service-is) | The external 📡 case: address, protocol, secrets and what it has no queue for |
| ↳ [Resources](03-records-resource.md#what-a-resource-is) | The 📚 kind: the card, its source, listing and reading through MCP |
| ↳ [Key-value store](03-records-kv.md#key-value-store) | Every record's durable string, int and JSON values: verbs, atomic JSON operations, limits and coordination patterns |
| [Messaging](04-messaging.md#inbox-queues) | Delivery, receipts, deadlines, TTL, overflow and durability |
| [Discovery](05-discovery.md#faces) | Catalog, listing, dashboard, administration and what a refusal answers |
| [Dedicated runner](06-dedicated-runner.md#script-agents) | `agent-bus run`: one script behind one agent, optionally sandboxed |
| [Session launchers](06-launchers.md) | Push adapters and the `ab-*` runtime launchers |
| [Runner](runner/README.md) | Installed script agents: packaging, configuration, operations and current security limits |
| [Setup](07-setup.md#the-programs) | Programs, accounts, paths, installation and build information |
| ↳ [Backup and restore](07-setup.md#backup-and-restore) | Encrypted record exports and imports, and the stopped-daemon backup |
| [Processes](08-processes.md#the-processes) | Supervisor, bus and web; privileges, listeners and process titles |
| [Daemon API](09-daemon-api.md#how-a-call-is-made) | The HTTP routes, grouped; each one's meaning stays with its topic |
| [Create an agent](agents/create-agent-prompt.md#prompt-to-paste) | Copyable AI prompt for implementing, packaging and verifying a new agent |
| [Bundled agent guides](agents/README.md) | Ready-made helpers: choosing, installing and using them |
| [User guides](user/README.md) | What to type: the command-line tools, running an agent, the daemon, and agent sessions; the *why* stays in the topics above |
| [Glossary](glossary.md#names) | Current vocabulary |
| [Decisions](decisions.md#settled) | Index of current contracts |

## Goal

- One daemon provides a registry for agents and services, and message queues for agents.
- The CLI, MCP face and the TypeScript [web face](08-processes.md#the-web-face) expose that bus.
- It requires no external broker.

## Principles

- Credentials identify callers, and the bus enforces access before delivery
  ([access](02-access.md#what-a-call-carries), [ACL](02-access.md#acl)).
- A name owns its inbox; the process serving it may disappear while work waits
  ([messaging](04-messaging.md#inbox-queues)).
- Replies and receipts are ordinary messages; the daemon keeps no exchange state
  ([messaging](04-messaging.md#reply-routing)).
- The dashboard sees a filtered envelope feed, with bodies removed in the bus
  ([discovery](05-discovery.md#dashboard)). This is not body encryption.
- One SQLite database holds records, credentials and queues; queue state is
  checkpointed every minute ([durability](04-messaging.md#durability)).
- Runtime privilege boundaries and code dependency boundaries are separate
  ([processes](08-processes.md#the-rule), [modules](../src/MODULES.md#the-rule)).

## Process roles

The built runtime is described in [processes § the processes](08-processes.md#the-processes).
Optional future process roles are owned by their release plans, not this document.

## Trade offs

| Choice | Accepted cost |
|---|---|
| [At most once consumption](04-messaging.md#one-reader-per-inbox) | A reader dying after dequeue can lose the message |
| [In-memory queues with a minute checkpoint](04-messaging.md#durability) | An unclean stop can lose traffic since the last checkpoint |
| [Trusted host](02-access.md#trust-boundary) | The daemon can read bodies and stored configuration |
| [Filtered waits](04-messaging.md#one-reader-per-inbox) | A simultaneous unfiltered reader can take an exchange's next reply between waits |
