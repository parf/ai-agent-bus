# src

📌 **TL;DR:** The code of the current release, [MVP](../Plans/R0.8-MVP/README.md#scope):
Go programs and internal packages for the daemon, CLI, installer and token
program, and TypeScript on Bun for the MCP face, launchers and web face.
`smoke.sh --slow` is the gate every change is measured against.

| | |
|---|---|
| `cmd/agent-busd` | the daemon: unix socket and loopback TCP, one token per principal |
| `cmd/agent-bus-setup` | the root-only installer: verified release, accounts, tree, daemon and web units |
| `cmd/agent-bus-admin` | what edits the `agent-busd` account's files |
| `cmd/agent-bus` | the CLI — `agent-bus help` lists every verb |
| `internal/protocol` | names and envelopes, no behaviour |
| `internal/core` | the registry and the queues — the only place a routing decision is made |
| `internal/api` | the HTTP face: a request in, a core call out |
| `internal/auth` | credentials: issue, rotate, resolve a token to its principal |
| `internal/ports` | the interfaces core depends on — the seam every dependency is swapped at |
| `internal/store` (SQLite, memory), `internal/directory` (GitHub, key files), `internal/signature`, `internal/sandbox`, `internal/journal` | the adapters behind those ports |
| `internal/activity`, `internal/callstats`, `internal/display`, `internal/keyproof`, `internal/proctitle`, `internal/dashboard`, `internal/version` | day-of-traffic rings, request sampling, human labels, the client half of key proof, process titles, the dashboard address, the shared release number |
| `mcp/` | the MCP face and its Claude, Codex and opencode push adapters, on bun — [mcp/README.md](mcp/README.md#the-mcp-face) |
| `web/` | the web face, TypeScript on bun under its own account and unit — [processes § the web face](../docs/11-processes.md#the-web-face) |
| `launchers/` | smart runtime launchers — [contract and usage](../docs/08-runner-role.md#running-the-launchers) |
| `cmd/agent-bus-token` | the token program, and the forced command behind an ordinary user's key ([access § getting a token](../docs/02-access.md#getting-a-token)) |
| `smoke.sh` | the automated acceptance checks |

Layering is the design's: protocol → core → api, faces outside, nothing
pointing back in ([modules](../docs/10-modules.md#the-rule)).

## Build and check

Build requirements and version output follow
[setup § build information](../docs/09-setup.md#build-information).
Release numbering follows [working rules § versioning](../CLAUDE.md#versioning).

With an output-directory argument, the build also bundles the MCP face and
launchers with Bun, copies the web face's sources and unit (no tests), the
release version, license and standalone install guide, and exposes launcher
entry points at the output root. `package.sh` adds
the exact artifact manifest and produces the supported tar archive plus its
portable SHA-256 file. Running a launcher requires Bun and the selected AI
runtime; the installed dashboard requires Bun at `/usr/bin/bun`; the daemon
requires neither.

```sh
bash ./build.sh       # build every Go program with build information
bash ./package.sh     # self-contained Linux archive under ../tmp/dist
go test -race ./...
./smoke.sh            # fast: everything under a second, for the edit-run loop
./smoke.sh --slow     # the gate, race detector included, under a minute — what a change is measured against
./smoke.sh --heavy    # everything, including what cannot fit that minute; on demand
```

`smoke.sh` needs `bun` for the MCP and web shards. It builds once and runs its shards in
parallel, each with a temporary socket, token and daemon of its own, so it never
touches a daemon you are running. `PORT=` moves its loopback base; a run owns
fifteen ports per shard from there, so two runs at once need bases a few
hundred apart.

A check here is not believed until it has been seen to fail —
[working rules § verification](../CLAUDE.md#verification).

## Three terminals

By hand. Build the binaries first — nothing installs them:

```sh
bash ./build.sh
```

**1 — the daemon**, as you, so you are its owner. A port other than 6767
keeps it clear of an installed node; `-create` is for the first run only:

```sh
./agent-busd -owner "$USER" -addr 127.0.0.1:6791 -create
```

Your own shell needs no token: the CLI finds your own socket by itself, and it
answers as you. An agent's name begins with `#`, so quote it.

**2 — an agent.** An agent *is* a name, and the shell that answers takes its
credential — the name is the inbox, so two shells sharing one would read each
other's messages
([messaging § one reader per inbox](../docs/04-messaging.md#one-reader-per-inbox)):

```sh
./agent-bus register "#echo@$(hostname -s)" --descr "answers"
export AGENT_BUS_TOKEN=$(./agent-bus-token "#echo@$(hostname -s)")
./agent-bus consume --wait 5m          # prints the envelope, with its message_id
./agent-bus ack   <message-id>         # got it
./agent-bus reply <message-id> "42"    # the answer
```

…or let a shell script be the agent, which is the same thing without the
typing ([runner § script agents](../docs/08-runner-role.md#script-agents)).
`start` registers the name and fetches its credential itself; the script path
must be absolute:

```sh
echo 'echo "Hello $1"' > hello-world.sh && chmod +x hello-world.sh
./agent-bus start "#hello@$(hostname -s)" --algo args "$PWD/hello-world.sh" -4 --descr "greets you"
```

**3 — the caller** is you, on your own socket, calling whichever of the two
you started:

```sh
./agent-bus ls -h                                             # find it
./agent-bus call "#echo@$(hostname -s)"  "what is 6 times 7?"  # answered by hand, above
./agent-bus call "#hello@$(hostname -s)" world                 # → Hello world
```

A queue needs nobody reading when it is sent — the consumer may start later:

```sh
./agent-bus channel create jobs@$(hostname -s) --descr "work queue"
./agent-bus publish --channel jobs@$(hostname -s) "sweep the floor"
./agent-bus consume --inbox jobs@$(hostname -s)    # later, from anywhere
```

## Two agents

Claude Code, Codex and opencode sessions talk over the same daemon through
the MCP face; the `ab-*` launchers load it ([agents](../docs/user/agents.md)),
and loading it by hand is in [mcp/README.md](mcp/README.md#loading-it).
