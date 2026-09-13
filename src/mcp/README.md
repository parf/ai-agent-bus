# The MCP face

One package, on bun. Five tools over stdio, and two ways to push into a live
session. Design: [modules § languages](../../docs/10-modules.md#languages);
scope: [stages § MVP](../../docs/12-stages.md#mvp).

| File | |
|---|---|
| `bus.ts` | the daemon as seen from TypeScript — HTTP+JSON over its unix socket or loopback TCP |
| `server.ts` | the five tools, and the push wiring |
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
restart therefore leaves it connected but unreachable: the daemon reloads its
snapshot, but the face's registration is not refreshed. **Restart the face
with the daemon.**

## Loading it

**Claude Code, for push.** Channels are still a development feature, and the
server has to be named on the command line:

```sh
claude --mcp-config claude-mcp.json \
  --dangerously-load-development-channels server:agent-bus
```

where `claude-mcp.json` declares `agent-bus` with `AGENT_BUS_PUSH=claude`.
Two traps, both silent:

- **`--strict-mcp-config` breaks it.** The flag then has no server to bind to
  and messages are consumed but never surface. The warning it prints —
  *no MCP server configured with that name* — is the only sign.
- **A plugin-provided server cannot be a channel source.** Neither
  `server:agent-bus` nor `server:plugin:ab:agent-bus` resolves. The plugin is
  how the *tools* load; the channel needs this form.

**Claude Code, as a plugin.** `.claude-plugin/plugin.json` declares the
server, so `claude --plugin-dir <this dir>` gives the session the five tools
and `/ab:ls`, `/ab:send`. No push — `ab_consume` is how messages arrive.

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
somebody is typing in. Legacy-V1 — the NATS system at `/rd/service/agent-bus/`, and
every mention of it on this page — solved this by topology (`/rd/bin/ai-codex`): one
App Server, the TUI attached to it, the notifier attached to the same one.

Same shape here — and for Codex it is **two processes under one name**:

| Process | Started by | Role |
|---|---|---|
| tools | the App Server, from `config.toml`, `AGENT_BUS_PUSH=off` | gives the session `ab_send`, `ab_ls`, `ab_consume` |
| pusher | a launcher, beside the App Server, `AGENT_BUS_PUSH=codex` | holds the inbox's read and steers the session's turn |

They must **not** be the same process: the App Server starts its own MCP
servers, so a face started that way cannot dial back into the App Server that
is still starting it. Verified — the push loop never came up. Legacy-V1 keeps the
pusher a sidecar for the same reason.

Give both the same `AGENT_BUS_NAME`: one session, one name. Only the pusher
consumes, so they do not contend for the inbox's one read.

```sh
codex app-server --listen ws://127.0.0.1:8421 &          # one server, tools inside it
AGENT_BUS_NAME=me@host AGENT_BUS_PUSH=codex \
  AGENT_BUS_CODEX_WS=ws://127.0.0.1:8421 \
  AGENT_BUS_CWD="$PWD" bun run server.ts &               # the pusher attaches
codex --remote ws://127.0.0.1:8421 -C "$PWD"             # so does the TUI
```

**A Codex session answers with `ab_send`, not `ab_reply`**: the message was
consumed by the pusher, and the tools live in the other process, so its reply
context is empty. The pushed text therefore carries the routing — sender,
topic and tag — which is all a reply is
([messaging § reply routing](../../docs/04-messaging.md#reply-routing)).

**Approvals.** A turn started by a bus message has no human at the keyboard,
so the App Server asks the client that started it — the pusher. It approves
**only agent-bus's own tool calls** and declines everything else, so a remote
peer cannot spend the user's permissions. Set `AGENT_BUS_CODEX_APPROVAL` to
match the session (`on-request` by default; `never` means *deny*, not *allow*,
and blocks the reply).

A **loopback WebSocket**, not `unix://`: both are WebSocket listeners, and bun
can open one over a port but not over a unix socket — which is exactly what
put Legacy-V1's Codex notifier on Node. A port removes the `ws` dependency and its
`permessage-deflate` workaround, and the App Server binds localhost only,
which is the daemon's own rule (loopback only).

Without `AGENT_BUS_CODEX_WS` the face still works and says so in its log: it
drives its own App Server and answers in a headless thread. That is a
different demo, not the one wave B is for.

The thread is chosen at the **first message**, not at startup: the face is
Codex's own MCP server, so it starts before the session has a thread, and
choosing one then would pick its own headless thread instead of the one the
person is typing in. After that it is fixed — following someone who opens a
*new* session in the same directory would need Legacy-V1's re-selection machinery;
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
