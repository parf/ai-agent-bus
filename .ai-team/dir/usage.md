# Usage and workload log (#ab-dir, per common.md § Usage and workload control)

| Time (EST) | Window | Left | Resets | Decision |
|---|---|---|---|---|
| 11-05 20:07 | W2C (Codex) | ctx 43%, weekly 38% | — | T-33 writer |
| 11-05 20:07 | W1C (Codex) | ctx 14%, weekly 38% | — | idle; compact before next task |
| 11-05 20:07 | W2B (Claude) | ctx used 50%; 5h/7d not read | — | T-33 reader (confirmed available by W2C) |
| 11-05 20:07 | W1B (Claude) | ctx used 54%; 5h/7d not read | — | idle |
| 11-05 20:07 | leads | not on the bus | — | route directly to workers until back |
| 11-05 20:22 | LEAD1, LEAD2, W1A, W2A (Claude acct 1) | readers 0, queued 5/2/1/1 (lead1 oldest 48h39m) | — | NOT receiving pushes → CEO asked to restart; J95 if still 0 after |
| 11-05 20:24 | W1A (Claude acct 1) | 5h 74%, weekly 96% | — | idle; best Claude capacity now (authority/secret work) |
| 11-05 20:58 | W2C (Codex) | ctx 31%, weekly 36% | — | triage 11-02; may need compact mid-run |
| 11-05 20:58 | W1C (Codex) | ctx 83%, weekly 36% | — | triage 11-01+Oct |
| 11-05 21:14 | W2A \/ W2B \/ W2C | 5h 58% (ctx 36% used) \/ 5h 69% \/ weekly 35% | — | W2C weekly binding: container only if >20% |
| 11-05 21:22 | QA2 (GLM) | 5h LIMIT | 00:49 EST (11-06 13:49 CST) | send nothing; team2 reads → W2C (F1 first), W2B (2c, C3), team1 lends W1C (from ~21:30) for C3 then F6; QA1 stays team1 |
| 11-05 21:39 | QA1 (GLM) | 5h LIMIT | 00:49 EST | send nothing; team1 reads → W1A\/W1B\/W1C |
| 11-05 22:02 | W2B (Claude) | ctx 88% used, 5h 39% (reset 1h47m) | ~23:50 | declined T-39 read; compact after J102 verdict |
| 11-05 22:02 | W1A (Opus) | 5h 31%, 7d 89%, ctx 21% | 5h ~23:10 | review fixes; T-38 S3 after S1 ~23:30 |
| 11-05 23:30 | account 2 (DIR, W1B, W2B, CEO) | 5h 1% | ~23:50 | PAUSED |
| 11-06 01:02 | W2C (Codex) | ctx 25%, weekly 25% | — | F2 rebased e2f37474 → QA2; F1 → W2A ~01:35; compact W2C after checkpoint (lead2 sends path → I forward to CEO) |
| 11-06 01:04 | QA1 (GLM) | reset passed 00:49; idle with 2 stale queued prompts | — | compact QA1 → CEO; team1 reads (T-43, T-39, G3, G-low, G-flags, G4) → W1A/W1C meanwhile |
| 11-06 01:08 | QA1 \/ W2C | QA1 active again (G3 4f364d01 no high), stale prompts in OpenCode session queue, CEO compacts when idle (ctx 58%); W2C compacted, ctx 83% left | — | QA1 back for team1 reads |
| 11-06 01:13 | W1C (Codex) | weekly 24%, ctx 31% | — | finish D1 (2bc65e07, mutant waits lock), then small reads only; G5 → reassign if needed |
| 11-06 01:21 | W1C (Codex) | ctx <30%, weekly 23% | — | compact → CEO; T-43 re-ready 93db87f9; after: D1 mutant, C4 LOWs |
| 11-06 01:27 | QA1/QA2 | now glm-5.3-flash (owner) | — | rule to leads: ⚠️/two-reader changes need ≥1 non-QA reader; W1C + W2B compacted, idle |
| 11-06 02:00 | QA1 (GLM flash) | 5h LIMIT | 06:20 EST (19:20 CST) | send nothing; J111 + T-44 reads → W1B; J116 + RAG-source → W2B (lent from team2); T-45 W1C only, gated after RAG-source |
| 11-06 02:27 | W2A \/ W2B \/ W2C | 5h 25% (7d 73%) \/ 59% \/ weekly 22% | W2A 5h ~04:10 | FS.2 2nd read → W2B; team2 idle |
| 11-06 02:39 | W1C (Codex) | ctx 21%, weekly 21% | weekly 11-10 21:35 | no new tasks; compact only if lead1 asks |
| 11-06 14:01 | W1C (Codex) | ctx 20%, weekly 19% | weekly 11-10 21:35 | compact → CEO; T-50 reader only |
| 11-06 14:02 | W1C (Codex) | ctx 100% (compacted), weekly 19% | weekly 11-10 21:35 | T-50 reader |
| 11-06 14:37 | W2C (Codex) | weekly 18%, ctx 27% | weekly ~11-10 | J123 java/php; compact after; thin for more tasks |
| 11-06 14:41 | W2C (Codex) | ctx 100% (compacted), weekly 18% | ~11-10 | light tasks only |
| 11-06 18:15 | W2C (Codex) | weekly 11% (17% an hour earlier, idle) | ~11-10 | one-off reads only; idle drain noted |
| 11-06 18:15 | W2A \/ W2B | weekly 61% \/ 66%, W2B fresh 5h | — | team2 capacity; J123 restore → W2A if owner says |
| 11-06 19:09 | W2C (Codex) | weekly 7% (idle drain continues) | ~11-10 | no work; short read only if QA2 out |
| 11-06 19:09 | W2A \/ W2B \/ QA2 | 5h 67% 7d 55% \/ 5h 59% 7d 59% \/ up | — | team2 capacity: J126 W2B |
| 11-06 19:30 | QA2 (GLM) | 5h LIMIT | 21:12 EST (10:12 CST) | send nothing; J126 read → W2C (short) |
| 11-06 19:34 | QA1 (GLM) | usable (old limit msg, reset 06:20 passed) | — | lendable to team2 while QA2 out |
