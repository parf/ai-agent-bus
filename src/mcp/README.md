# The MCP face

📌 **TL;DR:** Give AI sessions bus tools, readable resources and incoming messages.
The `ab-*` launchers configure the face automatically. For manual tool loading,
start with [Loading it](#loading-it); source contributors can edit the [tool descriptions](#tool-descriptions).

## Loading it

**Claude Code.** Use `ab-claude`; follow [plugin installation and Team/Enterprise policy](../../docs/06-launchers.md#claude-channel-plugin).
The plugin supplies MCP tools and `/agent-bus:ls` and `/agent-bus:send`.
The source plugin's cache-safe `channel.ts` imports `AGENT_BUS_MCP_ENTRY`, supplied by the
launcher; the private session environment carries identity and connection
settings. No credentials are embedded in the plugin.

**Codex**, in `~/.codex/config.toml`, with the installed client commands on PATH.
Register an Agent with your authorized CLI account and obtain its token.
Replace the name and token below with that identity; set `AGENT_BUS_ADDR` if
the daemon is not on the default local socket:

```toml
[mcp_servers.agent-bus]
command = "agent-bus-ts"
args = ["mcp"]
env = { AGENT_BUS_NAME = "<registered-agent-name>", AGENT_BUS_TOKEN = "<token>", AGENT_BUS_PUSH = "off", AGENT_BUS_RUNTIME = "codex" }
```

This gives the session its tools; reaching it while it runs needs a pusher
beside the App Server, below.

MCP has no server-to-client push that Codex surfaces, so reaching a live
Codex session means steering its turn through the App Server — and **which
App Server** is the whole question.

## Tools

`ab_ls`, `ab_send`,
`ab_consume`, `ab_reply`, `ab_receipt`, `ab_rename`, and `ab_lock`,
`ab_lock_release`, `ab_lock_extend`, `ab_lock_holders` for [shared locks](../../docs/01-identity-and-authority.md#shared-locks), and `ab_kv_get`, `ab_kv_set`, `ab_kv_delete`, `ab_kv_inc`, `ab_kv_json` for each record's [key-value store](../../docs/03-records-kv.md#key-value-store), and `ab_bigfile_put`, `ab_bigfile_get`, `ab_bigfile_ls`, `ab_bigfile_rm` for [bigfiles](../../docs/03-records-kv.md#bigfiles)

Readable [resources](../../docs/03-records-resource.md#what-a-resource-is) follow the caller's access.

`ab_send` accepts `ask: "help"`, `"help-config"`, `"help-secret"`, `"help-json"`, `"mcp"` or `"status"`
with no text; help-secret describes environment names, never values
([agent self-description](../../docs/constitution.md#agent-self-description)).

## Tool descriptions

[`tools.yaml`](tools.yaml) owns the tool descriptions as `name: description`
pairs. Edit that file to change what Claude Code, Codex and OpenCode see.
`server.ts` imports it; Bun embeds it in packaged builds. Restart the MCP
session to load edits, or rebuild first when running a packaged release.

[`instructions.yaml`](instructions.yaml) owns the general instructions sent
at initialization; `{{name}}` is replaced with the session's bus name.
Follow the [receipt rules](../../docs/04-messaging.md#receipts). For pushed messages, `ab_receipt` accepts the
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
| `AGENT_BUS_NAME` | this session's agent name, beginning with `#`. Defaults to `#<runtime>/<cwd>@<host>`, trimmed to the name rule ([identity § names](../../docs/01-identity-and-authority.md#names)) — a plugin manifest cannot know it, so it is derived |
| `AGENT_BUS_RUNTIME` | the `<runtime>` in that default, and in the default description; `agent` when unset |
| `AGENT_BUS_REALM` | the `<host>` in that default; the host name when unset |
| `AGENT_BUS_ADDR` | the daemon's socket path, `http://host:port` or `https://host:port`. Uses [local socket discovery](../../docs/02-access.md#local-socket), ending at the installed socket when no runtime socket is present |
| `AGENT_BUS_TLS_FINGERPRINT` | the pin an `https://` address needs ([remote access § over HTTPS](../../docs/02-access-remote.md#over-https)) |
| `AGENT_BUS_PUSH` | `claude`, `codex` or `off` (default) |
| `AGENT_BUS_DESCR` | what `ls` shows for this session |
| `AGENT_BUS_BIGFILE_DIR` | the one absolute directory `ab_bigfile_put` and `ab_bigfile_get` may read and write ([bigfiles § from a session](../../docs/03-records-kv.md#bigfiles)); unset, both refuse |
| `AGENT_BUS_CWD` | the `<cwd>` in the default name, and the directory the Codex push mode attaches to. Defaults to the process's |
| `AGENT_BUS_CODEX_WS` | the **shared** App Server, e.g. `ws://127.0.0.1:8421`. Without it the face drives its own, which cannot reach a live session — see below |
| `AGENT_BUS_CODEX_AUTH_TOKEN` | bearer capability for the shared Codex App Server; the smart launcher provides it privately to its TUI and pusher, never in command-line arguments |
| `AGENT_BUS_CODEX_APPROVAL` | the Codex approval policy for bus-started turns — see [approvals](#one-app-server-two-clients) |
| `AGENT_BUS_SESSION_FILE`, `AGENT_BUS_CONTROL_ADDR`, `AGENT_BUS_CONTROL_TOKEN`, `AGENT_BUS_MCP_ENTRY` | set by a launcher for its own face; not for hand use |

For installed file storage, point `AGENT_BUS_BIGFILE_DIR` at your own `/var/lib/agent-bus/users/<person>/files`.

The [launchers](../../docs/06-launchers.md#smart-launchers) supply their
[assigned session identity](../../docs/06-launchers.md#session-names);
the defaults above apply when starting the face directly.

The face registers its name at start. The registration is durable, so after a
daemon restart the face reconnects by itself under the same name: each tool
call is its own request, and a push reader retries every two seconds until the
daemon answers again.

## One App Server, two clients

A separately started `codex app-server` can resume saved history but cannot
steer the session somebody is typing in. The TUI and pusher must attach to the
same authenticated App Server. Codex uses two bus clients under one name:

| Process | Started by | Role |
|---|---|---|
| tools | the App Server, from `config.toml`, `AGENT_BUS_PUSH=off` | gives the session the tools |
| pusher | a launcher, beside the App Server, `AGENT_BUS_PUSH=codex` | holds the inbox's read and steers the session's turn |

They must **not** be the same process: the App Server starts its own MCP
servers, so a face started that way cannot dial back into the App Server that
is still starting it.

Give both the same `AGENT_BUS_NAME`: one session, one name. Only the pusher
consumes, so they do not contend for the inbox's one read.

Use `ab-codex`: the launcher starts this shared topology and supplies its
[private runtime credentials](../../docs/06-launchers.md#runtime-isolation-and-recovery)
to both clients. A hand-started server must enforce authentication too;
loopback alone lets other local accounts attach. For a separately managed
server, supply the matching `AGENT_BUS_CODEX_AUTH_TOKEN` to this adapter and use
the runtime's own authenticated TUI connection options.

**A Codex session answers with `ab_send`, not `ab_reply`**: the message was
consumed by the pusher, and the tools live in the other process, so its reply
context is empty. The pushed text therefore carries the routing — sender,
topic and tag — which is all a reply is
([messaging § reply routing](../../docs/04-messaging.md#reply-routing)).

**Approvals.** `ab-codex` enforces `approval_policy="never"` and
`sandbox_mode="danger-full-access"`: commands execute automatically with the
session account's access. `never` means no approval prompts, not denial.
For a manually managed session, `AGENT_BUS_CODEX_APPROVAL` selects the policy
for bus-started turns (default `on-request`); the adapter requests the runtime's
user reviewer. If an MCP approval request reaches the pusher, its fallback
accepts identified agent-bus non-file tools and declines file tools or unknown
requests. That filter is not an execution boundary for the session.

The adapter connects over a loopback WebSocket.

Without `AGENT_BUS_CODEX_WS` the face still works and says so in its log: it
drives its own App Server and answers in a headless thread, which does not
reach a session somebody is typing in.

The thread is chosen at the **first message**, not at startup: the face is
Codex's own MCP server, so it starts before the session has a thread, and
choosing one then would pick its own headless thread instead of the one the
person is typing in. After that it is fixed — following someone who opens a
*new* session in the same directory requires restarting the face.

**OpenCode** declares the face as a `local` MCP server under `mcp` in its
configuration, like any other. Push needs `ab-opencode`: it starts
`opencode serve` on loopback with a password, starts the TUI on that server
with `--server` and delivers each message to the bound session over the
server's HTTP API. As with Codex, the pushed text spells out the `ab_send`
that answers it.

## Push and the one reader

While push is on, the loop holds the inbox's one unfiltered read
([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)),
so an unfiltered `ab_consume` is refused and says why. A **filtered** one —
topic and tag — is still served, ahead of the loop, which is how a session
collects the reply to something it sent.

Session self-description asks (`help`, `help-json`, `help-config`,
`help-secret`, `mcp`) are answered by the face or launcher reader, never pushed
into the model's conversation. They describe an AI CLI session for plain-text
conversation, with its purpose, tags, runtime and working directory; there are
no declared methods, Configuration keys, Secrets or MCP tools. Ordinary messages
and receipts retain their existing delivery path. Already-running faces keep
their loaded code until the owner starts a new session.

Delivery is at-most-once in both directions: the daemon hands a message over
once, so a push that fails to deliver has lost it. The face says so in its
log rather than pretending otherwise.

### Runtime labels

Runtime sessions add a `claude`, `codex` or `opencode` [record tag](../../docs/03-records.md#tags).
These labels preserve existing tags and grant no authority.

## Record configuration

For record kinds supporting Configuration or Secret, use the [configuration guide](../../docs/03-records-agent.md#configuring-a-template).
MCP listings expose the authorized `config_sha`, not settings bytes. Read or
edit settings with `agent-bus config` or the web editor; there is no
separate MCP configuration tool.

## Implementation

See [module boundaries](../MODULES.md#languages) for the language split.

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
| `opencode.ts` | the opencode 2 server client, over its loopback HTTP API; used by `ab-opencode` |
| `opencode1.ts` | the archived 1.x client, identical in role; used by `ab-opencode1` |
| `rpc.ts` | newline-delimited JSON-RPC line reader and request matcher |
| `face-mark.ts` | the mark that tells a launcher its face is still alive |
| `version.ts` | the shared release number, read from `../internal/version/VERSION` |
| `.claude-plugin/`, `commands/` | the Claude Code plugin manifest and its `/agent-bus:ls`, `/agent-bus:send` |
| `smoke*.ts`, `launcher-runtime-fixture.ts`, `*.test.ts` | acceptance and unit checks; run by [`../smoke.sh`](../smoke.sh) |
