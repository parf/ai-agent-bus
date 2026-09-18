# Group editors and numeric tables

📌 **TL;DR:** Group members use the same one-term-per-line textarea as ACL and
Maintainers editors, and numeric web-table columns align right.

## Result

Every editable Group renders its direct members one per textarea line. The
registration and existing-group forms share the same parser and plain syntax;
identity and Group glyphs never enter submitted values.

One shared numeric class right-aligns both headings and values and uses tabular
figures. It covers Service Readers and Queued, Activity samples, refusal
counts, backlog Readers/held/age, registry and loss counters, and retained
envelope counts. Figures embedded in prose remain inline.

This owner-requested web refinement does not bump the shared version. It ships
with the already committed but not yet deployed 0.5.68 tree.

## Checks

Web package tests exercise multiline member parsing and round-trip rendering,
the absence of the old single-line control, the shared numeric style, and
numeric headers and values across Services, Activity and every Diagnostics
table. Five targeted mutations remove or flatten the textarea, reverse the
alignment, or remove numeric classes from independent tables; all five fail.

Real Chromium at 375 px rendered current source against the live read-only
daemon. It found both Group textareas, two direct Administrator members on two
lines, an empty `@sample-group`, no legacy group name, and right-aligned
tabular Readers and Queued columns without page overflow. The first two browser
attempts receive no credit: the first assumed request-context sign-in state;
the second used a loopback address without the required `http://` prefix, so
the client correctly treated it as a Unix-socket path. The final run exercised
the browser form and corrected endpoint.

OpenCode independently reviewed the numeric inventory and line-list behavior
and found no missed table surface or divergent parser.

Fast smoke passes **488/0**. Full slow smoke passes **609/0**, including vet and
race, with all eight frozen web source/test hashes unchanged. Documentation
validation checks **161 files** and **2,864 local links** with zero errors.

## Live data cleanup

Before the web change, the live daemon reported `@administrators-legacy` with
no members and no visible ACL or Maintainers reference. With the daemon stopped,
the snapshot key was renamed atomically to `@sample-group`, preserving file
ownership and mode. After restart, `/groups` reported `@administrators` and the
empty `@sample-group`; the legacy name was absent. This changes no group
deletion rule and grants no authority.

## Live postflight

Commit `35f4fe4` deployed in the same restart as the already committed 0.5.68
fresh-install slice. Public identity reports 0.5.68 with build
`parf@parf.us 2026-09-17 21:59:07`. Real Chromium at 375 px repeated the source
checks against port 6780: the two line-list editors, empty `@sample-group`,
absent legacy name, right-aligned tabular Readers and Queued values, and no page
overflow.

The web child remains inside its private PID namespace with zero effective
capabilities and `NoNewPrivileges`; its live cgroup retains 256 MiB memory,
zero swap, 64 tasks and one CPU. The development node intentionally continues
to select the git-install checkout. The standalone package/install path is
proved by the fresh-host acceptance rather than by replacing this node's
development layout.
