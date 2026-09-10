# src — the PoC

The code for [stages § PoC](../docs/12-stages.md#poc), built wave by wave
against [Plans/PoC/TODO.md](../Plans/PoC/TODO.md).

| | |
|---|---|
| `cmd/agent-busd` | the daemon: unix socket and loopback TCP, one master token |
| `cmd/agent-bus` | the CLI — the eleven verbs; `agent-bus help` lists them |
| `internal/protocol` | names and envelopes, no behaviour |
| `internal/core` | the registry and the queues — the only place a routing decision is made |
| `internal/api` | the HTTP face: a request in, a core call out |
| `mcp/` | the MCP face and both push adapters, on bun — [mcp/README.md](mcp/README.md) |
| `static-token` | the SSH forced command that hands out the token ([access § getting a token](../docs/02-access.md#getting-a-token)) |
| `smoke.sh` | every acceptance criterion of every wave, in one script |

Layering is the design's: protocol → core → api, faces outside, nothing
pointing back in ([modules](../docs/10-modules.md)).

## Build and check

```sh
go test -race ./...
./smoke.sh            # builds both binaries, runs a throwaway daemon, exits 0 or not
```

`smoke.sh` needs `bun` for the MCP half. It uses a temporary socket and token
of its own, so it never touches a daemon you are running; `PORT=` moves its
loopback port if the default one is taken.

A check here is not believed until it has been seen to fail —
[plan § mutation first, then belief](../Plans/PoC/README.md#mutation-first-then-belief).

## Three terminals

The PoC by hand. Every shell needs a name and the token — the file the daemon
makes on first run; the CLI finds the socket by itself, and
`AGENT_BUS_ADDR` is for a bus on another host.

```sh
export AGENT_BUS_NAME=$USER@$(hostname -s)
export AGENT_BUS_TOKEN=$(cat ~/.config/agent-bus/token)
```

**1 — the daemon.**

```sh
go run ./cmd/agent-busd
```

**2 — a service.** Either answer by hand:

```sh
agent-bus register echo@$(hostname -s) --kind generic --descr "answers"
agent-bus consume --wait 5m          # prints the envelope, with its message_id
agent-bus ack   <message-id>         # got it
agent-bus reply <message-id> "42"    # the answer
```

…or let a shell script be the service, which is the same thing without the
typing ([runner § script services](../docs/08-runner-role.md#script-services)):

```sh
echo 'echo "Hello $1"' > hello-world.sh && chmod +x hello-world.sh
agent-bus start hello@$(hostname -s) --algo args ./hello-world.sh -4 --descr "greets you"
```

**3 — the caller**, with a name of its own:

```sh
export AGENT_BUS_NAME=caller@$(hostname -s)
agent-bus ls                                     # find it
agent-bus call hello@$(hostname -s) world        # → Hello world
```

Topics need no service at either end — a publisher that is nobody, and a
consumer that was not running when it was sent:

```sh
agent-bus topic create jobs@$(hostname -s) --descr "work queue"
agent-bus publish --topic jobs@$(hostname -s) "sweep the floor"
agent-bus consume --topic jobs@$(hostname -s)    # later, from anywhere
```

## Two agents

A Claude Code session and a Codex session talk over the same daemon through
the MCP face; loading it into each, and the live recipe, are in
[mcp/README.md](mcp/README.md).
