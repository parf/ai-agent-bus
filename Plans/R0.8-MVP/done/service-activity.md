# Service activity graphs

📌 **TL;DR:** 0.5.67 embeds honest record-scoped Activity on Service and
Channel detail and gives the compact and complete views one graph definition.

## Result

Detail performs one activity read only after the caller-visible record is
loaded. Failure leaves the detail readable with an unavailable statement;
hidden and missing records never reach Activity. The compact section links to
the same named scope on the complete page, where the sample-value table remains.
No message body is read or rendered.

One combined inline SVG uses actual sample timestamps on its x-axis and one
maximum across the displayed nonzero series. Accepted is solid blue, Dequeued
is dashed orange, and loss/refusal series have labelled colors and line shapes.
A single partial sample is a visible point. Zero-only series are summarized,
while no retained sample says collection has only just started after restart.
Both states include the available restart, uptime, window and partial-sample
context; sampled-history definitions sit behind an accessible native help
button on both pages.

The named view is record-scoped. On the unfiltered view, accepted, dequeued,
dropped and expired cover currently visible records; Refused is node-wide for
the daemon Owner and configured masters and visible-record scoped otherwise.

## Checks

Focused tests pin irregular 10/40-minute x-spacing, one shared scale with a
smaller second series, identical detail/full series, the visible single point,
zero versus unobserved history, compact versus complete content, one-attempt
failure degradation, caller-dependent refusal scope and hidden-record
non-disclosure. The full Go suite passes.

Real Chromium at 375 px rendered the current-source web against the live daemon
without changing it. A quiet `claude/ab-dvp@parf.us` detail used the measured
zero state. Active `opencode/oab@parf.us` detail and complete Activity showed
Accepted and Dequeued over the same scale, retained the filter and had no
page-level overflow. Visual inspection caught a CSS shorthand overriding legend
colors; the corrected render showed solid blue Accepted and dashed orange
Dequeued in both graph and legend.

The targeted mutation run caught **15/15** named breaks: index-based time,
per-series scaling, plotted zero series, hidden single-sample data and marker,
zero/absence conflation, detail table leakage, retry after failure, removed
section, removed complete table, false completion wording, hidden refusal scope,
removed zero summary, missing detail help and a misrouted filtered link. OpenCode reviewed the
corrected source and found no issue.

Fast smoke passed **488/0**. Final slow smoke passed **609/0**, including vet,
race, MCP and launcher checks. All **205** frozen source hashes remained
unchanged. The tracked and frozen smoke scripts both had SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.
Documentation validation checked **157 files** and **2,858 local links** with
zero errors.

## Live postflight

Commit `a6d624e` deployed as 0.5.67 with build
`parf@parf.us 2026-09-17 20:57:12`. Public identity reported the same version
and build; anonymous `/ls` remained 401.

Real Chromium at 375 px rendered the live `opencode/oab@parf.us` detail and
complete Activity views. The detail showed one combined chart with actual
observed timestamps, a shared 0–1 scale, labelled nonzero series, an explicit
zero-series summary, closed/open native help and the filtered complete-view
link. The complete view retained that filter and graph. Neither page overflowed
horizontally, and no live record was changed.

The web child remained in its private PID namespace with zero effective
capabilities, `NoNewPrivileges`, its two-value environment and the installed
memory, swap, process and CPU limits. OpenCode confirmed same-session AgentBus
reconnection through the restart.

## Limits

This slice does not complete the planned Overview, remaining envelope
diagnostics, installed role/action acceptance, or longer/persistent history.
It adds no activity endpoint, schema, timer or script.
