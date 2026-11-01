# T-24 client compatibility

## Behavior

The Go, Python and TypeScript clients, CLI and MCP check HTTP `GET /identity` before their first application request. They use client protocol 1, independently of the program version. A daemon minimum above it refuses connection; an older daemon warns once without refusing connection; a later `no_route` refusal names the unsupported operation. Missing fields count as zero; malformed or negative integers and a minimum above the daemon protocol refuse with the field named. Existing lazy SDK connection behavior is retained.

SDK refusal errors expose `kind`; CLI messages retain it and MCP tool errors print it. `unknown` remains missing data, distinct from `no_route`; a 500 says `internal daemon error`. Normal and streaming bigfile paths share refusal decoding. No retired alias or services field is used by these clients.

The handshake keeps the caller's deadline and cancellation. MCP warnings go to stderr. The CLI identity probe closes its HTTP connection so it does not prime Go's idle-connection retry before a consume; its existing ghost-reader recovery check stays green.

## Checks

Fake HTTP servers test current and future-compatible daemons (silent), absent/old fields (warning), too-new minimum (fatal before any application request), malformed values, refusal kinds, unavailable causes and 500 messages. TypeScript's normal client check explicitly includes the new protocol tests; Python discovers them, Go runs them with the client suite, CLI with its owning package, and MCP with the existing `face.test.ts` normal entry point.

`TestRealDaemonProtocolAndRemovedRoute` and `TestCLIRealDaemonProtocolAndRemovedRoute` use disposable real daemon fixtures, not the live daemon: `/identity` reports protocol 1/minimum 1 without warnings; `POST /subscriber/remove` is 404 `no_route` in the Go client's Kind and the CLI's displayed message.

Each of the five too-new guards was independently removed under the slow lock. All five compiled and failed their named handshake assertion: Go 0.458 s; Python 5.085 s; TypeScript 0.040 s; CLI 0.887 s; MCP 0.067 s. Source restored and lock released immediately. No build failure or exception-only mutant is credited.
