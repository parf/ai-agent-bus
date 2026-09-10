# DONE — PoC

| Wave | Result |
|---|---|
| **A — two shells talk** | `agent-busd` on a unix socket and loopback HTTP, in-memory registry and inboxes; `agent-bus` with `status`, `register`, `ls`, `send`, `consume`, `reply`. `src/smoke.sh` is the acceptance (22 checks) and `go test -race ./...` the regressions. |
| **A review** | Codex reviewed `c8cde3a` over the V1 bus and found two real defects; both fixed, both now covered by tests that fail against the old code. |
| **B — the live slice** | `src/mcp/` on bun: the MCP face (`ab_ls`, `ab_send`, `ab_consume`, `ab_reply`), self-registration, and both push modes. `src/smoke.sh` is 26 checks and carries two harnesses of its own — 12 for the face, 7 for Claude push. |
| **B review** | Codex reviewed `170de8f` and reproduced four defects, two of which the smoke passed *for the wrong reason*. All fixed; every one now has a check that fails against the old code. |
| **B.3 review** | Codex reviewed `1779e62` against a mock App Server and found the Claude blocker, the Codex **topology** mistake, and four state and lifecycle defects. All fixed. The Codex adapter now has a fake-App-Server harness of its own. |

**A, what it proved**

- Both listeners speak the same HTTP+JSON; the CLI reaches either.
- The two parameters are enforced: a wrong token and a name without a realm are both refused.
- A message sent while nobody is reading waits, and arrives afterwards.
- `reply <message-id>` works with **no reply state in the daemon** — the CLI resolves the id from the envelope it consumed ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)).
- One reader per inbox: a second unfiltered `consume` is refused with 409, while a filtered waiter is served beside it ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)).

**What the review caught**

| | |
|---|---|
| **Filtered priority was never implemented** | `Send` walked waiters in registration order, so an unfiltered reader that blocked first took a filtered waiter's reply — the exact thing the rule exists to prevent. Now two passes: matching filtered waiters, then the reader. |
| **A cancelled wait could swallow a message** | a `Send` that reported success could hand the envelope to a waiter whose deadline had just fired, and it vanished. Now the cancel path settles under the same lock: take what was delivered, or remove the waiter so nothing can be. |
| **Names were not canonical** | `" x@y "` registered one inbox and `send` reached another. Canonicalisation moved into core, so every face gets the same answer. |
| **The docs promised more than the code** | the bus enforces *one outstanding unfiltered read*, not process ownership of an inbox. The doc now says that. |
| **The smoke script proved less than it claimed** | one assertion passed unconditionally, and nothing tested two competing waiters — which is why 13/13 missed the priority bug. |
| **`trap 'kill ${DPID:-0}'`** | with a build failure before `DPID` was set, that is `kill 0` — a SIGTERM to the whole process group. |

Smaller fixes: no reply body kept on disk (routing fields only, one file per
message, so parallel consumes cannot overwrite each other), one HTTP transport
per CLI process instead of one per poll, a stale socket is checked before
removal and a live one refused, `chmod 0600` and `Serve` errors are fatal
instead of ignored, header and idle timeouts, and a bounded graceful shutdown.

---

## B.0 — the runtime spike

**Question**: does the TypeScript side have to run on Node, as V1 does, because
bun cannot speak WebSocket over a unix socket?

**Answer: no. Nothing needs a WebSocket.** `codex app-server` speaks
**newline-delimited JSON-RPC on stdio**, and bun drives it with `Bun.spawn`
and a `flush()` — `initialize` answered, and `thread/list` returned the live
sessions with their `cwd`, including the interactive Codex session this repo
had exchanged messages with minutes earlier.

| Tried | Result |
|---|---|
| `codex app-server` on stdio, NDJSON | **works**, from bun |
| the same with `Content-Length` framing | refused: *"Failed to deserialize JSONRPCMessage"* |
| `codex app-server --listen unix://…` spoken to directly | silent — it is a **WebSocket** listener, which is exactly what bit V1 |
| `codex app-server proxy [--sock …]` | exits 0 with no output; its control socket is not plain JSON-RPC. Not needed, so not pursued |

**Consequence**: one runtime. The push adapter spawns `codex app-server` and
talks NDJSON, instead of attaching to a WebSocket listener a launcher had to
start. It also removes V1's `ws` dependency and its `permessage-deflate`
workaround.

**Not proven here**: that `turn/steer` reaches a thread a live TUI owns. That
is B.3's job; B.0 only had to decide the runtime.

The managed daemon started during the spike was stopped again; nothing was
left running.

**Not done in A, on purpose**: `call`, `ack`, topics, script services, the MCP
face, any push adapter, SSH token issuance.

## B — the live slice

**B.1/B.2 — the MCP face**

One TypeScript package on bun: `bus.ts` (the daemon as seen from TypeScript),
`server.ts` (four tools over stdio). Each tool is one `fetch` and nothing
else, over `fetch(url, { unix })` — the layer rule, in the smallest possible
form. It registers `AGENT_BUS_NAME` at start, which is what makes "find each
other by name" have a name.

Reply context lives in the face, because the daemon keeps none
([messaging § reply routing](../../docs/04-messaging.md#reply-routing)): the
last 200 messages, **routing fields only** — sender, topic, tag. Bodies are
unbounded and a reply needs none of one.

**B.3 — the two push modes**

One loop (`push.ts`), two deliveries. The loop long-polls `consume` and hands
the envelope to a mode; while it runs it holds the inbox's one unfiltered
read, so `ab_consume` says so instead of colliding with it.

| Mode | How |
|---|---|
| `claude` | `notifications/claude/channel` — content plus string metadata. The session needs `--dangerously-load-development-channels` |
| `codex` | spawn `codex app-server`, NDJSON on stdio: `initialize` → `thread/list` newest for this cwd → `thread/resume`, else `thread/start`; per message `turn/steer` if a turn is running, else `turn/start` |

**Proven**: Claude push end to end in the smoke — a peer sends, the
notification arrives with the body and routing, `ab_reply` answers it, and the
peer receives the answer with the original topic and tag. Codex push proven as
far as a fresh cwd goes: `thread/start` then `turn/start` both succeeded
against a real `codex app-server`.

⚠️ **Still not proven**: that `turn/steer` reaches a thread a live TUI owns —
B.0 could not answer it and neither could this, because testing it means
steering a session somebody is using. It is the by-hand criterion.

**B — V1 compared**

| V1 | Taken | Skipped, and why |
|---|---|---|
| `notifier-claude/server.ts` | the notification shape (`content` + string `meta`), and saying in the instructions that a transport ack is not an answer — this bit us live, when a reply stayed in the UI | the PHP handoff socket, the `agent-bus consume channel` subprocess, the raw signed-wire store with its TTL and byte budget. All of them serve the journal; we keep routing fields and nothing else |
| `notifier-codex/dispatcher.ts` | the call sequence, the steer-vs-start branch, tracking the active turn from `turn/started` / `turn/completed`, and falling back to `turn/start` when steer is rejected — V1 hit that and left the reason in a comment | the adapter journal, delivery events, `ADAPTER_PENDING` recovery, sandbox-policy plumbing, thread re-selection on `thread/goal/cleared`. Each is a durability contract PoC does not have |
| `notifier-codex/app-server.ts` | nothing structural — a WebSocket client we do not need | the `ws` dependency and the `permessage-deflate` workaround, both removed by B.0 |
| dedup by `event_hash` | nothing | V1 is at-least-once because the journal can redeliver; our `consume` is at-most-once, so there is nothing to deduplicate |

**B review — what Codex found**

| | Finding | Fix |
|---|---|---|
| HIGH | a cancelled `ab_consume` kept polling, took the next message and threw the answer away | the handler's abort signal reaches `fetch`; a smoke check that fails without it |
| MED | the low-level `Server` does not enforce the schema it advertises, so `ab_send` with no `text` sent an empty body | explicit argument checks, and a smoke check |
| MED | the reply-context map held whole envelopes — 200 bodies, unbounded | routing fields only |
| — | `ab_reply` on an evicted id claimed the session never consumed it | says which, and what to do instead |
| smoke | the harness read `stderr` before killing the server, so a hang became a permanent hang | drain from the start, kill with a bound |
| smoke | nothing checked what the peer received: with `ab_reply`'s topic and tag mutated to `WRONG`, every check still passed | the peer is in-process now and asserts body, sender, topic and tag. Verified by re-running the mutation: it fails |
| smoke | a JSON-RPC error collapsed to `text: ""`, `isError: false` | checked before the result shape |
| smoke | a host without bun passed green with the face unchecked | missing bun is a failure |

**Accepted as a PoC limit, not fixed**: the face registers once at start, so a
daemon restart leaves it connected but unreachable. Memory-only storage means
a restart loses everything anyway
([stages § PoC](../../docs/12-stages.md#poc)); restarting the face with the
daemon is the contract, and the MCP instructions say so.

**B.3 review — what Codex found**

It ran the adapter against a mock App Server rather than a real one, which is
how it reached states a live session will not produce on demand.

| | Finding | Fix |
|---|---|---|
| BLOCKER | the server declared only `capabilities.tools`. Claude Code registers a channel listener **only** for a server that declares `experimental: {"claude/channel": {}}`, so the connection was refused outright. The raw smoke accepted any JSON notification, so it could not see this | declared in `claude` mode |
| **topology** | the adapter spawned its **own** `codex app-server`. That is a second process: it can resume a thread's saved history, but steering it never reaches the session a person is typing in. V1 does not do this — one App Server, TUI and notifier both attached | the face attaches to a shared server given by `AGENT_BUS_CODEX_WS`, and says so in the log when it is driving its own instead |
| | a rejected `turn/steer` fell through to `turn/start` for *any* error, so a timeout — where the steer may well have landed — started a second turn | only a protocol rejection, and only after the server confirms no turn is running. Overload and timeouts propagate |
| | `thread/resume` reporting a turn already in progress was ignored, so the first message started a turn instead of steering | the active turn comes from the resume |
| | a `turn/completed` arriving before a slow response let that response resurrect a finished turn | an epoch counter; a response that is out of date does not win |
| | server-initiated requests were parsed as notifications and left unanswered, hanging the turn | refused with `-32601`. This client approves nothing |
| | `stop()` did not abort the poll, so a delivery could run after it | the loop aborts the request and re-checks after every await |
| | push was not tied to the transport closing | `server.onclose` and the signals stop it |
| | a 409 retried every two seconds, which would steal the read from whoever legitimately owns it | one message, then stop |
| smoke | **a doubled `deliver()` left everything green** — nothing counted notifications | two distinct messages, asserted by id, order and count. Verified: the mutation now fails |
| smoke | the stderr-before-kill hang, fixed in `smoke.ts`, had been copied into `smoke-push.ts` | fixed there too |

Codex also confirmed what did **not** need changing: no lock is needed while
`start()` completes before a serial loop; the pending entry is registered
before the write, so a fast answer cannot race it; and the line framing is
correct. Refusing an unfiltered `ab_consume` while allowing a filtered one is
the right cut — with the caveat that the filter only wins once the waiter is
registered, so a *pushed* reply cannot be re-read.

## B.4 — loading it

| | |
|---|---|
| Claude, as a plugin | `.claude-plugin/plugin.json` declares the server. `claude --plugin-dir src/mcp` loads it and `mcp list` reports **connected** |
| Claude, without it | `--mcp-config .mcp.json` — proven by a real session calling `ab_ls` and getting the registry back |
| Codex | `[mcp_servers.agent-bus]` in `~/.codex/config.toml` |

A plugin manifest cannot know the session's name, so the face **derives** one:
`<runtime>.<cwd>@<host>`, trimmed to the name rule
([identity § names](../../docs/01-identity.md#names)) — the way V1 names its
channels. Zero configuration beyond the token. Verified: a session started with
no `AGENT_BUS_NAME` registered as
`claude-code.home-parf-src-ai-agent-bus-src-mcp@parf.us`.

❓ **Channel push into a headless session did not surface.** With the
capability declared and the MCP server connected, a `claude -p` run registered,
the push loop took the message off the daemon — and the model never saw a
channel message. Channels appear to be an interactive-session path. This costs
nothing in the plan, which already says the live criterion is checked by hand,
but it does mean **the headless harness cannot prove it**. *Settled by:* one
interactive run.

## B.5 — the plugin's own commands

`/ab:ls` and `/ab:send` are in `commands/`. Colon namespace, because these are
plugin commands and not MCP tools ([glossary](../../docs/glossary.md)).

**The SessionStart hook was dropped**: the MCP server already registers itself
at start, so a hook doing it again is a second implementation of B.2 with
nothing to add.

**Not done in B, on purpose**: `call`, `ack`, topics, script services, SSH
token issuance.
