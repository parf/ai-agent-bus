# The MCP face

📌 **TL;DR:** One package, on bun: fifteen tools over stdio — `ab_ls`, `ab_send`,
`ab_consume`, `ab_reply`, `ab_receipt`, `ab_rename`, and `ab_lock`,
`ab_lock_release`, `ab_lock_extend`, `ab_lock_holders` for [shared locks](../../docs/01-identity-and-authority.md#shared-locks), and `ab_kv_get`, `ab_kv_set`, `ab_kv_delete`, `ab_kv_inc`, `ab_kv_json` for each record's [key-value store](../../Plans/R1.0-Release/kv.md#per-record-storage)
— the [MCP Resources](../../docs/03-records.md#resource-records) the caller may
read, and push into a live Claude Code, Codex or opencode session. The `ab-*` launchers load it for you;
this page is for loading it by hand. Design:
[modules § languages](../MODULES.md#languages).

| File | |
|---|---|
| `bus.ts` | the daemon as seen from TypeScript — HTTP+JSON over its unix socket or TCP (`http://`, or `https://` pinned) — and the default session name |
| `pin.ts` | reaching an `https://` daemon by its fingerprint: the pinned certificate is the only one trusted |
| `server.ts` | the tools, the Resources handlers and the push wiring |
| `tools.yaml` | tool names and descriptions sent to every MCP client |
| `instructions.yaml` | general MCP instructions, including receipt and completion policy |
| `resources.ts` | 📚 cards as MCP Resources: listed one to one, and a read asked of the card's source — an Agent over the bus, an MCP Service, or an `https://` fetch |
| `push.ts` | the reader loop every push mode shares |
| `messages.ts` | how a delivered message reads in the session, with the route to answer it |
| `catalogue.ts` | what `ab_ls` lists (by default only agents with a reader) and shows for each record |
| `codex.ts` | the Codex App Server client, over a loopback WebSocket or stdio |
| `opencode.ts` | the opencode server client, over its loopback HTTP API; used by `ab-opencode` |
| `rpc.ts` | newline-delimited JSON-RPC line reader and request matcher |
| `face-mark.ts` | the mark that tells a launcher its face is still alive |
| `version.ts` | the shared release number, read from `../internal/version/VERSION` |
| `.claude-plugin/`, `commands/` | the Claude Code plugin manifest and its `/ab:ls`, `/ab:send` |
| `smoke*.ts`, `launcher-runtime-fixture.ts`, `*.test.ts` | acceptance and unit checks; run by [`../smoke.sh`](../smoke.sh) |

## Tool descriptions

[`tools.yaml`](tools.yaml) owns the tool descriptions as `name: description`
pairs. Edit that file to change what Claude Code, Codex and OpenCode see.
`server.ts` imports it; Bun embeds it in packaged builds. Restart the MCP
session to load edits, or rebuild first when running a packaged release.

[`instructions.yaml`](instructions.yaml) owns the general instructions sent
at initialization; `{{name}}` is replaced with the session's bus name.
Every non-receipt message must be acknowledged before work starts. Send the
requested answer when finished, or a `done` receipt when no answer was requested.
Never acknowledge a receipt. For pushed messages, `ab_receipt` accepts the
explicit return route shown in the delivery, without local consume context.
These instructions guide the agent; the face cannot determine when its work
has finished.

Parameter schemas, parameter descriptions and handlers remain in `server.ts`.
The smoke check verifies the initialization instructions and `tools/list`
descriptions against YAML, including a description for every advertised tool.

## Environment

| | |
|---|---|
| `AGENT_BUS_TOKEN` | **required.** Same token the CLI uses ([access § getting a token](../../docs/02-access.md#getting-a-token)) |
| `AGENT_BUS_NAME` | this session's agent name, beginning with `#`. Defaults to `#<runtime>.<cwd>@<host>`, trimmed to the name rule ([identity § names](../../docs/01-identity-and-authority.md#names)) — a plugin manifest cannot know it, so it is derived |
| `AGENT_BUS_RUNTIME` | the `<runtime>` in that default, and in the default description; `agent` when unset |
| `AGENT_BUS_REALM` | the `<host>` in that default; the host name when unset |
| `AGENT_BUS_ADDR` | the daemon's socket path, `http://host:port` or `https://host:port`. Defaults to `$XDG_RUNTIME_DIR/agent-bus/bus.sock` |
| `AGENT_BUS_TLS_FINGERPRINT` | the pin an `https://` address needs ([remote access § over HTTPS](../../docs/02-access-remote.md#over-https)) |
| `AGENT_BUS_PUSH` | `claude`, `codex` or `off` (default) |
| `AGENT_BUS_DESCR` | what `ls` shows for this session |
| `AGENT_BUS_CWD` | the `<cwd>` in the default name, and the directory the Codex push mode attaches to. Defaults to the process's |
| `AGENT_BUS_CODEX_WS` | the **shared** App Server, e.g. `ws://127.0.0.1:8421`. Without it the face drives its own, which cannot reach a live session — see below |
| `AGENT_BUS_CODEX_AUTH_TOKEN` | bearer capability for the shared Codex App Server; the smart launcher provides it privately to its TUI and pusher, never in command-line arguments |
| `AGENT_BUS_CODEX_APPROVAL` | the Codex approval policy for bus-started turns — see [approvals](#one-app-server-two-clients) |
| `AGENT_BUS_SESSION_FILE`, `AGENT_BUS_CONTROL_ADDR`, `AGENT_BUS_CONTROL_TOKEN` | set by a launcher for its own face; not for hand use |

The [launchers](../../docs/08-runner-role.md#smart-launchers) supply their
[assigned session identity](../../docs/08-runner-role.md#session-names);
the defaults above apply when starting the face directly.

The face registers its name at start. The registration is durable, so after a
daemon restart the face reconnects by itself under the same name: each tool
call is its own request, and a push reader retries every two seconds until the
daemon answers again.

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

**Claude Code, as a plugin**, from a source checkout. `.claude-plugin/plugin.json`
declares the server, so `claude --plugin-dir <this dir>` gives the session the
six tools and `/ab:ls`, `/ab:send`. No push — `ab_consume` is how messages
arrive.

**Codex**, in `~/.codex/config.toml`, with the installed face (or
`<checkout>/src/mcp/server.ts`):

```toml
[mcp_servers.agent-bus]
command = "bun"
args = ["run", "/usr/local/lib/agent-bus/current/mcp/server.js"]
env = { AGENT_BUS_TOKEN = "<token>", AGENT_BUS_PUSH = "off", AGENT_BUS_RUNTIME = "codex" }
```

This gives the session its tools; reaching it while it runs needs a pusher
beside the App Server, below.

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
| tools | the App Server, from `config.toml`, `AGENT_BUS_PUSH=off` | gives the session the six tools |
| pusher | a launcher, beside the App Server, `AGENT_BUS_PUSH=codex` | holds the inbox's read and steers the session's turn |

They must **not** be the same process: the App Server starts its own MCP
servers, so a face started that way cannot dial back into the App Server that
is still starting it. Verified — the push loop never came up. Legacy-V1 keeps the
pusher a sidecar for the same reason.

Give both the same `AGENT_BUS_NAME`: one session, one name. Only the pusher
consumes, so they do not contend for the inbox's one read.

Use `ab-codex`: the launcher starts this shared topology and supplies its
[private runtime credentials](../../docs/08-runner-role.md#runtime-isolation-and-recovery)
to both clients. A hand-started server must enforce authentication too;
loopback alone lets other local accounts attach. For a separately managed
server, supply the matching `AGENT_BUS_CODEX_AUTH_TOKEN` to this adapter and use
the runtime's own authenticated TUI connection options.

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
`permessage-deflate` workaround, while native bearer authentication provides the account boundary.

Without `AGENT_BUS_CODEX_WS` the face still works and says so in its log: it
drives its own App Server and answers in a headless thread, which does not
reach a session somebody is typing in.

The thread is chosen at the **first message**, not at startup: the face is
Codex's own MCP server, so it starts before the session has a thread, and
choosing one then would pick its own headless thread instead of the one the
person is typing in. After that it is fixed — following someone who opens a
*new* session in the same directory would need Legacy-V1's re-selection machinery;
here, restart the face.

**OpenCode** declares the face as a `local` MCP server under `mcp` in its
configuration, like any other. Push needs `ab-opencode`: it starts
`opencode serve` on loopback with a password, attaches the TUI to it and
delivers each message to the bound session over the server's HTTP API. As with
Codex, the pushed text spells out the `ab_send` that answers it.

## Push and the one reader

While push is on, the loop holds the inbox's one unfiltered read
([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)),
so an unfiltered `ab_consume` is refused and says why. A **filtered** one —
topic and tag — is still served, ahead of the loop, which is how a session
collects the reply to something it sent.

Delivery is at-most-once in both directions: the daemon hands a message over
once, so a push that fails to deliver has lost it. The face says so in its
log rather than pretending otherwise.
