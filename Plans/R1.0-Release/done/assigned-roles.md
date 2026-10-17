# Assigned roles

Built in 0.8.110, 2026-10-16. The
[current contract](../../../docs/03-records-agent.md#roles) owns behavior;
the [checklist](../roles-implementation.md#assigned-roles-implementation) owns scope.

## Verification

- Full gate: `bash src/smoke.sh --slow`, 935 passed, 0 failed, final log
  `tmp/roles-slow-reviewed.log`. One earlier run failed an untouched launcher
  title-restoration assertion during concurrent mutation runs; three subsequent
  idle standalone runs and the final full run pass. The intermittent failure
  is recorded, not claimed fixed.
- Core checks cover role unions, management access, reserved-role atomic refusal,
  runtime terms, Personal rules, nested/cyclic/inactive groups, rollback/restore,
  deltas (including bare-name revocation and reordered role sets), name reuse
  and transfer, forwarding and fanout. Existing queued-message
  and sender-forgery checks remain in the gate.
- Real web/daemon check creates and edits an Agent, consumes the exact assigned
  roles, verifies the help popovers, and confirms a refused save changes nothing.
  The CLI smoke sends assigned roles through to a script's environment.
- Seventeen mutation probes compile first, fail their named assertion, then fail the
  corresponding `--slow` Go or web shard. Scratch harnesses and results live in
  `tmp/roles-mutations/`. An initial serialization mutant survived a weak field-name
  assertion; the assertion now compares the whole wire output and kills it.
- Plain-ACL paired benchmarks: send stays at 3 allocations / 40 bytes; lookup
  stays at 2 allocations / 16 bytes. Timings overlap at roughly 1.9 microseconds
  and 0.9 microseconds respectively; no speedup is claimed. Benchmarks cover
  nonmatching, direct, grouped, large/long-role unions and fanout too.
  Commands, before/after runs and a mutex profile: `tmp/roles-performance/`.
- Claude `#claude/ab-dvp@parf.us` closed the technical review after independently
  running the protocol/core/runner tests, reproducing the revocation issue,
  verifying its fix against a fresh daemon and checking the final gate log.

No client endpoint or storage migration was added. MCP and all six SDKs pass
ACL strings through; no per-language assignment parser was needed.

Owner confirmation of the implementation representation and preserved nested-group
behavior remains [Q146/Q147](../QUESTIONS.md#assigned-roles); neither is recorded
as an explicit owner answer.
