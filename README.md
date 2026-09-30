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

**Simple to run**

<dl>
<dt><a href="docs/00-overview.md#goal">One daemon, no broker</a></dt>
<dd>One Go binary and one SQLite file.</dd>
<dt><a href="docs/02-access-remote.md#choosing-a-way">Any number of hosts</a></dt>
<dd>Over HTTP, HTTPS or a forwarded socket.</dd>
<dt><a href="docs/09-setup.md#install">One-command install</a></dt>
<dd>A verified setup; an upgrade that rolls back on failure.</dd>
<dt><a href="docs/09-setup.md#build-information">You can tell what runs</a></dt>
<dd>Every program and <code>ps</code> line shows its version.</dd>
</dl>

**Safe by design**

<dl>
<dt><a href="docs/02-access.md#what-a-call-carries">Every call is someone</a></dt>
<dd>Its own credential, and the ACL checked before delivery.</dd>
<dt><a href="docs/02-access.md#getting-a-token">No token files for local users</a></dt>
<dd>Your Unix socket is your credential.</dd>
<dt><a href="docs/constitution.md#actors-and-ascii-textarea-syntax">Names say what they are</a></dt>
<dd><code>alice@team</code> a User, <code>#worker@team</code> an Agent, <code>@ops</code> a Group.</dd>
<dt><a href="docs/constitution.md#authority-rules">Users own everything</a></dt>
<dd>An Agent never owns what it creates.</dd>
<dt><a href="docs/11-processes.md#the-rule">Least privilege</a></dt>
<dd>Separate accounts, hardened units, no secrets in logs.</dd>
<dt><a href="docs/04-messaging.md#durability">Durable where it matters</a></dt>
<dd>Administrative writes commit before they are answered.</dd>
</dl>

**Made for agents**

<dl>
<dt><a href="docs/08-runner-role.md#smart-launchers">AI-native</a></dt>
<dd>Live Claude Code, Codex and OpenCode sessions get messages pushed in.</dd>
<dt><a href="docs/08-runner-role.md#script-agents">Any script is an agent</a></dt>
<dd><code>agent-bus start</code> serves it under a name, through restarts.</dd>
<dt><a href="CLAUDE.md#mutation-first-then-belief">Tested by breaking it</a></dt>
<dd>Every check has been seen to fail first.</dd>
</dl>

## Concepts

| Concept | What it is |
|---|---|
| **Records** | everything on the bus is a named record of one of six [kinds](docs/constitution.md#-record-kind): 👤 user, 👾 agent, 📮 queue, 📣 pubsub, 📡 service, 👥 group; an optional realm (`@team`) is part of the name |
| **Inbox** | a User, an Agent and a queue each hold one; a message waits there until a reader takes it ([inbox queues](docs/04-messaging.md#inbox-queues)) |
| **Queue and pubsub** | a 📮 queue hands each message to one competing reader; a 📣 topic copies each publication to its Deliver-To list ([channels](docs/07-channels.md#the-two-channel-kinds)) |
| **Forwarding** | an agent or queue may pass what it receives on to one destination, which must allow it ([channels](docs/constitution.md#-channels)) |
| **ACL** | who may reach a record: Users, Agents, Groups (nested), `*`, `@owner` and `@agent` ([ACL](docs/02-access.md#acl)) |
| **Personal** | a record meant only for its Owner and that Owner's Agents ([records](docs/03-records.md#record-kinds)) |
| **Request and reply** | replies are ordinary messages matched by topic and tag; the daemon keeps no conversation state ([request and reply](docs/04-messaging.md#request-and-reply)) |
| **Receipts and deadlines** | a reader may confirm it took or finished a message, and a caller may say when an answer stops being useful ([receipts](docs/04-messaging.md#receipts)) |
| **One reader per inbox** | a message is taken at most once; readers that say so may share an inbox as a pool ([one reader](docs/04-messaging.md#one-reader-per-inbox)) |
| **TTL, bound and overflow** | how long an inbox keeps a message, how many it holds, and whether a full one refuses or drops the oldest ([overflow](docs/04-messaging.md#overflow)) |
| **Services** | a 📡 record describes an external service — address, protocol and a secret only its allow list reads ([services](docs/06-services.md#what-a-service-is)) |
| **Private values** | an Agent, Service or Group may carry a configuration and a secret; everyone else sees only a digest ([private values](docs/constitution.md#-private-values)) |
| **Inactive** | an inactive record is no such entity: hidden, refused, granting nothing, its name kept ([common fields](docs/constitution.md#common-record-fields)) |
| **Roles** | the daemon Owner, Administrators (the protected `@administrators` group), Users, and the Agents they own ([identity and roles](docs/01-identity-and-roles.md#identities)) |
| **Faces** | the `agent-bus` CLI; the MCP face and its launchers; the foreground runner, which puts a script behind an agent name, optionally sandboxed; and the web face ([discovery](docs/05-discovery.md#faces)) |
| **Web face** | its own process and hardened unit: overview, every record and person, activity history by day, week and month, and diagnostics, with bodies never shown ([web face](docs/11-processes.md#the-web-face)) |
| **Logs** | an audit log of every administrative action, an error log copied to syslog, and an on-demand debug log ([logs](docs/constitution.md#logs)) |

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
