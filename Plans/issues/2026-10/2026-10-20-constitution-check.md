# Constitution check — Oct 20, 2026 · 978f0278..473e6fc3

📌 **TL;DR:** The last two days of commits (since 2026-10-18) were read against
[the constitution](../../../docs/constitution.md) and [current decisions](../../../docs/decisions.md#settled).
The core of "check once at the door" is faithful. Most breaks are a newer owner decision that never reached the
constitution (it is now stale or contradicts the code), plus five code paths that break a stated rule. Nothing was
changed; every item below is for the owner to accept or reject. Items marked *read* were checked by reading
every call site, *ran* by a probe, *not run* by reading only.

## Code against the constitution

- 🟠 C1 `src/agents/debug/debug.py:58-69` `#debug env` prints `AGENT_BUS_CONFIG` in clear to any caller its ACL admits
  - Constitution § Private values: the allow list "grants use, not these values" (Q125); `doctor` is role-gated, `env` is not
  - Evidence (ran): `AGENT_BUS_CONFIG='{"k":"CFGBODY"}' python3 -I debug.py env` → the body is in the output; docs bless it at `docs/runner/security.md:190`, `docs/agents/debug.md:63`
  - Impact: bites once an Owner stores a Configuration on the debug record; then every ACL-admitted caller reads it
  - ( ) mask `AGENT_BUS_CONFIG` like the token 🌟
  - ( ) show it only when the delivered roles include owner or maintainer

- 🟠 C2 `src/internal/core/deliverto.go:161-170` a group-expanded copy is skipped with no log and no `dropped`
  - Constitution § Errors and alerts: a configured flow that broke goes to the error log; § Flow outcomes: a failed recipient counts as its own `dropped`
  - Evidence (read): the unknown-member branch above calls `warnRepeated`, the consent branch `return`s silently; `deliverto_test.go` asserts only the unknown-member line
  - Impact: after an ownership transfer or a removed Maintainer a recipient stops receiving and nothing says why
  - Related (read): `manages` passes the daemon Owner (`manage.go:227`), so a daemon-Owner-owned Agent "consents" to every channel through a group, against `docs/01-identity-and-authority.md:115-118`
  > 💡👾 `warnRepeated` before the `return`; use `resourceManages` for the consent test.

- 🟠 C3 `src/internal/core/bus.go:1527` the eleventh 📣 "fails the whole send" but earlier branches already stored copies
  - Constitution § Channels: the eleventh fails the send; "every check comes before anything is stored"
  - Evidence (read, not run): `fanouts++` and `errFanouts` come after `deliver` on earlier branches; `fanout_count_test.go:30` only asserts `<= maxForwards` copies
  - Impact: the caller is told the send failed while up to ten receivers hold it; a retry multiplies them
  - ( ) count the reachable 📣 before storing 🌟
  - ( ) keep the copies and document at-least-once, which contradicts "fails the whole send"

- 🟠 C4 `src/agents/telegram/telegram.ts:332`, `src/internal/agentmeta/help.go:16-18` an `mcp` ask is not answered `{"tools":[]}`
  - Constitution § Rules "No MCP tools": every answering agent answers `mcp` with `{"tools": []}`; slack, git, systemd, server-info, logwatch do
  - Evidence (read, not run): telegram handles only the four help asks; the runner and `agent-bus run` (`serve.go:707`, `run.go:531`) do not treat `mcp` as a metadata ask, so the script runs with `AGENT_BUS_ASK=mcp` and an empty body
  - Impact: a pure ask returns an arbitrary script result, possibly with side effects
  > 💡👾 Answer `mcp` from the manifest in the runner and `run`, and add it to telegram.

- ⚪ C5 `src/agents/standalone/git/git_agent.py:295` `--help` in a body is treated as the help method
  - Constitution § Rules "The call": no body text is reserved, `--help` is ordinary input (Q179)
  - Evidence (read): `if body in ('help', '--help')`; `test_git.py:140` asserts both answer alike
  > 💡👾 Drop `'--help'` from the tuple and the test.

- ⚪ C6 `src/internal/journal/journal.go:176-177` the debug-log `Method` is not cut at 200 characters
  - Constitution § Logs: every field keeps its first 200 characters; Caller and Path use `field()`, Method is printed raw
  - Evidence (ran): a 20,000-character method token gave a 20,071-byte debug.log line; `net/http` allows about 1 MB
  - Impact: one unauthenticated request grows an on-demand log line; low
  > 💡👾 `field(r.Method)`.

- ⚪ C7 `docs/runner/security.md:181`, `docs/runner/operations.md:117` "the daemon never stores runner secrets" is not strictly true
  - Evidence (read, not run): the web editor and `write runner/<agent>/env` carry the env text as a message body; an unconsumed write sits in the runner's durable inbox and is flushed to SQLite
  - Impact: contradicts RD105 "a runner secret never leaves its host" only when the runner is offline or slow
  - ( ) reword the claim to "never stored beyond the in-flight write"
  - ( ) make the runner write non-durable, or refuse it while the runner is not reading

- ⚪ C8 `src/mcp/README.md` (5698ec1b), `src/mcp/server.ts` the MCP face cannot read or edit Configuration
  - Constitution § Private values: Owner and Maintainers see and edit it "in every face, including the web"
  - Evidence (read): the MCP exposes `secret()` only; `tools.yaml` points to the web or CLI
  - ( ) add an MCP read/edit tool
  - ( ) narrow the constitution to "CLI and web" 🌟

## Constitution text against newer decisions

- 🟠 D1 `docs/constitution.md:625-629` § Channels still says a term is asked "when added and never again"; A1 (bde3b8b2) asks again at every publish for group-expanded copies
  - Evidence (read): the substance sits only in `docs/04-messaging.md:393`; `04-messaging.md:389` and `decisions.md:61` still say "never again"; the decision row `decisions.md:62` links only to 04-messaging
  - Impact: the normative doc calls the new behaviour a bug; CLAUDE.md "Decisions need two edits" skipped the owning constitution
  - Also: the constitution's "MUST NOT add a check at delivery" (`:204-207`) sits beside the live checks at `bus.go:1365-1369` (forwarding: B's ACL must list A at delivery) and the A1 check; nothing says a record's own route state is not caller authority
  > 💡👾 One sentence in § Channels for the group exception, one in § Authority at the door that route and flow state checks are not caller authority, and the same in `04-messaging.md:389-395`.

- 🟠 D2 `docs/constitution.md:15-16,60-83` "ignores — never repairs" vs `snapshot.go:562-570` `dropOversizedConfigurations` (37d8aa11) which clears `Config` and `ConfigSHA` and keeps the record
  - Evidence (read): documented only at `docs/03-records-agent.md:163`; no row in the constitution tables or `decisions.md`; Q123 (a 📣 loses `ttl`, `bound`, `overflow` at load) is the same pattern
  - Also: `docs/03-records-service.md:162` says "refused at load" and `:99` says "ignored and reported"; Q122 (`decisions.md:77`) lists neither as a refusal
  - Impact: unclear whether a drop at load is a repair; an oversized Configuration disappears after one Alert line
  > 💡👾 Add a "what startup drops" row to the constitution, a decision row, and "ignored" at `03-records-service.md:162`.

- 🟠 D3 `docs/constitution.md:579,792-794` Administrator on a Group "may do what a Maintainer may"; code allows membership only
  - Evidence (read): `manage.go:570` admits an Administrator only when `membershipOnly(change)`; `review1017_test.go:37`; `docs/01-identity-and-authority.md:338` agrees with the code; from issue A6 (e182ebf0), owner decision never recorded
  - Impact: two normative texts disagree and the constitution loses; "except reactivating" is now moot
  - ( ) edit the constitution to "membership only" and add a decision row 🌟
  - ( ) restore Maintainer-level edits for Administrators, minus reactivation

- ⚪ D4 `docs/decisions.md:64` (Q94, Q104) says an Agent may send to a User "its own ACL admits"; the constitution (`:634-636`) adds "or the User is its Owner or a Maintainer" (Q125)
  > 💡👾 Reword the row to the full rule, keeping its one link.

## Decisions

- 🟠 E1 `Plans/done/decisions-archive.md` vs CLAUDE.md:33 replaced rows were deleted, not archived
  - Evidence (read): `git show 48efdf8a^:docs/decisions.md` held a `## Superseded` section (54 rows) and rows 14d39169 replaced; none is in the archive ("Forwarding stops after one hop", "Empty ACL admits only the record owner", and the 2026-09-15 row `decisions.md:40` replaces); 48efdf8a deleted them, 69061902 then restored the archive rule
  - ( ) backfill the rows from `48efdf8a^` 🌟
  - ( ) amend CLAUDE.md:33 and the archive header: replaced rows live in Git only

- 🟠 E2 `Plans/done/decisions-archive.md:61,62,64,82,83,131` live rules sit in the archive, "never a current contract"
  - Evidence (read): Q123, Q126, Q127 (cited as live at `constitution.md:644,734`), Q128 (`:393`), the env-file reference refusal (stated as current at `docs/03-records-service.md:128-135`), tags and RD102 have no current row
  - Impact: a reader of the current index cannot find the decision name or its Why
  > 💡👾 Move the still-current rows back into `decisions.md`, or add current rows linking to their sections.

- ⚪ E3 `docs/decisions.md` owner decisions in the window without a row: RD100, RD104, RD105 (only in `Plans/R1.0-Release/DECISIONS.md:16,21,42`), A13 (`accounts.go:158`, `docs/07-setup.md:333`), A5 (D2), A6 (D3)
  - Impact: the current index misses built behaviour on authority; D2 and D3 are the visible result
  > 💡👾 Add the rows, each linking to the existing owning section.

- ⚪ E4 `Plans/R1.0-Release/QUESTIONS.md:11-27,30,50-56,65`, `docs/decisions.md:139` settled questions stay in QUESTIONS; the "Open" pointer names only R1
  - Evidence (read): "№Q145/Q146/Q147/Q17/Q20/Q182/RQ1 is settled" paragraphs against the file's own header; open IDs live in `Plans/R1.1/QUESTIONS.md` (Q178, Q35, Q72), R1.2, R1.3, R2.0
  > 💡👾 Delete the settled paragraphs; point "Open" at `Plans/README.md#stages`.

- ⚪ E5 `docs/03-records-agent.md:74` says the roles-refusal error log "names the sender, the record and the roles claimed"; `bus.go:1297-1301` logs a count only (e182ebf0)
  - Also: that line goes through `warnRepeated` to the error log although § Errors and alerts keeps an ordinary refusal to its caller (old exception c28399b5)
  - ( ) update the doc to the count-only text 🌟
  - ( ) drop the `warnRepeated` call so the constitution holds strictly

Checked and sound: links and anchors of constitution, decisions, archive, docs and R1.x plans; Q and RD ID uniqueness; the door (`Authenticate` before each handler, no mid-operation re-check in Consume, Send, Manage, locks, readers, Unregister); may-send dry run (shares `sendTarget`, no counters, no body); request body read after authentication; `release --force` audit; forwarding ACL at store and each hop; ten-step and ten-📣 limits; PubSub `in`/`out` and independent branches; the daemon stores and interprets no `agent.json`/`env.dist`; the `ask` field carries no body; help is role-filtered; receipts; the MCP dated exception; launcher names (74c0270e, 88d5fb4c); lock wait re-checks nothing; Configuration/Secret gating (`Config()`, `Secret()`, `Configure()`, daemon Owner excluded unless Maintainer, digests only to readers); web Configuration card and 403; runner host env `read` answers a digest; audit logs name only; token, secret and Configuration bodies absent from the daemon error log.
