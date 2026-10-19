# Reader visibility

📌 **TL;DR:** 0.5.53 shows one live count for every outstanding consume request.

## Result

The daemon derives `readers` from the inbox's current waiter set. Filtered and
unfiltered consumes contribute to the same count; completion, cancellation and
timeout remove their wait. The optional wire value distinguishes unavailable
from a measured zero. It is cleared at registration and restore, and is never
durable state.

WEB, `agent-bus ls -h` and the MCP catalogue render the integer for every
visible record, including external-protocol records. The earlier `reading`
boolean remains on the JSON wire for compatibility and is not rendered by a
human face.

## Checks

- Core fixtures cover zero, filtered-only, mixed filtered/shared reads,
  matched delivery, cancellation and timeout.
- Registration, restore and snapshot checks reject fabricated or durable
  reader observations.
- WEB shows measured zero and nonzero counts, a filtered reader beside an
  unmatched backlog, an external protocol beside its independent count, and
  unavailable when an older answer omits the field.
- CLI smoke exercises filtered and unfiltered waits; MCP smoke pins the
  catalogue label.

OpenCode reviewed the original integer form and identified that an omitted
field would be rendered as zero by a newer MCP face. The correction made the
wire value optional while requiring a current daemon to attach an explicit
zero; OpenCode then approved the corrected core and all three renderers.

The targeted run caught all **13 of 13** final mutations
(`tmp/readers-53/mutations.log`).

| Changed behavior | Named catch |
|---|---|
| exclude filtered waits | filtered-only and mixed counts disagree |
| omit an explicit measured zero | idle liveness is unavailable instead of zero |
| retain a delivered read | the count does not fall after the matching message arrives |
| retain a cancelled read | the count does not fall after cancellation |
| retain a timed-out read | the timeout-only check still observes one reader |
| restore live reader state | the next snapshot contains fabricated `reading` and `readers` |
| keep a registrant's stated count | registration returns the caller's `99` |
| CLI invents zero for absence | unavailable is rendered as `0` |
| CLI hides a nonzero count | `2` is rendered as `0` |
| WEB invents zero for absence | unavailable is rendered as `0` |
| WEB hides a nonzero count | `2` is rendered as `0` |
| MCP invents zero for absence | missing data is rendered as `0 outstanding` |
| MCP hides a nonzero count | `2 outstanding` is rendered as zero |

The first mutation pass left `restore-live-reader-state` alive because the new
test looked for lowercase `records` while `ports.Snapshot` serializes
`Records`. The helper now requires the exact decoded shape before inspecting
fields; the retained initial log receives no final mutation credit. The second
pass made the web nonzero mutant fail to build through an unused import. Its
replacement keeps the import live and fails the behavioral assertion; that
intermediate log also receives no credit.

Final frozen `src/reader-visibility-smoke.local.sh --slow`: **590 passed, 0
failed**, exit 0; vet and race passed (`tmp/readers-53/slow-final.log`, lines
10–11 and 946). The frozen script and tracked `src/smoke.sh` both have SHA-256
`30f96b5795bd286099f70ed4f41ab0b39a3f74401d807402127718995bc56efc`.
All 182 tracked/new source files matched the pre-run manifest
(`source-frozen.sha256` and `source-verify.log`). Documentation validation
checked 138 files and 2,821 local links with zero errors.

## Limits

This is an instantaneous count of outstanding requests. Zero does not mean a
service is dead; a process may be between reads. A positive count promises no
message match and no completed work. It counts neither processes nor sessions.

## Live postflight

Commit `7efb957` was pushed before deployment. `src/build.sh` stamped every Go
program as **0.5.53**, `parf@parf.us 2026-09-17 11:15:12`; the live
`agent-busd.service` restarted at 11:15 EDT.

The public identity and anonymous header reported 0.5.53. A raw authenticated
listing stated `readers` on all five visible records: four measured one and one
measured zero at that instant. The human CLI rendered the same numeric column.
Signed-in WEB service and diagnostic pages rendered the Readers column, its
bounded explanation and both zero and nonzero values. The confined web child
remained in its delegated cgroup with the 256 MiB memory, zero-swap, one-CPU and
64-process limits.

A fresh MCP catalogue process against the live daemon rendered `readers: 1
outstanding` for each of its two visible records. Existing launcher sessions do
not reload their already-running MCP module when the daemon restarts; the
current Codex sidecar therefore retained the earlier boolean wording until its
next launcher restart. This is a process-lifetime deployment boundary, not
credit for a live reload that did not occur.
