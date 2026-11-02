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
| Bigfile | ✅ [conformance_state_test.go:286](../../src/clients/go/conformance_state_test.go#L286) | missing B — [capability matrix](../../docs/09-daemon-api.md#client-capability-matrix), source search below | missing B — [capability matrix](../../docs/09-daemon-api.md#client-capability-matrix), source search below | ✅ [test_conformance_state.py:123](../../src/clients/python/tests/test_conformance_state.py#L123) | missing B — [capability matrix](../../docs/09-daemon-api.md#client-capability-matrix), source search below | ✅ [client.test.ts:544](../../src/clients/typescript/tests/client.test.ts#L544) |

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

The missing-B result combines the [documented capability row](../../docs/09-daemon-api.md#client-capability-matrix)
with a case-insensitive source search at the snapshot: no `bigfile` symbol in
`src/clients/java/src/main`, `src/clients/php/src` or `src/clients/rust/src`.
Matches in those clients' test trees only describe the excluded vector set;
a link to the start of a record wrapper does not establish absence.

### Shared suite wiring

[check.sh:17](../../src/clients/check.sh#L17) builds the private
[Go fixture:31](../../src/clients/fixture/main.go#L31) and dispatches the native
checks. All six read `send`, `call`, `self-call`, `kv`, `locks`, `identity` and
`refusals` from the same [RT8 directory](../../src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors-t-24).
Only Go, Python and TypeScript run `bigfile`, because only those expose it.
The rows above cite send/call and state adapters; identity/refusal adapters are
at the refusal evidence links and their adjacent identity tests. This is
applicable-vector coverage, not a claim that missing bigfile is implemented.

Java and PHP map fixture names into their own real daemon, started by
[Java tests/daemon.sh](../../src/clients/java/tests/daemon.sh) and
[PHP tests/daemon.sh](../../src/clients/php/tests/daemon.sh), and run native code.
Go, Python, TypeScript and Rust use the Go fixture. Identity/refusal vectors use
fake servers deliberately. Neither harness substitutes raw HTTP calls for the
SDK under test. Native check entry points are
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


## Event calls without new APIs

**Answer for every client: the generic transport is sufficient.** At the audit
snapshot, the libraries can express all four operations, but the daemon event
verbs are not yet on main. Executable agent-log examples must land after P1;
subscription delivery also depends on P3. This is a feasibility conclusion,
not a claim that those event operations passed an integration test today.

Use the existing [T-29 verbs and authority](agent-events/README.md#r1-min-surface-and-authority)
and [write answer rule](agent-events/README.md#p2-write-transport).
Call for a confirmed append, read or list; send without wait proves only queue
acceptance and deliberately receives no write answer. Inspect the call outcome
and the existing method reply, not just HTTP success. Method refusals such as
`forbidden` and `not_found` are separate from HTTP refusal kinds. Do not retry
an uncertain append.

| Client | Generic write / read / list | Subscribe and receive | Helper assessment |
|---|---|---|---|
| Go | [Call / Send, bus.go:494–603](../../src/clients/go/bus.go#L494), with context and timeout options | Call the subscription verbs; Consume on the authorized destination | No write/read/list helper; optional subscription lifecycle helper only after an example exposes a gap |
| Java | [call / send, Bus.java:120–153](../../src/clients/java/src/main/java/agentbus/Bus.java#L120), with Call timeout | Subscription calls, then consume with Consume options | Same; use native cancellation and preserve Outcome handling |
| PHP | [call / send, Bus.php:130–153](../../src/clients/php/src/Bus.php#L130), with CallOptions timeout | Subscription calls, then consume with ConsumeOptions | Same; keep the single-reader constraint explicit |
| Python | [call / send, client.py:205–286](../../src/clients/python/agent_bus/client.py#L205), with timeout | Subscription calls, then consume on the selected inbox | Same; the missing asks-only selector is not needed here |
| Rust | [call, lib.rs:683](../../src/clients/rust/src/lib.rs#L683), with CallOptions; send for unconfirmed writes | Subscription calls and async consume | Same; cancellation must not imply remote work was undone |
| TypeScript | [call / send / consume, bus.ts:111–134](../../src/clients/typescript/src/bus.ts#L111), with the existing call budget | Subscription calls, then consume on the selected inbox | Same; subscription copies are not correlated call replies |

The implementation deliverable is one tested native-language example per
client, not speculative snippets in this plan. Each uses three identities:
the target agent writes its own log; its User Owner reads, lists, creates and
deletes the subscription, and consumes the authorized destination's copy; a
third, unmanaged agent probes private refusal. The target agent is not granted
read access merely because the log is its own. The Owner must be admitted to
consume the destination, and the subscription sender must be admitted there.
The example uses the canonical reply meanings rather than introducing another response format or SDK model.

### Subscription pitfalls and the helper threshold

- Subscription registration, listing and deletion already fit ordinary calls.
  The extra work is managing the destination reader, cancellation and receipts.
- Start with a dedicated authorized destination and raw consume loop. Do not
  classify arbitrary inbox messages as events by guessing a body, tag or topic.
  Do not run a second reader beside a library's serve loop.
- Raw consume gives the caller control; generic serve has automatic receipt and
  answer behavior. A copy must not accidentally become a request to answer, or
  start a receipt loop. Ignore receipts as data and exercise this in the example.
- Add a helper only if the native example requires repeated, error-prone
  lifecycle code. Keep it thin, within the clients' [MIT boundary](../../src/clients/LICENSE.md#mit-license),
  and tested through shared vectors for correlation, receipt handling,
  cancellation and no automatic replay. It must not add authority rules,
  transport retries or a second event model. Helpers are not included in the
  minimum estimate below; each needs an approved scope before implementation.

### Named logs use the same model

`#eventlog` named logs fit the same [canonical log model](../../docs/agents/eventlog.md#log-model)
and existing verb meanings. The destination and authority differ: named logs
use the instance's short log names and application-role grants; daemon agent
logs use canonical registry names and the T-29 management rule. An instance's
named logs do not become visible through another instance or through `#bus`.
The [front transition](agent-events/README.md#eventlog-front-and-existing-state)
is separate work; the R1 examples must not claim it has happened or claim
verified runner provenance for an entry.

## Implementation acceptance

- Extend the [private client fixture](../../src/clients/fixture/main.go#L31)
  once: establish the reserved daemon record and start/join its
  [ServeDaemonAgent reader](../../src/internal/api/daemon_agent.go#L22).
  Today's fixture starts HTTP and storage but no `#bus` reader. Use the landed
  P1/P3 implementation, not six fake event handlers and never the live daemon.
  This fixture serves Go/Python/TypeScript/Rust. Java/PHP instead need their
  tests/daemon.sh harnesses to expose the same three test identities and
  authorized destination against their real test daemon; after P1 that daemon
  already owns the `#bus` reader. Verify readiness in both harness types.
- Put representative event calls and subscription receipt cases into the
  [shared client vectors](../../src/testdata/client-conformance/README.md#state-identity-and-refusal-vectors-t-24),
  with a native adapter in all six clients. Reuse engine vectors for ring rules;
  do not copy the whole engine suite into every SDK.
- Each example verifies confirmed append/read/list, private refusal before
  existence disclosure, an empty active log versus missing log, one subscription
  copy, receipt handling and deletion/cancellation. The agent credential does
  confirmed append only; its User Owner performs read/list/subscription and
  destination consumption; the unmanaged third credential receives the same
  private refusal for existing and never-written logs. Also assert the writing
  agent's own read is forbidden. Bound every wait. A plain
  no-wait write leaves no answer in its caller's inbox.
- Retain all existing protocol vectors. A deliberately wrong event expectation
  must fail a named adapter assertion; a helper, if approved, has shared-vector
  coverage before it lands.
- Promote only executed examples to the owning client READMEs and link the
  canonical event model. Correct docs/09's stale S2-pending status in that
  approved follow-up, using the Part 1 evidence; no protocol change is needed.

## AI-time and freeze impact

Estimates are observed agent wall-clock minutes for B/C workers, not human
hours. They start when P1/P3 and the agreed reply semantics are on main. They
cover examples and native integration, not six new high-level API layers.

| Client | Minimum work | AI minutes, one B/C worker | Before-freeze fit |
|---|---|---|---|
| Go | Native example and shared event adapter | 6–10 | Fits if owner accepts the common schedule below |
| Python | Native example and shared event adapter | 6–10 | Fits on the same condition |
| TypeScript | Native example and shared event adapter | 6–10 | Fits on the same condition |
| Java | Native example, shared event adapter and real-daemon harness identities/readiness | 8–12 | Fits on the same condition; no bigfile work required |
| PHP | Native example, shared event adapter and real-daemon harness identities/readiness | 8–12 | Fits on the same condition; no bigfile work required |
| Rust | Native example and shared event adapter | 8–12 | Fits on the same condition; no bigfile work required |
| Shared | Real `#bus` Go fixture and shared cases | 8–12 | Required once, before native integrations |
| Documentation | Six client README examples and docs/09 S2-status correction | 5–10 | Required after executed examples; included in this schedule |
| Review and landing | Reader, required assertion proof, owning checks and gate | 15–25 | Required after the implementation |

Minimum total: **70–113 AI minutes with one worker**, including review/landing.
With two B/C workers, budget 8–12 shared minutes, 21–33 minutes for balanced
client work, 5–10 minutes for documentation and 15–25 minutes for review/landing:
**about 50–80 minutes on the critical path** (49–80 before rounding). Against
tree-ready **15:10–17:10**, making this entire minimum
mandatory shifts it to approximately **16:00–18:30**, if dependencies and both
workers are available. Any P1/P3 wait or gate queue adds directly; this is not a
promise to keep the original freeze window unchanged.

Recommendation for the owner's scope decision: accept the minimum examples
and tests if that shift fits the release; do not require helper APIs without a
concrete example showing the need. A subscription helper would add roughly
6–10 minutes per Go/Python/TypeScript client or 8–12 per Java/PHP/Rust client,
plus shared lifecycle cases and another review. Optional bigfile parity and
removed-alias coverage have their separate sizes above; neither is a dependency
for event calls. Implementation waits on the owner's [Q322 scope choice](QUESTIONS.md#r1-release-decisions); this plan does not authorize code.
