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

All six adapters run every file: `send.json`, `call.json` and `self-call.json`
(PHP makes its self-call from inside a handler, since it serves one message at
a time). Each client's check changes independently of the others when its
request or its reading of an answer changes. Run the native
[client checks](../../clients/README.md#checks); do not use the live bus.
