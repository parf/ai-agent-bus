# agent-bus

Connect AI agents, bots and services so they can find and message each other.
The Go daemon provides a registry and broker with a dashboard; the TypeScript
MCP face and adapters run on bun.

![Agents Bus](docs/img/agent-bus.png)

## Status

**MVP is in progress.** The PoC is complete. Current documentation describes the
whole [MVP scope](Plans/MVP/README.md#scope), with built and pending explicit.

| Built | Still pending |
|---|---|
| Principal credentials, local sockets, service ACL, profiles and Maintainers | — |
| Registry, topics, calls and restart snapshots | — |
| Foreground scripts, MCP, runtime adapters, launchers and fresh-host package | [Live-runtime, upgrade and installed browser acceptance](Plans/MVP/TODO.md#remaining-work) |
| Signed-in dashboard, people view and process split | Installed browser acceptance |

[Current docs](docs/00-overview.md#document-ownership) own the contracts;
[remaining work](Plans/MVP/TODO.md#objective) owns acceptance.

## Using it

New here? The [user guide](docs/user/README.md) is the shortest path from
nothing to a running agent.

Build and development setup are in [source instructions](src/README.md#build-and-check).
The self-contained archive and installer are built; [setup](docs/09-setup.md#install)
describes the path and its prerequisites. A configured local account uses its assigned socket as its credential
([access](docs/02-access.md#local-socket)).

### CLI example

On a configured bus, with permission to register these names:

```sh
agent-bus register mysql-prod@srv1 --addr host:3306 --protocol mysql
agent-bus topic create alerts.prod@srv1 --kind pubsub
agent-bus topic create build-jobs@srv1 --kind queue --ttl 1h --bound 1000
agent-bus ls
agent-bus publish --topic alerts.prod@srv1 "disk nearly full"
```

For a script service, use the [foreground runner](docs/08-runner-role.md#script-services).
The daemon is trusted with bodies in the MVP ([trust boundary](docs/02-access.md#trust-boundary));
restart persistence has the [documented loss window](docs/04-messaging.md#durability).

## Plans

| Plan | Status |
|---|---|
| [MVP](Plans/MVP/README.md#scope) | Current development |
| [R1](Plans/R1/README.md#scope) | Proposed distributed identity and managed services |
| [R1.1](Plans/R1.1/README.md#scope) | Proposed tools stage |
| [R1.2](Plans/R1.2/README.md#scope) | Unscheduled exploration after tools |
| [Future](Plans/Future/README.md#topics) | Generic undecided or unassigned ideas |

All local conventions live in [CLAUDE.md](CLAUDE.md#working-rules).

## License

[PolyForm Noncommercial](LICENSE.md#polyform-noncommercial-license-100).
