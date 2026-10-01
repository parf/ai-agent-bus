# Rust client

Async Tokio client for the existing daemon API. `Bus` is the authenticated
caller; `Record` selects a target without changing credentials. KV and locks
belong to the target handle. The package is unpublished development code;
`agent_bus::VERSION` comes from the checkout's canonical daemon version.

```rust,no_run
use agent_bus::{Bus, CallOptions, CallOutcome, KvValue, SetMode};

#[tokio::main]
async fn main() -> agent_bus::Result<()> {
    let bus = Bus::builder("#worker@team").connect().await?;
    let jobs = bus.record("jobs@team")?;
    jobs.kv().set("progress", KvValue::Int(1), SetMode::Set).await?;
    match bus.call("#builder@team", "build", CallOptions::default()).await? {
        CallOutcome::Reply(message) => println!("{}", message.body()),
        CallOutcome::Done(_) => println!("Completed without an answer"),
    }
    bus.close().await
}
```

`AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN` and `AGENT_BUS_TLS_FINGERPRINT` follow the
CLI. Without an address, local socket discovery checks XDG runtime then
`/run/agent-bus`, choosing an account socket without a token and `bus.sock`
with a token, then falls back to the CLI default (XDG bus socket or the
account's default socket in the system temporary directory). Explicit `.unix_socket(path)` supports forwarded sockets.
HTTPS uses ordinary certificate verification or an explicit SHA256 DER pin;
pin mismatch aborts TLS before HTTP credentials. Redirects and transport
retries are disabled on the built-in client. Use `.root_certificate(certificate)` to add an explicit CA while preserving
these policies.

`identity()` returns public node facts from `/identity`; `name()` and
`status().you` identify the authenticated caller.

Config/secret reads are lazy, shared and cached. Invalidate with
`invalidate_private(PrivateSelection::Both)`; retained values remain readable
by their owners even after server revocation. Config is shared as `Arc<Value>`;
clone its contents before modifying. Secret logging is redacted; `.bytes()` is
explicit access. Private writes and credential minting are outside this client.

`serve` acknowledges before handling, runs at most `workers` handlers and
continues routing call replies at capacity. `Body("")` is an empty answer;
`Done` is completion without an answer; `Manual` delegates completion to the
handler. Errors produce no success receipt. Cancellation stops intake and
waits up to `drain_timeout`, reporting unfinished tasks before aborting them.
Known successful takes returned during intake stop still drain through handling.
Already expired deadlines are skipped before acknowledgment and handling.
Consumed messages are never requeued. Aborting a task cannot undo its writes.

A call's ack is optional progress and its answer/done is terminal. After confirmed
acceptance, timeout preserves whether ack was seen. Before acceptance is known,
a deadline or lost write response is `UncertainOutcome`. A lost consume response
is also uncertain because GET /consume removes the message. Such operations
are never replayed. Cancelling a call drops its waiter only.
The whole call, including POST, is bounded by its deadline. Receipts must name
the original message id. A client-specific tag prefix lets serving ignore late
responses to its own calls, even from older runners that omit `re`. Explicit
`re` responses never become work. Close also rejects cached private reads;
values returned before close remain caller-owned.
Raw consume conflicts locally with serve or calls. One shared dispatcher polls
exact filters fairly, using short waits so new calls and shutdown remain bounded.

Leases are local conveniences, not fences. They are not Clone and Drop makes
no request. Explicit release or expiry ends a hold. A release attempt or failed extension retires the helper, so it
cannot replay an uncertain mutation. A stale handle can affect
a later same-principal hold; local expiry checks narrow this window, without
closing it. Every HTTP lock refusal propagates, including try-acquire 409.

Verify with `cargo test --locked` in this directory. The real daemon test uses
Go to run `src/clients/fixture`, or an `AGENT_BUS_FIXTURE_BINARY` built by the
root integration runner. `AB_RUST_CLIENT_PATH` selects a scratch crate for
`src/clients/check.sh rust` and the `client_rust` slow shard. Preserve the
relative canonical VERSION file when copying a crate for a mutation run.
Tests cover Unix/HTTP/TLS,
identity, concurrent call correlation, receipt outcomes, cancellation, serving
at capacity, caches and typed storage. Root integration runs the daemon smoke
suite separately. The [reviewed API plan](../../../Plans/R1.0-Release/client-libraries/rust.md#surface)
owns the intended scope.

`./package.sh` produces and verifies a local archive. It snapshots the canonical
version into ignored generated `VERSION.local`, included in the archive so it
builds outside the checkout. There is no second tracked release value, and the
package remains marked `publish = false` until release packaging is approved.
