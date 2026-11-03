# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

- ❓ Q329 May W1C perform the prepared daemon and CLI cutover while the CEO is unavailable
  - Details: one stop/start cycle for candidate `06fabb3c` (0.9.303): back up the stopped daemon home and old binaries privately, atomically replace only the staged daemon and Go CLI binaries, then check identity and deployment health. Existing MCP faces and Codex pushers are not refreshed; no runner/web restart, push, tag or publication is included.
  - State: both binaries are staged with the exact candidate commit in their build stamp; W2C reviewed the cutover plan CLEAN. No live change has run.
  - Evidence: the exact candidate tree `82388f930a03213fd9f26d074a2ca722e7c8368e` passed the corrected `--slow all` run, 1305/0, exit 0. The first red run and its diagnosed wrapper/fixture causes remain recorded in [J94](../issues/done/2026-11/2026-11-02.md#release-validation).
  - Settled by: owner, before the live restart; the current local authority rule reserves daemon restarts to the owner/CEO.
  - Home: [stopped-daemon backup](../../docs/07-setup.md#stopped-daemon-backup), [minimal deployment](../../docs/07-setup.md#deploy-only-the-changed-component).
  - ( ) Authorize W1C's one backed-up daemon/CLI cutover and post-deploy checks; keep every other live action outside this approval. 🌟
  - ( ) Keep the live node unchanged and wait for the CEO to return. ⭐
  > 💡👾 Authorize the narrow cutover: the exact binaries are staged, the candidate is slow-green, and the plan has an independent clean read. A failed start stops for owner input before any second restart or rollback.

- ❓ Q330 May the completed independent Codex reviews replace the unavailable CEO/Fable final delta pass for this release candidate
  - Details: the original Fable review found the twelve blockers; all are fixed and independently reviewed. W1C and W2C completed finding-resolution audits, but those are Codex work, not a new Fable pass. The release runbook still calls for a CEO/Fable delta review, and that account is out of usage.
  - State: candidate `06fabb3c` is frozen and slow-green; no STABLE commit exists.
  - Evidence: 1305/0 on the exact candidate tree; all final-review rows in [TODO](TODO.md#10-final-review) are closed. The J94 fixture correction also has a clean independent read and a green normal gate.
  - Impact: accepting the substitute allows preparation of a local STABLE candidate for review; publishing, pushing and tagging still need their own authorization. A questions-only documentation commit after the slow run does not change the tested executable tree.
  - Settled by: owner, before STABLE preparation; this is a one-release review substitution, not a standing change to the review rule.
  - ( ) Accept the independent Codex reviews plus the green slow run, and authorize local STABLE preparation for review only. 🌟
  - ( ) Retain the CEO/Fable delta gate and wait for that account to return; do not prepare STABLE yet. ⭐
  > 💡👾 Accept the one-release substitute: the changes have independent reads and the full exact-tree run is green. Keep the evidence labelled as Codex review and leave all external publication steps separate.
