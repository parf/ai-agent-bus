# Group detail and GitHub availability

📌 **TL;DR:** 0.5.75 gives Groups a compact linked list and authority-scoped
detail editor, and saves a valid unique GitHub login even when optional public
profile metadata is unavailable.

## Result

`/groups` is a two-column Group/Members table. A Group name opens
`/group?name=`, where Administrators edit ordinary membership and only the
daemon Owner edits `@administrators`. Ordinary callers receive the existing
hidden-membership statement rather than an empty-membership claim. Registration
opens the new Group detail. Group retirement, nesting, authority and the
runtime-only `@owner` ACL term are unchanged.

GitHub login is an identity attribute that must remain valid and unique.
Optional provider metadata does not decide whether that attribute may be saved:
an unavailable adapter or failed public-profile lookup leaves provider facts
unobserved and commits the login. Explicit **Refresh GitHub profile** still
reports provider failure without changing the stored profile. A later successful
refresh fills permitted blank facts and provider metadata under the existing
uniqueness and trusted-adapter rules.

## Checks

Focused core and web tests cover the linked table, registration redirect,
missing detail, hidden membership, ordinary-group Administrator editing,
Owner-only protected-group editing, line-list round trip, provider failure,
missing adapter, explicit-refresh atomicity and later successful refresh.

Real Chromium at desktop and 375-pixel widths rendered current source against
the live read-only daemon. The list had no inline textarea or horizontal
overflow; linked detail retained its editor and current Groups navigation.

The first mutation run caught **6/8** and receives no final credit. It exposed
two test hollows: Group detail checked hidden membership but the list did not,
and the unfetched-provider UI mutant was aimed at an unrelated registration
test. After adding the list assertion and selecting the populated User-detail
test, the corrected **8/8** set catches removal of the detail link, protected
editing widened to Administrators, hidden membership labelled empty, the old
registration return, either provider-failure path blocking login, explicit
Refresh swallowing failure and removal of the unfetched label.

Fast smoke passes **489/0**. Documentation validation checks **173 files** and
**2,908 local links** with zero errors. Final slow smoke passes **610/0**,
including vet, race and launcher/MCP checks, with all **236** frozen candidate
files unchanged. The smoke runner SHA-256 is
`a21ad6a6a022acecc28ec87162c2a009d2b9c9fe6c94362a0079915742bec1e5`.
This measured result was filled into the evidence after the immutable run.

## Boundaries

This advances the remaining F.13.4 journeys. It does not expose group
references, complete role/browser acceptance, make provider metadata mandatory,
or weaken GitHub-login uniqueness. Page rendering still never calls GitHub.

## Live postflight

Commit `1bf7790` was built and deployed as 0.5.75 with build
`parf@parf.us 2026-09-18 13:51:22`. The live Groups page renders one linked
Group/Members table with no inline editor; `@sample-group` opens its editable
detail with Groups current in navigation.

The previously refused live update now saves `github_user=parf` for
`parf@parf` with a 303 redirect. User detail labels public provider data
unfetched and shows no lookup error. An explicit Refresh still returns 400 for
the unavailable lookup and preserves both the login and the unfetched state.

The web child retains zero effective capabilities, `NoNewPrivileges`, its
explicit `AGENT_BUS_ADDR=/bus.sock` and `PWD=/` environment, and its 256 MiB
memory, zero-swap, 64-task and one-CPU limits.
