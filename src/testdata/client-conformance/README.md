# Shared client protocol cases

These are concrete existing-protocol examples, not a schema or a generator.
Every language adapter reads the same files and uses its native SDK against a
disposable daemon. Fixture names and message text are synthetic.

- `send.json`: send the request fields through the SDK, receive at the named
  worker, and compare every expected envelope field and the accepted message ID.
- `call.json`: the worker's handler returns `answer`; the calling SDK must return
  a reply with the expected envelope fields, including an empty answer. The
  handler must run once and its reply must name the original request in `re`.
- `self-call.json`: run the caller's own handler, call that same name, and compare
  the handler answer. The request body is deliberately different: returning the
  correlated request cannot satisfy this case. Its answer must carry `re`.

Generated IDs, tags and deadlines are checked by adapters through their runtime
relationships rather than copied into fixtures. Existing receipt, authority,
transport and capacity tests remain independent.

## State, identity and refusal vectors (T-24)

Same rule: concrete examples, read by each adapter, run through the SDK's own
calls. `send`, `call` and `self-call` need a worker; these five do not share it.

| File | Needs | One row |
|---|---|---|
| `kv.json` | the real fixture daemon | `id`, optional `fresh`, `steps` |
| `locks.json` | the real fixture daemon | `id`, optional `setup`, `steps` with `as` (`owner` or `worker`) |
| `bigfile.json` | the real fixture daemon; Go, Python and TypeScript only | `id`, `steps` |
| `identity.json` | a fake daemon | `id`, `identity` (the raw `/identity` body), `expect` |
| `refusals.json` | a fake daemon | `id`, `identity` (`current` or `old`), `daemon`, `expect` |

**Steps.** Each step has an `op` and the fields that op names (`kind`, `name`,
`value`, `ttl_s`, ...). An integer that may pass 2^53 is a string of digits.
A large value is `repeat: {text, count}`; bytes are `value_b64`; a JSON value is
a string. `expect` is what a successful step must read back (`value`, `length`,
`json`, `changed`, `holders`, `displaced`, `deleted`, `size`, `sha256`, `held`,
`replaced`, `names`). A refused step says `"refused": {"status": N,
"message_has": "text"}` for a refusal the daemon sent, or `"refused": true` for
one the client or the daemon may send (a TTL or size the client checks first).
A case with `fresh: true` fills a store and gets a daemon of its own; the
adapters give every case a clean daemon where that is cheap. Setup
`worker-maintains-jobs` makes `#worker@fixture` a Maintainer of `jobs@fixture`.

**Identity.** The fake daemon answers `/identity` with the row's raw body and
`{"you":"owner@fixture"}` elsewhere; the adapter calls the client's status
operation. `expect` is `{"ok": true}` (a `warns` text means the client logs that
warning), or `{"fatal": "text"}`: the call fails with the text in its message and
the daemon saw only the `/identity` request.

**Refusals.** The fake daemon answers `GET /ls` with `daemon.status`,
`daemon.body` (raw) and an optional `Allow` header; the adapter calls the
client's record listing. `expect` is the exact `status`, `kind` (empty when the
client keeps none) and `message`: the daemon's `error` text, `internal daemon
error` for a 500, and `daemon too old for GET /ls` when the identity was `old`
and the kind is `no_route`.

Route coverage is not a vector: `internal/api/route_contract_test.go` fails when
a registered route has no row in [the route table](../../../docs/09-daemon-api.md#every-route-and-its-class)
or a stable route is absent from [the client contract](../../../docs/09-daemon-api.md#the-client-contract).

All six adapters run `send.json`, `call.json` and `self-call.json`
(PHP makes its self-call from inside a handler, since it serves one message at
a time). Each client's check changes independently of the others when its
request or its reading of an answer changes. Run the native
[client checks](../../clients/README.md#checks); do not use the live bus.
