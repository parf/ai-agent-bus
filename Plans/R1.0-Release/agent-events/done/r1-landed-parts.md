# Landed agent-event parts

📌 **TL;DR:** P1, P2 and P4 are built. Subscriptions (P3) and durable
alerts (P5) remain open in the [TODO](../TODO.md#r1-min). These are the
completed check details; current usage lives in the [bus guide](../../../../docs/agents/bus.md#agent-logs).

<a id="p1"></a>

## P1 — landed 3b87ce4c, 0.9.289

Completed acceptance checklist (historical evidence):

### P1 checks

- Run the same `src/testdata/event-rings.json` model cases through Python and
  Go adapters. Cover default bounds, independent levels, TTL edge, control/ANSI
  removal, Unicode truncation, fixed-minute folding, backwards clock, read/pull
  order and inclusive clean. A changed model assertion can fail both engines.
- P1 exposes a ring storage seam for id reservation and optional alert
  retention. An injected backend failure cannot return an accepted half-write
  or reissue an id; P5 must not require changing the parser, ACL or ring model.
- Restart the memory-entry engine: all entries are absent; newly issued ids
  exceed every id issued before restart, including a clean-then-restart path.
  Crash between reservation and append skips ids, never repeats them. A failed
  durable reservation accepts no entry. Test retention profiles explicitly;
  do not silently skip the existing durable-alert expectation.
- Bus setup rejects per-agent ring sizes whose sum exceeds the shared
  500-entry cap; defaults total 450. A 501-entry configuration is refused
  without publishing it, while the boundary and defaults are accepted. A
  compiling cap-disabled mutant fails the explicit configuration assertion.
- Read authorization precedes log existence: a non-reader gets `forbidden`
  for a private existing log and a never-written log alike. An authorized
  manager of an existing agent with no log gets not-found. An existence-first
  mutant fails the explicit forbidden-code assertion, not only an empty-body
  check.
- Owner/Maintainer/daemon Owner can read/list/pull/clean. A bare admitted caller,
  a `read` role and an agent with write-only self access see no log contents or
  counts. Revocation is honored on the next operation; replacement records
  never expose old private logs.
- A caller writes only its own agent log; daemon Owner may also write. Another
  agent, a same-owner runner token and an Owner-set `runner` tag do not bypass
  this check. A body claiming a runner source grants nothing and is not shown
  as verified. Existing `#bus` help/status/version/ls still behave as before.
- Burst/rate and oversized-input fixtures prove bounded memory, no id allocated
  on refusal, and a drop count without recursive warnings. A blocked event
  operation does not hold core's global lock over disk/network work.
- Never-written and fully cleaned logs refuse read without data; active empty
  logs return zero counts and no placeholder. Read returns a complete selected
  ring set up to 500 entries, never a cut success. List contains sorted active
  summaries only, including active-empty, and either a full result or a size
  refusal. A level is a read selector, while pull's count and clean's id remain
  destructive, never hidden read pagination.
- P2's actual `send` body `write <agent> <level> <text>` and `reply_to` route
  reach the real `#bus` parser. A refused append returns an `error:` body on
  that route when wait is set; the HTTP send's success is not mistaken for
  append success. A waiting Call without explicit reply_to gets its response;
  no-wait writes send neither success nor refusal, even with reply_to, so the
  agent's process inbox remains untouched. Other verbs retain their behavior.
  Read/list retain eventlog forms, selected empty levels are lists and private
  refusal code is forbidden. Full canonical target names are accepted.
- A new verb is reachable through existing `call` without SDK additions;
  protocol/min remain 1. T-24 vectors remain green; representative call and ACL
  cases exercise real credentials, not supplied caller strings.

- **No-answer HIGH, daemon-backed:** after the real daemon has completed a
  no-wait `write` from an agent, that agent's inbox is empty, for success and
  refusal alike. Wait for task completion, not merely HTTP send acceptance;
  otherwise an early empty read is vacuous. A waiting write gets exactly one
  correlated answer, including without explicit reply_to. Old verbs still
  answer as before. A compiling always-send-answer mutant fails the explicit
  inbox-empty/count assertion, not a timeout. No late reply can become a
  runner-read agent's script input. P2 must not deploy before this P1 guarantee.

- P1 has bounded independent read/control and mutation dispatch. Hold an
  injected mutation operation on a readiness barrier; unrelated `read` and
  `status` answer before releasing it, within the named bound. Overflow returns
  busy without more goroutines; one agent cannot fill every mutation worker.
  Mutants serialize dispatch or remove the per-agent fairness limit and fail
  their own assertions, not a test timeout.

<a id="p2"></a>

## P2 — landed d8228153, 0.9.290

Completed acceptance checklist (historical evidence):

### P2 checks

- Start/requested stop are notices; an unrequested exit, signal or failed start
  is an alert. A requested stop produces no failure alert. One lifecycle event
  is not emitted twice by overlapping stop/exit hooks.
- Tee stderr only after the existing secret masker, with a bounded partial-line
  and forwarding queue. Plant configured secrets and verify they are masked;
  other text remains private/untrusted. A full/unavailable event path neither
  blocks the child nor retries it as a second event.
- Assert calls use each agent's `s.bus` token, never `r.bus`, and cannot select
  another agent. No log label or web view presents this as verified producer
  provenance. Existing stdout/JSONL protocol parsing remains intact.

<a id="p4"></a>

## P4 — landed bedc2a2f, 0.9.291

Completed acceptance checklist (historical evidence):

### P4 checks

- For a managed agent, preview the three highest ids across all levels when
  more entries exist; a newer notice is not hidden by older alerts. Expand
  reveals the complete bounded log in the shared level/order convention.
  Alerts stand out more than warnings; escaped markup remains text.
- Unrelated/read-role/self-write-only callers receive no preview, expand body
  or counts, including a forged direct page request and a revoked manager.
  No presentation claims a verified runner source.
- Test empty, warning-only and alert logs; expansion reads the current log
  without consuming it. No new stream route or dashboard work is required.
