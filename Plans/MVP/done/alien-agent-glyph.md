# Agent glyph

📌 **TL;DR:** 0.5.61 uses `👾` for Agent in WEB and human CLI output.

## Result

The shared daemon-kind display function maps Agent to `👾 Agent` and exposes
`👾` for compact WEB name prefixes. WEB forms, lists, details, diagnostics and
the identity directory use that one mapping. Human `agent-bus ls -h` uses the
same label with its terminal-width compensation.

Raw CLI JSON, API kinds, MCP output, URLs, form values, CLI arguments and ACL
expressions remain plain. The service-script guide uses the Service glyph
rather than using the former Agent glyph for a service.

## Checks

Shared display, WEB and CLI tests require `👾`, retain daemon-stated-kind and
absence rules, and prohibit glyphs in machine values. Targeted mutation and
frozen repository results are recorded after verification.

The targeted run caught **4/4** named changes: restoring the former compact
glyph, restoring the former full label, removing CLI width compensation and
hardcoding the former glyph in the WEB form. `wcwidth` measured both `👾️` and
`⚙️` as two terminal columns, preserving the alignment premise.

Two exploratory fast-smoke runs each reported **484 passed, 4 failed** and
receive no credit. The human-list fixture still required the retired glyph; the
first exposed that stale expectation, while a path error left it unchanged for
the second. After correcting the tracked fixture, fast smoke passed **488/0**.

Final frozen `src/agent-glyph-smoke.local.sh --slow`: **609 passed, 0 failed**,
exit 0; vet and race passed (`tmp/agent-glyph/slow-final.log`, lines 10–11 and
965). All 198 source/version manifest entries matched after the run. The frozen
and tracked scripts both have SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.

The final build reported 0.5.61 with stamp `parf@parf.us 2026-09-17 15:56:14`.
