# TypeScript client library

Status: proposal, not implemented. Shared contract and cross-language vision:
[client libraries](../client-libraries.md#scope). This file owns the TypeScript
design.

## Connection

`connect` returns a `Bus` bound to one named principal. It discovers the
account socket by uid, or takes an explicit address (unix path, `http://` or
a pinned `https://`; supported runtimes: Node with undici for unix-socket
fetch, Bun with native `fetch(url, {unix})`. Browsers are not supported:
the daemon sends no CORS headers.
Construction performs no I/O.

```ts
import { Bus } from "@agent-bus/client";

const bus = Bus.connect({
  addr: process.env.AGENT_BUS_ADDR,
  token: process.env.AGENT_BUS_TOKEN,
  expectName: "#worker@team", // the first authenticated call checks this
});
// or: Bus.connect({ addr: null }) — discovers the account socket by uid
const me = await bus.whoami(); // { you: "#worker@team" } — via GET /status
await bus.close();
```

### Verifying the caller

The caller's identity comes from `GET /status` (`you`), not `GET /identity`
(which is the node's facts, not the caller's). Use `whoami()` to check; a
mismatch raises `IdentityMismatch`.

```ts
const you = await bus.whoami(); // { you: "#worker@team" } — via GET /status
if (you.you !== expected) throw new IdentityMismatch(you.you);
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

Registry `register` belongs to `Bus` (it creates a record the caller owns).
`manage`, `unregister` and `lookup` belong to `Record` (they act on a target
record the caller owns or maintains). Registry listing is `bus.records(...)`.

## Messaging

`send` is fire-and-forget (the daemon's acceptance is the answer; it does
not mean the peer read it). `call` registers a pending entry under a fresh
correlation tag before sending, so a fast reply cannot race the
registration, then waits for a matched reply. The outcome is a
discriminated union:

```ts
type CallOutcome =
  | { kind: "answer"; message: Message }
  | { kind: "done" }
  | { kind: "timeout"; acked: boolean };

const outcome = await bus.call("#other@team", "work", { timeout: 10_000 });
if (outcome.kind === "answer") { process(outcome.message.body); }
if (outcome.kind === "timeout" && outcome.acked) { /* taken, not answered */ }
```

`call` never throws `BusTimeout`: timeout is an outcome kind. The ack is
delivered through an optional `onAck` callback, not the return value.
`consume` takes one message, filtered or not. `reply`, `ack` and `done`
act on a consumed message's `message_id`.

```ts
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
handlers (default 1); `options.signal` stops intake (no further
unfiltered consume is issued); in-flight handlers are awaited to
`options.drainTimeout`, and unfinished work is reported. In-flight
handlers awaiting `call` replies still need their filtered reply polls
during drain.

**Dispatcher at capacity.** The worker slot is reserved before the
unfiltered consume. At capacity, the inbox reader must still drain exact
pending-call filters so in-flight handlers' outgoing `call` requests are
delivered — pausing all consume polls deadlocks handler calls. The exact
scheduler (fair share between unfiltered work and filtered drains) is an
open design choice, not settled here.

```ts
await bus.serve(async (msg) => {
  const reply = await process(msg.body);
  return reply; // null → done
}, { workers: 4, signal: AbortSignal.timeout(3600_000) });
```

## Key-value store

`Record.kv` reads and writes the record's three stores (string, int, json).
Every write names its mode — `set(key, value, { mode: "set" })`, `add`
(refuses when present), `replace` (refuses when absent). Reads answer the
stored value, and a missing value is a 404 from the daemon: the client
surfaces it as `KVNotFound`, which is the same 404 an unseen record produces
— the current wire text does not distinguish them, and the library does
not parse refusal text to try
([Q17](../QUESTIONS.md#open-questions)).

JSON operations are a static ops list sent to the daemon, all applied or
none, at most 100 per call. The daemon applies them atomically; the client
never reads the current value to compute operations.

```ts
await rec.kv.set("string", "cursor", "1200", { mode: "set" });
await rec.kv.increment("runs", { by: 1 });
const result = await rec.kv.jsonApply("state", [
  { op: "inc", key: "done", value: 1 },
  { op: "push", key: "items", value: "widget" },
]); // { results: [{ op: "inc", key: "done", changed: true }, ...] }
const v = await rec.kv.get("string", "cursor"); // { kind: "string", value: "1200" }
```

Values are size-limited (512 KiB per value, 10 000 names per record per
kind). An integer read returns a `bigint` (lossless past JavaScript's safe
range); writes accept `bigint` and serialize it as raw JSON digits. The
library parses the daemon's answer with a scoped reviver (reading
`ctx.source` on the JSON context, or scanning the int fields directly),
never through an unscoped `JSON.parse` that rounds before a reviver
could act.

## Locks

`Record.locks` acquires, extends, releases and lists named locks. A hold
lasts its ttl or until released. `forceRelease(name)` releases another
holder's lock, is audited, and answers the displaced holder. A hold whose
record's Owner is deactivated ends at once. A waiting take takes an
`AbortSignal`; a cancelled wait may still have been granted — the hold
then expires by ttl, or `holders()` answers whether it survived.

```ts
const lock = await rec.locks.acquire("deploy", { ttl: "30s", signal });
await rec.locks.extend("deploy", "1m");
await rec.locks.release("deploy");
const previous = await rec.locks.forceRelease("deploy"); // the displaced holder
await rec.locks.holders(); // [{ name, holder, expires }] — reshaped from the daemon's {record, locks: {…}} answer
```

A second `acquire` on a lock the caller already holds is refused at once
(409, not queued): two concurrent handlers on one Bus both acquiring the
same lock means one gets it and the other gets an immediate refusal. An
in-process queue per (record, lock) is a caller's own building, not the
library's.

The lock API has no lease or fencing token: a release names only the
record, the lock and the caller, so a stale handle can release a later
acquisition by the same name. The library must not promise that a stale
handle is safe to release; the caller checks the error.

## Config and secret

Read-only in this version. Both follow the private-values rule: the
record's Owner and Maintainers read them, an Agent reads its own; the
allow list and group membership do not grant access. Reads are not cached;
each call reaches the daemon. A secret is a `Secret` wrapper: redacted
`toString()`, explicit `reveal()` for the bytes, never logged.

Config: absent config answers a successful `null` from the daemon, which
`config()` returns as `null` — not a refusal, not `KVNotFound`.
Secret: absent secret is a 404 from the daemon, the same 404 an unseen
record produces, and the library does not parse refusal text to
distinguish it.

```ts
const cfg = await rec.config();  // object | null (successful empty)
const sec = await rec.secret();  // Secret; .reveal() for the bytes; 404 if absent
```

## Timeouts, errors and cancellation

Every method accepts an optional `AbortSignal` (or a timeout in the
options). Transport errors (`BusUnreachable`) and daemon refusals
(`BusError` with status and the daemon's own message) are distinct
exception types. The daemon's refusal text does not identify every
internal reason: the wire carries status and text only, and fine-grained
reason codes are an [open question](../QUESTIONS.md#open-questions). `call`
times out as an outcome kind (`{ kind: "timeout", acked }`), not as a
thrown exception. A cancelled or aborted operation raises `AbortError`;
it never implies the remote work stopped, and the client does not retry.

## Async model

All I/O is `async`/`await` on the JavaScript event loop. No worker
threads, no blocking calls. The inbox dispatcher in `serve` runs
concurrent handlers on the same event loop; a CPU-bound handler blocks
the loop, so handlers that need heavy work should delegate it and return
a promise.

## Explicitly out of scope

Setters for config and secret (daemon writes remain through other
faces); resource content reads (the MCP face resolves those); retry of
uncertain sends; credential issuance and administration (the CLI and
admin program own those).
