# Go client

📌 **TL;DR:**
- **Problem:** a Go service that should take part on the bus has to handle HTTP, call correlation and uncertain writes itself.
- **Outcome:** `Dial` gives you send, call, serve, key-value, locks and read-only settings with contexts and bounded goroutines.
- **Start:** copy [the example](#try-this-first); it needs a name, a token and a running bus.

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

## Why you'd want it

- 🧭 **Contexts everywhere.** Every call takes a context first, so deadlines and cancellation work as usual.
- 🔑 **Credentials stay out of your code.** The address, token and TLS fingerprint default from the CLI's environment.
- 🛡️ **No silent repeats.** An uncertain request is never retried.

## What you get

- ✅ **Call and serve:** `Call(ctx, to, body, opts...)` returns an outcome; `Serve(ctx, handler, opts...)` answers with a bounded pool of goroutines.
- ✅ **Targets:** `Record(name)` selects a record and exposes `KV` and `Locks`.
- ✅ **Your own name works:** a call to yourself is answered by your own handler.
- ✅ **Safe secrets:** formatting and JSON are redacted; `Bytes()` explicitly returns a copy.

## Try this first

Serve one handler and call it from the same process (a disposable bus, token in `AGENT_BUS_TOKEN`):

```go
package main

import (
	"context"
	"fmt"
	"time"

	bus "github.com/parf/ai-agent-bus/src/clients/go"
)

func main() {
	ctx, stop := context.WithCancel(context.Background())
	b, err := bus.Dial(bus.Options{ExpectedName: "#worker@localhost"})
	if err != nil {
		panic(err)
	}
	defer b.Close()
	served := make(chan error, 1)
	go func() {
		served <- b.Serve(ctx, func(_ context.Context, m bus.Envelope) (*string, error) {
			answer := "pong " + m.Body
			return &answer, nil
		}, bus.WithWorkers(2))
	}()
	outcome, err := b.Call(ctx, "#worker@localhost", "hello", bus.WithTimeout(5*time.Second))
	if err != nil {
		panic(err)
	}
	if outcome.Answered {
		fmt.Println(outcome.Message.Body)
	}
	n, err := b.Record("#worker@localhost").KV.Increment(ctx, "attempts", 1)
	if err != nil {
		panic(err)
	}
	fmt.Println(n)
	stop()
	<-served
}
```

```text
pong hello
1
```

- ▶️ The [shared examples](../examples/README.md) run agents in every language together.

## Technical details

- **Module:** the [reviewed Go API](../../../Plans/R1.0-Release/done/client-libraries/go.md) is implemented; the import is `github.com/parf/ai-agent-bus/src/clients/go`, which is not published.
- **Connecting:** `Dial(Options{})` makes a caller handle without network I/O. The first operation checks `/status.you`, and `ExpectedName` requires a match. The CLI's address, token and TLS fingerprint defaults apply; unix sockets, HTTP and HTTPS work.
- **TLS and transport:** HTTPS validates normal certificates or an explicit SHA256 DER pin before sending credentials. HTTP/1 connections disable keep-alive and replay, and redirects are refused.
- **Private values:** config and secrets are read on every call.
- **Calls:** each registers a fresh client-specific tag before sending. One dispatcher owns reads, routes exact pending-call filters even at full worker capacity, and consumes new work only with a reserved slot. Raw consume excludes calls and serving.
- **Receipts and replies:** receipts must name the accepted request ID, and replies honor `reply_to`. Serving ignores receipts, explicit replies and late answers carrying its own call prefix. `Done` is nil; a pointer to an empty string sends an empty answer.
- **Self-calls:** a call to the client's own name is answered by its own serving handler. The request it took back is work for a free slot, never the answer, and is drained like other taken work; a handler waiting on such a call needs another slot.
- **Timeouts:** calls default to the connection timeout (30 seconds), bounded further by their context or `WithTimeout`. Individual requests share the caller's deadline and a 30-second default transport timeout. Raw consumes default to 100 milliseconds, and `WithWait` ranges from zero to the daemon's 60-second cap. Lock waits default to zero, and the transport budget adds the requested lock or consume wait; a caller context can shorten the total.
- **Uncertainty:** an unconfirmed mutation or destructive consume is `UncertainOutcome`, with the underlying error retained. A confirmed call timeout is `ErrCallTimeout`, including the observed ack. No uncertain request is retried, and closing wakes waiters without unregistering records.
- **Serving stop:** cancellation stops intake and drains handlers up to `WithDrainTimeout` (default 30 seconds); each handler's context stays live until then. Failures send no success receipt, `ServeError` reports failed and unfinished work, and Go cannot terminate handlers that ignore cancellation.
- **Leases:** helpers are not fences. They use conservative local expiry and retire before release or extension, keeping a live helper only after a confirmed extension. A stale same-principal helper may still affect a newer hold, because there is no fencing token.

## Checks

- Use `bash src/clients/check.sh go` from the checkout root. It builds the real daemon fixture and sets an isolated temporary directory.
- To use a copied crate, set `AB_GO_CLIENT_PATH` to its directory. Equivalent commands, with `AB_CLIENT_FIXTURE` and `TMPDIR` set, are `go test -race -count=1 ./...` and `go vet ./...`.
- Tests cover the real daemon, protocol refusals, uncertainty, correlation, binary and int64 KV, atomic JSON, locks, full-capacity nested calls, shutdown, secrets and TLS.
