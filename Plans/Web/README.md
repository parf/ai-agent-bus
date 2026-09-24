# Web face rewrite

📌 **TL;DR:** Replace the Go `agent-bus-web` with a TypeScript face on bun that
runs as its **own process under its own system account**, talks to the daemon
only over the shared socket with the visitor's session, and looks like a
modern product: dark-first, one design system, cross-page transitions, a
command palette. The behaviour contract is [docs/web-face](../../docs/web-face/site-map.md#every-address)
and nothing else; work steps are in [TODO](TODO.md#steps), open choices in
[QUESTIONS](QUESTIONS.md#open-questions).

## Scope

| In | Out |
|---|---|
| Every address in the [site map](../../docs/web-face/site-map.md#every-address), both homepages included | Daemon API changes; the web face renders what the daemon answers |
| The [shared rules](../../docs/web-face/shell.md#process-model): session, origin checks, headers, problem pages, form recovery, paging, `return` | New authority: the face still [acts on the visitor's token and nothing else](../../docs/11-processes.md#web-authority-boundary) |
| The "worth fixing" items each spec file ends with ([behaviour changes](#behaviour-changes)) | The daemon-side refusal text that names a hidden record's kind; that is a daemon fix |
| A new look ([design language](#design-language)) | CLI and MCP faces |

## Clean-room rule

The rewrite is written from `docs/web-face/` only. Nobody working on it reads
`src/cmd/agent-bus-web/` or its templates. A gap in the spec is fixed in the
spec first (one commit), then built. Tests are new too: the Go face's smoke
checks are retired at [cutover](#cutover), not ported.

## Process and account

| | |
|---|---|
| Account | system account `agent-bus-web`, `nologin`, no home contents, no SSH keys, not in the account map |
| Unit | `agent-bus-web.service`, its own systemd unit, `User=agent-bus-web`, ordered `After=agent-busd.service`, `Restart=on-failure` |
| Daemon link | `/run/agent-bus/bus.sock` (shared, mode `666`, [supplies no identity](../../docs/02-access.md#local-socket)). Never a mapped account socket |
| Credential | none of its own. `POST /session` with the typed token, then the session id per call, as [shell § process model](../../docs/web-face/shell.md#process-model) states |
| State | none: no session map, no cache, no writable path. A restart ends nothing |
| Binary | one self-contained executable from `bun build --compile`, version from `src/internal/version/VERSION`, build stamp as [setup § build information](../../docs/09-setup.md#build-information) requires; `--version` prints both |
| Confinement | systemd sandboxing in place of the supervisor's bubblewrap: read-only filesystem, private `/tmp`, no new privileges, no capabilities, address families `AF_UNIX AF_INET AF_INET6`, `MemoryMax=256M`, `MemorySwapMax=0`, `TasksMax=64`, `CPUQuota=100%`, only the socket directory and TLS files readable. Owner decision: [Q116](DECISIONS.md#decisions) |
| Listen | `AGENT_BUS_WEB_ADDR`, default `127.0.0.1:6780`; TLS only with both cert and key, a missing one refuses to start |

## Stack

Built-in first, per [external tools](../../docs/10-modules.md#external-tools).

| Concern | Choice | Why |
|---|---|---|
| Runtime and HTTP | bun, `Bun.serve` | already the MCP runtime; HTTP is built in |
| Daemon client | bun `fetch` with its `unix` socket option; one small module | no dependency; one place maps refusals |
| HTML | server-rendered TSX through our own ~100-line JSX runtime that escapes every string by default | type-checked templates, no React, no hydration; raw HTML only through one named helper |
| CSS | one hand-written stylesheet on design tokens, bundled and served as a hashed `/app.<hash>.css` | lets the CSP drop `style-src 'unsafe-inline'` ([Q117](QUESTIONS.md#q117-csp)) |
| Client script | one bundled `/ui.<hash>.js`, progressive enhancement only; every page works without it | CSP `script-src 'self'` stays |
| Fonts | any open-licensed faces the design needs, self-hosted `woff2`, their licences shipped with the package; the look comes first, size second ([Q118](DECISIONS.md#decisions)) | no CDN |
| Tests | `bun test`; a disposable `agent-busd` for contract tests; Playwright + axe in the container for browser checks | as the existing acceptance scripts do |

### Layout of the code

| Path | Holds |
|---|---|
| `src/web/server.ts` | `Bun.serve`, headers, origin checks, routing table, body cap |
| `src/web/daemon.ts` | the client: one call function, timeouts, `{error}` → a typed refusal, transport errors never shown |
| `src/web/session.ts` | cookie read and write, sign-in, sign-out |
| `src/web/problem.ts` | the problem-page table, `sectionProblem`, `formRefusal`, line attribution |
| `src/web/forms.ts` | parsing, per-form retention allowlists, field marking |
| `src/web/pages/` | one file per page family: `node`, `records`, `people`; each page = loader (parallel daemon calls) + view |
| `src/web/ui/` | components: frame, nav, card, pill, table, toolbar, pager, tabs, help popover, empty state, charts, form fields |
| `src/web/client/` | the browser script: palette, theme, shortcuts, submit-on-change |
| `src/web/style/` | tokens and stylesheet |
| `src/web/assets/` | logo, favicon, landing picture, fonts |

A page never computes authority. Where the spec says a face-side rule
(attention items, exchange folding, "used by visible records", may-edit on
groups), it lives in a pure function with its own unit tests.

## Design language

The spec fixes content, fields and states; the look is free. Goal: a calm,
dense operator console, not a document.

| Element | Direction |
|---|---|
| Theme | dark-first, light as equal; follows `prefers-color-scheme`, overridable by a toggle kept in a non-authority cookie `ab_theme` |
| Palette | deep ink surfaces (`#0b0d12` → `#161a22`), one electric accent (the bus red, softened to coral `#ff5a4e`, with an indigo secondary), semantic green / amber / red / blue for states; contrast AA on both themes |
| Shell | left sidebar with the nine sections and their marks, collapsible to icons; glass top bar with release, host, account menu and the palette hint (`⌘K`); a phone gets a bottom bar and a drawer |
| Type | Inter for text, tabular figures in every number; JetBrains Mono for names, addresses and ACL lines; a display face for headings, the landing and KPI figures (Geist or Space Grotesk, picked on the W.2 style guide) |
| Surfaces | cards with 12 px radius, 1 px hairline borders, soft inner glow on hover; no heavy shadows |
| Status | pills with a dot (`● Active`, `● Inactive`), severity as a coloured rail on attention items, `INACTIVE` badge kept |
| Data | tables with sticky headers, zebra-free, row hover, right-aligned tabular numbers; stacked cards at ≤ 40 rem |
| Charts | inline SVG, server-drawn: the day chart with hour ticks and a hover crosshair; a 144-cell **day ribbon** (heat strip) per record on lists and detail; tiny sparklines in the Overview tiles |
| Motion | CSS cross-document View Transitions (`@view-transition { navigation: auto }`) for page changes, shared element names on the page title; 150 ms ease; `prefers-reduced-motion` turns it all off |
| Forms | floating labels, inline help as native popovers, the error summary as a pinned alert card, list fields (allow, subs, maintainers, members) as monospace textareas with line numbers so `Line N:` refusals point at a visible line |
| Danger | the Danger Zone and every confirmation in a red-rimmed card; the confirm button repeats the verb and the name |
| Landing | full-bleed hero over the bus picture with a gradient veil, the heading and lede on it, four feature cards below, sign-in as a floating card |
| Overview | attention items as a stack of alert cards; the node strip as KPI tiles with sparklines; the Find row as quick-action chips |

### Enhancements (JavaScript only; nothing depends on them)

| Feature | Behaviour |
|---|---|
| Command palette | `⌘K` / `Ctrl-K`: jump to any section, or to any record, user or group the visitor may see; data from a same-origin `GET /palette.json` made with the visitor's session ([Q117](QUESTIONS.md#q117-csp)) |
| Shortcuts | `g o` Overview, `g a` Agents, `g s` Services, `g q` Queues, `g p` PubSub, `g u` Users, `g g` Groups, `/` focuses search, `?` lists them |
| Theme toggle | writes `ab_theme`, swaps without reload |
| Submit on change | the spec's `data-submit-on-change`, kept |
| Copy | a copy button beside every `<code>` name |
| Flash | a success toast after a `303`, carried by a short-lived `ab_flash` cookie that holds only a message key |

## Behaviour changes

The rewrite fixes what the spec files list as worth fixing, unless the owner
keeps one ([Q119](QUESTIONS.md#q119-behaviour-changes)).

| From | Change |
|---|---|
| [shell](../../docs/web-face/shell.md#differences-from-older-docs) | one signed-out answer everywhere (`401`, `sign in to open this page`), `/` excepted; a real `404` page for unknown paths; a status failure never renders a blank account link; sign-out clears the cookie with the attributes it was set with; `Origin` required on every POST; assets cost no daemon call; `405` pages framed; no `Try again` on `404` |
| [node](../../docs/web-face/node.md#inconsistencies-worth-fixing-in-the-rewrite) | Diagnostics renders exchanges with `reply_to`; held and loss tables include inactive records; a `/users` failure shows a section notice; an unreachable daemon at sign-in says so; Activity accepts inactive records visible to the caller; one Uptime source per page |
| [records](../../docs/web-face/records.md#differences-from-the-older-specs) | `kind` kept on paging, Back and Clear filters; the confirmation guard **always** applies to transfer and delete; managers see the description; detail addresses redirect to the kind's own path; a user inbox is not offered Remove; no unused daemon reads; valid HTML on Deliver-To |
| [people](../../docs/web-face/people.md#worth-fixing-in-the-rewrite) | "Not visible to you" renders on hidden groups; register refuses an existing group name instead of replacing it; post-create failures say what was saved; no Transfer on `@<user>/…` groups; redirects use the daemon's stored name; one identity pill; Account lists inactive owned records; `/avatar` dropped or used; `/users?kind=other` after sign-in |

## Cutover

1. The TS face runs beside the Go one on `127.0.0.1:6781` until every
   [site-map](../../docs/web-face/site-map.md#every-address) row passes its checks.
2. Setup installs the account and unit; the daemon unit stops passing `-web`;
   the TS face takes `6780`.
3. The Go face, its `-web` supervision, its bubblewrap and cgroup code and its
   smoke checks are removed in one commit after the owner accepts the pages.
4. Current docs take the substance: [discovery § dashboard](../../docs/05-discovery.md#dashboard),
   [processes § web authority boundary](../../docs/11-processes.md#web-authority-boundary),
   [setup § the two accounts](../../docs/09-setup.md#the-two-accounts), with decision rows.
   `docs/web-face/` stays the internal page spec, updated to the TS behaviour.
