# Web title marks and compact help

📌 **TL;DR:** 0.5.65 gives every current web page a category-marked title and
moves the largest collection definitions behind accessible native help.

## Result

Every current `h1` starts with one fixed decorative image or glyph and retains
its visible title. Section pages and registration inherit their section mark;
Service, Agent and Channel detail choose from the daemon-stated record kind.
An absent or unrecognised record kind uses the Service section mark rather than
inventing an entity type. All SVGs are inline and repository-owned. No caller
text enters trusted markup, and URLs, JSON, filters, forms and ACL syntax stay
plain.

Services, Channels, Personal and Users now show a visible `ⓘ` button beside
the title. It opens a native, script-free popover with an accessible name,
heading and short bullets. Definitions and observation limits moved there;
current scope, filter state and decision-time constraints remain in the main
flow.

## Checks

Package tests cover every title category, decorative accessibility, fixed-only
markup, daemon-kind selection, unknown fallback, public sign-in, problem and
confirmation pages, the two popover controls, headings and bullets, and the
absence of the former Services and Users prose walls. Full Go tests pass. A
real Chromium run at 375 px covered 13 routes, verified the help panels start
closed and open from their buttons, and found no page-level horizontal
overflow.

The targeted mutation run caught **14/14** named breaks covering trusted-markup
injection, focusable decoration, wrong or missing title categories, ignored
daemon kind, missing popover semantics or accessible names, Service copy on the
Channel page, restoration of a prose wall and a wrong Problem mark. Fast smoke
passed **488/0**. Documentation validation checked **154 files** and **2,852
local links** with zero errors.

Two browser-harness attempts receive no credit. The first launched its temporary
server in a shell that ended before the browser ran. The second compared raw
flex-layout `innerText`, where the decorative span creates a line break; the
corrected check normalises whitespace and then exercises the rendered mark.
Neither attempt caused a product change.

Final slow smoke passed **609/0**, including `go vet` and the race checks. All
**204** frozen source hashes remained unchanged through the run. The tracked
and frozen smoke scripts both had SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.

## Limits

This slice does not add the planned local User photo, owner photos on resource
detail, service activity placement, remaining collection search/sort/paging or
compact help on pages whose journey is still pending.
