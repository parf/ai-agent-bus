# The MCP face

One package, on bun. Four tools over stdio, and two ways to push into a live
session. Design: [modules § languages](../../docs/10-modules.md#languages);
scope: [stages § PoC](../../docs/12-stages.md#poc).

| File | |
|---|---|
| `bus.ts` | the daemon as seen from TypeScript — HTTP+JSON over its unix socket or loopback TCP |
| `server.ts` | the four tools, and the push wiring |
| `push.ts` | the reader loop both push modes share |
| `codex.ts` | the Codex App Server client, over a loopback WebSocket or stdio |
| `smoke.ts`, `smoke-push.ts`, `smoke-codex.ts` | acceptance; run by [`../smoke.sh`](../smoke.sh) |

## Environment

| | |
|---|---|
| `AGENT_BUS_TOKEN` | **required.** Same token the CLI uses ([access § getting a token](../../docs/02-access.md#getting-a-token)) |
| `AGENT_BUS_NAME` | this session's `user@realm`. Defaults to `<runtime>.<cwd>@<host>`, trimmed to the name rule ([identity § names](../../docs/01-identity.md#names)) — a plugin manifest cannot know it, so it is derived |
| `AGENT_BUS_ADDR` | the daemon's socket path, or `http://host:port`. Defaults to `$XDG_RUNTIME_DIR/agent-bus/bus.sock` |
| `AGENT_BUS_PUSH` | `claude`, `codex` or `off` (default) |
| `AGENT_BUS_DESCR` | what `ls` shows for this session |
| `AGENT_BUS_CWD` | which directory the Codex push mode attaches to. Defaults to the process's |
| `AGENT_BUS_CODEX_WS` | the **shared** App Server, e.g. `ws://127.0.0.1:8421`. Without it the face drives its own, which cannot reach a live session — see below |

The face registers its name at start and does not re-register. A daemon
restart therefore leaves it connected but unreachable — storage is memory
only, so a restart loses everything anyway. **Restart the face with the
daemon.**

## Loading it

**Claude Code, as a plugin.** `.claude-plugin/plugin.json` declares the server
with `AGENT_BUS_PUSH=claude`, so a message arrives in the session instead of
waiting to be polled. Channels are still a development feature, so the session
also needs the flag:

```
claude --dangerously-load-development-channels server:agent-bus
```

**Claude Code, without the plugin.** `--mcp-config .mcp.json` from this
directory. Same four tools, no push: `ab_consume` is how messages arrive.

**Codex**, in `~/.codex/config.toml`:

```toml
[mcp_servers.agent-bus]
command = "bun"
args = ["run", "/home/parf/src/ai-agent-bus/src/mcp/server.ts"]
env = { AGENT_BUS_PUSH = "codex", AGENT_BUS_RUNTIME = "codex" }
```

MCP has no server-to-client push that Codex surfaces, so reaching a live
Codex session means steering its turn through the App Server — and **which
App Server** is the whole question.

## One App Server, two clients

A `codex app-server` the face starts for itself is a *second process*. It can
resume a thread's saved history, but steering it does not reach the session
somebody is typing in. V1 solved this by topology (`/rd/bin/ai-codex`): one
App Server, the TUI attached to it, the notifier attached to the same one.

Same shape here, launched by hand:

```sh
codex app-server --listen ws://127.0.0.1:8421 &     # one server
AGENT_BUS_CODEX_WS=ws://127.0.0.1:8421 \
AGENT_BUS_PUSH=codex bun run server.ts &            # the face attaches
codex --remote ws://127.0.0.1:8421 -C "$PWD" resume --last   # so does the TUI
```

A **loopback WebSocket**, not `unix://`: both are WebSocket listeners, and bun
can open one over a port but not over a unix socket — which is exactly what
put V1's Codex notifier on Node. A port removes the `ws` dependency and its
`permessage-deflate` workaround, and the App Server binds localhost only,
which is the PoC's exposure rule anyway
([stages § PoC](../../docs/12-stages.md#poc)).

Without `AGENT_BUS_CODEX_WS` the face still works and says so in its log: it
drives its own App Server and answers in a headless thread. That is a
different demo, not the one wave B is for.

The thread is chosen at the **first message**, not at startup: the face is
Codex's own MCP server, so it starts before the session has a thread, and
choosing one then would pick its own headless thread instead of the one the
person is typing in. After that it is fixed — following someone who opens a
*new* session in the same directory would need V1's re-selection machinery;
here, restart the face.

## Push and the one reader

While push is on, the loop holds the inbox's one unfiltered read
([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)),
so an unfiltered `ab_consume` is refused and says why. A **filtered** one —
topic and tag — is still served, ahead of the loop, which is how a session
collects the reply to something it sent.

Delivery is at-most-once in both directions: the daemon hands a message over
once, so a push that fails to deliver has lost it. The face says so in its
log rather than pretending otherwise.
