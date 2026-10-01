# Java client

Proposal by Claude, 2026-09-30, for review; nothing here is owner-approved
beyond the [shared baseline](../client-libraries.md#scope). Same meaning as
every other language; the shape follows Java's: typed values, blocking calls
that virtual threads make cheap, and explicit lifecycles through
`AutoCloseable`.

## Runtime model

| Choice | Proposal |
|---|---|
| Baseline | Java 21: records, sealed interfaces, virtual threads |
| Sync or async | **blocking API only.** A blocked virtual thread costs little, so a long poll or a lock wait needs no callback style. A `CompletableFuture` facade can wrap it later; it is not the core |
| Threads | one `Bus` is thread-safe and meant to be shared. It owns one inbox reader thread and a bounded handler executor, virtual by default |
| Transport | `java.net.http.HttpClient` for HTTP/HTTPS. It cannot reach a Unix socket, so the socket path is a small HTTP/1.1 client over `SocketChannel` with `UnixDomainSocketAddress` (JDK 16+), one connection per in-flight call. That way a 60 s long poll never holds the connection a `config()` read needs. No third-party dependency is the aim ([external tools](../../../src/MODULES.md#external-tools)) |
| JSON | the library needs a parser for envelopes. It hides one behind an interface, with an optional Jackson module; user-facing JSON is a `String` or a type the caller binds |

## Identity and handles

`Bus` is **the caller**, as the credential says; a `Record` is **a target**.

- `Bus.connect(Options)`: `addr`, `token` (sent as `X-Agent-Bus-Token`),
  `expectName`, `timeout`. The defaults are the CLI's `AGENT_BUS_ADDR`,
  `AGENT_BUS_TOKEN` and `AGENT_BUS_NAME`. With `expectName`, connect checks
  `/status`'s answer and throws `IdentityMismatchException` when it differs.
- `bus.self()` is the caller's own `Record`; `bus.record(name)` is any other.
  `kv()`, `locks()`, `config()` and `secret()` live on `Record`.

## Outline

```java
public final class Bus implements AutoCloseable {
  static Bus connect(Options o);
  String name();  Status status();  Identity identity();
  Record self();  Record record(String name);  List<RecordView> list(Filters f);
  RecordView register(RecordSpec spec);
  MessageId send(String to, String body, Send o);         // accepted, not read
  Optional<Message> consume(Consume o);                   // throws IllegalStateException while serving
  Outcome call(String to, String body, Call o);           // blocks the calling thread
  MessageId reply(Message m, String body);  void ack(Message m);  void done(Message m);
  MessageId publish(String channel, String body, Send o);  void unsubscribe(String channel);
  Server serve(Handler h, Serve o);
  void close();
}
public final class Record {
  String name();  KV kv();  Locks locks();
  RecordView lookup();  RecordView manage(Changes c);  void unregister();
  Optional<String> config();  <T> Optional<T> config(Class<T> type);
  Secret secret();  void refresh();
}
public record Message(String id, String from, String to, Optional<String> topic, Optional<String> tag,
                      String body, Instant at, Optional<Receipt> receipt, Optional<String> re,
                      Optional<ReplyTo> replyTo, List<String> roles, Optional<String> originalTo,
                      int forwards, Optional<Instant> expires, Optional<Instant> deadline) {
  public Message { roles = List.copyOf(roles); }   // the received envelope, nothing dropped
}
public sealed interface Outcome permits Answer, Done, TimedOut {}  // TimedOut carries the acks seen
```

`Send` is a builder over the envelope's own fields: `topic`, `tag`, `ttl`,
`wait` and `replyTo`. `roles` is never settable.

## Private values

Reads only, lazy, blocking, cached per `Record`. One lock **per value**, so
config and secret never share one. The lock is held only while fetching:
concurrent first readers share that one fetch, later readers take the cached
value without blocking, and a failure is thrown to every waiter and not
cached. The fetch is bounded by `timeout`, so a hung daemon wedges no thread.

- **The cache holds bytes; every access gets its own value.** `config()`
  returns the stored JSON text. `config(Class<T>)` binds a **fresh** `T` on
  each call, because a record is shallow and `T` may be a mutable POJO or a map.
- `secret()` returns a new `Secret` each time, holding its own copy of the
  cached bytes. Its `bytes()` returns a copy, `env()` an unmodifiable map, and
  `toString()` gives `Secret[redacted]`. `close()` zeroes only that instance,
  so it never reaches another thread's copy or the cache. It is never put in
  `System.getProperties()` or a child's environment.
- Waiting for the per-value lock is bounded by `timeout` too, not only the
  fetch.
- `refresh()` drops both and bumps a generation number. A fetch that began
  before it finds the generation moved and does not repopulate the cache.
- A cached value outlives a rotated secret and a removed Maintainer until refreshed: it is a client-side copy, and
  revocation never reaches it. A token's rotation is the credential's, not a
  record secret's.

## The inbox dispatcher

One reader thread per `Bus` consumes the caller's inbox; nothing else does.

- `call` registers its fresh `(topic, tag)` before sending. The reader hands
  matching replies and receipts to that call and new work to `serve`'s
  handlers.
- **Demand decides what is read.** With only calls pending, the reader polls
  only their filters, so no unrelated request is taken into a local queue. An
  unfiltered read happens only for a running `serve` with a free permit, or
  for a `consume()` the user is blocked in. `consume()` while `serve` runs
  throws.
- The `Call` deadline ends a wait as `TimedOut`. `Thread.interrupt()` is
  cancellation: `call` throws `CancellationException` and the thread's
  interrupt flag stays set. Neither stops the remote work.
- **Other processes reading the same inbox defeat this.** Two JVMs serving
  one name take each other's replies. The proposal: a caller that must share
  an inbox passes `replyTo` naming an inbox **only it reads and may consume**.
  That is an inbox the **current caller** may consume under its ACL — a 📮
  queue whose allow list admits it — or a separate `Bus` authenticated as that
  other name. Holding another name's credential does not let this `Bus` read
  as it: its requests still carry its own credential. The dispatcher then
  polls the selected reply inbox, filtered by the call's topic and tag. An
  address alone grants no access, and selecting an inbox never changes who
  the caller is.
- No retry of an uncertain write: a send whose answer was lost throws
  `UncertainOutcomeException`, and is never repeated silently.

## Serving

`Server s = bus.serve((msg, ctx) -> ..., Serve.workers(16).topic("jobs"))`

- **Bounded and never prefetched:** the reader takes a permit before each
  *unfiltered* consume, so with 16 handlers busy no new work is taken from the
  daemon. A taken message is lost if the process dies.
- **Replies still flow at capacity.** Busy handlers may be blocked in their own
  `call()`s, so pausing every poll would deadlock them. While the pool is full
  the reader keeps polling **filtered** by each pending call's topic and tag.
  A consume takes one filter, not an OR of them, so it goes round the pending
  calls with short waits. The exact scheduler (wait lengths, fairness against
  many pending calls) is open. The one reader stays the only one: no extra
  competing readers.
- The handler returns a `Response`: `reply(body)`, `done()` or `none()`. The
  library has already sent the `ack`. A thrown exception is logged and sends
  no `done`; whether an error reply exists stays open.
- `ctx` holds the message's `roles`, the caller's absolute `deadline` as
  received (never reset from `wait` at receipt) and a cancellation flag set on
  shutdown.
- `s.close()` stops consuming, waits up to `Serve.drain(Duration)` for
  running handlers, then interrupts them. `bus.close()` closes its servers
  first.

## KV and locks

- `kv().getInt(name)`, `getString`, `getBytes`, `getJson` throw `NotFound`
  for an absent name. The daemon answers it with the same 404 as an unseen
  record, so an empty `Optional` would turn a mistyped record into "no value".
  `set(name, value, How.SET | ADD | REPLACE)` takes the daemon's
  `how`, overloaded by type: `long` is int64, `String` and `byte[]` are
  strings (non-UTF-8 sent as `value_base64`), and `Json` wraps JSON text.
- The rest of KV: `delete`, `inc(name, n)` and `json(name, Op... ops)` with
  the daemon's op names, all or nothing. `list()` lists by kind.
- `locks().acquire(name, ttl, wait)` returns a `Lease` implementing
  `AutoCloseable`, for try-with-resources. `tryAcquire` is the same with no
  wait, and a refusal propagates as `Busy`. A held lock and the caller's own
  hold are both 409, and the library does not pretend to tell them apart. It
  never collapses a 409 into an empty answer. Also `holders()`, and
  `forceRelease(name)`, which answers the displaced holder.
- **A `Lease` is a convenience, not a fence.** The daemon has no lease or
  fencing token: a release names only the record, the lock and the caller. A
  stale `Lease` whose hold expired can therefore release a later acquisition
  by the same name. The `Lease` refuses locally once its own known expiry has
  passed, which narrows that window but cannot close it. Fencing needs a
  protocol change.
- A `Lease` has `extend(ttl)`, `expires()` and `close()` (release). Opt-in
  renewal, `Lease.keepAlive(period)` on a shared scheduler, is proposed; it
  is off by default, so a stuck holder still expires.
- **A lock is held by a name, not by a thread.** Two threads of one `Bus` are
  one holder. The daemon refuses the second at once, so exclusion holds, but
  nobody waits. The library therefore queues same-`Bus` acquires of one
  `(record, lock)` behind an in-process fair lock before asking the daemon.
  Across JVMs acting as one name, the refusal still excludes. Distinct names
  are what give each process its own waiting and its own clean-up: as one
  name, any of them can release or extend the others' hold.

## Errors

All exceptions are unchecked and extend `BusException`. A refusal is
`RefusalException(status(), message())`: the daemon sends an HTTP status and
`{"error": text}`, and one status covers several reasons. So subclasses go
by status only:

| Status | Exception |
|---|---|
| 400 | `Invalid` |
| 401 | `Unauthenticated` |
| 403 | `Forbidden` |
| 404 | `NotFound`: unknown, unseen, no secret, KV absent or lock not held, told apart only by text |
| 409 | `Busy` |
| 412 | `Exists` |
| 413 | `TooLarge` |
| 429 | `Full` |

The library also has its own `TransportException`, `TimeoutException`,
`UncertainOutcomeException` and `IdentityMismatchException`. Stable reason
codes need a protocol change ([Q17](../QUESTIONS.md#open-questions)).

## Out of scope in this version

- Administration (users, groups, accounts, daemon ownership, debug and
  activity) stays with the CLI and the web face.
- Minting tokens (`/token`, `/enrol`).
- Writing config and secrets.
- Resource content, which the MCP face resolves.

## Example

```java
try (Bus bus = Bus.connect(Options.env().expectName("#indexer@team"))) {
  Record jobs = bus.record("jobs@team");
  try (Server s = bus.serve((m, ctx) -> {
        try (Lease l = jobs.locks().acquire("shard-" + m.body(), Duration.ofSeconds(30), Duration.ofSeconds(5))) {
          long n = jobs.kv().inc("indexed", 1);
          return Response.reply("indexed " + n);
        }
      }, Serve.workers(16).topic("index"))) {
    s.awaitTermination();
  }
}
```
