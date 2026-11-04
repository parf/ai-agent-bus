# T-31 Client event support completion

## Result

All six clients exercise the daemon's [agent logs](../../../docs/agents/bus.md#agent-logs)
through ordinary calls and sends. Native examples run against the shared real
fixture, and client READMEs link the executed sources without copied output.
No event-specific SDK helper was needed.

The [audit](../../issues/done/2026-11/2026-11-02-client-api-audit.md#audit-snapshot) is a dated snapshot, not the
current contract. The [implementation acceptance](../../issues/done/2026-11/2026-11-02-client-api-audit.md#implementation-acceptance)
records the built scope.

## Landed evidence

| Part | Commit | Result |
|---|---|---|
| Owner Q322 approval | `2ea865a8` | Event support included in 1.0.0; [decision](../DECISIONS.md#recorded-decisions) |
| T-31a | `3249e133` | Shared event vectors, real fixture reader and Go adapter/example |
| T-31b | `b4928628` | Python, TypeScript and Rust event adapters/examples |
| T-31c | `fc298c4c` | Java and PHP event adapters/examples using the same fixture |
| T-31e | `1b9b1d05` | Built status, executed-source README links and findable Rust event tests |
| T-31d | `18efe47d` | Separate subscription vectors, creation refusals and Go adapter/example |
| T-31g | `d4bf4814` | Python, TypeScript and Rust subscription adapters/examples |
| T-31f | `25be118f` | Java and PHP subscription adapters/examples |

## Completed acceptance

- Confirmed append, Owner read/list, private refusal before existence disclosure,
  writer-own-read refusal, empty versus missing and no-wait inbox fencing.
- One copy per matching write, filtering, subscription listing, deletion and
  cancellation; non-waiting notices receive no ack/done.
- Creation rejects an unmanaged source or unreachable target, with accepted
  controls differing only in the subject under test.
- The daemon profile keeps delivery after source-right revocation, while target
  admission is checked on every send. This follows Q325, without changing the
  app emitters' profile.
- Bounded client waits, compiled wrong-expectation mutants failing named adapter
  assertions, owning checks and executed native examples in all six clients.
- Original event vectors remain unchanged; subscriptions have their own shared
  file so each adapter could land independently.
