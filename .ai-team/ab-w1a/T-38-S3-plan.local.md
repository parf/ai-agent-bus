# T-38 S3 — DBSync build plan (MySQL, PostgreSQL)

📌 **TL;DR:** `#dbsync/<name>` is a kept process on the S1 core (`change_sync.py`). It pages one table through a read-only `#db/<name>` with a per-subscriber (`updated`, key) position, and keeps only a bounded window of rows in state, never the whole table. The steps are D1 (agent, statement builder, cursor and paging), D2 (lookback, de-duplication, soft delete) and D3 (DB-fixture lines, guide, mutants). Code starts when S1 lands. The plan is [Plans/R1.1/dbsync.md](../../Plans/R1.1/dbsync.md); this note covers how I build it and the points the lead asked about.

## 1. Credentials boundary

| Holds | Where | Never in |
|---|---|---|
| DB host, user, password | `#db/<name>`'s env file and Secret, as today | DBSync's Configuration, Secret, state, messages, status |
| What to read | DBSync's Configuration: DB agent name, table, schema, key column, `updated` column and unit, columns to send, soft-delete column and value, interval, lookback, state cap, first-run mode | — |
| DBSync's bus token | handed over once (kept process serving itself) | state, messages |

- DBSync talks to the database only as `agent-bus call '#db/<name>' 'query <sql>'`, using its own bus identity. The DB agent picks the account and the tier.
- Configuration keys are an exact allowlist. Unknown keys are refused by name, so `password`, `dsn` and the like can't slip in.
- A `#db` error text is already redacted by the DB agent. DBSync puts it in `status` and the Owner's alert only, never in a subscriber's message.
- State holds keys, `updated` text, soft-delete flags and per-subscriber positions. It holds no column values (section 3), so a leaked state file carries no row data and no credential.

## 2. Change key and deletes

- **Item key** in the S1 state is the row key as the DB returned it, as text. **Fingerprint** is the `updated` text plus a deleted bit: `"<updated>"` or `"<updated>|del"`. S1's `pending()` then gives `create` (no mark), `change` (a newer `updated`, or deleted now) and S1 `delete` (mark present, item gone).
- **S1 `delete` is never produced by DBSync's pruning** (no phantoms). Pruning drops an item and every subscriber's mark for it in one state write, so the "mark without item" shape can't arise from a prune. Only rows that left the window after all subscribers confirmed them are pruned.
- **Soft delete** (the only way in S3, Q316): the Owner names a marker column and its "deleted" value. A row read with the marker set gets the `|del` fingerprint and is sent as a delete message.
- **A tombstone for a subscriber that never had the row is not sent.** That needs one small S1 hook: `pending()` must skip `create` when the record is a tombstone (`record.get('gone')`), marking nothing. `matches(row, key)` can't do it because it never sees the record.
  - ➜ **Ask:** W1B adds `gone` handling to S1 before it lands, or I add it as the first S3 commit in `change_sync.py` with its own test and mutant. Lead's choice; I'd rather S1 carry it.
- Key diff (hard deletes) is out of S3: no setting, no code path, and the guide says a table without a marker reports no deletes.

## 3. Small per-item state (my S1 LOW-B)

S1's model of "all items plus marks per subscriber" would be O(rows × subscribers) for a table. DBSync keeps a **window** instead:

| State part | Holds | Bound |
|---|---|---|
| `meta.scan` | the scan position (`updated`, key) of the running poll | 1 pair |
| `meta.pos[sub]` | each subscriber's confirmed high-water pair, below which nothing is pending | 1 pair per subscriber (≤100) |
| `items` | rows read above the lowest subscriber position and not yet pruned: key → fingerprint | `window_max` rows (Configuration, default 10 000, about 1 MB) |
| `marks[sub]` | the S1 marks, but only for keys in the window | ≤ window per subscriber |
| `meta.dedup` | delivered pairs at or after the lookback floor | pruned each poll |

- **Back-pressure:** when the window is full, the poll stops reading new pages until the slowest subscriber confirms. `status` names that subscriber and its backlog. The state never grows with the table.
- **Bodies are not stored.** `deliver(body=…)` takes the row's columns from the last page held in memory. After a restart, or for a row no longer in memory, one `SELECT <cols> FROM <t> WHERE <key> = <literal>` re-reads it; the key is numeric or a timestamp by the type guard, so the literal is safe. A row that changed meanwhile is sent at its newest state, which matches "latest state at poll time".
- **Save cost:** DBSync passes S1 a state of window size, so per-subscriber saves stay small. My LOW-B fix in S1 (save only on a confirmation) still applies.
- **Replay for a new subscriber** (default): its `pos` starts below every row, and the scan starts at the minimum over the subscribers' `pos` and the lookback floor, so the window refills from the start for it. Subscribers already past a row skip it through their own `pos`. `replay:false` sets `pos` to the current high-water pair.

## 4. Read-only DB role

- The Owner grants DBSync's identity only the `read` role on `#db/<name>`. The DB agent opens a read-only transaction (MySQL `START TRANSACTION READ ONLY`, PG read-only session), so the database refuses any write, as `db.py` enforces today.
- The statement builder emits exactly three shapes, all `SELECT`:
  1. the page query;
  2. the single-row re-read;
  3. the column-type query on `information_schema.columns`.

  A golden test pins each shape per dialect.
- Identifiers must match `^[A-Za-z_][A-Za-z0-9_]{0,63}$` and are quoted per dialect (`` `x` `` or `"x"`). Cursor values must be numeric or a timestamp in a strict pattern, and nothing else reaches the text. The statement never carries a quote taken from a value.
- **A gap I can't close from DBSync:** it cannot see which tier `#db` gives it, because the DB agent reports the tier only in its own stderr log. Acceptance proves the database refuses a write under the `read` tier (fixture). The guide tells the Owner to grant `read` only. A DBSync-side check would need a new `#db` answer field, which is out of scope ("invents no call format").
  - ➜ If you want it, that's a follow-up row, not S3.
- **Type guard (Q318):** at configuration load and at `sub add`, one `information_schema.columns` query (table and column names as string literals from the identifier pattern) returns the data types. A key must be numeric or a timestamp. `updated` must be a timestamp, or numeric with a declared unit (`s` or `ms`). The error counts the failing columns ("1 of 2 …").

## 5. Tests without a live DB server

| Layer | Fixture | Runs in |
|---|---|---|
| Statement builder, cursor and paging, window, back-pressure, lookback, de-duplication, soft delete, type guard, refusals | **A fake `#db` in-process**: a list-of-rows table and an injected bus whose `call('#db/x', 'query …')` parses only the three known statement shapes (a regex, not SQL). It answers `{columns, rows, truncated}` exactly as `db.py`'s collector does, including the byte-cut and the `max_rows` cut. The fake clock is the DB's `now()`. The agent's clock is skewed on purpose | `agents/dbsync/test_*.py`, fast and normal |
| Subscriber confirm, resend, authority | S1's fake bus (may_send, record lookup, call and send recorders) | same |
| The real SQL on both engines, read-only refusal, late commit with two transactions | **the existing `agent_db` podman fixture** (MySQL/MariaDB and PG containers from `agents/db/fixtures.py`) | `agent_dbsync` normal shard only. podman needs the lock and the lead's approval, so I run it by hand once in D3 at most and leave it to the gate's normal |

- Every acceptance item in [DS.1](../../Plans/R1.1/TODO.md#dbsync) maps to one named fake-DB test. The four that need a real engine (dialect `now()`, read-only refusal, late commit across transactions, the type query) also get a fixture line.
- Mutants, each compiled and failing on an assertion, on the ⚠️ parts:
  - the statement builder: allow a quote; the cursor on `updated` only; drop the key tie-break;
  - state and window: prune without dropping marks, which makes a phantom delete; no back-pressure;
  - authority: skip the `#db` may-send at `sub add` and before a forward;
  - paging: the no-progress refusal removed, bounded so it fails rather than hangs.
- Hand-run tests use `env -u AGENT_BUS_SESSION_FILE -u AGENT_BUS_TOKEN` and never the live bus.

## 6. Two readers

- **#ab-w1c (Codex):** the statement builder and dialects, paging and cursor maths.
- **A Claude reader, #ab-qa1 or #ab-w2b:** the window and pruning (phantoms, state size), authority and the `#db` may-send, the credentials boundary.
- ⚠️ parts: the statement builder (parser-like), state writes, authority. Each reader gets the D-step diff as it lands, not one review at the end.

## 7. Steps and ETA (agent time, start when S1 lands, ~23:30)

| Step | Builds | Ready |
|---|---|---|
| D0 | S1 `gone` hook, if S1 doesn't carry it: one commit, a test, a mutant | +15 min |
| D1 | `src/agents/dbsync/`: agent.json, Configuration allowlist, type guard, statement builder (3 shapes × 2 dialects), cursor and paging (page = max_rows − ceil(max_rows/10), no-progress refusal), window and back-pressure, deliver via S1, poll, status and reset, `sub` verbs, package.sh, smoke-map and shard | ~01:30 |
| D2 | lookback by DB `now()`, de-duplication pruning, soft delete, first-run `replay:false`, the single-row re-read | ~02:45 |
| D3 | the `agent_dbsync` fixture lines (MySQL, PG), `docs/agents/dbsync.md` guide, help golden, mutants, TODO line | ~03:45 |

- About 4 h of agent time plus review loops, which matches TODO DS.1's "four to five hours". dbsync.md says D1–D3 "three to four hours" while TODO says D0–D3 "four to five". I'll align the plan text to D0–D3 in D3.
- Quota: my 5h window resets around 23:10. If it drops under 15% mid-step, I checkpoint and tell you.
- Owner steps at the end, not mine: the `#db` read-only account and record, the DBSync token hand-over, and a live RAG run.

## Lead's calls (2026-11-05 22:35)

1. **The `gone` hook is my D0**, built on W1B's coming S1 fix, which adds an *uncertain* mark at send so that a timed-out create later gets its delete.
   - Rule: `pending()` skips a tombstone **only when the subscriber has no mark at all**: no confirmed mark and no uncertain mark. It never sent that row, so there is nothing to retract, and it is marked done.
   - When an uncertain or confirmed mark exists, the tombstone is a `change` to a deleted state, sent as a delete. A possibly-delivered key still gets its delete.
   - Tests: (a) never sent, then tombstoned → nothing sent; (b) send timed out (uncertain), then tombstoned → delete sent; (c) confirmed, then tombstoned → delete sent; (d) S1's hard-delete path is unchanged.
   - Mutants: skip on an uncertain mark too → (b) fails; never skip → (a) fails.
   - I check against W1B's diff that the skip reads the same mark field the fix writes.
2. **The `#db` tier:** the guide-only rule for S3. In D3, a FUTURE line in Plans/R1.1/FUTURE.md: a `#db` answer field that reports the tier it served, so DBSync can refuse a non-read tier.
3. **`window_max` 10 000:** an overridable default.
4. **The window escape:** back-pressure must not let one dead subscriber stall the rest.
   - A subscriber is **parked** when S1 refuses its admission (`admission_refused`: may-send false, or the target record gone) in each round for `park_after` (default 10 min), or when it confirms nothing while having pending rows for `park_after`.
   - A parked subscriber does not count toward the window's lower bound, its marks are dropped from the window, and `status` names it with the reason and since when.
   - It **unparks** when an admission check passes again. Its own `pos` is still below the window, so the scan floor drops to it and it catches up through the normal back-pressure.
   - Its `pos` never moves while parked, so no row is skipped for it.
   - Test: subscribers A (refused) and B (healthy), window_max 3, 10 rows. B gets all 10 after the park, and A's `pos` is unchanged. A becomes admissible and then gets all 10.
   - Mutant: parked subscribers still count toward the bound → B stalls at 3, and the test fails on its count (bounded, not a hang).
1b. **D0 also holds my body() LOW** (S1 lands at c196c32c without it).
   - In `deliver`, build the payload before `attempted.add`. A body() exception is a restore of that key's maybe, like a Refusal, plus a `body_failed` warning, and the subscriber's batch stops there so order holds.
   - Test: body() raises for item 1, then item 1 is removed at the source → no delete is sent, and `body_failed` is warned.
   - Mutant: the payload built after `attempted.add` (or the exception kept as unknown) → a phantom delete is sent and the test fails on its count.
5. **podman:** the gate's normal run covers the fixture lines; a hand-run needs the lead's approval.

Readers: W1C plus a Claude reader for each D-step. Code starts when S1 lands.

## D1 design notes (22:59, before the S2 read)

- #db config is unreadable to DBSync (GET /config: Owner/Maintainer/self only), so DBSync's own Configuration carries `engine` and `max_rows` (set to the #db value). A mismatch is safe: a lower #db cap gives a cut with progress, which continues.
- Item record: {'fp': u or u+'|del', 'u': updated text, (gone)}; pair = (u, key); sort keys by column type (numeric→Decimal, timestamp→fromisoformat).
- meta: scan, base, floor, pos[sub], parked[sub]={reason,since}, quiet[sub], refused[sub].
- matches(row,key) = key in items and pair > pos[sub] (no delete(None) from kept maybes; no resend to a sub already past it).
- prune: drop an item when every ACTIVE sub has pos ≥ pair or mark==fp with no maybe; drop marks for all subs, keep parked subs' maybes.
- pos advance: walk items in (pos, scan] in pair order while confirmed; only if base ≤ pos.
- floor = min active pos; floor below base → scan=base=floor (re-read).
- read only while len(items) < window_max; limit = min(page, room).
- body: memory row cache, else the single-row re-read; a missing row → drop item+marks+maybe (no hard-delete reports).
- park: admission refused, or pending without a confirmation, for park_after. Unpark: admission passes (admission reason) / it confirms (silent reason) / silent with nothing pending → probe after park_after.

## Checkpoint 23:07 (paused for J105 P1)

- Branch ab-w1a/T-38-S3, worktree tmp/ab-w1a-T-38-S3.
- D0 done: c970482e (gone hook), e62b3865 (body restore), 71ccfa93 (delete recorded as '?'); W1C is reading 71ccfa93.
- D1 WIP commit (squash before ready): src/agents/dbsync/{statements,settings,dbsync}.py compile; no tests yet.
- D1 still to do: test_statements.py (goldens per dialect), fake #db plus test_dbsync.py (cursor, paging, known rows, byte cut, no progress, type guard, authority, resend, window/back-pressure, park test A/B), agent.json, env.dist, config.json.dist, package.sh, help.golden.json, smoke-map entry plus shard, docs pointer. Mutants after.
- Open design point: tombstones and lookback are D2.

## Checkpoint 2026-11-05 23:37 (J105)
- ab-w1a/J105 c8b6c04e ready (watchdog 93f1527a + reused-pid f80fe399 + race c8b6c04e), W1C reads; mutants done.
- Bisect: 2.1.319 starts plugin MCP in every scratch copy since 23:24, even with the updater on; c/d failure 23:12–14 not reproducible. Logs in tmp/j105-logs, tmp/j105-c..e.
- Next: (A) compaction let-go; then S2 delta read, S3 D1 (WIP 52f73026 to squash).
- 23:45: J105 18b48652 ready (never-unlink + seen set). (A) static: SIGINT pairs match session restarts (mdhouse-w1 22:58, w1b 21:01), not compaction (03:29Z); proposed closing (A). Waiting on lead. Then: file J105 issue (docs branch, after landing), S2 delta read, S3 D1.
- 00:05: D1 ready 8121bf6a (WIP squashed; 17 tests, 5 mutants). Deferred until S2 lands: smoke shard/map, agents/package.sh, docs/agents/dbsync.md. Next: S2 delta read when W1B sends; D2 (lookback, dedup, soft delete) after lead's answer.
- 00:40: D2 soft delete 2cd087f1, replay:false 98a23c15, HIGH eda434f7, LOW 9b8e9673 ready. Next lookback+dedup.
- 01:05: D2 lookback 67a51b87 ready; D2 complete. Next: D3 (dialect goldens, PG fixture lines, guide) + D1 tail after S2.
- 00:20 11-06: S3 owes() model f416bea8; HIGH/MED fix 2d6194ed + 3a7694d6 + 0636e96e ready (43 tests; mutants M1–M8, M10 red; M9 dead code removed). Engine run 2 passed (b7ccc719). Docs owed in D1 tail: "changed" covers new+updated, lone-delete edge, lookback limit, replay:false join edge, Stale round, lookback_degraded (position only, late commits may be missed). J105-A docs branch ab-w1a/J105-A 6c4ef584 ready; J107 → Q343 (owner). Next: S2 tiny delta read when W1B sends; D1 tail after S2 lands.
- 00:35 11-06: S3 rebased on S2 97cc22f2 (both change_sync lines kept) + D1 tail: e05c4c5f skipped symmetry, af78590a packaging (bundleFiles, tests, acceptance), cfeb57e6 shard agent_dbsync, 591799fe docs/agents/dbsync.md + index. Ready 591799fe → W1C delta read. After S2 lands: rebase on main, resend sha. Close-out after landing: R1.1 TODO/DONE DS.1.
- 00:21 11-06: T-34 PARKED (owner moved it to R1.1): keep worktree tmp/ab-w1a-calendar and branch ab-w1a/calendar (9b372d46) untouched; no rebase, no removal until the owner tickets it. agent-pack (T-39) live at c7f86a25, QA1 read 00:49.
- 2026-11-06: live spec written (T-38-S3-live-spec.local.md), sent to lead; waiting for S2 on main to rebase S3.
- 2026-11-06 00:3x: S3 rebased on main a25330d9 → 87540a96, focused green, ready sent; waiting W1B whole read + W1C delta.
- 2026-11-06: W1B round (2 MED, 3 LOW) fixed → ee8e7a55, 5 mutants killed, ready sent; waiting W1B/W1C delta reads.
- 2026-11-06 00:36: S3 landed 23cf6ff3 (1.0.37); worktree and branch removed. Kept: agent-pack (T-39), calendar (T-34 parked). Free.
- 2026-11-06 01:2x: T-39 agent-pack rebased on d2347200 → da4a5399 (filescan/dbsync pack VERSION, shard moves, changelog), ready sent; HOLD (daemon restart) active: no locks/smoke.
- 2026-11-06 01:2x: J111 ready in .ai-team/ab-w1a/J111 (gate.sh.new + gate-lock.sh + 2 tests, 8 mutants killed); swap is #ab-ceo's while gate idle. T-39 with QA1/W1B.

## Checkpoint 2026-11-06 02:00
- J111: ab-w1a/J111-map 00d4b509 ready (3 commits); gate part .ai-team/ab-w1a/J111 (gate.sh.new, gate-lock.sh, tests, mutants). Readers W1C, QA1.
- T-44: .ai-team/ab-w1a/T-44 (gate.sh.new = J111 + apply.py, deploy-bump.sh, tests, 14 mutants). LOWs fixed; W1C rereads.
- T-39: ab-w1a/agent-pack ef9187ad rebased on 491b5b4d, ready; open: bump-agents vs TD6 (asked lead).
- J117: approach sent; waits on (d) bump-agents decision.
- Worktrees: tmp/ab-w1a-J111-map, tmp/ab-w1a-agent-pack, tmp/ab-w1a-calendar (parked T-34).

## Checkpoint 2026-11-06 02:14
- J111 landed (ab05e2a7); gate swapped (.ai-team/gate.sh = T-44 copy 11173f0dfa0d). Keep .ai-team/ab-w1a/J111, T-44 until lead confirms.
- T-39: ab-w1a/agent-pack abc512b3 ready (W1B hunk read). After it lands: README.md:39 + roles/ceo.md --agents-summary usage commit.
- deploy-bump: .ai-team/ab-w1a/T-44/deploy-bump.sh 10f3c4255916 (pack bump + edge), W1C reading.
- J117: .ai-team/ab-w1a/J117 gate.sh.new 85956bfa72b9 ready; swap after T-39.
