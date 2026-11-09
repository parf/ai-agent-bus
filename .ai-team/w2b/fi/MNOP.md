- 🟠 M1 `Plans/subsystems.md:19` `WRITING.md` matches no row, so editing it promotes all 17 subsystems to the requested level
  - Evidence: `smoke-select.py` `plan(m.ROOT,'normal','',['WRITING.md'])` gave fallback `unmapped path: WRITING.md` with every letter A..Q at `normal`; a `Plans/README.md` edit selects only O; row N lists `docs/`, `README.md`, `CHANGELOG`, `LICENSE.md`, `licenses/NOTICE.md`; WRITING.md changed in the range
  - Impact: every prose-only edit to WRITING.md forces a full normal gate, against "docs-only changes need link checks, no smoke"
  > 💡👾 Add `WRITING.md` to the N row's path prefixes (done in this run's commit).
- ⚪ O1 `Plans/issues/done/2026-10/2026-10-20-constitution-check.md:28` a tracked file holds Russian text (owner replies quoted at lines 28, 41, 56)
  - Evidence: `grep -nP '[\x{0400}-\x{04FF}]'` matches those three lines; the only Cyrillic hit among changed docs, Plans, acceptance, README, WRITING, CLAUDE and .claude files
  - Impact: low; a closed archive file, but it breaks the English-only rule
  > 💡👾 Replace the quoted replies with an English paraphrase of the owner's answer.
(Its N1, the stale `status.services` fallback in node.md:208, is A3 above.)

Checked and sound (M, N, O, P): links and anchors in 256 changed Markdown files outside done dirs, src/agents and src/clients (0 broken); CHANGELOG head 0.9.264 matches VERSION; `.claude` and CLAUDE.md `--normal`/`--slow` wording against Verification; src/README.md smoke options; .gitignore; acceptance `|| true` uses guarded; removed 0.9.264 routes absent from docs, clients, MCP. Not checked: 35 acceptance files in full, smoke shard internals beyond selection.
