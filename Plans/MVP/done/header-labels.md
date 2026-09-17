# The header says less than it measures — 0.5.40

> At 0.5.41 the [bus logo spans identity and navigation rows](header-logo.md#header-bus-mark--0541). The labels and build handling recorded below remain.

Presentation only. No counter, sampler or API change; `GET /identity` answers
exactly what it answered at
[0.5.39](node-calls.md#a-node-publishes-the-calls-it-has-served--0539).
Contract: [what a node says about itself](../../../docs/05-discovery.md#what-a-node-says-about-itself).

## What the owner changed

| Was | Is |
|---|---|
| `min: 31 (observed 1m13s) ; hr: 73 (observed 5m13s)` | **`uptime`**`: 5m13s` … **`calls`**`: minute: 31 ; hour: 73 ; total: 73` |
| footer: two build lines, repeated web version, an "About call counts" block | footer: **one build line**, nothing else |

The owner's reason for dropping the spans: *"no junk info - we have uptime"*.

## The deliberate imprecision

`observed` was not decoration. A window begins at the newest reading at or
before its cutoff, so `minute:` usually covers **more than a minute** — and the
header no longer says how much.

That was put to the owner before it was built, with three ways to narrow the
gap offered (nearest-sample selection, finer sampling, or accept it). The answer
was *"ok, still call it a minute"*. codex chose the same option independently
and for a better reason than mine: nearest-sample selection would change **which
requests count**, not merely how precisely they are counted.

So the page is knowingly less precise than the data behind it, in exactly one
place. The measurement is unchanged and `observed` still travels on the wire, so
the answer remains available to anything that asks `/identity` — it is simply
not on the dashboard. Recorded as a decision so that "why is a minute not a
minute" has an answer in six months.

## Builds

One string when the daemon's and the web child's builds match; both, labelled,
when they differ. The owner thought divergence impossible; it is not
(`supervisor.go:102` launches a separate binary from beside the daemon), so the
common case is the single line they asked for while a partial upgrade stays
visible. A missing daemon build falls to the split branch rather than rendering
an empty line.

## What proves it

`src/smoke.sh --slow`: **584 passed, 0 failed**, exit 0, vet and race green
(`tmp/header-40/slow-final.log:925`), from a frozen byte copy sharing the tracked
script's SHA-256 — `75b94013…f5fac`.

**9 isolated mutations, 9 caught**, each by
`TestLoginUsesOnlyPublicDaemonFactsAndSeparatesBuilds`
(`tmp/header-40/mutations.log`): `restore-observed`, `restore-short-labels`,
`remove-uptime-label`, `remove-calls-label`, `bold-numbers`, `restore-about`,
`duplicate-matching-build`, `hide-mismatched-build`, `repeat-web-version`.
Package-scoped, not full smoke per mutant. An earlier six-mutation run completed
against the previous footer. Its full smoke run was terminated when the scope
changed; neither is evidence for this final scope.

## Three wrong claims of mine, caught in source

| I wrote | Why it was false |
|---|---|
| a window is "never less than its name, never exactly it" | a node up thirty seconds has `hour:` covering thirty seconds; and the span matches exactly when the cutoff falls on a retained reading |
| `minute:` covers one to two minutes "permanently" | a delayed tick widens it and a young node shortens it; the cadence is not a guarantee |
| the footer "says so" about observed spans | there is no longer any footer text at all |

The first is the one worth keeping: told that a universal was unsupported, I
replaced it with a different universal. Removing the claim was the fix.
