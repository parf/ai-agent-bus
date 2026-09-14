# src

The code for [stages § MVP](../Plans/MVP/README.md#scope), built wave by wave
against [MVP work](../Plans/MVP/TODO.md#objective).

| | |
|---|---|
| `cmd/agent-busd` | the daemon: unix socket and loopback TCP, one token per principal |
| `cmd/agent-bus-setup` | the root-only installer: two accounts, the tree, the unit |
| `cmd/agent-bus-admin` | what edits the `agent-busd` account's files |
| `cmd/agent-bus-web` | the dashboard, a separate process speaking the API |
| `cmd/agent-bus` | the CLI — `agent-bus help` lists every verb |
| `internal/protocol` | names and envelopes, no behaviour |
| `internal/core` | the registry and the queues — the only place a routing decision is made |
| `internal/api` | the HTTP face: a request in, a core call out |
| `internal/auth` | credentials: issue, rotate, resolve a token to its principal |
| `internal/ports` | the interfaces core depends on — the seam every dependency is swapped at |
| `internal/store`, `internal/dump`, `internal/directory`, `internal/signature`, `internal/sandbox` | one adapter each behind those ports |
| `mcp/` | the MCP face and both push adapters, on bun — [mcp/README.md](mcp/README.md#the-mcp-face) |
| `launchers/` | smart runtime launchers — [contract and usage](../docs/08-runner-role.md#running-the-launchers) |
| `cmd/agent-bus-token` | the token program, and the forced command behind an ordinary user's key ([access § getting a token](../docs/02-access.md#getting-a-token)) |
| `smoke.sh` | automated acceptance checks; installed and manual gates remain in the MVP plan |

Layering is the design's: protocol → core → api, faces outside, nothing
pointing back in ([modules](../docs/10-modules.md#the-rule)).

## Build and check

Build requirements and version output follow
[setup § build information](../docs/09-setup.md#build-information).
Release numbering follows [working rules § versioning](../CLAUDE.md#versioning).

With an output-directory argument, the build also bundles the MCP face and
launchers with Bun, copies the release version and license, and exposes launcher
entry points at the output root. Move that whole directory together; running it
requires Bun and the selected AI runtime, but no checkout or npm dependencies.

```sh
bash ./build.sh       # build every Go program with build information
go test -race ./...
./smoke.sh            # fast: everything under a second, for the edit-run loop
./smoke.sh --slow     # all of it, plus the race detector — what a change is measured against
```

`smoke.sh` needs `bun` for the MCP half. It uses a temporary socket and token
of its own, so it never touches a daemon you are running; `PORT=` moves its
loopback port if the default one is taken.

A check here is not believed until it has been seen to fail —
[working rules § verification](../CLAUDE.md#verification).

## Three terminals

By hand. Build the binaries first — nothing installs them:

```sh
bash ./build.sh
```

**1 — the daemon.** It writes the token file on first run, so it goes first:

```sh
./agent-busd
```

The other two shells each need **a credential of their own** — the name is the
inbox, so two shells sharing one would read each other's messages
([messaging § one reader per inbox](../docs/04-messaging.md#one-reader-per-inbox)).
The CLI finds the socket by itself; `AGENT_BUS_ADDR` is for a bus on another
host.

```sh
export AGENT_BUS_TOKEN=$(./agent-bus-token echo@$(hostname -s))
```

**2 — a service.** A service *is* a name, so the shell that answers takes it:

```sh
./agent-bus register echo@$(hostname -s) --kind generic --descr "answers"
./agent-bus consume --wait 5m          # prints the envelope, with its message_id
./agent-bus ack   <message-id>         # got it
./agent-bus reply <message-id> "42"    # the answer
```

…or let a shell script be the service, which is the same thing without the
typing ([runner § script services](../docs/08-runner-role.md#script-services)).
This shell still needs a credential — it registers the service before becoming
it, and that registration is a call like any other — but the token it
registers with is the *owner's*, not the service's:

```sh
echo 'echo "Hello $1"' > hello-world.sh && chmod +x hello-world.sh
./agent-bus start hello@$(hostname -s) --algo args ./hello-world.sh -4 --descr "greets you"
```

**3 — the caller**, with a credential of its own, calling whichever of the two
you started:

```sh
export AGENT_BUS_TOKEN=$(./agent-bus-token caller@$(hostname -s))
./agent-bus ls                                        # find it
./agent-bus call echo@$(hostname -s)  "what is 6 times 7?"   # answered by hand, above
./agent-bus call hello@$(hostname -s) world                  # → Hello world
```

Topics need no service at either end — a publisher that is nobody, and a
consumer that was not running when it was sent:

```sh
./agent-bus topic create jobs@$(hostname -s) --descr "work queue"
./agent-bus publish --topic jobs@$(hostname -s) "sweep the floor"
./agent-bus consume --topic jobs@$(hostname -s)    # later, from anywhere
```

## Two agents

A Claude Code session and a Codex session talk over the same daemon through
the MCP face; loading it into each, and the live recipe, are in
[mcp/README.md](mcp/README.md#the-mcp-face).
