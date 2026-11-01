# T-24 client compatibility

## Behavior

The Go, Python and TypeScript clients, CLI and MCP check HTTP `GET /identity` before their first application request. They use client protocol 1, independently of the program version. A daemon minimum above it refuses connection; an older daemon warns once without refusing connection; a later `no_route` refusal names the unsupported operation. Missing fields count as zero; malformed or negative integers and a minimum above the daemon protocol refuse with the field named. Existing lazy SDK connection behavior is retained.

SDK refusal errors expose `kind`; CLI messages retain it and MCP tool errors print it. `unknown` remains missing data, distinct from `no_route`; a 500 says `internal daemon error`. Normal and streaming bigfile paths share refusal decoding. No retired alias or services field is used by these clients.

The handshake keeps the caller's deadline and cancellation. MCP warnings go to stderr. The CLI identity probe closes its HTTP connection so it does not prime Go's idle-connection retry before a consume; its existing ghost-reader recovery check stays green.

## Checks

Fake HTTP servers test current and future-compatible daemons (silent), absent/old fields (warning), too-new minimum (fatal before any application request), malformed values, refusal kinds, unavailable causes and 500 messages. TypeScript's normal client check explicitly includes the new protocol tests; Python discovers them, Go runs them with the client suite, CLI with its owning package, and MCP with the existing `face.test.ts` normal entry point.

`TestRealDaemonProtocolAndRemovedRoute` and `TestCLIRealDaemonProtocolAndRemovedRoute` use disposable real daemon fixtures, not the live daemon: `/identity` reports protocol 1/minimum 1 without warnings; `POST /subscriber/remove` is 404 `no_route` in the Go client's Kind and the CLI's displayed message.

Each of the five too-new guards was independently removed under the slow lock. All five compiled and failed their named handshake assertion: Go 0.458 s; Python 5.085 s; TypeScript 0.040 s; CLI 0.887 s; MCP 0.067 s. Source restored and lock released immediately. No build failure or exception-only mutant is credited.

## Owning suites

- Go client: `clients/check.sh go` (vet and race), PASS, test body 4.049 s.
- Python: 39/0, 18.053 s, including the real client fixture.
- TypeScript: the owning check (typecheck, 46/0 Bun tests, packaged Node smoke) passes; Bun body 5.28 s.
- CLI: whole owning package PASS, 23.988 s; real protocol/removed-route and refusal-message checks pass.
- MCP: exact normal rpc-unit command list, 177/0 in 17.65 s; it includes the face protocol tests and launcher endpoint diagnostic update. A standalone MCP typecheck still reports the same two `rename-smoke.ts:176–177` possibly-undefined errors on untouched main; no new type errors were introduced. It is not the normal rpc-unit command.

The first added identity request changes fake-daemon expectations: old fixtures now answer protocol 1 before their scenario, certificate tests count both identity and caller-status requests, and the unavailable-socket diagnostic names `/identity`. These fixture updates do not skip application assertions.

## Gate integration repair

The first gate (log `tmp/gate-logs/20261101-145056-T-24.log`) failed four owning checks twice. Each was reproduced before the repair:

| Check | Red cause | Repair and full owning shard |
|---|---|---|
| `go_unit` | `TestEveryFileTheAgentPackagerWritesIsInTheBundleContract` named seven unlisted Python `protocol.py` copies | All independent bundle/runtime inventories include the module; vet and race pass, 37.372 s |
| `process_titles` | Before + 5 expected, before + 7 observed | Three refused curl requests, then two distinct CLI processes each making one identity probe and one status request; shard passes, 7.872 s |
| `client_examples` | Go example fake reported `unexpected request /identity` | Fake supplies current protocol before the inventory operation; full examples shard passes, 19.838 s, PHP journal unchanged |
| `agent_subscriptions` | Admission fake refused identity, producing four failed delivery assertions | Fake supplies current protocol and permits the read-only identity route in its request inventory; full subscription shard passes, 4.464 s |

The two status client processes in the passing process-title run were the ordinary shared-socket `agent-bus status` (PID 129827) and `AGENT_BUS_ADDR=<fixture user socket> AGENT_BUS_TOKEN= agent-bus status` (PID 130166). Intermediate title counts isolate each one's two requests; the replacement-daemon status process likewise has an identity request plus status. These are fixture PIDs, not live-node actions.

`TestCLIProtocolHandshake` now makes two application calls through one client and asserts one `/identity` probe. Disabling the cache fails three assertions with `one process probed identity 2 times` and paths `/identity /status /identity /status`; restored check passes. The changed process counts do not hide per-call probing. All four full owning shards ran under the slow lock, then it was released. No whole normal gate was run by the worker.
