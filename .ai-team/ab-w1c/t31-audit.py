from pathlib import Path
home=Path('/usr/local/src/ai-agent-bus/tmp/ab-w1c-T-31/Plans/R1.0-Release/clients-events.md')
def link(path,line):
 return f'[{path.split("/")[-1]}:{line}](../../src/clients/{path}#L{line})'
langs=['Go','Java','PHP','Python','Rust','TypeScript']
P={
'Go':('go/protocol_test.go',15),'Java':('java/src/test/java/agentbus/BusTest.java',1019),'PHP':('php/tests/run.php',787),'Python':('python/tests/test_protocol.py',23),'Rust':('rust/tests/protocol.rs',51),'TypeScript':('typescript/tests/protocol.test.ts',18)}
R={'Go':('go/conformance_state_test.go',407),'Java':('java/src/test/java/agentbus/BusTest.java',1348),'PHP':('php/tests/vectors.php',233),'Python':('python/tests/test_conformance_state.py',227),'Rust':('rust/tests/conformance_state.rs',328),'TypeScript':('typescript/tests/protocol.test.ts',65)}
K={'Go':('go/conformance_state_test.go',104),'Java':('java/src/test/java/agentbus/BusTest.java',1184),'PHP':('php/tests/vectors.php',69),'Python':('python/tests/test_conformance_state.py',51),'Rust':('rust/tests/conformance_state.rs',98),'TypeScript':('typescript/tests/client.test.ts',494)}
L={'Go':('go/conformance_state_test.go',217),'Java':('java/src/test/java/agentbus/BusTest.java',1268),'PHP':('php/tests/vectors.php',139),'Python':('python/tests/test_conformance_state.py',96),'Rust':('rust/tests/conformance_state.rs',192),'TypeScript':('typescript/tests/client.test.ts',523)}
C={'Go':('go/failure_test.go',167),'Java':('java/src/test/java/agentbus/BusTest.java',367),'PHP':('php/tests/run.php',203),'Python':('python/tests/test_client.py',215),'Rust':('rust/tests/conformance.rs',191),'TypeScript':('typescript/tests/client.test.ts',110)}
T={'Go':('go/failure_test.go',91),'Java':('java/src/test/java/agentbus/BusTest.java',770),'PHP':('php/tests/run.php',597),'Python':('python/tests/test_client.py',520),'Rust':('rust/tests/transport.rs',196),'TypeScript':('typescript/tests/client.test.ts',146)}
V={'Go':('go/conformance_test.go',85),'Java':('java/src/test/java/agentbus/BusTest.java',488),'PHP':('php/tests/run.php',255),'Python':('python/tests/test_conformance.py',37),'Rust':('rust/tests/conformance.rs',78),'TypeScript':('typescript/tests/client.test.ts',320)}
B={'Go':('go/conformance_state_test.go',286),'Python':('python/tests/test_conformance_state.py',123),'TypeScript':('typescript/tests/client.test.ts',544)}
text='''# Client APIs and agent events

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
'''
for label,mapping in [('protocol / protocol_min',P),('Refusal kinds, including not_found',R),('KV (typed state vectors)',K),('Locks / holders (state vectors)',L),('Receipts and return route',C),('Timeouts / uncertain sends',T),('RT8 send / call / self-call',V)]:
 text+='| '+label+' | '+' | '.join('✅ '+link(*mapping[x]) for x in langs)+' |\n'
text+='| Removed aliases | ✅ '+link('go/protocol_test.go',132)+' | '+' | '.join('partial A — '+link(*R[x]) for x in langs[1:])+' |\n'
text+='| Bigfile | '+' | '.join('✅ '+link(*B[x]) if x in B else 'missing B — '+link({'Java':'java/src/main/java/agentbus/Record.java','PHP':'php/src/Record.php','Rust':'rust/src/storage.rs'}[x],1) for x in langs)+' |\n'
text+='''
The protocol tests include absent fields, malformed values, compatible newer
daemons, too-new fatal rejection and older-daemon warnings. Shared refusal
vectors check exact status, kind and message (including `not_found`, `no_route`,
500 and 503); they do not silently recast a missing route as missing data.
Receipts tests also check original-message correlation: '''
text+=', '.join(link(*p) for p in [('go/failure_test.go',167),('java/src/test/java/agentbus/BusTest.java',836),('php/tests/run.php',649),('python/tests/test_client.py',379),('rust/tests/client.rs',528),('typescript/tests/client.test.ts',135)])+'.\n'
text+='''
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
'''
home.write_text(text)
