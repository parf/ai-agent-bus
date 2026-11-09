# B.29 cron/#at move: plan, branch ab-w2b/B29-cron (worktree tmp/ab-w2b-B29cron), 2026-10-30 16:20

Rulings (#ab-dir via #ab-lead2): RD161 replaces RD138 for cron: a refused/unknown-outcome copy is DROPPED with one warning, the entry stays
active; a one-shot `later` is dropped with one warning and its entry REMOVED (no "unconfirmed"/paused state). Same for #at.
Ids stay random collision-checked 12-hex; "deterministic" = only the legacy non-12-hex id migration (new key written before the old is
removed; a restart finishes an interrupted move; fixture test). Add `sub edit` -> "delete it and add it again". RD138 row moves to
Plans/done/decisions-archive.md naming RD161. Update docs/agents/cron.md 128-182 and at.md. Kill test counts deliveries (never twice/missing).

Engine: src/agents/shared/scheduler.py (shared by cron.py and at.py). Code changes planned:
- drop(): sent/dropped counts, one warning line, sub stays active, later -> finish('dropped'); used by denied() 403/404, refused() (no retry),
  unsure() (unknown acceptance), not_author lost/vanished; sweep(): 'sending' claim = unknown -> dropped; legacy claims unconfirmed/denied/
  gone/retrying -> settled; legacy stored `paused` subs are unpaused on load, paused one-shots removed as dropped (migration).
- remove retries/backoff machinery; listing no longer shows `paused`.
- load(): non-12-hex sub./later. keys migrate: new id = sha256(legacy key) 12 hex, collision-checked, write new first, then delete old.
- cron.py: `sub edit` refused. Tests in src/agents/cron/test_cron.py rewritten from pause to drop (about 20 tests), plus new: kill test,
  migration fixture test, edit refusal. Mutants (agent not in the mapped warning list? check subsystem L/agents) as CLAUDE.md asks.
