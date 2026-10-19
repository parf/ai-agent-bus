# TypeScript client

Keep secrets in Secret and config in Configuration; see [configuration](../../../docs/03-records-agent.md#configuring-a-template).

Node 22+ and Bun 1.3+ ESM client. All network I/O uses Node's built-in HTTP/HTTPS
interfaces, including Unix sockets. Browsers are unsupported.

```ts
import { Bus } from '@agent-bus/client';
const bus = Bus.connect({ expectName: '#worker@team' });
const config = await bus.record('#worker@team').config();
await bus.serve(async message => `hello ${message.body}`, { workers: 4 });
await bus.close();
```

`Bus.connect()` does no I/O. The first authenticated operation verifies
`/status.you` against `expectName` when supplied. `record(name)` selects a target,
never a different caller. Addresses/tokens and certificate pins default from the
same `AGENT_BUS_*` variables as the CLI. An HTTPS pin is SHA256 of the actual
peer certificate; it is checked before any HTTP credential is sent. Without a
pin, HTTPS uses normal certificate validation.

`call()` returns `answer`, `done`, or `timeout` (with `acked`); cancellation throws
`AbortError`. Empty text is an answer. Calls use one demand-driven reader, with
unique tags and filtered reply polls even when handlers fill worker capacity.
Raw `consume()` refuses while calls or serving are active. At-most-once consumes
and uncertain writes are never retried. Network failures after a destructive
request leaves raise `UncertainOutcome`, distinct from daemon `BusError(status)`.

Serving skips a request whose caller deadline is already past before ack or
handling; the Go zero timestamp means no deadline. Serving sends ack before handling. A string return replies; null sends done;
throwing logs only the message ID and error type, and sends no terminal response.
Manual reply/done prevents an automatic duplicate. `stop()` ends intake and
`serve()` drains to `drainTimeout` (milliseconds), raising `DrainTimeout` for
unfinished work. JavaScript cannot forcibly stop a handler; its promise may
continue after drain expiry. `close()` closes connections and refuses all later
operations; it does not unregister the record.

Config and secrets are uncached async methods. Secret wrappers redact string,
JSON and inspect output; `reveal()` explicitly returns a copy of the bytes.
A caller can log revealed bytes; redaction is not a security boundary. Received
message envelopes, roles and nested metadata are frozen. Late answers carrying
`re` or this Bus's private call-tag prefix do not become handler requests.
TypeScript-only `serve(handler, {onReply})` opts into unmatched non-receipt `re` messages;
its callback shares worker/drain limits and sends no automatic ack or answer.
Optional `matchesReply(message)` also selects non-`re` answers by an application’s
stored correlation; false leaves ordinary request handling unchanged.
Like the rest of this client, this option is covered by the [MIT license](LICENSE.md#mit-license).

`record.kv` has typed string/int/JSON operations. Int64 reads return `bigint`,
including values beyond JavaScript's safe integer range. Static JSON operations
are sent atomically; modes are set/add/replace. A 404 `KVNotFound` can mean a
missing value or an unseen record. `record.locks` exposes acquire, tryAcquire,
extend, release, forceRelease and holders. There is no fencing token: stale
same-caller releases can release a newer acquisition. Cancellation of a waiting
acquire may race a grant; check holders or let its TTL expire.

Build an installable local package with `node build.mjs` (Bun and TypeScript are
build dependencies). `dist/package.json` takes its version from the repository's
canonical VERSION; the source package's private 0.0.0 is not a release version.
The bundle includes lossless-json under its supplied license; runtime users need
no separate dependency. The package includes ESM and TypeScript declarations.

Tests: build `src/clients/fixture` first, then run `tests/check.sh`. It runs strict
TypeScript checks, Bun's real-daemon/network suite, builds the package and runs a
Node HTTP/Unix/HTTPS integration smoke. `AB_CLIENT_FIXTURE` selects the isolated
fixture; `AB_TYPESCRIPT_CLIENT_PATH` selects scratch sources for mutation runs.
