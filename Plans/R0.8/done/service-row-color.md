# Service row color key

📌 **TL;DR:** My and Personal links now key the owned and Personal row colors;
the repeated Yours word is gone, and Personal remains the stronger treatment.

## Result

The **My** link and caller-owned rows use blue. Owned rows retain a leading
rule and semibold linked name without repeating **Yours**. The **Personal** link
and Personal rows use orange and bold weight; the visible Personal word stays.
When both facts apply, the later Personal rule overrides the owned treatment.
URLs, active-link state, access, ordering and detail labels are unchanged.

This owner-requested visual refinement does not bump the shared version. It
supersedes only the row-label presentation recorded in the 0.5.66 and 0.5.64
evidence; those histories remain unchanged below their supersession notes.

## Checks

Package and full Go tests passed. Targeted mutations caught **7/7** removals or
reversals: both link classes, the no-Yours rule, blue owned-name emphasis,
Personal-over-owned CSS order, the retained Personal word and its stronger
weight.

Real Chromium at 375 px rendered current source against the live read-only
daemon. It measured the My link as blue, Personal as orange at weight 700 and
the owned `claude/ab-dvp@parf.us` name as blue at weight 600 with no Yours;
the page did not overflow. The first browser run also required that live row to
remain external. Current live data no longer stated that unrelated fact, so the
run failed and received no credit; the corrected visual check removed that
assumption rather than changing product behavior.

Fast smoke passed **488/0**. Final slow smoke passed **609/0**, including vet
and race. All **205** frozen source hashes remained unchanged. The tracked and
frozen smoke scripts both had SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.
Documentation validation checked **158 files** and **2,859 local links** with
zero errors. OpenCode reviewed the corrected source and found no issue.

## Live postflight

Commit `8a4c6bd` deployed without changing 0.5.67. Public identity reported
build `parf@parf.us 2026-09-17 21:12:12`. The first unprivileged systemd
restart request timed out before stopping the old process; status and public
identity proved the old build was still serving, so it received no deployment
credit. The privileged retry started a new unit at 21:13.

Real Chromium at 375 px measured the live My link as blue, Personal as orange
at weight 700 and the owned `claude/ab-dvp@parf.us` name as blue at weight 600
with its leading rule and no Yours. The page did not overflow and no record was
changed. The web child retained its private PID namespace, zero effective
capabilities, `NoNewPrivileges`, two-value environment and installed resource
limits. OpenCode confirmed same-session AgentBus reconnection through the
successful restart.
