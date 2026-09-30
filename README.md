# agent-bus

Connect AI agents, scripts, people and services — on one machine or across
many — so they can find and message each other safely. One Go daemon is the registry and the broker;
the CLI, the MCP face for Claude Code, Codex and OpenCode, and the TypeScript
web face are its doors.

![Agents Bus](docs/img/agent-bus.png)

## Status

**MVP is released** — the current release, on the 0.8 line
([changelog](CHANGELOG.0.8.md)). Everything in [docs](docs/00-overview.md#document-ownership)
is built; later work lives in the [release plans](#plans).

## Why agent-bus

AI sessions, scripts and services need to reach each other — across machines,
by name, with a say over who may call whom. The usual answer is a broker, a
database and an auth server. agent-bus is one daemon.

### Simple to run

<dl>
<dt><strong><a href="docs/00-overview.md#goal">One daemon, no broker</a></strong></dt>
<dd>One Go binary and one SQLite file.</dd>
<dt><strong><a href="docs/02-access-remote.md#choosing-a-way">Any number of hosts</a></strong></dt>
<dd>Over HTTP, HTTPS or a forwarded socket.</dd>
<dt><strong><a href="docs/09-setup.md#install">One-command install</a></strong></dt>
<dd>A verified setup; an upgrade that rolls back on failure.</dd>
<dt><strong><a href="docs/09-setup.md#build-information">You can tell what runs</a></strong></dt>
<dd>Every program and <code>ps</code> line shows its version.</dd>
</dl>

### Made for agents

![Codex and Claude sessions greeting each other and OpenCode over the bus](docs/img/agents-talking.png)

<dl>
<dt><strong><a href="docs/08-runner-role.md#smart-launchers">AI-native</a></strong></dt>
<dd>Live Claude Code, Codex and OpenCode sessions get messages pushed in.</dd>
<dt><strong><a href="docs/05-discovery.md#mcp-minimum">MCP tools</a></strong></dt>
<dd>AI CLI tools see agents, services, queues and topics over MCP, and can list, send, consume and reply.</dd>
<dt><strong><a href="docs/08-runner-role.md#script-agents">Any script is an agent</a></strong></dt>
<dd><code>agent-bus start</code> serves it under a name, through restarts.</dd>
<dt><strong><a href="CLAUDE.md#mutation-first-then-belief">Tested by breaking it</a></strong></dt>
<dd>Every check has been seen to fail first.</dd>
</dl>

### Web management panel

![The web panel's overview](docs/img/web-overview.png)

- **Overview** — what needs attention (refusals, full queues, lost messages), then live counters per kind.
- **A page per kind** — Agents, Services, Queues, PubSub, Users, Groups: search, filters, details.
- **Management** — register records, edit ACLs and group members, manage users, transfer or retire.
- **Activity graphs** — by day, week and month, per record or node-wide, 400 days back with ‹ Prev.
- **Diagnostics** — refusals by reason and stray names, refreshed on demand; message bodies never shown.
- **Isolated** — its own process and `nobody`-like account, `agent-bus-web`; holds no bus state.

### Safe by design

<dl>
<dt><strong><a href="docs/02-access.md#what-a-call-carries">Every call is someone</a></strong></dt>
<dd>Its own credential, and the ACL checked before delivery.</dd>
<dt><strong><a href="docs/02-access.md#getting-a-token">No token files for local users</a></strong></dt>
<dd>Your Unix socket is your credential.</dd>
<dt><strong><a href="docs/constitution.md#actors-and-ascii-textarea-syntax">Names say what they are</a></strong></dt>
<dd><code>alice@team</code> a User, <code>#worker@team</code> an Agent, <code>@ops</code> a Group.</dd>
<dt><strong><a href="docs/constitution.md#authority-rules">Users own everything</a></strong></dt>
<dd>An Agent never owns what it creates.</dd>
<dt><strong><a href="docs/11-processes.md#the-rule">Least privilege</a></strong></dt>
<dd>Separate accounts, hardened units, no secrets in logs.</dd>
<dt><strong><a href="docs/04-messaging.md#durability">Durable where it matters</a></strong></dt>
<dd>Administrative writes commit before they are answered.</dd>
</dl>

## Concepts

### Record kinds

Everything on the bus is a named <a href="docs/03-records.md#record-kinds">record</a> of one of six kinds.
An optional realm (<code>@team</code>) is part of the name.

<dl>
<dt>👤 <strong><a href="docs/01-identity-and-roles.md#users-and-profiles">User</a></strong></dt>
<dd>A person, and the inbox they read: <code>alice</code> or <code>alice@team</code>.</dd>
<dt>👥 <strong><a href="docs/01-identity-and-roles.md#groups">Group</a></strong></dt>
<dd>A named list of actors for ACLs; can have owners, maintainers and secrets.</dd>
<dt>👾 <strong><a href="docs/08-runner-role.md#what-the-runner-does">Agent</a></strong></dt>
<dd>An AI session or a script, and the inbox it reads; its name begins with <code>#</code>.</dd>
<dt>📮 <strong><a href="docs/07-channels.md#the-two-channel-kinds">Queue</a></strong></dt>
<dd>A shared inbox that hands each message to one competing reader.</dd>
<dt>📣 <strong><a href="docs/07-channels.md#the-two-channel-kinds">PubSub</a></strong></dt>
<dd>Keeps nothing; copies each publication to everyone on its Deliver-To list.</dd>
<dt>📡 <strong><a href="docs/06-services.md#what-a-service-is">Service</a></strong></dt>
<dd>A card for something outside the bus: address, protocol and a secret only its allow list reads.</dd>
</dl>

### Records and access

<dl>
<dt><strong><a href="docs/constitution.md#authority-rules">Owner</a></strong></dt>
<dd>The User a record belongs to; may transfer it and names its Maintainers.</dd>
<dt><strong><a href="docs/constitution.md#authority-rules">Maintainers</a></strong></dt>
<dd>Users or Agents the Owner lets edit a record's description, ACL and status.</dd>
<dt><strong><a href="docs/02-access.md#acl">ACL</a></strong></dt>
<dd>Who may reach a record: Users, Agents, Groups (nested), or <code>*</code> for every registered user.</dd>
<dt><strong><a href="docs/03-records.md#personal-and-shared">Personal</a></strong></dt>
<dd>A record meant only for its Owner and that Owner's Agents.</dd>
<dt><strong><a href="docs/constitution.md#-private-values">Private values</a></strong></dt>
<dd>An Agent, Service or Group may carry a configuration and a secret; everyone else sees only a digest.</dd>
<dt><strong><a href="docs/constitution.md#common-record-fields">Active / Inactive</a></strong></dt>
<dd>Turns any record on or off — a user, agent, group or service. An inactive one is hidden and refused; its name is kept.</dd>
</dl>

### Messaging

<dl>
<dt><strong><a href="docs/04-messaging.md#inbox-queues">Inbox</a></strong></dt>
<dd>A User, an Agent and a queue each hold one; a message waits there until a reader takes it.</dd>
<dt><strong><a href="docs/constitution.md#-channels">Forwarding</a></strong></dt>
<dd>An agent or queue may pass what it receives on to one destination, which must allow it.</dd>
<dt><strong><a href="docs/04-messaging.md#request-and-reply">Request and reply</a></strong></dt>
<dd>Replies are ordinary messages matched by topic and tag; the daemon keeps no conversation state.</dd>
<dt><strong><a href="docs/04-messaging.md#receipts">Receipts and deadlines</a></strong></dt>
<dd>A reader may confirm it took or finished a message, and a caller may say when an answer stops being useful.</dd>
<dt><strong><a href="docs/04-messaging.md#one-reader-per-inbox">One reader per inbox</a></strong></dt>
<dd>A message is taken at most once; readers that say so may share an inbox as a pool.</dd>
<dt><strong><a href="docs/04-messaging.md#overflow">TTL, bound and overflow</a></strong></dt>
<dd>How long an inbox keeps a message, how many it holds, and whether a full one refuses or drops the oldest.</dd>
</dl>

### Faces and logs

<dl>
<dt><strong><a href="docs/05-discovery.md#faces">Faces</a></strong></dt>
<dd>The <code>agent-bus</code> CLI; the MCP face and its launchers; the foreground runner, which puts a script behind an agent name, optionally sandboxed; and the web face.</dd>
<dt><strong><a href="docs/11-processes.md#the-web-face">Web face</a></strong></dt>
<dd>Its own process and hardened unit: overview, every record and person, activity history by day, week and month, and diagnostics, with bodies never shown.</dd>
<dt><strong><a href="docs/constitution.md#logs">Logs</a></strong></dt>
<dd>An audit log of every administrative action, an error log copied to syslog, and an on-demand debug log.</dd>
</dl>

The [constitution](docs/constitution.md#project-constitution) is the model in
one page; the [glossary](docs/glossary.md#names) names everything.

## Using it

New here? The [user guide](docs/user/README.md) is the shortest path from
nothing to a running agent. Installing is [setup](docs/09-setup.md#install);
building from source is [source instructions](src/README.md#build-and-check).

### CLI example

On a configured bus, with permission to register these names:

```sh
agent-bus register mysql-prod@srv1 --addr host:3306 --protocol mysql
agent-bus channel create alerts.prod@srv1 --kind pubsub
agent-bus channel create build-jobs@srv1 --kind queue --ttl 1h --bound 1000
agent-bus manage alerts.prod@srv1 --add-to-set-allow '@ops'
agent-bus ls --all
agent-bus publish --channel alerts.prod@srv1 "disk nearly full"
agent-bus send '#worker@srv1' "an agent's name begins with #"
```

The daemon is trusted with message bodies ([trust boundary](docs/02-access.md#trust-boundary)),
and a crash may lose queue traffic since the last checkpoint
([durability](docs/04-messaging.md#durability)).

## Plans

| Release | Status |
|---|---|
| [R0.8 (MVP)](Plans/R0.8-MVP/README.md#scope) | Released, current (0.8) |
| [R1](Plans/R1.0-Release/README.md#scope) | Proposed: distributed identity and managed services |
| [R1.1](Plans/R1.1/README.md#scope) | Proposed tools stage |
| [R1.2](Plans/R1.2/README.md#scope) | Unscheduled exploration after tools |
| [R2.0](Plans/R2.0-Future/README.md#topics) | Undecided or unassigned ideas |

All local conventions live in [CLAUDE.md](CLAUDE.md#working-rules).

## License

[PolyForm Noncommercial](LICENSE.md#polyform-noncommercial-license-100).
