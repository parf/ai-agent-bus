# PHP client

Proposal by Claude, 2026-09-30, for review; nothing here is owner-approved
beyond the [shared baseline](../client-libraries.md#scope). Same meaning as
every other language; the shape follows PHP's process model: a short request
under FPM, or a long-running CLI worker, single-threaded either way.

## Runtime model

| Context | What the library assumes |
|---|---|
| FPM / web request | one short call or two: send, a `wait: 0` consume, a KV or lock op. No serving, no long poll — a 60 s wait outlives `request_terminate_timeout` |
| CLI worker | `serve()` blocks the process; parallelism is **more processes**, run by systemd, supervisord or `agent-bus start`, never threads |
| Async | none in core. Every call blocks, which is ordinary PHP; a Fiber/Revolt adapter (AMPHP, ReactPHP) may come later over the same transport |
| Transport | ext-curl: `CURLOPT_UNIX_SOCKET_PATH` for a socket, HTTP/HTTPS otherwise, one handle reused per client. PHP ≥ 8.1, 64-bit only (KV ints are int64) |

## Identity and handles

`Bus` is **the caller**: who the credential says, never a name argument. A
`Record` is **a target** the caller acts on.

- `Bus::connect(?string $addr, ?string $token, ?string $as)` takes the
  CLI's defaults: `AGENT_BUS_ADDR`, `AGENT_BUS_TOKEN` (sent as
  `X-Agent-Bus-Token`) and `AGENT_BUS_NAME`. With `$as`, the first call checks
  `/status`'s answer and throws `IdentityMismatch` when it differs — no
  silent acting as someone else.
- `$bus->me()` is the caller's own `Record`; `$bus->record($name)` any other.
  `kv`, `locks`, `config()` and `secret()` live on `Record`, so reading
  another record's config is visible in the code.

## Outline

```php
final class Bus {
  static function connect(?string $addr = null, ?string $token = null, ?string $as = null): Bus;
  function name(): string;  function status(): array;  function identity(): array;
  function me(): Record;    function record(string $name): Record;
  function list(array $filters = []): array;                 // GET /ls
  function register(array $record): Record;                  // as the caller
  function send(string $to, string $body, SendOptions $o = new SendOptions): string; // message_id
  function consume(ConsumeOptions $o = new ConsumeOptions): ?Message;               // null: nothing waiting
  function call(string $to, string $body, CallOptions $o): Outcome;
  function reply(Message $m, string $body): string;
  function ack(Message $m): void;  function done(Message $m): void;
  function publish(string $channel, string $body, SendOptions $o = new SendOptions): string;
  function unsubscribe(string $channel): void;               // the caller, itself
  function serve(callable $handler, ServeOptions $o = new ServeOptions): void;
  function stop(): void;  function close(): void;
}
final class Record {
  readonly string $name;  readonly Kv $kv;  readonly Locks $locks;
  function lookup(): array;  function manage(array $changes): array;  function unregister(): void;
  function config(): mixed;  function secret(): Secret;  function refresh(): void;
}
final class Message { readonly string $id, $from, $to, $body; readonly ?string $topic, $tag, $originalTo;
                      readonly array $roles; readonly ?string $receipt, $re; }
```

`SendOptions`: `topic`, `tag`, `ttl`, `wait`, `replyTo` — the envelope's own
fields, nothing new. `roles` is never a send option.

## Private values

Reads only. `config()` and `secret()` are **methods**, lazy and blocking: the
first call fetches, a success is cached on that `Record`, a failure throws
and is not cached. PHP has no threads, so there is no mutex; a PHP 8.4
property hook (`$record->config`) is a possible later spelling.

- `config()` decodes with `JSON_THROW_ON_ERROR | JSON_BIGINT_AS_STRING`,
  objects as `stdClass`, so `{}` and `[]` survive a round trip.
- `secret()` is a `Secret`: `bytes()`, `env()` (the env-file parsed), and
  `__toString`, `__debugInfo` and `var_export` all redacted. Never put in
  `$_ENV` or `putenv()`.
- **Cache scope is the object.** Under FPM a `Record` dies with the request,
  so each request reads fresh and a rotated secret or a removed Maintainer
  is seen next request. A static or APCu cache across requests is the
  application's choice, never the library's: shared memory would put a
  secret where every pool worker reads it. In a CLI worker the cache lives
  as long as the process; `refresh()` drops it, and revocation never reaches
  a value already held.
- What the daemon answers today limits what can be told apart: an absent
  config is `null`, but an absent secret and an unseen record are both 404,
  and a kind with no private values is a 400 like any malformed call. The
  library says no more than that ([errors](#errors)).

## Calls, receipts and the inbox

- `send` returns once the bus accepted; that says nothing about reading.
- `call` sends with a fresh tag and waits on a filtered consume for that
  topic and tag. Its `Outcome` says which came: `Answer`, `Done`, or
  `TimedOut` with the `ack`s seen. A timeout never cancels the remote work.
- **One reader per inbox per process.** A process is single-threaded, so
  `call` inside a handler is safe only when no sibling process reads the
  same inbox: N workers serving one name would take each other's replies. The
  proposal: a worker that calls passes `replyTo` naming an inbox only it
  reads — its own agent name — or the pool runs as distinct names.
- No retry of an uncertain write: a send whose answer was lost throws
  `UncertainOutcome`, never repeated silently.

## Serving

`serve($handler, new ServeOptions(topic: ..., maxMessages: ..., idle: ...))`:
consume, `ack`, call the handler, then reply with a returned string or `done`
on `null`. A throwing handler is logged and sends no `done`; whether an error
reply exists stays open. One message at a time — no prefetch, because a taken
message is lost if the process dies. `SIGTERM`/`SIGINT` (pcntl) finish the
current message and return; `maxMessages` lets a supervisor recycle a worker
that leaks memory.

## KV and locks

- `$r->kv->get($name, Kind::Int)`, `set($name, $value, Kind::Json, How::Add)`
  — `How` is `Set`, `Add` or `Replace`, the daemon's `how`; `delete`,
  `inc($name, $n = 1)`, `json($name, Op::push('x'), Op::shift(), ...)` with
  the daemon's op names, all or nothing; `list()` by kind. `get` throws
  `NotFound` for an absent name rather than returning `null`: the daemon's 404
  is the same for an unseen record, so `null` would hide a mistyped name. A string that is
  not UTF-8 goes as `value_base64`; PHP strings are bytes, so nothing is lost.
- `$lease = $r->locks->acquire('build', ttl: 30, wait: 10)` returns a `Lease`
  (`extend()`, `release()`, `expires()`); `tryAcquire` returns `?Lease`;
  `holders()`; `forceRelease($name)` for whoever may use the record's locks, answering the displaced holder. No background
  renewal — PHP has no thread for it; extend between steps.
- Release in `finally`. `Lease::__destruct` only logs a lease left unreleased.
  It never releases: a destructor does not run on a fatal error or a kill, and
  one that runs late could release somebody's later hold.
- **A `Lease` is not a fence.** The daemon has no lease or fencing token, and a
  release names only the record, the lock and the caller. A stale `Lease`
  whose hold expired can release a later acquisition by the same name; it
  refuses locally once past its own known expiry, which narrows that window
  but cannot close it.
- **A handler's `call` at capacity:** a worker is one process with one message
  in hand, so its `call` does its own filtered consume on topic and tag while
  no new work is taken. Nothing is paused that it waits on.
- **A lock is held by a name.** Two processes acting as the same name are one
  holder, so the second is refused at once as already holding it rather than
  made to wait. Mutual exclusion between sibling workers needs distinct names;
  this is a question for the owner, not a library workaround.

## Errors

The daemon answers a refusal as an HTTP status and `{"error": text}`; its
internal reason is counted, not sent, and one status covers several reasons.
So the library promises the status, never a parsed reason:

| Exception | When |
|---|---|
| `Refusal` (`status()`, `message()`) | any daemon refusal; subclasses by status only — `Invalid` 400, `Unauthenticated` 401, `Forbidden` 403, `NotFound` 404, `Busy` 409, `Exists` 412, `TooLarge` 413, `Full` 429 (the recipient's inbox) |
| `Transport` | socket or TLS failure before a request was sent |
| `Timeout` | the client's own deadline; a consume or call finding nothing is not one |
| `UncertainOutcome` | a write sent whose answer never came; never retried |
| `IdentityMismatch`, `InvalidArgument` | checked locally, before any request |

Stable reason codes need a protocol change ([Q17](../QUESTIONS.md#open-questions)).
A token's rotation is the credential's, not a record secret's: a rotated
token makes calls `Unauthenticated` and leaves cached values in place.

## Out of scope in this version

Users, groups, accounts, daemon ownership, debug and activity (administration
stays with the CLI and web face); minting tokens (`/token`, `/enrol`);
config and secret writes; resource content, which the MCP face resolves.

## Example

```php
$bus = Bus::connect(as: '#mailer@team');
$cfg = $bus->me()->config();
$bus->serve(function (Message $m) use ($bus, $cfg): ?string {
    $jobs = $bus->record('jobs@team');
    $lease = $jobs->locks->acquire('smtp', ttl: 30, wait: 5);
    try { $n = $jobs->kv->inc('sent'); } finally { $lease->release(); }
    return "sent #$n to {$m->body}";
}, new ServeOptions(topic: 'mail'));
```
