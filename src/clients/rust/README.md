# Rust client

📌 **TL;DR:**
- **Problem:** an async service that should take part on the bus has to handle HTTP, call correlation and uncertain writes itself.
- **Outcome:** `Bus` gives you send, call, serve, key-value, locks and read-only settings with bounded handler tasks.
- **Start:** start with the [isolated examples](../examples/README.md#try-this-first), or adapt [the snippet](#try-this-first) to your bus.

## Why you'd want it

- ⚡ **Async all the way.** Calls and handlers run on Tokio, and a slow handler never blocks the rest.
- 🔑 **Credentials stay out of your code.** The environment variables match the CLI.
- 🛡️ **No silent repeats.** An uncertain write or consumption is never replayed.

## What you get

- ✅ **Call and serve:** `bus.call(to, body, CallOptions)` returns a reply or a completion; `bus.serve(handler, ServeOptions, cancel)` answers with a bounded pool of tasks.
- ✅ **Targets:** `bus.record(name)` selects a record and gives its `kv()` and `locks()`; `bus.own_record()` is your own.
- ✅ **Your own name works:** a call to yourself is answered by your own handler.
- ✅ **Safe secrets:** logging is redacted; `.bytes()` is explicit access.

## Try this first

Use a Rust toolchain supporting edition 2024. Add `agent-bus` as a path dependency pointing to this directory, and `tokio = { version = "1", features = ["rt-multi-thread", "macros"] }` to your application's Cargo dependencies. Save the example as `src/main.rs`.

1. Register `#worker@localhost` as an Agent using an authorized CLI account. Construction alone does not register the identity.
2. Obtain its token with `agent-bus token '#worker@localhost'` and set `AGENT_BUS_TOKEN` to that token.
3. Set `AGENT_BUS_ADDR` to the same daemon socket or URL; HTTPS also needs the appropriate trust configuration.

Serve one handler and call it from the same process:

```rust,no_run
use agent_bus::{Bus, CallOptions, CallOutcome, CancellationToken, HandlerReply, KvValue, ServeOptions, SetMode};
use std::time::Duration;

#[tokio::main]
async fn main() -> agent_bus::Result<()> {
    let bus = Bus::builder("#worker@localhost").connect().await?;
    let stop = CancellationToken::new();
    let server = {
        let (bus, stop) = (bus.clone(), stop.clone());
        tokio::spawn(async move {
            bus.serve(
                |m| async move { Ok(HandlerReply::Body(format!("pong {}", m.body()))) },
                ServeOptions { workers: 2, ..Default::default() },
                stop,
            )
            .await
        })
    };
    let options = CallOptions { timeout: Duration::from_secs(5), ..Default::default() };
    match bus.call("#worker@localhost", "hello", options).await? {
        CallOutcome::Reply(message) => println!("{}", message.body()),
        CallOutcome::Done(_) => println!("Completed without an answer"),
    }
    let me = bus.own_record();
    me.kv().set("progress", KvValue::Int(1), SetMode::Set).await?;
    stop.cancel();
    server.await.unwrap()?;
    bus.close().await
}
```

```text
pong hello
```

## Private agent-log example

The [executed native example](tests/conformance_state.rs) (`example_event_log / native_event_example`) uses ordinary calls to `#bus` on a disposable daemon.

- Three separate handles: the agent writes, its Owner reads and lists, and an unmanaged identity receives a private refusal.
- The writing agent cannot read its own log just because it wrote it.
- Each call has a bounded wait; no event-specific SDK helper is involved.
- See [agent logs](../../../docs/agents/bus.md#agent-logs) for authority and reply behavior.

## Event subscriptions

The [executed subscription example](tests/conformance_state.rs)
(`example_event_subscription / native_event_subscription_example`) uses ordinary calls to `#bus` and consumes the authorized queue.

- It lists the subscription, checks exactly one copy and deletes the subscription.
- It checks that cancellation stops further copies.
- Copies are non-waiting notices: send no ack or done for them.

See [agent logs](../../../docs/agents/bus.md#agent-logs) and
[receipts](../../../docs/04-messaging.md#receipts). No subscription helper is needed.

## Technical details

- **Package:** a local path dependency; `agent_bus::VERSION` comes from the checkout's canonical daemon version; see [the Rust API source](src/lib.rs).
- **Roles:** `Bus` is the authenticated caller; `Record` selects a target without changing credentials, and KV and locks belong to the target handle.
- **Address:** `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN` and `AGENT_BUS_TLS_FINGERPRINT` follow the CLI. Without an address, local socket discovery checks the XDG runtime, then `/run/agent-bus`, choosing an account socket without a token and `bus.sock` with one (the environment's or `.credential()`'s), then the CLI default.
- **Sockets and TLS:** `.unix_socket(path)` supports forwarded sockets. HTTPS uses ordinary verification or an explicit SHA256 DER pin, and a pin mismatch aborts TLS before HTTP credentials. Redirects and transport retries are off. `.root_certificate(certificate)` adds a CA and keeps these policies.
- **Identity:** `identity()` returns public node facts from `/identity`; `name()` and `status().you` identify the authenticated caller.
- **Protocol:** The client requires both `/identity` protocol fields to equal the [current protocol version](../../../docs/09-daemon-api.md#protocol-version). Missing, malformed and unsupported versions are refused before application requests. Refusals retain the daemon’s `kind` and cause; HTTP 500 hides internal details.
- **Private values:** config and secret reads are lazy, shared and cached. Invalidate with `invalidate_private(PrivateSelection::Both)`; retained values stay readable by their owners after server revocation. Config is an `Arc<Value>`, so clone before modifying. Private writes and credential minting are outside this client.
- **Serve:** it acknowledges before handling, runs at most `workers` handlers and keeps routing call replies at capacity. `Body("")` is an empty answer, `Done` is completion without an answer, `Manual` delegates completion to the handler, and errors produce no success receipt. Cancellation stops intake and waits up to `drain_timeout`, reporting unfinished tasks before aborting them; known successful takes during the stop still drain. Expired deadlines are skipped before acknowledgment. Consumed messages are never requeued, and aborting a task cannot undo its writes.
- **Calls:** an ack is optional progress and an answer or done is terminal. After confirmed acceptance, a timeout preserves whether an ack was seen; before acceptance is known, a deadline or lost write response is `UncertainOutcome`. A lost consume response is also uncertain, because `GET /consume` removes the message. Such operations are never replayed, and cancelling a call drops its waiter only.
- **Deadlines and replies:** the whole call, including the POST, is bounded by its deadline. Receipts must name the original message id. A client-specific tag prefix lets serving ignore late responses to its own calls, for route-correlated replies without `re`, and an explicit `re` never becomes work.
- **Self-calls:** a call to the caller's own name is answered by its own `serve` handler; the request it took back is work for a free worker, never the answer, so a handler awaiting such a call needs another worker.
- **Close and consume:** close also rejects cached private reads, and values returned before it remain caller-owned. Raw consume conflicts locally with serve or calls; one shared dispatcher polls exact filters fairly, with short waits so new calls and shutdown stay bounded.
- **Leases:** leases are local conveniences, not fences. They are not `Clone`, and `Drop` makes no request. Release or expiry ends a hold; a release attempt or failed extension retires the helper so it cannot replay an uncertain mutation. A stale handle can affect a later same-principal hold; local expiry checks narrow the window without closing it. Every HTTP lock refusal propagates, including try-acquire 409.

## Checks

- Verify with `bash ../check.sh rust` in this directory; the [shared Cargo target](../README.md#checks) avoids a build cache per worktree.
- The real daemon test uses Go to run `src/clients/fixture`, or an `AGENT_BUS_FIXTURE_BINARY` built by the root integration runner.
- `AB_RUST_CLIENT_PATH` selects a scratch crate for `src/clients/check.sh rust` and the `client_rust` slow shard; keep the relative canonical VERSION file when copying a crate for a mutation run.
- Tests cover Unix, HTTP and TLS, identity, concurrent call correlation, receipt outcomes, cancellation, serving at capacity, caches and typed storage.
- `./package.sh` produces and verifies a local archive. It snapshots the canonical version into the ignored `VERSION.local`, included in the archive so it builds outside the checkout; the package declares `publish = false`.
