# Personal services web grouping

📌 **TL;DR:** Personal services have a dedicated owner view; grouping changes no access.

## Result

0.5.51 completes the web half of [Personal services](../../../docs/03-services-and-topics.md#personal-and-shared):

| Surface | Result |
|---|---|
| Main services | Personal records are excluded, including the visitor's own |
| Personal services | Ordinary visitors see their own; the daemon owner filters by owner among ACL-visible records |
| Direct detail | Remains reachable when the daemon authorizes it; the Personal tab is marked from record data |
| Owner controls | Classification, Allow and Maintainers change atomically in one form |
| Access | Unchanged; the view makes no node-wide inventory claim |

The existing Channels tab now marks itself current instead of marking Services.

## Checks

The focused web tests use owned Personal and non-Personal records together, two
owners, a channel, direct detail, an ordinary owner-filter attempt, owner and
Maintainer sessions, atomic changes in both directions, settings preservation,
creation and a core-refused invalid ACL.

Named overlay mutations changed one behavior at a time:

| Mutation | Pinned claim |
|---|---|
| keep Personal on the main list | grouping excludes even an owned Personal record |
| keep non-Personal on the Personal list | the dedicated view is classification-specific |
| silently accept an ordinary `owner=` query | shared URLs are canonical rather than caller-dependent |
| ignore the daemon-owner owner filter | per-owner rows do not mix |
| mark Personal detail as Services | detail navigation follows record data |
| omit Personal from the atomic request | the classification field reaches core |
| let settings submit an absent Allow as empty | Personal sharing survives unrelated settings edits |
| show classification to a Maintainer | only the record owner gets that control |
| omit Personal at creation | registration carries the choice |
| claim a node-wide owner view | the ACL-visible boundary stays on the page |

All ten were caught by named assertions (`tmp/personal-services-web/mutations.log`). OpenCode reviewed grouping, visibility,
canonicalization, form ownership, atomic wiring and alternative write paths with
no findings.

The byte-frozen `src/personal-services-web-smoke.local.sh --slow` run passed
**589 checks with 0 failures**; vet and race passed
(`tmp/personal-services-web/slow-final.log`, lines 10–11 and 936). The frozen
copy and tracked `src/smoke.sh` both have SHA-256
`e401546193343b037b16df4c5e408d1f208aa35f6cbe8180a64274a0e6e23b21`.
The 175-entry source/version manifest was unchanged after the run
(`source-frozen.sha256` and `source-verify.log`).

## Limits

This is grouping and owner control, not a second registry or access mode. The
daemon owner sees only what the existing `/ls` authorization returns. CLI, MCP
and API listings remain unchanged.
