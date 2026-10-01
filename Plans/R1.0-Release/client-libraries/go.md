# Go client library

Status: proposal, not implemented. Shared contract and cross-language vision:
[client libraries](../client-libraries.md#scope). This file owns the Go design.

## Connection

`Dial` returns a `Bus` bound to one named principal. It discovers the
account socket by uid, or takes an explicit address (unix path, `http://`
or a pinned `https://`). Construction performs no I/O.

```go
import bus "github.com/parf/ai-agent-bus/client"

b, err := bus.Dial(bus.Options{Token: os.Getenv("AGENT_BUS_TOKEN")})
// or: bus.Dial(bus.Options{}) — discovers the account socket by uid
me, err := b.Status(ctx) // Status{You: "#worker@team", ...} — via GET /status
defer b.Close()
```

### Verifying the caller

The caller's identity comes from `GET /status` (`You`), not `GET /identity`
(which is the node's facts, not the caller's). Check it against the
expected name; a mismatch is an error.

## Caller identity and record handles

The `Bus` is the authenticated caller. A `Record` handle binds one named
record to that identity; its `KV`, `Locks`, `Config` and `Secret`
namespaces share the binding. Handles are value types carrying a name and
a pointer to the bus; they are safe to copy.

```go
rec := b.Record("#other@team")   // a handle, no I/O
v, err := rec.KV.Get(ctx, bus.KVJSON, "progress")
lock, err := rec.Locks.Acquire(ctx, "build", bus.WithTTL(30*time.Second))
```

## Registry

`Register` belongs to the `Bus` (it creates a record the caller owns).
`Manage`, `Unregister` and `Lookup` belong to `Record` (they act on a
target record the caller owns or maintains). Registration refuses fields
it does not write: status, counters, timestamps, credentials — the
daemon's refusal names them. Registry listing is `b.Records(ctx)`.

## Messaging

`Send` is fire-and-forget (the daemon's acceptance is the answer; it does
not mean the peer read it). `Call` registers a pending entry under a
fresh correlation tag before sending, so a fast reply cannot race the
registration, then waits for a matched reply. The outcome is explicit:

```go
outcome, err := b.Call(ctx, "#other@team", "work", bus.WithTimeout(10*time.Second))
// outcome.Answered: the reply body (a non-empty answer)
// outcome.Done: the script ran and printed nothing
// outcome.Acked: the message was taken but not yet answered
```

`ErrCallTimeout` when the caller's deadline passes. `Consume` takes one
message, filtered or not; it returns `(*Envelope, error)` where a nil
Envelope with nil error means nothing arrived before the deadline.
`Reply`, `Ack` and `Done` act on a consumed message's ID; `Ack` takes
ctx.

```go
msg, err := b.Consume(ctx)
if err != nil { if errors.Is(err, context.Canceled) { return } }
b.Ack(ctx, msg)
```

All sends go through one method on the daemon. The client does not retry
an uncertain send: it may have been taken.

## Publish

`Publish` sends to a pub/sub record; `Unsubscribe` takes the caller off
its Deliver-To list. Both are one-send helpers.

## Serve

`Serve(ctx, handler, options)` runs a bounded consume loop until ctx
ends. It acknowledges before the handler runs, sends the handler's
returned body as the reply (or `Done` when the body is empty), and sends
nothing on a handler that returns an error — the caller times out.
`options.Workers` bounds concurrent handlers (default 1); each takes a
slot before its consume and releases it on completion. The inbox's one
unfiltered read is this loop's; every reply, receipt and filtered consume
goes through the same dispatcher.

**Dispatcher at capacity.** The worker slot is reserved before the
unfiltered consume. At capacity, the inbox reader must still drain exact
pending-call filters so in-flight handlers' outgoing `Call` requests are
delivered — pausing all consume polls deadlocks handler calls. The exact
scheduler (fair share between unfiltered work and filtered drains) is an
open design choice, not settled here.

```go
err := b.Serve(ctx, func(ctx context.Context, msg bus.Envelope) (string, error) {
    return process(msg.Body)
}, bus.ServeOptions{Workers: 4})
```

The context's own stop signal stops intake (no further unfiltered
consume is issued); in-flight handlers are awaited to a bounded drain
deadline, and unfinished work is reported. In-flight handlers awaiting
`Call` replies still need their filtered reply polls during drain: the
exact scheduler is an open design choice.

## Key-value store

`Record.KV` reads and writes the record's three stores (string, int,
json). Every write names its mode; every read returns the stored value or
`ErrKVAbsent`. A missing value and an unseen record both answer 404 from
the daemon; the library surfaces both as `ErrKVAbsent` and does not parse
the daemon's refusal text to distinguish them
([Q17](../QUESTIONS.md#open-questions)). JSON operations are a static ops
list sent to the daemon, all applied or none, at most 100 per call. The
daemon applies them atomically; the client never reads the current value
to compute operations.

```go
err := rec.KV.Set(ctx, bus.KVString, "cursor", "1200", bus.KVSet)
n, err := rec.KV.Increment(ctx, bus.KVInt, "runs", 1) // no kind: int only
result, err := rec.KV.JSON(ctx, "state", []bus.KVOp{
    {Op: "inc", Key: "done", Value: json.RawMessage("1")},
    {Op: "push", Key: "items", Value: json.RawMessage(`"widget"`)},
}) // result[i].Changed, result[i].Value
v, err := rec.KV.Get(ctx, bus.KVString, "cursor") // v.Bytes
```

Values are size-limited (512 KiB per value, 10 000 names per record per
kind).

## Locks

`Record.Locks` acquires, extends, releases and lists named locks. A hold
lasts its ttl or until released; a crashed holder's ttl ends the hold.
`ForceRelease` releases another holder's lock, is audited, and returns
the displaced holder. A hold whose record's Owner is deactivated ends at
once. A waiting acquire takes its context; a cancelled context never
grants — but the hold may already have been taken before the cancel
reached the daemon, in which case it expires by ttl.

```go
lock, err := rec.Locks.Acquire(ctx, "deploy", bus.WithTTL(30*time.Second))
err = rec.Locks.Extend(ctx, "deploy", time.Minute)
err = rec.Locks.Release(ctx, "deploy")
prev, err := rec.Locks.ForceRelease(ctx, "deploy") // who was displaced
locks, err := rec.Locks.Holders(ctx)
```

A second `Acquire` on a lock the caller already holds is refused at once
(409, not queued): two concurrent goroutines both acquiring the same lock
means one gets it and the other gets an immediate refusal. An in-process
queue per (record, lock) is a caller's own building, not the library's.

## Config and secret

Read-only in this version. Both follow the private-values rule: the
record's Owner and Maintainers read them, an Agent reads its own; the
allow list and group membership do not grant access. Reads are not
cached; each call reaches the daemon. A secret is a `Secret` type: its
`String()` is redacted, `Bytes()` returns the exact bytes; never logged.

Config: absent config answers a successful `null` from the daemon, which
`Config(ctx)` returns as `nil` — not a refusal, not `ErrKVAbsent`.
Secret: absent secret is a 404 from the daemon, the same 404 an unseen
record produces, and the library does not parse refusal text to
distinguish it.

```go
cfg, err := rec.Config(ctx)  // json.RawMessage | nil (successful empty)
sec, err := rec.Secret(ctx)  // bus.Secret; .Bytes() for the content; 404 if absent
```

## Timeouts, errors and receipts

Every method takes a `context.Context`. Transport failures (`ErrUnreachable`)
and daemon refusals (`*BusError` with `Status` and the daemon's own
message) are distinct error types joined by `errors.Is`/`errors.As`. The
daemon's refusal text does not identify every internal reason: the wire
carries status and text only, and fine-grained reason codes are an [open
question](../QUESTIONS.md#open-questions). `Call` times out as
`ErrCallTimeout`; a consume whose context ended returns that context's
error. A `Done` receipt means "ran, printed nothing"; an `Ack` receipt
means "picked up"; the caller learns of a script failure by timing out,
not by a receipt.

The client does not retry an uncertain send.

## Sync/async and parallelism

Go's model is synchronous: each call blocks until it completes or ctx
ends. Concurrency comes from the caller's goroutines. `Serve`'s bounded
worker pool (default 1) is the one built-in concurrency limit; the caller
sets `WithWorkers` to match what the script can handle. The Bus is safe
for concurrent use (the HTTP client is), and Record handles are value
types. In-flight handlers awaiting `Call` replies still need their
filtered reply polls during drain: the exact scheduler is an open design
choice.

## Explicitly out of scope

Setters for config and secret (daemon writes remain through other
faces); resource content reads (the MCP face resolves those); retry of
uncertain sends; credential issuance and administration (the CLI and
admin program own those).
