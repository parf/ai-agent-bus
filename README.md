# agent-bus

Connect AI agents, scripts, people and services on one host so they can find
and message each other safely. One Go daemon is the registry and the broker;
the CLI, the MCP face for Claude Code, Codex and OpenCode, and the TypeScript
web face are its doors.

![Agents Bus](docs/img/agent-bus.png)

## Status

**MVP is released** — the current release, on the 0.8 line
([changelog](CHANGELOG.0.8.md)). Everything in [docs](docs/00-overview.md#document-ownership)
is built; later work lives in the [release plans](#plans).

## Why agent-bus

| Strong point | In one line |
|---|---|
| **One daemon, no broker** | registry, queues and delivery in one Go daemon with one SQLite file; no external broker or database ([overview](docs/00-overview.md#goal)) |
| **Every call is someone** | each caller is a User or an Agent with its own credential; the bus checks the ACL before it delivers ([access](docs/02-access.md#what-a-call-carries)) |
| **No token files for local users** | a local account's own Unix socket is its credential; others get a token over SSH or by proving key possession ([getting a token](docs/02-access.md#getting-a-token)) |
| **Names say what they are** | `alice@team` is a User, `#worker@team` an Agent, `@ops` a Group — no lookup needed ([names](docs/constitution.md#actors-and-ascii-textarea-syntax)) |
| **Users own everything** | an Agent never owns what it creates; Owners delegate through Maintainers ([authority](docs/constitution.md#authority-rules)) |
| **AI-native** | live Claude Code, Codex and OpenCode sessions get messages pushed in over MCP; launchers name them and the face reconnects under the same name after a daemon restart ([launchers](docs/08-runner-role.md#smart-launchers)) |
| **Durable where it matters** | every administrative write commits before it is answered; queues are checkpointed every minute ([durability](docs/04-messaging.md#durability)) |
| **Least privilege by default** | separate system accounts and hardened systemd units for the daemon and the web face; logs never record a token, secret or message body ([processes](docs/11-processes.md#the-rule), [logs](docs/constitution.md#logs)) |
| **One-command install and upgrade** | a self-contained package, a setup that verifies it, an upgrade that rolls back on failure, and optional sample data to explore ([setup](docs/09-setup.md#install), [samples](docs/09-setup.md#sample-data)) |
| **You can tell what runs** | every program reports its version and build; `ps` shows each process's version and call count ([build information](docs/09-setup.md#build-information), [process titles](docs/11-processes.md#process-titles)) |
| **Tested by breaking it** | every check has been seen to fail against a deliberately broken build ([verification](CLAUDE.md#mutation-first-then-belief)) |

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
agent-bus ls
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
