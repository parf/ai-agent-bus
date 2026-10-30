# src

📌 **TL;DR:** The code of the current 0.9 development line, following the released [MVP](../Plans/done/R0.8-MVP/README.md#scope):
Go programs and internal packages for the daemon, CLI, installer and token
program, and TypeScript on Bun for the MCP face, launchers and web face.
`smoke.sh --normal` is the landing gate for changed subsystems.

| | |
|---|---|
| `cmd/agent-busd` | the daemon: unix socket and TCP on any address, plain HTTP and, with `--tls-dir`, TLS on the same port; one token per principal |
| `cmd/agent-bus-setup` | the root-only installer: verified release, accounts, tree, daemon and web units |
| `cmd/agent-bus-admin` | what edits the `agent-busd` account's files |
| `cmd/agent-bus` | the CLI — `agent-bus help` lists every verb |
| `internal/protocol` | names and envelopes, no behaviour |
| `internal/core` | the registry and the queues — the only place a routing decision is made |
| `internal/api` | the HTTP face: a request in, a core call out |
| `internal/auth` | credentials: issue, rotate, resolve a token to its principal |
| `internal/ports` | the interfaces core depends on — the seam every dependency is swapped at |
| `internal/store` (SQLite, memory), `internal/directory` (GitHub, key files), `internal/signature`, `internal/sandbox`, `internal/journal` | the adapters behind those ports |
| `internal/duallisten`, `internal/tlsdir` | TLS beside plain HTTP on one port; the TLS directory and its fingerprint |
| `internal/activity`, `internal/callstats`, `internal/display`, `internal/keyproof`, `internal/proctitle`, `internal/dashboard`, `internal/version` | day-of-traffic rings, request sampling, human labels, the client half of key proof, process titles, the dashboard address, the shared release number |
| [Client libraries](clients/README.md) | Language clients and isolated daemon conformance checks |
| `mcp/` | the MCP face and its Claude, Codex and opencode push adapters, on bun — [mcp/README.md](mcp/README.md#the-mcp-face) |
| `web/` | the web face, TypeScript on bun under its own account and unit — [processes § the web face](../docs/08-processes.md#the-web-face) |
| `launchers/` | smart runtime launchers — [contract and usage](../docs/06-launchers.md#running-the-launchers) |
| `internal/tokencmd` | the CLI token verb, reached through the fixed wrapper behind an ordinary user’s SSH key; `--fingerprint` prints the node's TLS pin ([access § getting a token](../docs/02-access.md#getting-a-token)) |
| `smoke.sh` | the automated acceptance checks |

Layering is the design's: protocol → core → api, faces outside, nothing
pointing back in ([modules](MODULES.md#the-rule)).

## Build and check

Build requirements and version output follow
[setup § build information](../docs/07-setup.md#build-information).
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
./smoke.sh             # touched fast checks; NOT A LANDING GATE
./smoke.sh --normal    # changed subsystems normal, all others fast
./smoke.sh --normal A,F # explicit subsystem override
./smoke.sh --slow L    # huge agent change: agents slow, others fast
./smoke.sh --slow all  # release/STABLE: every subsystem slow
```

Historical reports use the old option names: their `--slow` means today's
normal gate, and their `--heavy` means today's `--slow all`. The rename changes
no test membership; the separate trim must bring the normal gate below 60 s.

`smoke.sh` needs `bun` for the MCP and web shards. It builds once and runs its shards in
parallel, each with a temporary socket, token and daemon of its own, so it never
touches a daemon you are running. A run owns fifteen loopback ports per shard
from its base. Without `PORT` it claims a free block of 1000 from 10000 and
holds its lock for the run (`smoke-port.sh`), so runs started at once never
meet; `PORT=` pins the base, and two pinned runs need bases a thousand apart.

A check here is not believed until it has been seen to fail —
[working rules § verification](../CLAUDE.md#verification).

## Three terminals

By hand. Build the binaries first — nothing installs them:

```sh
bash ./build.sh
```

**1 — the daemon**, as you, so you are its owner. A port other than 6767
keeps it clear of an installed node; `--create` is for the first run only:

```sh
./agent-busd --owner "$USER" --addr 127.0.0.1:6791 --create
```

Your own shell needs no token: the CLI finds your own socket by itself, and it
answers as you. An agent's name begins with `#`, so quote it.

**2 — an agent.** An agent *is* a name, and the shell that answers takes its
credential — the name is the inbox, so two shells sharing one would read each
other's messages
([messaging § one reader per inbox](../docs/04-messaging.md#one-reader-per-inbox)):

```sh
./agent-bus register "#echo@$(hostname -s)" --kind=agent --descr="answers"
export AGENT_BUS_TOKEN=$(./agent-bus token "#echo@$(hostname -s)")
./agent-bus consume --wait 5m          # prints the envelope, with its message_id
./agent-bus ack   <message-id>         # got it
./agent-bus reply <message-id> "42"    # the answer
```

…or let a shell script be the agent, which is the same thing without the
typing ([runner § script agents](../docs/06-dedicated-runner.md#script-agents)).
`run` registers the name and fetches its credential itself; the script path
must be absolute:

```sh
echo 'echo "Hello $1"' > hello-world.sh && chmod +x hello-world.sh
./agent-bus run "#hello@$(hostname -s)" "$PWD/hello-world.sh" -4 --descr "greets you"
```

**3 — the caller** is you, on your own socket, calling whichever of the two
you started:

```sh
./agent-bus ls --table --kind agent                                # find it
./agent-bus call "#echo@$(hostname -s)"  "what is 6 times 7?"  # answered by hand, above
./agent-bus call "#hello@$(hostname -s)" world                 # → Hello world
```

A queue needs nobody reading when it is sent — the consumer may start later:

```sh
./agent-bus register jobs@$(hostname -s) --kind=queue --descr "work queue"
./agent-bus publish --channel jobs@$(hostname -s) "sweep the floor"
./agent-bus consume --inbox jobs@$(hostname -s)    # later, from anywhere
```

## Two agents

Claude Code, Codex and opencode sessions talk over the same daemon through
the MCP face; the `ab-*` launchers load it ([agents](../docs/user/ai-cli.md)),
and loading it by hand is in [mcp/README.md](mcp/README.md#loading-it).

Selection is printed before the build. The static `smoke-map.json` tags checks
with the existing [subsystem letters](../Plans/subsystems.md); `smoke-select.py`
reads that map and the diff against main. VERSION and CHANGELOG files never
select tests. Shared dependencies and unmapped paths fall back to all sets.
Fast allowlists in the same map run essential unit operations without building
or starting a full fixture. They overlap normal; normal retains the original
security, authority, delivery, durability, bounds and mutant checks. The fast
runner refuses empty test matches and reports/fails a candidate over 15 seconds.
Slow-test moves are separate and require the original production mutant to fail
a faster normal check.

| Fast group | Essential operations |
|---|---|
| A–H, M | ACL, credential write refusal, KV reopen, names/env parsing, HTTP ACL, runner manifest, setup address, CLI asks, selector/mode/allowlist contracts |
| I | Web escaping, links, forms and display helpers |
| J | RPC framing, restart policy and local launcher parsing |
| K | Go client credential-safe options; every language's conformance remains normal |
| L | Telegram/Slack parsing, routing and metadata; standalone/debug metadata plus git validation/caps/roles, systemd names/scope, logwatch ring/filters, server-info commands and Slack forwarding |

The normal set is unchanged by these fast allowlists. A fast pass is feedback,
not evidence that every normal check or every client implementation ran.

The Telegram normal set checks Receiver's real 30-second poll and the actual
40-second AbortSignal passed to fetch without waiting. The original short-HTTP-
timeout production mutant fails that check. `--slow L` or `--slow all` also runs
the real elapsed 30-second idle response check; a direct Bun file run retains it.
Independent Bun agent files use six worker processes per platform, preserving
serial test order and globals inside each file. The measured longest files
(Telegram t6/t9 and Slack s5) start first, so short files do not delay them.
