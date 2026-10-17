# What we check

Every `/find-issues` reviewer checks its subsystem's changes against all of this. Adapted for
agent-bus from mdhouse's checklist; the last section is this project's.

## Common sense
- An action does not do what a person expects: e.g. opening a link writes.

## Correctness
- Logic errors: inverted conditions, off-by-one, wrong range bounds, swapped arguments of the same type.
- Unhandled cases: null / empty / zero / negative, an empty collection, the first / last element, time zones, Unicode.
- Error-handling errors: swallowed exceptions, a `catch` that does nothing, success returned on failure, a partial rollback.
- Races and concurrency: shared state without synchronisation, non-atomic check-then-act, a forgotten `close` / `unlock` / `defer`.
- Resource leaks: files, connections, goroutines / threads, subscriptions.
- Broken contracts: a function does not do what its name / doc says; a signature changed but not every call updated.

## Efficiency
- Quadratic or worse algorithms where there is a lot of data; a nested search instead of a map / set.
- N+1 queries, queries in a loop, no batching.
- Needless allocation / copying of large structures, the same thing computed again and again.
- No index / limit / pagination on a new database query.
- Blocking calls on a hot path, synchronous I/O where milliseconds are expected.

## Security
- Injection (SQL, shell, path, template), concatenation instead of parameters.
- Trusting user input without validation; permissions checked somewhere other than the boundary.
- Secrets / tokens in code, logs, error messages.
- Unsafe defaults: `0.0.0.0`, TLS verification off, `chmod 777`, wide CORS.
- Home-made cryptography, weak algorithms, predictable randomness.

## Behaviour and compatibility
- Broken backward compatibility: an API, a data format, a database schema, a config — with no migration / version.
- A changed default that silently changes behaviour for existing users.
- A migration with no rollback; a destructive migration (drop / rename) without a two-phase plan.
- A change to shared code (utils, a base class) with an unchecked effect on its other consumers.

## Tests
- A critical path changed, and there are no tests, or they do not touch the changed lines.
- A test changed "to fit the code" (an assert weakened, a case removed) — a red flag.
- No negative tests: errors, boundary values, empty input.
- Tests that depend on time, the network, order or randomness — flaky.
- Mocks that replace exactly what needed checking.
- A green check never seen to fail, or a mutant credited by a build failure: every shape in
  [mutation first, then belief](../../CLAUDE.md#mutation-first-then-belief) is a finding.
- A check that was deleted together with the code it guarded.

## Docs and consistency
- Behaviour / an API / a config changed, and the README, CHANGELOG, docs, examples, `--help`, OpenAPI were not updated.
- Comments and docstrings now contradict the code.
- Names (functions, variables, files) no longer say what the thing is after the change.
- A new config option with no documented default and no validation.
- Dead code left after the change; a TODO / FIXME without a ticket; commented-out code.

## Architecture and project style
- Breaking the project's existing conventions (layers, naming, where files go) — especially in a large codebase, where "how it is done here" matters more than abstract beauty.
- Duplicating an existing utility instead of using it.
- A leaking abstraction: a module reaching into another's internals, circular dependencies.
- A new dependency for something trivial; a heavy / unmaintained one / one with a bad licence.
- Magic numbers / strings that should be constants or config.

## Operations (deploy / runtime)
- Logging: no logs on a new critical path, or logs with PII / secrets, or spam.
- No metrics / alerts on a new feature that can break in production.
- A change that needs action at deploy (env, a migration, a restart, the rollout order) and does not say so.
- No timeouts / retries / limits on new external calls.
- A feature without a flag, where the project uses flags for risky changes.

## Meta-signals (for priority)
- A large diff in a "scary" file (auth, billing, migrations, parsers) → check harder.
- A change + no test + no docs in one commit → almost certainly an issue.
- A commit that touches many unrelated modules → split it, or check each part on its own.

## This project
- A [design boundary](../../CLAUDE.md#design-boundaries) crossed without its owning section changing:
  layering, authentication, ACL, private configuration, the trust boundary, the process boundary.
- A shared validator tightened where it also gates loading: `validateKind` decides what a restore keeps,
  so a record it now refuses is ignored at the next start, with its grants.
- A secret, token or private value repeated in an error, a log line or a refusal; a refusal must name
  the line, never its content.
- Two implementations of one rule (a parser, a name check, a walk) that can disagree — list every caller
  of the shared helper before calling it consistent.
- Docs that call a pending requirement built, or a built one pending; a value restated outside its
  [canonical home](../../CLAUDE.md#working-rules).
- A behaviour change without a PATCH bump and a changelog line, or a version literal outside
  `src/internal/version/VERSION` ([versioning](../../CLAUDE.md#versioning)).
- A tracked file that is not English, or scratch written outside `tmp/`, `local/` or `*.local*`.
