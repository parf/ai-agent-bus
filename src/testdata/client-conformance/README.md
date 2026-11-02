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
| `refusals.json` | a fake daemon | `id`, `identity` (`current` or `old`), then `daemon` and `expect`, or `call` and `expect` |

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
and the kind is `no_route`. A 5xx other than 500 keeps the daemon's text; a kind
outside the closed set passes through unchanged.

A row with `call` instead of `daemon` is a call the client must refuse itself
(lock `ttl_s` over 24 h or negative, `wait_s` over 60): the error is not a daemon
refusal, and the fake daemon never sees the `expect.not_requested` path.

## Event logs (T-31a)

`events.json` holds cases for an agent's private event log through the daemon's
own `#bus`, using only the existing [T-29 verbs](../../../Plans/R1.0-Release/agent-events/README.md#r1-min-surface-and-authority).
Each case has `id`, `fresh: true` (its own daemon) and `steps`. A step has `as`,
one of three fixture identities: `owner` (`owner@fixture`, the daemon Owner),
`agent` (`#caller@fixture`, the writer of its own log) or `third`
(`#outsider@fixture`, who manages neither).

| `op` | Does | Reads |
|---|---|---|
| `call` | `call` to `#bus` with `body`; the reply body is JSON, or text starting `error:` for a refused write | `expect`: dotted paths into the reply (`entry.text`, `counts.alert`, `rings.alert.0.text`, `error.code`); `same_answer_as`: the earlier step whose reply bytes this one must equal |
| `send` | a `send` to `#bus` with `body` and no wait | nothing: a write without wait gets no answer |
| `inbox` | one short consume of the caller's own inbox | `expect.empty` |

A reply is a mapping of what the verbs already answer; no field is defined here.

Route coverage is not a vector: `internal/api/route_contract_test.go` fails when
a registered route has no row in [the route table](../../../docs/09-daemon-api.md#every-route-and-its-class)
or a stable route is absent from [the client contract](../../../docs/09-daemon-api.md#the-client-contract).

All six adapters run `send.json`, `call.json` and `self-call.json`
(PHP makes its self-call from inside a handler, since it serves one message at
a time). Each client's check changes independently of the others when its
request or its reading of an answer changes. Run the native
[client checks](../../clients/README.md#checks); do not use the live bus.

## Event subscriptions (T-31d)

`event-subscriptions.json` extends event coverage separately from `events.json`,
so an adapter can adopt subscriptions without skipping or changing the existing
five cases. It uses the same real fixture, with `worker` (`#worker@fixture`)
as the source's temporary Maintainer and `jobs@fixture` as the destination.
The daemon Owner consumes that queue. Nothing is mocked or sent to a live bus.

The existing `call` and `inbox` operations also cover subscriptions:

- A successful `sub add` saves the reply's `id`. `$subscription` in subsequent
  call bodies and expected strings means that saved id; it is adapter
  substitution, not a new daemon field or verb.
- `inbox` optionally names an `inbox` (the queue). An `expect.empty` check waits
  briefly and fails on any message. Otherwise `expect` addresses envelope
  fields and decoded JSON body paths such as `body.text`. An absent optional
  envelope `wait` is read as the empty string. All waits have an upper bound.
- `manage` calls the existing record management API with `name` and `changes`,
  as the specified identity. It must succeed. This changes real Maintainers
  and target admission between writes, not fixture-side fake answers.

The creation case changes only the source or target from an accepted add,
proving a non-manager and an unreachable destination are refused.
The other cases prove one copy for each matching write, level/text filtering, list
counts, deletion and no later copy. A copy comes from `#bus` as a non-waiting
notice: no ack or done is sent for it ([receipts](../../../docs/04-messaging.md#receipts)).
The confirmed write fences its completed fan-out before the inbox is checked.

For the daemon profile, revoking the source right after creation keeps delivery;
revoking the creator's may-send to the target stops it. The target must also
continue admitting `#bus`. This is [Q325](../../../Plans/R1.0-Release/DECISIONS.md),
not a change to the app emitters' per-delivery source checks.
