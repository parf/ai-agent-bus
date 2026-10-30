# TypeScript client

📌 **TL;DR:**
- **Problem:** an async service that should take part on the bus has to handle HTTP, call correlation and uncertain writes itself.
- **Outcome:** `Bus` gives you send, call, serve, key-value, locks and read-only settings with bounded async handlers.
- **Start:** copy [the example](#try-this-first); it needs a name, a token and a running bus.

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

## Why you'd want it

- ⚡ **Async and typed.** Promises for every call, with type declarations in the package.
- 🔑 **Credentials stay out of your code.** Addresses, tokens and certificate pins default from the same `AGENT_BUS_*` variables as the CLI.
- 🛡️ **No silent repeats.** An uncertain write or consumption is never retried.

## What you get

- ✅ **Call and serve:** `call()` returns `answer`, `done` or `timeout` (with `acked`); `serve(handler, { workers })` answers with a bounded pool.
- ✅ **Targets:** `bus.record(name)` selects a record and exposes `kv` and `locks`.
- ✅ **Your own name works:** a call to yourself is answered by your own handler.
- ✅ **Safe secrets:** secret wrappers redact string, JSON and inspect output.

## Try this first

Serve one handler and call it from the same process (a disposable bus, token in `AGENT_BUS_TOKEN`):

```ts
import { Bus } from '@agent-bus/client';

const bus = Bus.connect({ expectName: '#worker@localhost' });
const server = bus.serve(async message => `pong ${message.body}`, { workers: 2 });
const outcome = await bus.call('#worker@localhost', 'hello', { timeout: 5000 });
if (outcome.kind === 'answer') console.log(outcome.message.body);
console.log(await bus.record('#worker@localhost').kv.increment('attempts'));
bus.stop();
await server;
await bus.close();
```

```text
pong hello
1n
```

- ▶️ The [shared examples](../examples/README.md) run agents in every language together.

## Technical details

- **Runtime:** an ESM client for Node 22+ and Bun 1.3+. All network I/O uses Node's built-in HTTP and HTTPS interfaces, including Unix sockets. Browsers are unsupported.
- **Connecting:** `Bus.connect()` does no I/O. The first authenticated operation verifies `/status.you` against `expectName` when supplied, and `record(name)` selects a target, never a different caller.
- **TLS:** an HTTPS pin is the SHA256 of the actual peer certificate, checked before any HTTP credential is sent. Without a pin, HTTPS uses normal certificate validation.
- **Calls:** cancellation throws `AbortError`, and empty text is an answer. Calls use one demand-driven reader with unique tags and filtered reply polls, even when handlers fill worker capacity. Raw `consume()` refuses while calls or serving are active.
- **Uncertainty:** at-most-once consumes and uncertain writes are never retried. A network failure after a destructive request leaves raises `UncertainOutcome`, distinct from the daemon's `BusError(status)`.
- **Serve:** it skips a request whose caller deadline is already past before ack or handling (the Go zero timestamp means no deadline) and sends the ack before handling. A string return replies, `null` sends done, and a throw logs only the message ID and error type and sends no terminal response. A manual reply or done prevents an automatic duplicate.
- **Stop:** `stop()` ends intake and `serve()` drains to `drainTimeout` (milliseconds), raising `DrainTimeout` for unfinished work. JavaScript cannot forcibly stop a handler, so its promise may continue after the drain. `close()` closes connections and refuses all later operations; it does not unregister the record.
- **Private values:** config and secrets are uncached async methods. `reveal()` explicitly returns a copy of the bytes, and bearer credentials are also omitted from client JSON and diagnostic inspection. A caller can log revealed bytes, so redaction is not a security boundary.
- **Messages:** received envelopes, roles and nested metadata are frozen. Late answers carrying `re` or this Bus's private call-tag prefix do not become handler requests.
- **TypeScript-only options:** `serve(handler, { onReply })` opts into unmatched non-receipt `re` messages; its callback shares worker and drain limits and sends no automatic ack or answer. `matchesReply(message)` also selects non-`re` answers by an application's stored correlation, and false leaves ordinary request handling unchanged. Classification runs within the serving worker budget, a slow classifier does not block pending call answers, and a thrown one is reported through `onError`. These options are covered by the [MIT license](LICENSE.md#mit-license) like the rest of the client.
- **Key-value:** `record.kv` has typed string, int and JSON operations. Int64 reads return `bigint`, including values beyond the safe integer range. Static JSON operations are sent atomically, and the modes are set, add and replace. A 404 `KVNotFound` can mean a missing value or an unseen record.
- **Locks:** `record.locks` exposes acquire, tryAcquire, extend, release, forceRelease and holders. There is no fencing token, so a stale same-caller release can release a newer acquisition, and cancelling a waiting acquire may race a grant; check holders or let its TTL expire.
- **Self-calls:** they use the same serving handler and return its answer, never the unmarked request. Already-taken self requests wait for free worker capacity and join graceful drain, including an early send response and alternate reply inboxes. A handler awaiting a call to itself needs another free worker. Legacy unmarked answers with a different message ID remain supported.

## Build and checks

- Build an installable local package with `node build.mjs` (Bun and TypeScript are build dependencies). `dist/package.json` takes its version from the repository's canonical VERSION; the source package's private 0.0.0 is not a release version.
- The bundle includes lossless-json under its supplied license, so runtime users need no separate dependency. The package includes ESM and TypeScript declarations.
- Tests: build `src/clients/fixture` first, then run `tests/check.sh`. It runs strict TypeScript checks, Bun's real-daemon and network suite, builds the package and runs a Node HTTP, Unix and HTTPS integration smoke.
- `AB_CLIENT_FIXTURE` selects the isolated fixture, and `AB_TYPESCRIPT_CLIENT_PATH` selects scratch sources for mutation runs.
- The [shared protocol examples](../../testdata/client-conformance/README.md) run in the native TypeScript adapter as well. `AB_CLIENT_CONFORMANCE_CASES` selects another case directory; the default is `src/testdata/client-conformance` in this checkout.
