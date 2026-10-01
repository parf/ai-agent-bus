# TypeScript client library

Status: proposal, not implemented. Shared contract and cross-language vision:
[client libraries](README.md#scope). This file owns the TypeScript design.

## Connection

`connect` returns a `Bus` bound to one named principal. It discovers the
account socket, or takes an explicit address (unix path, `http://` or a
pinned `https://`). Construction performs no I/O; the first call resolves
the identity and fails loudly if the daemon is unreachable or the
credential is wrong. `close()` stops the inbox reader and releases the
underlying transport.

```ts
import { Bus } from "@agent-bus/client";

const bus = await Bus.connect({ addr: process.env.AGENT_BUS_ADDR, token: process.env.AGENT_BUS_TOKEN });
// or: Bus.connect() — discovers the account socket by uid
// or: Bus.connect({ name: "#worker@team" }) — registers the name and mints a token
const me = await bus.identity(); // { you: "#worker@team", admin: false, ... }
await bus.close();
```

## Caller identity and record handles

The `Bus` is the authenticated caller. A `Record` handle binds one named
record to that identity; its `kv`, `locks`, `config` and `secret` namespaces
share the binding. Handles are cheap; they carry a name, not a connection.

```ts
const rec = bus.record("#other@team");   // a handle, no I/O
await rec.kv.get("json", "progress");
await rec.locks.acquire("build", { ttl: "30s" });
```

## Registry

`register`, `manage`, `unregister` and `lookup` belong to the `Bus` (they
are account-level or the caller's own operations). A `Record` handle's
`lookup` reads the record it binds.

## Messaging

`send` is fire-and-forget (the daemon's acceptance is the answer; it does
not mean the peer read it). `call` sends and waits for a matched reply,
raising `BusTimeout` on the caller's deadline and `BusClosed` if the
consumer stops. `consume` takes one message, filtered or not. `reply`,
`ack` and `done` act on a consumed message's `message_id`.

```ts
const reply = await bus.call("#other@team", "work", { timeout: 10_000 });
const msg = await bus.consume({ wait: "5s" });
await bus.ack(msg);
```

All sends go through one method on the daemon. The client does not retry
an uncertain send: it may have been taken.

## Publish

`publish` sends to a pub/sub record; `unsubscribe` takes the caller off its
Deliver-To list. Both are one-send helpers.

## Serve

`serve(handler, options)` runs a bounded consume loop. It acknowledges
before the handler runs, sends the handler's return value as the reply (or
`done` when it returns `null`), and does not send anything on a handler
that throws — the caller times out. `options.workers` bounds concurrent
handlers (default 1); `options.signal` stops the loop cleanly (no further
consume is issued, in-flight handlers are awaited). The inbox's one
unfiltered read is this loop's; every reply, receipt and filtered consume
goes through the same dispatcher.

```ts
await bus.serve(async (msg) => {
  const reply = await process(msg.body);
  return reply; // null → done
}, { workers: 4, signal: AbortSignal.timeout(3600_000) });
```

## Key-value store

`Record.kv` reads and writes the record's three stores (string, int, json).
Every write names its mode; every read answers the stored value or throws
`KVAbsent`. JSON operations are atomic on top-level keys: the edit function
receives the current object and returns the operations, all applied or none.

```ts
await rec.kv.set("string", "cursor", "1200");
await rec.kv.increment("int", "runs", 1);
await rec.kv.jsonApply("state", [
  { op: "inc", key: "done", value: 1 },
  { op: "push", key: "items", value: "widget" },
]);
const v = await rec.kv.get("string", "cursor"); // { kind: "string", value: "1200" }
```

Values are size-limited (512 KiB per value, 10 000 names per record per
kind). An integer that exceeds JavaScript's safe range arrives as its
digit string (the face's JSON reviver preserves it).

## Locks

`Record.locks` acquires, extends, releases and lists named locks. A hold
lasts its ttl or until released. `force: true` releases another holder's
lock and is audited. A hold whose record's Owner is deactivated ends at
once. Waiting takes an `AbortSignal`; a cancelled wait never grants.

```ts
const lock = await rec.locks.acquire("deploy", { ttl: "30s", signal });
await rec.locks.extend("deploy", "1m");
await rec.locks.release("deploy");
await rec.locks.holders(); // { name: holder, ... }
```

## Config and secret

Read-only in this version. Both follow the private-values rule: the
record's Owner and Maintainers read them, an Agent reads its own; the
allow list and group membership do not grant access. Reads are not cached;
each call reaches the daemon. A secret's bytes are never logged.

```ts
const cfg = await rec.config();  // { name: ..., value: ... }
const sec = await rec.secret();  // string
```

## Timeouts, errors and cancellation

Every method accepts an optional `AbortSignal` (or a timeout in the
options). Transport errors (`BusUnreachable`) and daemon refusals
(`BusError` with status and the daemon's own message) are distinct
exception types. A call that times out raises `BusTimeout`; a consume
that finds nothing returns `null`. A cancelled or aborted operation
raises `AbortError`; it never implies the remote work stopped, and the
client does not retry.

## Async model

All I/O is `async`/`await` on the JavaScript event loop. No worker
threads, no blocking calls. The inbox dispatcher in `serve` runs
concurrent handlers on the same event loop; a CPU-bound handler blocks
the loop, so handlers that need heavy work should delegate it and return
a promise.

At full worker capacity, the inbox reader must still deliver replies to
those workers' outgoing `call` requests: pausing all consume polls
deadlocks handler calls. The exact scheduler — how the dispatcher
handles unfiltered work and filtered reply drains without competing
readers — is an open design choice.

The lock API has no lease or fencing token: a local `using` block or
`try/finally` release cannot tell whether the hold was already displaced
by ttl or force-release and later re-acquired by the same principal. The
library must not promise that a stale handle is safe to release; the
caller checks the error.

## Explicitly out of scope

Setters for config and secret (daemon writes remain through other
faces); resource content reads (the MCP face resolves those); retry of
uncertain sends; credential issuance and administration (the CLI and
admin program own those).
