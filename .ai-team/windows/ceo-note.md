# T-13 note for #ab-ceo (from #ab-w1b, 2026-10-30)

Nothing has moved. I built the mechanism and applied none of it.

- Dry run: `python3 .ai-team/windows/windows.py` (writes nothing). Apply: `python3 .ai-team/windows/windows.py apply --window ab-w1a [--window ...]`, or `--all`.
- What it would do, per the dry run today: every window except DIR (already .ai-team/dir) gets `.ai-team/<bus name>/` as its `cwd` in `.ai-team/ab-dvp-v2-config.json`.
  Notes are COPIED from the memory of its current directory and account into the memory of the new directory:
  LEAD1, W1A: 39 notes each from ~/.claude (the repo-root memory); W1B: 33 from ~/.claude2; LEAD2, W2A, W2B: 0 (their `Plans` directory has no notes);
  W1C, W2C, QA1, QA2 (Codex, opencode): a directory only. The originals stay; an existing different note in a target is kept; DIR's memory is untouched.
- Apply backs the config up to `.ai-team/t13-backup/` first and changes only the `cwd` of the windows you name.
- Cost: a window resumes its session by its directory, so a moved window starts a fresh transcript; apply between tasks.
- Tests: `python3 -m unittest .ai-team/windows/test_windows.py`; five mutants (overwrite, apply-all, move, dry-run writes, DIR moved) each fail it.
