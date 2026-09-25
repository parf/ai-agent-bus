# Service and Channel journeys

📌 **TL;DR:** 0.5.78 completes distinct populated owner and visitor journeys
for Services, Queue channels and Pub/sub channels.

## Result

Channels now have their own document title, table vocabulary and canonical
detail route. Their mode filter remains plain URL state. Queue rows report held
work; Pub/sub rows report accepted work and subscriber count. Readers stays an
independent live observation.

Registration and ordinary edits return to the affected Channel. Exact list
state survives a detail visit. Operational facts remain readable to a caller
who may see the record, while edit controls continue to follow daemon-returned
management authority. Empty categories explain the entity and offer its
registration route; zero filter matches retain the filters and offer to clear
them.

## Checks

Focused tests use real core operations to create Queue and Pub/sub records,
hold queue work, fan out publications and establish a subscriber. They check
the distinct titles, columns, modes, work values, subscribers, filters, sort,
URL state, owner/visitor disclosure, registration/edit returns, Danger Zone
return and distinct empty states.

Real Chromium inspected the deployed-before screenshot and the corrected
current-source owner/visitor journeys at desktop and 375 px. Populated Queue
and Pub/sub pages, both details and the delivery-mode controls had zero page
overflow. The fixture changed no production record.

The corrected targeted mutation set catches **11/11** named breaks: wrong
Channel title, skipped mode predicate, queue-only pub/sub sorting, wrong
pub/sub work value, lost subscriber count, category/filter-empty conflation,
Service links or returns for a Channel, visitor editor disclosure, wrong Agent
title and lost mode URL state. The first sort mutation changed only one side of
the comparator and survived; it is retained without credit. The corrected
two-sided mutation fails the focused journey test.

Fast smoke passes **489/0**. The first frozen slow run passed **610/0**, then was
superseded after the stale-text sweep found two plan sentences still describing
the old Channel title/return defects as current. The corrected candidate was
frozen and its slow smoke also passes **610/0**, including vet and race. All
**400** corrected frozen files match after the run; the foreign `.gitignore`
edit and this measured evidence file were deliberately outside the manifest.
The tracked smoke script retains SHA-256
`a21ad6a6a022acecc28ec87162c2a009d2b9c9fe6c94362a0079915742bec1e5`.
Documentation validation checks **176 files** and **2,914 local links** with
zero errors.

## Scope

This completes F.13.3 without changing daemon, API or wire behavior. The
Overview/diagnostics journey, Users/Groups/Account journey, whole-redesign
acceptance and installed five-role acceptance remain open.

## Live postflight

Commit `fdb8538` was pushed and deployed as **0.5.78**, stamped
`parf@parf.us 2026-09-18 15:01:05`. Public identity reports that version,
build and the unchanged daemon Owner.

A fresh signed-in production session renders `/channels` with the Channels
document title and heading, delivery-mode filter, distinct no-channels
explanation and Channel registration link; it contains no Service table
heading. `/services` retains its Services title. Production had no visible
Channel record, so populated Queue/Pub/sub and visitor-detail claims remain
fixture-proven rather than inferred from the empty live page. No production
record was changed.

The restarted web child has zero effective capabilities, `NoNewPrivileges`,
256 MiB memory, zero swap, 64 tasks and one CPU in its delegated cgroup. A
post-restart AgentBus message to the OpenCode peer was accepted; delivery alone
is not credited as a peer reply.
