# Go client

Implementation of the [reviewed Go API](../../../Plans/R1.0-Release/done/client-libraries/go.md).
The module import is `github.com/parf/ai-agent-bus/src/clients/go`; it is not published.

`Dial(Options{})` creates a caller handle without network I/O. Its first operation
checks `/status.you`; `ExpectedName` requires a match. CLI address, token and TLS
fingerprint environment defaults apply. Unix sockets, HTTP and HTTPS are supported.
HTTPS validates normal certificates or an explicit SHA256 DER pin before sending
credentials. HTTP/1 connections disable keep-alive and replay; redirects are refused.

`Record(name)` selects a target and exposes `KV` and `Locks`. Registry and messaging
methods take context first. Config and secrets are read on every call. Secret
formatting and JSON are redacted; `Bytes()` explicitly returns a copy.

Calls register a fresh client-specific tag before sending. One dispatcher owns
reads, routes exact pending-call filters even at full worker capacity, and consumes
new work only with a reserved slot. Raw consume excludes calls and serving.
Receipts must name the accepted request ID; replies honor `reply_to`. Serving
ignores receipts, explicit replies and late answers carrying its own call prefix.
`Done` is nil; a pointer to an empty string sends an empty answer.

Calls default to the connection timeout (30 seconds), bounded further by their
context or `WithTimeout`. Individual requests share the caller's deadline and a
30-second default transport timeout. Raw consumes default to 100 milliseconds;
`WithWait` ranges from zero to the daemon's 60-second cap. Lock waits default to zero.
The transport budget adds the requested lock/consume wait; a caller context can
shorten the total deadline.

An unconfirmed mutation or destructive consume is `UncertainOutcome`, with the
underlying context/transport error retained. Confirmed call timeout is
`ErrCallTimeout`, including the observed ack. No uncertain request is retried.
Closing wakes waiters without unregistering records.

Serving cancellation stops intake and drains handlers up to `WithDrainTimeout`
(default 30 seconds). Each handler's context remains live until the drain deadline.
Failures send no success receipt; `ServeError` reports failed and unfinished work.
Go cannot terminate handlers that ignore cancellation.

Lease helpers are not fences. They use conservative local expiry and retire before
release or extension, retaining a live helper only after confirmed extension.
A stale same-principal helper may still affect a newer hold; there is no fencing token.

Use `bash src/clients/check.sh go` from the checkout root. It builds the real
daemon fixture and sets an isolated temporary directory. To use a copied crate,
set `AB_GO_CLIENT_PATH` to its directory. Equivalent commands with `AB_CLIENT_FIXTURE`
and `TMPDIR` set:

    go test -race -count=1 ./...
    go vet ./...

Tests cover the real daemon, protocol refusals, uncertainty, correlation, binary and
int64 KV, atomic JSON, locks, full-capacity nested calls, shutdown, secrets and TLS.
