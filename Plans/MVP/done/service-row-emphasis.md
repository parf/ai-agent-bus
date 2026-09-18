# Service row emphasis

📌 **TL;DR:** 0.5.66 makes Yours and Personal rows easy to scan without
conflating the facts, removes the duplicate Edit destination and compacts list
update time.

## Result

Service and Channel collections derive **Yours** only from the returned Owner
matching the signed-in identity. Those rows have a blue leading rule, a
semibold linked name and the visible Yours word. Personal comes only from the
returned Personal tag: those rows have an orange semibold name and the visible
Personal word. A caller-owned Personal row keeps both; a Personal record owned
by somebody else never gains Yours. Protocol/external hints affect neither.

The linked name is the single route to read-first detail. The former Controls
column and duplicate Edit link are gone; controls on detail still follow the
daemon's returned management authority. List update time is relative below 30
days and compact calendar text afterwards. Detail keeps the full timestamp;
missing time remains unavailable.

## Checks

Package and full Go tests cover the ownership/Personal matrix, a remote owned
record, visible non-colour words, detail authority for owner, Maintainer and
viewer, absence of the duplicate column, zero and future times, each relative
unit, the exact 30-day boundary, same-year and cross-year calendar forms.

Real Chromium at 375 px rendered the live remote
`claude/ab-dvp@parf.us` row through a temporary current-source web child. It
verified the blue leading rule, font weight 600, visible Yours word, external
fact, compact update value, absence of Controls/Edit and no page-level
horizontal overflow. It made no registry change.

The targeted mutation run caught **15/15** named breaks: lost or conflated row
facts, missing marker words, remote ownership tied to protocol, each visual
treatment removed, the duplicate column restored, and every compact-time
boundary changed. Fast smoke passed **488/0**. Documentation validation checked
**155 files** and **2,855 local links** with zero errors. OpenCode reviewed the
source and found no issue. Final slow smoke passed **609/0**, including vet and
race. All **204** frozen source hashes remained unchanged. The tracked and
frozen smoke scripts both had SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.

## Limits

This slice does not implement description search, sorting, paging, owner
photos, activity placement or the remaining service-detail redesign.

## Live postflight

Commit `6d5c4ba` was built and deployed as 0.5.66 with matching daemon/web
build `parf@parf.us 2026-09-17 20:31:16`. Public identity reported the new
version and anonymous daemon status remained 401. Live Chromium repeated the
375 px assertion on `claude/ab-dvp@parf.us`: blue ownership rule, weight 600,
visible Yours, external kept separately, `2d ago`, no Controls/Edit and no
page-level overflow. No live record changed.

The web child retained zero effective capabilities, environment
`AGENT_BUS_ADDR=/bus.sock` plus `PWD=/`, and its 256 MiB memory, zero-swap,
64-process and one-CPU limits. The peer AgentBus path reconnected after the
restart.
