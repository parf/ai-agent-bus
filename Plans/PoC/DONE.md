# DONE — PoC

| Wave | Result |
|---|---|
| **A — two shells talk** | `agent-busd` on a unix socket and loopback HTTP, in-memory registry and inboxes; `agent-bus` with `status`, `register`, `ls`, `send`, `consume`, `reply`. `src/smoke.sh` is the acceptance (22 checks) and `go test -race ./...` the regressions. |
| **A review** | Codex reviewed `c8cde3a` over the V1 bus and found two real defects; both fixed, both now covered by tests that fail against the old code. |

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
