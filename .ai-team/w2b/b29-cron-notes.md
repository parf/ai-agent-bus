# B.29 cron move: notes by #ab-w2b, 2026-10-30 16:10 (no code written yet)

Read so far: src/agents/cron/cron.py (99 lines), src/agents/shared/scheduler.py (710 lines, the engine behind #cron and #at),
docs/agents/subscriptions.md "The shared behaviour", Plans/R1.0-Release/subscriptions.md "Moving the agents", RD138/139/140/141.

Findings
- cron already uses a shared engine (scheduler.py): author check via /may-send (RD141) at add and before every send, the claim in KV
  before each send (RD138 "never twice"), 12-hex ids from `secrets.token_hex(6)` (scheduler.py `store`), limits 100/20.
- Conflicts with the shared behaviour (docs "Refused send"): cron PAUSES an entry on unconfirmed / denied / unknown destination and
  retries a definite refusal with backoff (RD138, docs/agents/cron.md lines 128-182). The shared rule: drop that copy, warn once, keep
  the subscription active, no pause, no retry, no outbox (RD161).
- sub.edit does not exist in cron today; "refused with delete and add" means adding a sub.edit method that answers that text.
- Ids: already 12-hex but random, not deterministic. No legacy non-12-hex rows are known for cron; need to check the live inventory read-only.

Open questions sent to #ab-lead2
1. Does the move replace RD138's pause/backoff with drop+warn for cron subscriptions? And for one-shot `later` entries (a one-shot cannot "stay
   active for the next match"): drop with a warning and remove the entry, or keep the paused `unconfirmed` state?
2. "Deterministic ids": derive new ids from content (hash of cron, target, body, creator) so a restart cannot mint another id for the same
   add? A duplicate identical add would then collide, so refuse it by name; or only the legacy-id migration is meant, which is empty for cron.
