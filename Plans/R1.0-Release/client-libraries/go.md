# Go client library

Status: proposal, not implemented. Shared contract and cross-language vision:
[client libraries](README.md#scope). This file owns the Go design.

## Connection

`Dial` returns a `Bus` bound to one named principal. It discovers the
account socket by uid, or takes an explicit address (unix path, `http://`
or a pinned `https://`). Construction performs no I/O; the first call
resolves the identity and fails with a concrete error if the daemon is
unreachable. `Close` stops the background reader and releases the
transport.

```go
import bus "github.com/parf/ai-agent-bus/client"

b, err := bus.Dial(bus.Options{Token: os.Getenv("AGENT_BUS_TOKEN")})
// or: bus.Dial(bus.Options{}) — discovers the account socket by uid
// or: bus.Dial(bus.Options{Name: "#worker@team"}) — registers and mints a token
me, err := b.Identity() // Identity{Name: "#worker@team", ...}
defer b.Close()
```

The zero `Options` is valid: it discovers the socket and authenticates by
the caller's uid on the account socket. A `Token` in the options switches
to the shared socket.

## Context

Every method takes a `context.Context` as its first argument. Cancellation
stops the HTTP request, ends a lock wait, and makes a consume return
`ctx.Err()`. The runner's own stop signal is its context; a cancelled
context never implies the remote work stopped.

```go
func (b *Bus) Send(ctx context.Context, to, body string, opts ...SendOption) (Envelope, error)
func (b *Bus) Consume(ctx context.Context, opts ...ConsumeOption) (Envelope, error)
```

The zero `ConsumeOption` is a 55-second long poll; `WithWait` overrides
it. `ctx.Err()` distinguishes a deliberate stop from a timeout.

## Caller identity and record handles

The `Bus` is the authenticated caller. A `Record` handle binds one named
record; its `KV`, `Locks`, `Config` and `Secret` namespaces share that
binding. Handles are value types carrying a name and a pointer to the
bus; they are safe to copy.

```go
rec := b.Record("#other@team")
v, err := rec.KV.Get(ctx, bus.KVJSON, "progress")
lock, err := rec.Locks.Acquire(ctx, "build", bus.WithTTL(30*time.Second))
```

## Registry

`Register`, `Manage`, `Unregister` and `Lookup` belong to the `Bus`. A
`Record` handle's `Lookup` reads the record it binds. Registration
refuses fields it does not write (status, counters, timestamps,
credentials): a caller cannot set them, and the daemon's refusal names
them.

## Messaging

`Send` is fire-and-forget (the daemon's acceptance is the answer). `Call`
sends and waits for a matched reply, returning `ErrCallTimeout` on the
caller's deadline. `Consume` takes one message, filtered or not; `nil,
ctx.Err()` means the context ended first. `Reply`, `Ack` and `Done` act
on a consumed message's ID.

```go
reply, err := b.Call(ctx, "#other@team", "work", bus.WithTimeout(10*time.Second))
msg, err := b.Consume(ctx)
if err != nil { if errors.Is(err, context.Canceled) { return } }
b.Ack(msg)
```

The client does not retry an uncertain send: it may have been taken.

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

```go
err := b.Serve(ctx, func(ctx context.Context, msg bus.Envelope) (string, error) {
    return process(msg.Body)
}, bus.WithWorkers(4))
```

The runner's own stop signal is its context: a cancelled context stops
the loop, no further consume is issued, and in-flight handlers are
awaited.

## Key-value store

`Record.KV` reads and writes the record's three stores (string, int,
json). Every write names its mode; every read returns the stored value or
`ErrKVAbsent`. JSON operations are atomic on top-level keys: the edit
function receives the current object and returns the operations, all
applied or none. Values are size-limited (512 KiB per value, 10 000 names
per record per kind).

```go
err := rec.KV.Set(ctx, bus.KVString, "cursor", "1200", bus.KVSet)
n, err := rec.KV.Increment(ctx, bus.KVInt, "runs", 1)
out, err := rec.KV.JSON(ctx, "state", []bus.KVOp{
    {Op: "inc", Key: "done", Value: json.RawMessage("1")},
    {Op: "push", Key: "items", Value: json.RawMessage(`"widget"`)},
})
v, err := rec.KV.Get(ctx, bus.KVString, "cursor") // v.Bytes
```

An integer that exceeds an int64 is refused, not silently narrowed.

## Locks

`Record.Locks` acquires, extends, releases and lists named locks. A hold
lasts its ttl or until released; a crashed holder's ttl ends the hold.
`Force: true` releases another holder's lock and is audited. A hold whose
record's Owner is deactivated ends at once. A waiting acquire takes its
context; a cancelled context never grants.

```go
lock, err := rec.Locks.Acquire(ctx, "deploy", bus.WithTTL(30*time.Second))
err = rec.Locks.Extend(ctx, "deploy", time.Minute)
err = rec.Locks.Release(ctx, "deploy")
locks, err := rec.Locks.Holders(ctx)
```

A hold that began before the record's latest deactivation ends at once
(the daemon's absence stamp), so a deactivated record's locks are gone
even if no call saw it away.

## Config and secret

Read-only in this version. Both follow the private-values rule: the
record's Owner and Maintainers read them, an Agent reads its own; the
allow list and group membership do not grant access. Reads are not
cached; each call reaches the daemon. A secret's bytes are never logged.

```go
cfg, err := rec.Config(ctx)
sec, err := rec.Secret(ctx)
```

## Timeouts, errors and receipts

Every method takes a `context.Context`. Transport failures (`ErrUnreachable`)
and daemon refusals (`*BusError` with `Status` and the daemon's own
message) are distinct error types joined by `errors.Is`/`errors.As`. A
call that times out returns `ErrCallTimeout`; a consume whose context
ended returns that context's error. A `Done` receipt means "ran, printed
nothing"; an `Ack` receipt means "picked up"; the caller learns of a
script failure by timing out, not by a receipt.

The client does not retry an uncertain send.

## Sync/async and parallelism

Go's model is synchronous: each call blocks until it completes or ctx
ends. Concurrency comes from the caller's goroutines. `Serve`'s bounded
worker pool (default 1) is the one built-in concurrency limit; the caller
sets `WithWorkers` to match what the script can handle. No
goroutine-safety issues: the Bus is safe for concurrent use (the HTTP
client is), and Record handles are value types.

## Explicitly out of scope

Setters for config and secret (daemon writes remain through other
faces); resource content reads (the MCP face resolves those); retry of
uncertain sends; credential issuance and administration (the CLI and
admin program own those).
