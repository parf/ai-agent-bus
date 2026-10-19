# Identity display labels

> **Superseded for WEB directory presentation in 0.5.60:** the same
> daemon-stated glyph now precedes the identity name and the repeated type line
> is gone. Group headings similarly use `👥` before the name. This file records
> what 0.5.54 shipped; see [compact labels](compact-web-glyphs.md).
>
> **Agent glyph superseded in 0.5.61:** current human surfaces use `👾`; see
> [Agent glyph evidence](alien-agent-glyph.md).

📌 **TL;DR:** 0.5.54 gives User, Agent and Service one shared human label in
WEB and `agent-bus ls -h`, without changing machine syntax.

## Result

A pure display function maps daemon-stated user/person, agent and generic
kinds to `👤 User`, `🤖 Agent` and `⚙️ Service`. Other stated kinds pass through
without an invented glyph. WEB uses it across directory, service, Personal,
detail and diagnostics views; its established topic term remains Channel.
Human CLI uses the same labels and compensates for terminal glyph width so the
following column remains aligned.

Directory rows receive a glyph only when the caller-visible daemon answers
state a kind. Credential-only, hidden and unclassified rows stay unlabeled;
name spelling is never used as a substitute. The directory obtains record
kinds with one caller-visible listing, not a request per row.

Raw CLI JSON, API kinds, MCP output, URL/filter values, form values, CLI
arguments and editable ACL expressions keep the plain vocabulary.

## Checks

- Shared-function fixtures cover User, Agent, Service, topic, empty and unknown
  kinds.
- WEB fixtures cover all three labels, Channel, credential-only absence, a
  slashed Agent name, the directory filter, service forms, detail ACLs and
  diagnostics.
- CLI fixtures compare the human table with raw JSON and require the width
  selector on Agent. A terminal-width measurement placed the next column at
  display column 38 for the header, Agent and Service rows.
- Fast integration smoke exercises human Agent and Service labels and raw JSON
  kinds.

The targeted overlay run caught all **12 of 12** mutations
(`tmp/display-labels/mutations.log`): wrong or missing labels, an invented
unknown-kind glyph, a guessed credential-only glyph, raw kinds in the human
faces, the removed terminal-width selector, a changed Channel label, and glyphs
entering filter, registration or ACL values.

Final frozen `src/identity-display-labels-smoke.local.sh --slow`: **592 passed,
0 failed**, exit 0; vet and race passed (`tmp/display-labels/slow-final.log`,
lines 10–11 and 948). The frozen and tracked smoke scripts both have SHA-256
`4f14d2bca700b0e586ad7e45fda637533f131921fdeda16c00727bc1854eeb34`.
All 184 tracked and new source files matched the pre-run manifest.
Documentation validation checked 143 files and 2,805 local links with zero
errors.

## Limits

These labels describe identity type only. They say nothing about authority,
health or activity. They do not create parser syntax, and an older answer that
does not expose a caller-visible kind remains unlabeled rather than guessed.

## Live postflight

Commit `f92e277` was pushed before deployment. `src/build.sh` stamped all Go
programs as **0.5.54**, `parf@parf.us 2026-09-17 11:38:17`; the live
`agent-busd.service` restarted at 11:38 EDT.

The public identity and anonymous header reported 0.5.54. Human
`agent-bus ls -h` rendered the five caller-visible records as Agent and kept
the following columns aligned; raw `ls` retained `kind: agent` with no glyphs.
Signed-in WEB service and diagnostics pages rendered those same records as
Agent, while the directory rendered all four profile-backed rows as User. Its
filter value remained plain `users`, and no record was created or changed to
manufacture a live Service row; Service remains covered by the unit,
integration and mutation fixtures.

The web child retained zero effective capabilities, `NoNewPrivileges`, 256 MiB
memory, zero swap, one CPU and 64 tasks. OpenCode replied from the same session
after the restart, the eleventh consecutive measured peer-path reconnection.
