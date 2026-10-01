# Rust client API

## Direction

Proposal, not implementation. Follow the [shared vision](../client-libraries.md#shared-vision).
Use async methods, `Result`, explicit ownership, `Duration` and typed builders.
Tokio is the proposed runtime; no blocking I/O hidden behind fields or `Drop`.
A separate blocking facade can be discussed later rather than nesting a runtime.

## Identity and handles

`Bus::builder(name).transport(...).credential(...).timeout(...).connect().await`
checks that the authenticated caller is `name`, returning an identity error on a
mismatch. Naming an Agent cannot substitute for its credential. Transport supports
Unix sockets and HTTP/HTTPS under the existing authentication rules.

`bus.record(name)` creates a cheap `Record` target, sharing the caller connection
and never changing identity. `bus.own_record()` selects the caller's record;
`bus.config()`, `secret()`, `kv()` and `locks()` are self conveniences.
Cloned Bus and Record handles share transport, caches and one dispatcher.

```rust
let bus = Bus::builder("#worker@team")
    .credential(worker_credential)
    .connect().await?;
let config = bus.config().await?;
let jobs = bus.record("jobs@team")?;
let lease = jobs.locks().acquire("build", lock_options).await?;
jobs.kv().inc("attempts", 1).await?;
lease.release().await?;
match bus.call("#builder@team", "build", call_options).await? {
    CallOutcome::Reply(message) => consume_answer(message),
    CallOutcome::Done(receipt) => note_completion(receipt),
}
bus.serve(handler, serve_options, cancellation).await?;
bus.close().await?;
```

## Surface

Methods doing network I/O are async and return `Result<T, BusError>`.
These are library type names and signatures, not new protocol definitions.

| Type | Proposed operations |
|---|---|
| `Bus` | `identity()`, `status()`, `records(filters)`, `record(name)`, `own_record()`, `close()` |
| `Record` | `lookup()`, `register(metadata)`, `manage(changes)`, `unregister()` |
| Messaging | `send(to, body, options)`, `call(to, body, options)`, `consume(options)` |
| Responses and channels | `reply(&message, body)`, `ack(&message)`, `done(&message)`, `publish(channel, body, options)`, `unsubscribe(channel)` |
| Serving | `serve(handler, options, cancellation)`; handler is `Fn(Message) -> Future<Output = Result<HandlerReply, HandlerError>>` |
| Private reads | `config() -> Option<Arc<Value>>`, `secret() -> Arc<Secret>`; `invalidate_private(selection)` on Bus or Record |
| `RecordKv` | `get(key, kind)`, `set(key, value, mode)`, `delete(key, kind)`, `list()`, `inc(key, i64)`, `json(key, operations)` |
| `RecordLocks` | `acquire(name, options)`, `try_acquire(name, ttl)`, `extend(name, ttl)`, `release(name)`, `force_release(name)`, `holders()` |
| `Lease` | Explicit `extend(ttl).await`, `release().await`; not Clone and not an asynchronous destructor |
| `Message` | Read-only envelope accessors, including roles and original destination; no setter for roles |
| `Secret` | Redacted Debug/Display; explicit access to retained bytes, no environment injection |

`call` returns `CallOutcome::Reply(Message)` or `Done(Message)`. Ack is a progress
notification through an optional observer, never a successful reply. Reply body
stays text. Deadlines yield `BusError::Timeout`; dropping a call future stops its
wait, not remote execution. Remove its pending entry without leaking a waiter.
A done receipt's `re` names the original message; it confirms completion without
an answer and does not identify whether a script or an explicit handler sent it.

## Tasks, cancellation and serving

Bus/Record handles are `Clone + Send + Sync`; a clone does not acquire another
inbox reader. One dispatcher owns polling and pending-call routing. Direct
`consume` is exclusive with serving and pending calls and fails locally on conflict.
Long polls have their own transport capacity; a task awaiting config or a reply
must not compete for the single connection occupied by its inbox poll.

`ServeOptions` bounds concurrent tasks, default one. Reserve capacity before an
unfiltered work read. At capacity, continue draining exact pending-call filters
fairly so in-flight handlers can finish their own calls; do not prefetch other
jobs into an unbounded channel. The exact scheduler is an implementation review
item, not a new daemon filter or extra consumer.

`HandlerReply::Body(text)` sends an answer; `Done` sends a completion receipt.
Automatic ack precedes the handler; receipts never trigger receipt loops. A
handler error reports locally and sends no success receipt or fabricated answer.
A handler uses one completion path; explicit manual replies require a separate
manual-completion mode, not an additional automatic send.

Cancellation stops intake and waits for in-flight tasks to the drain deadline.
Report unfinished work; already consumed messages are not requeued. Avoid aborting
a handler during a remote write and calling that write rolled back. CPU/blocking
work belongs outside async executor threads; [spawn_blocking](https://docs.rs/tokio/latest/tokio/task/fn.spawn_blocking.html)
is not generally abortable once started, so shutdown must account for it.

## Lazy reads and ownership

Use async lazy initialization per property; no synchronous mutex is held across
an await. [Tokio OnceCell](https://docs.rs/tokio/latest/tokio/sync/struct.OnceCell.html)
can share a successful initial read; failed/cancelled attempts are not cached.
Invalidate by replacing the shared cache generation, so an old in-flight load
cannot populate the refreshed generation. Existing returned Arc values remain
owned by callers and do not disappear on access revocation.

Config is `Option<Arc<Value>>`: absent config is the daemon's successful null,
not a refusal. Secret absence remains `Refusal` under the current API. Returned
configuration is read-only shared ownership; users clone its Value before editing.
Secret Debug does not expose bytes. No config or secret setters in this version.
Refresh policy beyond explicit invalidation remains an owner discussion.

## Storage, errors and scope

KV string values are bytes, with a fallible UTF-8 convenience; integer values use
`i64`. Stored JSON is an object. Set modes retain `set`, `add`, `replace`, and
atomic JSON operations retain `set`, `unset`, `inc`, `push`, `unshift`, `shift`,
`pop`, `add_to_set`, `remove_from_set`. List preserves the kind namespaces.

Any lock refusal, including try-acquire, is an error rather than an inferred
`None`. Lease helpers have no fencing token; a stale lease cannot be proven to
identify a later acquisition by the same principal. `Drop` does not send a release:
explicit release or TTL expiry ends it. A cancelled lease acquisition may have
succeeded remotely; do not retry it blindly.
Dropping a Lease without release leaves the hold until expiry.

`BusError` separates local validation, identity, reader conflict, transport,
timeout/cancellation and `Refusal { status, message }`. Preserve the HTTP refusal;
current wire text cannot safely identify every internal daemon reason.
Retry safety depends on the operation and possible remote success, not just
whether the error is a transport failure or an HTTP refusal.

First scope is registry, messaging, channels, serving, private reads, KV and locks.
Minting/enrolment and user/group/account administration, activity/debug and resource
content adapters remain outside this proposal. Close cancels client activity;
record unregistration stays explicit. Runtime/MSRV and serialization dependencies
are proposals to settle before implementation, not version commitments.
