# Constitution check — Oct 20, 2026 · 978f0278..473e6fc3

📌 **TL;DR:** The last two days of commits (since 2026-10-18) were read against
[the constitution](../../../docs/constitution.md) and [current decisions](../../../docs/decisions.md#settled).
The core of "check once at the door" is faithful. Most breaks are a newer owner decision that never reached the
constitution (it is now stale or contradicts the code), plus five code paths that break a stated rule. Nothing was
changed; every item below is for the owner to accept or reject. Items marked *read* were checked by reading
every call site, *ran* by a probe, *not run* by reading only.

## Code against the constitution

- ⏳ 🟠 C3 `src/internal/core/bus.go:1527` the eleventh 📣 "fails the whole send" but earlier branches already stored copies
  - Constitution § Channels: the eleventh fails the send; "every check comes before anything is stored"
  - Evidence (read, not run): `fanouts++` and `errFanouts` come after `deliver` on earlier branches; `fanout_count_test.go:30` only asserts `<= maxForwards` copies
  - Impact: the caller is told the send failed while up to ten receivers hold it; a retry multiplies them
  - ( ) count the reachable 📣 before storing 🌟
  - ( ) keep the copies and document at-least-once, which contradicts "fails the whole send"
  > 💬 👤parf we better make adjustent to constitution; its pretty expensive to calc it - and a goal of recursion check is to report error
  >
  > 💬👾 confirmed at 9424f07b; following the owner, change the constitution, not the code: "the eleventh is a Deliver-To loop and is reported: the send answers an error with an error-log entry, and copies already delivered on earlier branches stay; the limit reports a loop, it does not undo one" (constitution.md:679-681)
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ ⚪ C7 `docs/runner/security.md:181`, `docs/runner/operations.md:117` "the daemon never stores runner secrets" is not strictly true
  - Evidence (read, not run): the web editor and `write runner/<agent>/env` carry the env text as a message body; an unconsumed write sits in the runner's durable inbox and is flushed to SQLite
  - Impact: contradicts RD105 "a runner secret never leaves its host" only when the runner is offline or slow
  - (x) reword the claim to "never stored beyond the in-flight write"
  - ( ) make the runner write non-durable, or refuse it while the runner is not reading
  > 💬👾 confirmed at 9424f07b: reword as chosen, adding that SQLite free pages may keep the bytes until they are reused
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ ⚪ C8 `src/mcp/README.md` (5698ec1b), `src/mcp/server.ts` the MCP face cannot read or edit Configuration
  - Constitution § Private values: Owner and Maintainers see and edit it "in every face, including the web"
  - Evidence (read): the MCP exposes `secret()` only; `tools.yaml` points to the web or CLI
  - ( ) add an MCP read/edit tool
  - ( ) narrow the constitution to "CLI and web" 🌟
  > 💬👾 confirmed at 9424f07b: narrow constitution.md:749-750 to "CLI and web", and fix the same sentence in src/mcp/README.md:209
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

## Constitution text against newer decisions

- ⏳ 🟠 D1 `docs/constitution.md:625-629` § Channels still says a term is asked "when added and never again"; A1 (bde3b8b2) asks again at every publish for group-expanded copies
  - Evidence (read): the substance sits only in `docs/04-messaging.md:393`; `04-messaging.md:389` and `decisions.md:61` still say "never again"; the decision row `decisions.md:62` links only to 04-messaging
  - Impact: the normative doc calls the new behaviour a bug; CLAUDE.md "Decisions need two edits" skipped the owning constitution
  - Also: the constitution's "MUST NOT add a check at delivery" (`:204-207`) sits beside the live checks at `bus.go:1365-1369` (forwarding: B's ACL must list A at delivery) and the A1 check; nothing says a record's own route state is not caller authority
  > 💡👾 One sentence in § Channels for the group exception, one in § Authority at the door that route and flow state checks are not caller authority, and the same in `04-messaging.md:389-395`.
  >
  > 💬👾 confirmed at 9424f07b: in § Channels after :628, add "a 👥 term is the exception: each record it expands to receives a copy only while that record's Owner manages the channel, checked at every publication"; in § Authority at the door after :207, add "a check of the records' own routing state (a forward's destination ACL, a group recipient's Owner consent) is not caller authority". Scope "never again" at 04-messaging.md:389 to the caller adding the term
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ 🟠 D2 `docs/constitution.md:15-16,60-83` "ignores — never repairs" vs `snapshot.go:562-570` `dropOversizedConfigurations` (37d8aa11) which clears `Config` and `ConfigSHA` and keeps the record
  - Evidence (read): documented only at `docs/03-records-agent.md:163`; no row in the constitution tables or `decisions.md`; Q123 (a 📣 loses `ttl`, `bound`, `overflow` at load) is the same pattern
  - Also: `docs/03-records-service.md:162` says "refused at load" and `:99` says "ignored and reported"; Q122 (`decisions.md:77`) lists neither as a refusal
  - Impact: unclear whether a drop at load is a repair; an oversized Configuration disappears after one Alert line
  > 💡👾 Add a "what startup drops" row to the constitution, a decision row, and "ignored" at `03-records-service.md:162`.
  >
  > 💬👾 confirmed and wider at 9424f07b: three load drops (Q123 📣 settings, stray addr/proto, oversized Configuration). Add one "What startup drops" table after constitution.md:86, reword :15 to "never repairs a record", and change 03-records-service.md:162 to "ignored and reported"
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ 🟠 D3 `docs/constitution.md:579,792-794` Administrator on a Group "may do what a Maintainer may"; code allows membership only
  - Evidence (read): `manage.go:570` admits an Administrator only when `membershipOnly(change)`; `review1017_test.go:37`; `docs/01-identity-and-authority.md:338` agrees with the code; from issue A6 (e182ebf0), owner decision never recorded
  - Impact: two normative texts disagree and the constitution loses; "except reactivating" is now moot
  - ( ) edit the constitution to "membership only" and add a decision row 🌟
  - ( ) restore Maintainer-level edits for Administrators, minus reactivation
  > 💬👾 confirmed at 9424f07b: the owner decided "membership only" on 2026-10-18 (issue A6, e182ebf0). Edit constitution.md:579 and :792-794, and extend the Why of decisions.md row 21
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ ⚪ D4 `docs/decisions.md:64` (Q94, Q104) says an Agent may send to a User "its own ACL admits"; the constitution (`:634-636`) adds "or the User is its Owner or a Maintainer" (Q125)
  > 💡👾 Reword the row to the full rule, keeping its one link.
  >
  > 💬👾 confirmed at 9424f07b: reword the row to the full Q125 rule, as the item says
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

## Decisions

- ⏳ 🟠 E2 `Plans/done/decisions-archive.md:61,62,64,82,83,131` live rules sit in the archive, "never a current contract"
  - Evidence (read): Q123, Q126, Q127 (cited as live at `constitution.md:644,734`), Q128 (`:393`), the env-file reference refusal (stated as current at `docs/03-records-service.md:128-135`), tags and RD102 have no current row
  - Impact: a reader of the current index cannot find the decision name or its Why
  > 💡👾 Move the still-current rows back into `decisions.md`, or add current rows linking to their sections.
  >
  > 💬👾 confirmed at 9424f07b: archive rows :34 (Q125), :61 (env-file references), :62 (tags), :64 (Q123), :82 (Q126), :83 (Q127) and :131 (RD102) are live. Move them back into decisions.md; that also settles D4's archive half and RD102 in E3
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ ⚪ E3 `docs/decisions.md` owner decisions in the window without a row: RD100, RD104, RD105 (only in `Plans/R1.0-Release/DECISIONS.md:16,21,42`), A13 (`accounts.go:158`, `docs/07-setup.md:333`), A5 (D2), A6 (D3)
  - Impact: the current index misses built behaviour on authority; D2 and D3 are the visible result
  > 💡👾 Add the rows, each linking to the existing owning section.
  >
  > 💬👾 confirmed at 9424f07b: RD100, RD104 and RD105 are built, so they are current scope. Add five decisions.md rows (RD100, RD104, RD105, A13, A5), each linking to its home; A6 is D3
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ ⚪ E4 `Plans/R1.0-Release/QUESTIONS.md:11-27,30,50-56,65`, `docs/decisions.md:139` settled questions stay in QUESTIONS; the "Open" pointer names only R1
  - Evidence (read): "№Q145/Q146/Q147/Q17/Q20/Q182/RQ1 is settled" paragraphs against the file's own header; open IDs live in `Plans/R1.1/QUESTIONS.md` (Q178, Q35, Q72), R1.2, R1.3, R2.0
  > 💡👾 Delete the settled paragraphs; point "Open" at `Plans/README.md#stages`.
  >
  > 💬👾 confirmed at 9424f07b: delete the settled paragraphs in R1.0-Release/QUESTIONS.md (only Q183 and Q184 are open) and point decisions.md "Open" at Plans/README.md#stages, since R1.1, R1.2, R1.3 and R2.0 also hold open questions
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

- ⏳ ⚪ E5 `docs/03-records-agent.md:74` says the roles-refusal error log "names the sender, the record and the roles claimed"; `bus.go:1297-1301` logs a count only (e182ebf0)
  - Also: that line goes through `warnRepeated` to the error log although § Errors and alerts keeps an ordinary refusal to its caller (old exception c28399b5)
  - ( ) update the doc to the count-only text 🌟
  - ( ) drop the `warnRepeated` call so the constitution holds strictly
  > 💬👾 confirmed at 9424f07b: take the first option, one line at 03-records-agent.md:74 ("counts the roles claimed"); A7 already bounds the warning
  >
  > 💬 👤parf то что надо править точно - правь
  >
  > 💬👾 → Codex #3: docs, after its .61–.63, the simplest fix above

Checked and sound: links and anchors of constitution, decisions, archive, docs and R1.x plans; Q and RD ID uniqueness; the door (`Authenticate` before each handler, no mid-operation re-check in Consume, Send, Manage, locks, readers, Unregister); may-send dry run (shares `sendTarget`, no counters, no body); request body read after authentication; `release --force` audit; forwarding ACL at store and each hop; ten-step and ten-📣 limits; PubSub `in`/`out` and independent branches; the daemon stores and interprets no `agent.json`/`env.dist`; the `ask` field carries no body; help is role-filtered; receipts; the MCP dated exception; launcher names (74c0270e, 88d5fb4c); lock wait re-checks nothing; Configuration/Secret gating (`Config()`, `Secret()`, `Configure()`, daemon Owner excluded unless Maintainer, digests only to readers); web Configuration card and 403; runner host env `read` answers a digest; audit logs name only; token, secret and Configuration bodies absent from the daemon error log.
