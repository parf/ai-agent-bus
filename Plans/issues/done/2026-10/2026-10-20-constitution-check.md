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
- ✅ 🟠 C2 `src/internal/core/deliverto.go:161-170` a group-expanded copy is skipped with no log and no `dropped`
  - Constitution § Errors and alerts: a configured flow that broke goes to the error log; § Flow outcomes: a failed recipient counts as its own `dropped`
  - Evidence (read): the unknown-member branch above calls `warnRepeated`, the consent branch `return`s silently; `deliverto_test.go` asserts only the unknown-member line
  - Impact: after an ownership transfer or a removed Maintainer a recipient stops receiving and nothing says why
  - Related (read): `manages` passes the daemon Owner (`manage.go:227`), so a daemon-Owner-owned Agent "consents" to every channel through a group, against `docs/01-identity-and-authority.md:115-118`
  > 💡👾 `warnRepeated` before the `return`; use `resourceManages` for the consent test.
  >
  > 💬👾 confirmed at 9424f07b: the consent `return` at deliverto.go:167 is silent, and `manages` lets the daemon Owner through (manage.go:226). Simplest fix: `warnRepeated` before the return and `resourceManages` instead of `manages`; no `dropped` count, because a skipped leaf is "no recipient", as for User leaves (Q126)
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #2: code, the simplest fix above
  >
  > 💬👾 `293769d0` — fixed
- ✅ 🟠 C4 `src/agents/telegram/telegram.ts:332`, `src/internal/agentmeta/help.go:16-18` an `mcp` ask is not answered `{"tools":[]}`
  - Constitution § Rules "No MCP tools": every answering agent answers `mcp` with `{"tools": []}`; slack, git, systemd, server-info, logwatch do
  - Evidence (read, not run): telegram handles only the four help asks; the runner and `agent-bus run` (`serve.go:707`, `run.go:531`) do not treat `mcp` as a metadata ask, so the script runs with `AGENT_BUS_ASK=mcp` and an empty body
  - Impact: a pure ask returns an arbitrary script result, possibly with side effects
  > 💡👾 Answer `mcp` from the manifest in the runner and `run`, and add it to telegram.
  >
  > 💬👾 partly at 9424f07b: the packaged agents answer `mcp` themselves; telegram answers it with help prose, and a third-party script under the runner or `agent-bus run` is run with AGENT_BUS_ASK=mcp. Simplest fix: serve.go and run.go answer `{"tools":[]}` for `mcp` unless help is "native", plus one line in telegram
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #2: code, the simplest fix above
  >
  > 💬👾 `293769d0` — fixed
- ✅ ⚪ C6 `src/internal/journal/journal.go:176-177` the debug-log `Method` is not cut at 200 characters
  - Constitution § Logs: every field keeps its first 200 characters; Caller and Path use `field()`, Method is printed raw
  - Evidence (ran): a 20,000-character method token gave a 20,071-byte debug.log line; `net/http` allows about 1 MB
  - Impact: one unauthenticated request grows an on-demand log line; low
  > 💡👾 `field(r.Method)`.
  >
  > 💬 👤parf trim and report error
  >
  > 💬👾 confirmed at 9424f07b: `field(r.Method)` trims it. For "report error": `field()` already marks the line as cut, and the line carries the error status the caller got. An error-log line would let an unauthenticated caller write the error log (§ Errors and alerts), so the cut marker is the report
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #2: code, the simplest fix above
  >
  > 💬👾 `293769d0` — fixed

## Decisions
- 🚫 🟠 E1 `Plans/done/decisions-archive.md` vs CLAUDE.md:33 replaced rows were deleted, not archived
  - Evidence (read): `git show 48efdf8a^:docs/decisions.md` held a `## Superseded` section (54 rows) and rows 14d39169 replaced; none is in the archive ("Forwarding stops after one hop", "Empty ACL admits only the record owner", and the 2026-09-15 row `decisions.md:40` replaces); 48efdf8a deleted them, 69061902 then restored the archive rule
  - ( ) backfill the rows from `48efdf8a^` 🌟
  - ( ) amend CLAUDE.md:33 and the archive header: replaced rows live in Git only
  > 💬👾 rejected: the 53 superseded rows were removed on the owner's request to "remove historical stuff" (48efdf8a); Git keeps them, and backfilling history is cost without a reader
