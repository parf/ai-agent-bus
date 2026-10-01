# Client libraries

## Scope

API plans for Go, PHP, Python, Rust, TypeScript and Java; no implementation yet.
Shared meaning, language-specific syntax and execution. The protocol description
remains [Q17](QUESTIONS.md#open-questions); proposed library types are not wire schemas.

## Shared vision

Design baseline for cross-review, not final approval of every signature:

| Area | Shared meaning |
|---|---|
| Identity | A client acts as one credentialed caller. Verify that the credential/socket matches the requested name; selecting a target never changes the caller |
| Records | A named record handle shares the caller's connection; its `kv` and `locks` belong to that record. Self conveniences use the caller's own record |
| Registry | List, lookup, register, manage and unregister explicitly; constructing a handle makes no registry write |
| Messaging | Send, consume, call, reply, ack and done; preserve correlation and reply routing. Roles are received metadata, never a send option |
| Channels | Publish to a named channel; unsubscribe removes the caller from that channel, not an arbitrary actor |
| Private values | Read config and secrets under the existing [access contract](../../docs/constitution.md#-private-values); no setters this version |
| Storage | Typed KV with atomic operations and modes; per-record locks with TTL, extension, release, force release and holders |
| Serving | `serve(handler, options)` stays; bounded work, receipts, failure behavior and graceful stop are explicit |
| Concurrency | One dispatcher owns the inbox and matches replies to calls. Long polls do not monopolize the connections used by handlers or private reads |
| Outcomes | An ack is progress, not an answer; completion without an answer differs from an answer and a timeout |
| Failure | Deadlines and cancellation stop local waiting; uncertain writes are not automatically repeated |
| Cache | Successful private reads may be cached. Failed reads are not cached; refresh and retained copies after revocation must be explained |
| Lifecycle | Closing a client does not unregister a record automatically; registration and removal are separate operations |

The [daemon API](../../docs/09-daemon-api.md#how-a-call-is-made) supplies the
operations. Resource content reads remain a separate adapter: the [MCP face](../../docs/03-records-resource.md#what-a-resource-is)
resolves content; the daemon holds cards. Authors must state credential and
administrative scope rather than silently exposing every daemon operation.
`identity()` reads the node's identity; `status().you` identifies the authenticated
caller. These are separate operations in every language.

Current refusals expose HTTP status and error text, not the daemon's counted
reason code. Do not manufacture stable error kinds by parsing prose; retain the
original refusal and distinguish local validation, transport and deadline failures.

## Lazy private values

Python's accepted blocking-property behavior is owned by
[Python private values](client-libraries/python.md#private-values). Other
languages may use getters or async methods appropriate to their runtime.

## Language plans

Each author edits only their two files. Reviewers report findings to the author;
authors revise their own plans. The overview stays with Codex.

| Language | Author | Cross-reviewer | Plan |
|---|---|---|---|
| Python | Codex | ag-bus | [Python](client-libraries/python.md) |
| Rust | Codex | ag-bus | [Rust](client-libraries/rust.md) |
| PHP | Claude | Codex | [PHP](client-libraries/php.md) |
| Java | Claude | Codex | [Java](client-libraries/java.md) |
| TypeScript | ag-bus | Claude | [TypeScript](client-libraries/typescript.md) |
| Go | ag-bus | Claude | [Go](client-libraries/go.md) |

## Review sequence

1. Each author commits both API plans, checking current daemon behavior and language conventions.
2. Cross-review the committed plans against the shared vision and language constraints.
3. Authors resolve concrete inconsistencies; owner choices stay open, not silently settled by a reviewer.
4. Present the six plans and remaining choices before implementation.
