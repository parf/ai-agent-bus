# Compact web registry and service detail

📌 **TL;DR:** 0.5.72 applies the selected Option 2 visual structure with Option
1 operational content, compacts service and user pages, and keeps about one day
of Activity at ten-minute intervals.

## Result

The Services page now uses the selected broad, compact operational layout.
Descriptions lead and routing names appear once; search covers name, owner and
description; Kind and Delivery are URL-backed links; sort is a URL-backed
selector. My records keep blue ownership treatment, Personal records use the
stronger orange treatment, and Personal wins when both facts apply. Delivery
uses its established status glyphs. Numeric values stay right-aligned.

The shared header reads `AgentBus v<release> @ <host>`. The Version hover/focus
tooltip carries the daemon build, while Owner, Uptime and Calls live in the
compact footer. The footer does not repeat the version or build.

Service detail keeps its measured facts visible and moves the longer Delivery,
Policy, Observed and Activity explanations into immediate hover/focus tooltips
and structured native click popovers. Settings, classification, Maintainers and
subscriptions are collapsed disclosures unless a refused form needs reopening.
The User editor uses a responsive field grid. It explains that SSH public keys
are installed through `agent-bus-admin user add <user@realm> <key.pub>` and does
not invent a profile field the daemon cannot store.

The two compact selectors submit their GET forms on change through one local
same-origin script. URLs remain the state, full pages remain the response and a
`noscript` Apply control remains. The script neither reads credentials nor
handles editable profile, ACL or configuration data.

Activity history keeps at most 145 in-memory samples: a baseline plus about 24
hours at a ten-minute process-driven cadence. The page states the configured
cadence, the measured timestamps and the shorter final interval separately.
History still starts empty after restart and remains process-local.

## Checks

The complete Go test suite passes. Corrected fast smoke passes **488/0**. The
first fast smoke's **483/5** receives no credit: all five failures were stale
header/footer assertions from the superseded layout; the harness now checks the
Version build tooltip and the Owner/Uptime/Calls footer.

Twelve targeted mutations fail independently: one-minute cadence, one-hour
window, either selector losing change submission, the local script no longer
submitting, a fabricated SSH field, loss of the immediate service tooltip,
expanding an editor into the page, a duplicated routing name, loss of the build
tooltip, loss of the Delivery glyph and repeated all-zero Activity copy.

Real Chromium compared the selected source image with the implementation at
1487 × 1058, exercised both automatic selectors, hovered and opened the service
help, and checked the service detail at 375 px. Both desktop and narrow detail
had zero page-level horizontal overflow and the console had no messages. The
full visual record is in [design QA](../../../design-qa.md#design-qa--compact-agentbus-web-redesign).

Lighthouse snapshot checks on the populated Services page report 100 for
accessibility and 100 for best practices. Documentation validation checks **170
files** and **2,893 local links** with zero errors.

Final slow smoke passes **609/0**, including vet, race and the MCP/runtime
checks, with all **33** candidate source, test, documentation and evidence
hashes unchanged. The tracked smoke script and the frozen candidate both have
SHA-256 `3a86c82453ac06c77bbe325afd60adc72f4ac53526b12958e5f3420a628d5137`.

## Live postflight

The 0.5.72 restart reports the stamped build `parf@parf.us 2026-09-18
02:21:16`; both supervisor and bus process titles carry 0.5.72. Read-only,
authenticated production requests verify the four immediate service-detail
tooltips, collapsed editors, Delivery glyph, both automatic selectors and the
ten-minute / 24-hour Activity wording.

The web child still runs with no effective capabilities and
`NoNewPrivileges`; its delegated cgroup retains `memory.max=268435456`,
`memory.swap.max=0`, `pids.max=64` and `cpu.max=100000 100000`. Postflight did
not apply pressure or change registry data. The AgentBus accepted the
deployment notice for the OpenCode peer after restart; delivery alone is not
credited as a peer review or reconnection confirmation.

## Scope

This completes the owner-selected Services visual direction and the Activity
horizon change. It advances F.13.3 and F.13.5 without claiming the unfinished
reader-state filter, pagination, remaining service/channel journeys, Overview,
remaining diagnostics, `templ` migration or installed browser matrix.
