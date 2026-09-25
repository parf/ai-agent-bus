# Web performance, accessibility and migration acceptance — F.13.6

📌 **TL;DR:** A real Chrome drives every web page at desktop, 200% zoom and a
420 px phone against a populated, an empty and a bus-less fixture. Bus calls per
page are constant in directory size. Unnamed tables and a users-by-records scan
in the daemon's Users listing were fixed. Each of the four named mutations fails
its check. The installed rerun stays with F.12.

Completed requirement: F.13.6, whose row moved to [DONE](../DONE.md#done--mvp).
Current contract: [browser acceptance](../../../docs/05-discovery.md#browser-acceptance).
The installed role/action and restart rerun is
[F.12](../TODO.md#remaining-work).

## Gate

`src/acceptance/web-acceptance.py BUILT_DIR NEW_EVIDENCE_DIR [BASE_PORT]`
starts its own disposable daemons (`agent-busd -create` with their own `-db`
and ports from `BASE_PORT`, 35000 by default) and a separate `agent-bus-web` for
each. It never touches an installed daemon. The web reaches its daemon only
through a counting Unix-socket proxy, so **bus calls and bus bytes are counted
apart from browser requests and bytes**. Each wait has a bound, a crash counts as
a failure, and any `FAIL` line makes the exit non-zero. It writes
`measurements.json` to the evidence directory.

| Fixture | Contents |
|---|---|
| populated | 500 each of agents, queues, pub/sub topics, services, Users and Groups (three members each), and 80 messages so Activity and Diagnostics have data. A second seeding brings agents to 2,500 for the larger benchmark |
| empty | a fresh daemon holding only its Owner |
| error | the populated web face with its bus taken away: the proxy stops listening and closes its connections |

| Size | Browser context |
|---|---|
| desktop | 1440×900 |
| zoom200 | 640×450 CSS px at scale 2, which is a 1280 px window at 200% zoom |
| narrow | 420×860, mobile viewport, touch, scale 2 |

## Checks

769 checks. The last green run had 0 failures in 95 s.

| Check | What it measures | On |
|---|---|---|
| renders | HTTP 200, and signed in rather than sent back to sign-in | 17 pages plus 7 detail pages (1 when empty), × 3 sizes × 2 fixtures |
| reflow, no horizontal page scroll | `documentElement.scrollWidth` ≤ `clientWidth` | every page, every size, every fixture, error pages included |
| viewport follows the device | `meta[name=viewport]` has `width=device-width` | narrow |
| measured text contrast meets WCAG AA | every visible element with its own text, every form control and SVG `<text>`. Foreground from computed `color`/`fill` × ancestor opacity; background composited from the ancestors' computed background colours over white. Needs 4.5:1, or 3:1 for large text | desktop and narrow, every fixture |
| every table and graph has an accessible name | `<table>` has a `<caption>`, `aria-label` or resolving `aria-labelledby`; each non-hidden `<svg>` has a name; tables have header cells | desktop |
| a graph is drawn and labelled | the populated Activity chart is `role=img` with a name | desktop |
| keyboard | the token field has focus on arrival, sign-in by typing and Enter, first Tab is the skip link, and the next Tab after it is inside `main`. Tab order on `/agents`, `/users`, `/groups`, `/queues/new`, `/activity` and `/diagnostics` reaches all nine navigation entries, the search field, the selects, a row link, a form field and submit. Every focused control has a visible outline or shadow and is scrolled into view. Search typed plus Enter narrows to one row. A queue registered by keyboard alone opens its page | desktop |
| legacy entry points | `/channels`, the same with a query, `?kind=pubsub` with and without a query, `/channels/new` and its `?kind=pubsub`, `/channel` and `/channel/edit` for a queue and a topic, `/users?kind=other` → `/diagnostics#leftovers` and `/users?kind=users` each land on the new address with its heading. `/healthz`, `/ui.js`, `/favicon.svg` and `/agent-bus.jpg` answer 200, and `/favicon.ico` answers its deliberate 404 | desktop |
| bus unavailable | 502, "The bus is not answering" in `main`, no address or socket name, no horizontal scroll, AA contrast | 8 pages × 3 sizes |
| budgets | [below](#budgets) | desktop |
| cost does not grow with rows | populated minus empty bus calls ≤ 0, and populated browser requests ≤ empty ones, on the 7 listings, Overview, Diagnostics and Activity. `/agents` makes the same number of bus calls at 500 and 2,500 agents, and a search at 2,500 finds one row at that same cost | desktop |

## Conditions

| | |
|---|---|
| Host | AMD Ryzen 9 5900X, 24 threads, 125.7 GiB, Linux 7.2.4-200.fc44, load ≈1.1–1.9 (other agents working) |
| Browser | Google Chrome 153.0.8010.36 through Playwright, headless |
| Build | 0.8.26 (this change merged over main at 0.8.25), `build.sh` output; the merged build reran green, 769/0 in 96 s |
| Data | 3,000 records plus 500 Users, SQLite 1.1 MB. Seeding took 20 s; the 2,000 extra agents took 13 s |
| Timing | Navigation Timing `responseEnd − requestStart`, median of 5 loads, loopback |

## Measurements

Desktop, populated (500 of each) and empty:

| Page | Requests | Browser KB, 500 / empty | Bus calls, 500 / empty | Bus KB, 500 / empty | Server ms, 500 / empty |
|---|---|---|---|---|---|
| `/` | 4 | 29 / 29 | 5 / 5 | 889 / 2.0 | 26 / 2 |
| `/agents` | 3 | 44 / 28 | 5 / 5 | 889 / 2.0 | 25 / 2 |
| `/services` | 3 | 40 / 27 | 5 / 5 | 889 / 2.0 | 24 / 2 |
| `/queues` | 3 | 42 / 28 | 5 / 5 | 889 / 2.0 | 25 / 2 |
| `/pubsub` | 3 | 41 / 27 | 5 / 5 | 889 / 2.0 | 24 / 2 |
| `/personal` | 3 | 28 / 28 | 5 / 5 | 889 / 2.0 | 23 / 2 |
| `/users` | 3 | 41 / 27 | 6 / 6 | 1081 / 2.4 | 28 / 2 |
| `/groups` | 3 | 223 / 26 | 5 / 5 | 932 / 2.0 | 32 / 2 |
| `/activity` | 3 | 202 / 45 | 5 / 5 | 901 / 14.4 | 31 / 4 |
| `/diagnostics` | 3 | 69 / 31 | 6 / 6 | 1097 / 2.4 | 34 / 2 |
| `/account` | 3 | 306 / 27 | 6 / 6 | 1081 / 2.6 | 45 / 2 |
| each `/…/new` form | 3 | 26–30 / same | 3 / 3 | 1.2 / 1.2 | 3 / 2 |
| a record page (`/agent`, `/queue`, `/pubsub/topic`, `/service`) | 3 | 30 | 8 | 250 | 10 |
| `/group?name=` | 3 | 25 | 5 | 932 | 26 |
| `/user?name=` a User / the Owner | 3 | 27 / 171 | 6 | 1081 | 28 / 37 |

A bus-less page is 3 requests, 25 KB and 0 bus calls.

At 2,500 agents (5,000 records), the median server times were: `/agents` 42 ms,
`/users` 49 ms, `/account` 68 ms, `/diagnostics` 45 ms, `/groups` 44 ms and a
record page 13 ms. `/agents` still made 5 bus calls, reading 1.5 MB from the bus.

## Budgets

Set from the measurements above; the constants are at the top of the gate.

| Budget | Value | Measured |
|---|---|---|
| Browser requests per page, every fixture | ≤ 4 | 4 max (Overview) |
| Browser bytes per page, 500 rows | ≤ 400 KB | 306 KB max (`/account`) |
| HTML of one paginated listing page | ≤ 60 KB | 44 KB max |
| HTML of the unpaginated Groups page, 500 groups | ≤ 300 KB | 222 KB |
| Bus calls per page, every fixture | ≤ 8 | 8 max (record page) |
| Bus calls populated minus empty | ≤ 0 | 0 on every page |
| Median server time, 500-row listing | ≤ 250 ms | 31 ms max |
| Median server time at 2,500 agents | ≤ 500 ms | 68 ms max |

## Defects fixed

| Defect | Fix | Named check |
|---|---|---|
| Five tables had no accessible name: Activity slot values, Account credentials, Diagnostics loss and leftovers, and Group "used by visible records" | each is labelled by its section heading, or given an `aria-label` where no heading exists | web `TestEverySignedInPageIsTitledUniquelyAndCarriesItsShell` (every table on 32 routes, with a positive control that each fixed table was drawn); gate "every table and graph has an accessible name" |
| The daemon's Users listing rescanned every record for every User: 119 ms for `/users` at 500 Users and 3,000 records, paid by `/users`, `/diagnostics`, `/account`, `/user` and every record page | one index of memberships (the same nested-group walk) and live owned records is built per answer. 2.6 ms | `core` `TestUsersListingMatchesTheSingleUserViewForEveryRow` (each row equals the single-User view across nesting, a cycle, the Administrator group and an inactive owner) |

Killed mutants of the fixes, each failing on an assertion: each of the five
table labels removed; nested groups dropped from the index; an inactive owner's
records counted; a User's own record counted; memberships not read from the
index.

## Mutations

Each mutation was built with `build.sh` and run under the full gate on its own.

| Mutation | Result |
|---|---|
| viewport meta removed | 42 FAIL "viewport follows the device", every narrow page on both fixtures |
| phone reflow block (`@media (max-width:40rem)`) disabled | 46 FAIL "reflow, no horizontal page scroll": 42 narrow pages and 4 at 200% zoom |
| secondary text `--text-2` raised from `#56544c` to `#8f8d85` | 100 FAIL "measured text contrast meets WCAG AA", desktop and narrow, all three fixtures |
| legacy `/channels?kind=pubsub` no longer recognised | 2 FAIL "legacy /channels?kind=pubsub lands on /pubsub", with and without a query |
| one `/lookup` per listed record added to the directory read | 23 FAIL: "bus calls do not grow with rows" (6 listings), "bus calls within budget" (8), "bus calls equal at 500 and 2500 rows", and the server-time budgets (8) |

## Findings

These were measured and bounded, but not changed. Each is a page-design choice
for the owner rather than a defect:

| Finding | Measured |
|---|---|
| Groups, Account, the Owner's User page and Activity are not paginated, so their HTML grows with the directory | about 0.4 KB per group; `/account` and the Owner's `/user` list every owned record (306 KB and 171 KB at 3,000) |
| Every listing reads the whole visible directory from the bus and paginates in the web child. The number of calls is constant; the bytes are not | 889 KB at 3,000 records, 1.5 MB at 5,000 |
