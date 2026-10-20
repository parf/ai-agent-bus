📌 **TL;DR:** History: the closed items of the October 20 review, moved from the [open file](../../2026-10/2026-10-20-constitution-check.md) with their records; not current contracts.

# Constitution check — Oct 20, 2026 · 978f0278..473e6fc3

## Code against the constitution
- 🚫 🟠 C1 `src/agents/debug/debug.py:58-69` `#debug env` prints `AGENT_BUS_CONFIG` in clear to any caller its ACL admits
  - Constitution § Private values: the allow list "grants use, not these values" (Q125); `doctor` is role-gated, `env` is not
  - Evidence (ran): `AGENT_BUS_CONFIG='{"k":"CFGBODY"}' python3 -I debug.py env` → the body is in the output; docs bless it at `docs/runner/security.md:190`, `docs/agents/debug.md:63`
  - Impact: bites once an Owner stores a Configuration on the debug record; then every ACL-admitted caller reads it
  - ( ) mask `AGENT_BUS_CONFIG` like the token 🌟
  - ( ) show it only when the delivered roles include owner or maintainer
  > 💬👾 rejected: debug is @owner-only by its docs and declares no Configuration, so no ACL caller can ever see one; nothing to protect
- 🚫 ⚪ C5 `src/agents/standalone/git/git_agent.py:295` `--help` in a body is treated as the help method
  - Constitution § Rules "The call": no body text is reserved, `--help` is ordinary input (Q179)
  - Evidence (read): `if body in ('help', '--help')`; `test_git.py:140` asserts both answer alike
  > 💡👾 Drop `'--help'` from the tuple and the test.
  >
  > 💬👾 rejected: no caller sends `--help` as a git body in practice, and the alias harms nothing

## Decisions
- 🚫 🟠 E1 `Plans/done/decisions-archive.md` vs CLAUDE.md:33 replaced rows were deleted, not archived
  - Evidence (read): `git show 48efdf8a^:docs/decisions.md` held a `## Superseded` section (54 rows) and rows 14d39169 replaced; none is in the archive ("Forwarding stops after one hop", "Empty ACL admits only the record owner", and the 2026-09-15 row `decisions.md:40` replaces); 48efdf8a deleted them, 69061902 then restored the archive rule
  - ( ) backfill the rows from `48efdf8a^` 🌟
  - ( ) amend CLAUDE.md:33 and the archive header: replaced rows live in Git only
  > 💬👾 rejected: the 53 superseded rows were removed on the owner's request to "remove historical stuff" (48efdf8a); Git keeps them, and backfilling history is cost without a reader
