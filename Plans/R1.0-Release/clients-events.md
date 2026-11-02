# Client APIs and agent events

📌 **TL;DR:** this is a dated client audit and an unapproved implementation plan,
not a replacement protocol contract. The six libraries already carry ordinary
messages and calls; T-29 event verbs need examples and integration checks first.
Java, PHP and Rust lack bigfile, as the existing capability matrix declares.

## Audit snapshot

**As of `5cf5e946`, 2026-11-02.** Read-only inspection of production paths,
native tests and check wiring against [the client contract](../../docs/09-daemon-api.md#the-client-contract).
A ✅ cites executable test source; it means the capability has those assertions,
not that every protocol case is covered or that the six suites were rerun for
this docs-only audit. Partial distinguishes a coverage gap from missing code.
The links retain file:line evidence for this snapshot; line numbers may drift.

| Feature | Go | Java | PHP | Python | Rust | TypeScript |
|---|---|---|---|---|---|---|
| protocol / protocol_min | ✅ [protocol_test.go:15](../../src/clients/go/protocol_test.go#L15) | ✅ [BusTest.java:1019](../../src/clients/java/src/test/java/agentbus/BusTest.java#L1019) | ✅ [run.php:787](../../src/clients/php/tests/run.php#L787) | ✅ [test_protocol.py:23](../../src/clients/python/tests/test_protocol.py#L23) | ✅ [protocol.rs:51](../../src/clients/rust/tests/protocol.rs#L51) | ✅ [protocol.test.ts:18](../../src/clients/typescript/tests/protocol.test.ts#L18) |
| Refusal kinds, including not_found | ✅ [conformance_state_test.go:407](../../src/clients/go/conformance_state_test.go#L407) | ✅ [BusTest.java:1348](../../src/clients/java/src/test/java/agentbus/BusTest.java#L1348) | ✅ [vectors.php:233](../../src/clients/php/tests/vectors.php#L233) | ✅ [test_conformance_state.py:227](../../src/clients/python/tests/test_conformance_state.py#L227) | ✅ [conformance_state.rs:328](../../src/clients/rust/tests/conformance_state.rs#L328) | ✅ [protocol.test.ts:65](../../src/clients/typescript/tests/protocol.test.ts#L65) |
| KV (typed state vectors) | ✅ [conformance_state_test.go:104](../../src/clients/go/conformance_state_test.go#L104) | ✅ [BusTest.java:1184](../../src/clients/java/src/test/java/agentbus/BusTest.java#L1184) | ✅ [vectors.php:69](../../src/clients/php/tests/vectors.php#L69) | ✅ [test_conformance_state.py:51](../../src/clients/python/tests/test_conformance_state.py#L51) | ✅ [conformance_state.rs:98](../../src/clients/rust/tests/conformance_state.rs#L98) | ✅ [client.test.ts:494](../../src/clients/typescript/tests/client.test.ts#L494) |
| Locks / holders (state vectors) | ✅ [conformance_state_test.go:217](../../src/clients/go/conformance_state_test.go#L217) | ✅ [BusTest.java:1268](../../src/clients/java/src/test/java/agentbus/BusTest.java#L1268) | ✅ [vectors.php:139](../../src/clients/php/tests/vectors.php#L139) | ✅ [test_conformance_state.py:96](../../src/clients/python/tests/test_conformance_state.py#L96) | ✅ [conformance_state.rs:192](../../src/clients/rust/tests/conformance_state.rs#L192) | ✅ [client.test.ts:523](../../src/clients/typescript/tests/client.test.ts#L523) |
| Receipts and return route | ✅ [failure_test.go:167](../../src/clients/go/failure_test.go#L167) | ✅ [BusTest.java:367](../../src/clients/java/src/test/java/agentbus/BusTest.java#L367) | ✅ [run.php:203](../../src/clients/php/tests/run.php#L203) | ✅ [test_client.py:215](../../src/clients/python/tests/test_client.py#L215) | ✅ [conformance.rs:191](../../src/clients/rust/tests/conformance.rs#L191) | ✅ [client.test.ts:110](../../src/clients/typescript/tests/client.test.ts#L110) |
| Timeouts / uncertain sends | ✅ [failure_test.go:91](../../src/clients/go/failure_test.go#L91) | ✅ [BusTest.java:770](../../src/clients/java/src/test/java/agentbus/BusTest.java#L770) | ✅ [run.php:597](../../src/clients/php/tests/run.php#L597) | ✅ [test_client.py:520](../../src/clients/python/tests/test_client.py#L520) | ✅ [transport.rs:196](../../src/clients/rust/tests/transport.rs#L196) | ✅ [client.test.ts:146](../../src/clients/typescript/tests/client.test.ts#L146) |
| RT8 send / call / self-call | ✅ [conformance_test.go:85](../../src/clients/go/conformance_test.go#L85) | ✅ [BusTest.java:488](../../src/clients/java/src/test/java/agentbus/BusTest.java#L488) | ✅ [run.php:255](../../src/clients/php/tests/run.php#L255) | ✅ [test_conformance.py:37](../../src/clients/python/tests/test_conformance.py#L37) | ✅ [conformance.rs:78](../../src/clients/rust/tests/conformance.rs#L78) | ✅ [client.test.ts:320](../../src/clients/typescript/tests/client.test.ts#L320) |
| Removed aliases | ✅ [protocol_test.go:132](../../src/clients/go/protocol_test.go#L132) | partial A — [BusTest.java:1348](../../src/clients/java/src/test/java/agentbus/BusTest.java#L1348) | partial A — [vectors.php:233](../../src/clients/php/tests/vectors.php#L233) | partial A — [test_conformance_state.py:227](../../src/clients/python/tests/test_conformance_state.py#L227) | partial A — [conformance_state.rs:328](../../src/clients/rust/tests/conformance_state.rs#L328) | partial A — [protocol.test.ts:65](../../src/clients/typescript/tests/protocol.test.ts#L65) |
| Bigfile | ✅ [conformance_state_test.go:286](../../src/clients/go/conformance_state_test.go#L286) | missing B — [Record.java:1](../../src/clients/java/src/main/java/agentbus/Record.java#L1) | missing B — [Record.php:1](../../src/clients/php/src/Record.php#L1) | ✅ [test_conformance_state.py:123](../../src/clients/python/tests/test_conformance_state.py#L123) | missing B — [storage.rs:1](../../src/clients/rust/src/storage.rs#L1) | ✅ [client.test.ts:544](../../src/clients/typescript/tests/client.test.ts#L544) |

The protocol tests include absent fields, malformed values, compatible newer
daemons, too-new fatal rejection and older-daemon warnings. Shared refusal
vectors check exact status, kind and message (including `not_found`, `no_route`,
500 and 503); they do not silently recast a missing route as missing data.
Receipts tests also check original-message correlation: [failure_test.go:167](../../src/clients/go/failure_test.go#L167), [BusTest.java:836](../../src/clients/java/src/test/java/agentbus/BusTest.java#L836), [run.php:649](../../src/clients/php/tests/run.php#L649), [test_client.py:379](../../src/clients/python/tests/test_client.py#L379), [client.rs:528](../../src/clients/rust/tests/client.rs#L528), [client.test.ts:135](../../src/clients/typescript/tests/client.test.ts#L135).

### Coverage gaps and fixes

| Gap | One-line fix | AI work size | Release implication |
|---|---|---|---|
| A — Java, PHP, Python, Rust, TypeScript | Add a named removed-route refusal regression to each applicable adapter; existing no_route vectors already check kind handling | 3–5 min per client, 15–25 min total, one B/C worker | Coverage gap, not an observed old-route dependency; can accompany the examples if owner includes it |
| B — Java bigfile | Add native streamed put/get/list/delete with integrity, bounds, early-refusal and no-retry checks, then run shared bigfile vectors | 25–40 min, one B/C worker plus review/gate | New scope before freeze; currently explicitly absent |
| B — PHP bigfile | Same surface with bounded file/stream I/O and native uncertainty errors, plus shared vectors | 25–40 min, one B/C worker plus review/gate | New scope before freeze; currently explicitly absent |
| B — Rust bigfile | Same surface with bounded async streams, cancellation and integrity checks, plus shared vectors | 25–40 min, one B/C worker plus review/gate | New scope before freeze; currently explicitly absent |

Removed-alias search found no production use in any of the six clients of
`/subscriber/remove`, the `subscriber` fallback, the old `/big` routes, or the
retired `services` fields. Go's cited test deliberately calls the removed route
and expects `no_route`; the other five have generic refusal tests but no
explicit removed-route assertion. No client calls the web-only aliases.

### Shared suite wiring

[check.sh:17](../../src/clients/check.sh#L17) builds the private
[Go fixture:31](../../src/clients/fixture/main.go#L31) and dispatches the native
checks. All six read `send`, `call`, `self-call`, `kv`, `locks`, `identity` and
`refusals` from the same [RT8 directory](../../src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors-t-24).
Only Go, Python and TypeScript run `bigfile`, because only those expose it.
The rows above cite send/call and state adapters; identity/refusal adapters are
at the refusal evidence links and their adjacent identity tests. This is
applicable-vector coverage, not a claim that missing bigfile is implemented.

Java/PHP map fixture names into their private daemon and run native code;
identity/refusal vectors use fake servers deliberately. The fixture does not
substitute HTTP calls for the SDK under test. Native check entry points are
[Java run.sh:34](../../src/clients/java/tests/run.sh#L34),
[PHP run.php:861](../../src/clients/php/tests/run.php#L861),
[TypeScript check.sh:11](../../src/clients/typescript/tests/check.sh#L11), and
`check.sh` for Go/Python/Rust.

### Related documented limits

The [capability matrix](../../docs/09-daemon-api.md#client-capability-matrix)
already distinguishes absent APIs: may-send exists only in Python/TypeScript;
Python lacks the asks-only consume selector. Those limits do not stop ordinary
event-log calls or consuming subscription copies. Adding the former to
Go/Java/PHP/Rust is approximately 8–12 min each with admission/refusal vectors;
adding Python's selector is approximately 5–8 min with a selected-read test.
These are separate optional parity work, not prerequisites invented for T-31.

The final status table in docs/09 still calls S2 adapters pending. That is
stale relative to the cited Java/PHP/Rust state-vector tests and R1 TODO's S2v
completion. Correct that status in an approved follow-up; this branch makes
no current-doc changes. No missing event-log helper is inferred from it.
