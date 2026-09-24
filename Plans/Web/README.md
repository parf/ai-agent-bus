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

## Page structure

Owner direction, 2026-09-24: pages and their structure may change a little
where that gives a clearer logical view — merging or splitting a section,
reordering cards, moving a control to where its subject is. Not a redesign of
the site map: addresses, forms, field names and daemon calls stay as specified
unless a change is needed for the better view. Each change is written into its
`docs/web-face/` file in the step that builds it, and listed for the owner at
the W.2 or page acceptance.

## Clean-room rule

The rewrite is written from `docs/web-face/` only. Nobody working on it reads
`src/cmd/agent-bus-web/` or its templates. A gap in the spec is fixed in the
spec first (one commit), then built. Tests are new too: the Go face's smoke
checks are retired at [cutover](#cutover), not ported.

## Process and account

| | |
|---|---|
| Account | system account `agent-bus-web`, `nologin`, no SSH keys, not in the account map; it owns nothing on disk |
| Code | **development environment, kept simple:** `/var/lib/agent-bus/web` is a symlink straight to the git checkout, `/usr/local/src/ai-agent-bus/src/web` (owner, 2026-09-24). The account reads it and cannot write it. What runs is the working tree; `systemctl restart agent-bus-web` picks up edits |
| Runtime | the system bun: `/usr/bin/bun run /var/lib/agent-bus/web/server.ts`, from source, no build step, no npm dependency; `--version` prints `src/internal/version/VERSION` |
| Unit | `agent-bus-web.service`, its own locked-down systemd unit ([the unit](#the-unit)) |
| Daemon link | `/run/agent-bus/bus.sock` (shared, mode `666`, [supplies no identity](../../docs/02-access.md#local-socket)). Mapped account sockets are mode `600` for other accounts, so it cannot open them |
| Credential | none of its own. `POST /session` with the typed token, then the session id per call, as [shell § process model](../../docs/web-face/shell.md#process-model) states |
| State | none: no session map, no cache, no writable path. A restart ends nothing |
| Listen | `AGENT_BUS_WEB_ADDR`, default `127.0.0.1:6780`; TLS only with both cert and key, a missing one refuses to start |

### The unit

Secured as far as the face still works. `systemd-analyze security
agent-bus-web` must score **1.5 or lower** ("OK"); each directive left out is
listed with its reason.

| Area | Directives |
|---|---|
| Identity | `User=agent-bus-web`, `Group=agent-bus-web`, `UMask=0077`; `After=agent-busd.service`, `Restart=on-failure`, `RestartSec=2` |
| Privilege | `NoNewPrivileges=yes`, `CapabilityBoundingSet=` (empty), `AmbientCapabilities=`, `RestrictSUIDSGID=yes`, `LockPersonality=yes`, `RestrictRealtime=yes`, `RestrictNamespaces=yes`, `KeyringMode=private`, `RemoveIPC=yes` |
| Filesystem | `ProtectSystem=strict`, `ProtectHome=yes`, `PrivateTmp=yes`, `PrivateDevices=yes`, `DevicePolicy=closed`; `TemporaryFileSystem=/var/lib:ro` with `BindReadOnlyPaths=/var/lib/agent-bus/web` (resolves to the checkout) so no other state directory exists for it; the checkout under `/usr/local/src` is read-only by `ProtectSystem=strict` (`/run/agent-bus` is already read-only under `ProtectSystem=strict`; connecting to the socket needs no write) |
| Exec | `NoExecPaths=/`, `ExecPaths=/usr/bin/bun /usr/lib /usr/lib64`: bun and the shared libraries it maps, nothing else |
| Kernel | `ProtectKernelTunables`, `ProtectKernelModules`, `ProtectKernelLogs`, `ProtectControlGroups`, `ProtectClock`, `ProtectHostname`, `ProtectProc=invisible`, `ProcSubset=pid` |
| System calls | `SystemCallArchitectures=native`, `SystemCallFilter=@system-service` minus `@privileged @resources @mount @debug @cpu-emulation @obsolete @raw-io @reboot @swap`, `SystemCallErrorNumber=EPERM` |
| Network | `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6`; `IPAddressDeny=any` with `IPAddressAllow=localhost`: it answers on loopback and makes no outbound connection (the visitor's browser, not the face, fetches the CDN). A non-loopback `AGENT_BUS_WEB_ADDR` widens `IPAddressAllow` by setup, never by hand |
| TLS | `LoadCredential=cert:… key:…` when configured, with `AGENT_BUS_WEB_CERT=%d/cert` and `AGENT_BUS_WEB_KEY=%d/key`: the face reads them from the credentials directory, never from their own paths |
| Resources | `MemoryMax=256M`, `MemorySwapMax=0`, `TasksMax=64`, `CPUQuota=100%`, `LimitNOFILE=1024` |
| Environment | only `AGENT_BUS_ADDR`, `AGENT_BUS_WEB_ADDR`, `BUN_RUNTIME_TRANSPILER_CACHE_PATH=0` (no cache to write) and, with TLS, the two paths above; nothing inherited |
| Left out | `MemoryDenyWriteExecute`: bun's JavaScript JIT needs writable-executable memory. `PrivateNetwork`: it must listen |
| Proven in W.1 | `PrivateUsers=yes` (socket `0666`, directory `0711`), `ProcSubset=pid` and the `@resources` removal (bun raises `RLIMIT_NOFILE` at start) are kept only if the face serves `/healthz` under this unit in the container; a directive that breaks it moves to Left out with its reason |

## Stack

Built-in first, per [external tools](../../docs/10-modules.md#external-tools).

| Concern | Choice | Why |
|---|---|---|
| Runtime and HTTP | bun, `Bun.serve` | already the MCP runtime; HTTP is built in |
| Daemon client | bun `fetch` with its `unix` socket option; one small module | no dependency; one place maps refusals |
| HTML | server-rendered TSX through our own ~100-line JSX runtime (`tsconfig` `jsx: react`, `jsxFactory: h`) that escapes every string by default | type-checked templates, no React, no hydration; raw HTML only through one named helper |
| CSS | one hand-written stylesheet on design tokens, served as a hashed `/app.<hash>.css`; no `style=` attributes anywhere, SVG colours by `fill` and classes | lets the CSP drop `style-src 'unsafe-inline'` ([Q117](DECISIONS.md#decisions)) |
| Caching | hashed assets `public, max-age=31536000, immutable`; pages and everything else `no-store` | a hash in the name is what makes long caching safe; shell § security headers is amended in W.1 |
| Client script | our own `/ui.<hash>.js` with the CDN libraries; pages may rely on both | CSP allows only this site and the pinned CDN |
| Fonts | any open-licensed faces the design needs, loaded from the CDN ([external assets](#external-assets)); the look comes first, size second ([Q118](DECISIONS.md#decisions)) | — |
| Popular JS libraries | loaded from the CDN, never imported or bundled into our code ([external assets](#external-assets)) | owner rule |
| Tests | `bun test`; a disposable `agent-busd` for contract tests; Playwright + axe in the container for browser checks | as the existing acceptance scripts do |

### External assets

Owner rule, 2026-09-24: external fonts and popular JavaScript libraries are
welcome, loaded by the browser from a CDN and **never imported into our code**
— no npm dependency, no vendored copy, no bundling.

| Rule | |
|---|---|
| Host | one CDN, `cdn.jsdelivr.net`; the CSP names each file's exact URL (fonts: their `files/` directory), generated from `src/web/assets.ts`, never the bare host |
| Pinning | an exact version in every URL, never `latest`, a range or a `+esm` bundle (regenerated, so its bytes and hash are not stable) |
| Hash | every external `<script>` and `<link rel=stylesheet>` carries `integrity="sha384-…"` and `crossorigin=anonymous`; one table in `src/web/assets.ts` holds URL and hash, and a test fails on an external tag without them |
| Fonts | from `@fontsource` packages on the same CDN, their stylesheets hashed. Font files a stylesheet names cannot carry a hash of their own; a font is data, not code, and the hashed stylesheet fixes which files they are |
| Internet | **required** in the visitor's browser: the pages depend on the CDN libraries and fonts. No offline fallback is built. The face itself still reaches only the daemon socket; the browser, not the server, fetches the CDN |
| Choice | each library is named with its reason on the W.2 style guide: Lucide icons, uPlot for the day charts, a command-palette component |

### Content-Security-Policy

[Q117](DECISIONS.md#decisions). Today's policy with three changes:

| Directive | Value | Why |
|---|---|---|
| `script-src` | `'self'` and each library's exact CDN URL | the bare host would admit every file jsdelivr serves; SRI covers only tags we wrote |
| `style-src` | `'self'` and each font stylesheet's exact URL | no `'unsafe-inline'`: injected CSS can leak page content or overlay a false control |
| `font-src` | `'self'` and each font package's exact `files/` path | today's `default-src 'none'` would block every font |
| `connect-src` | `'self'` | the palette's `/palette.json` |

A test renders every page and fails on a `style=` attribute; the browser check fails on any `securitypolicyviolation` event.
| the rest | `default-src 'none'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'` | unchanged |

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
| `src/web/assets/` | logo, favicon, landing picture |
| `src/web/assets.ts` | the pinned CDN table: URL and hash of every external font and library; the CSP is generated from it |
| `src/web/glyphs.ts` | the entity and authority glyphs; canonical home stays `src/internal/display`, and a test compares this table with a dump from the Go package |

A page never computes authority. Where the spec says a face-side rule
(attention items, exchange folding, "used by visible records", may-edit on
groups), it lives in a pure function with its own unit tests.

## Design language

The spec fixes content, fields and states; the look is free. Goal: a calm,
dense operator console, not a document.

| Element | Direction |
|---|---|
| Theme | dark-first, light as equal; follows `prefers-color-scheme`, overridable by a toggle kept in a non-authority cookie `ab_theme` (`dark` or `light`, anything else ignored); the server writes `data-theme` on `<html>` so the first paint is right |
| Palette | elevation tokens `--surface-0` … `--surface-3` of deep ink (`#0b0d12` → `#1c212b`), one focus-ring token, one electric accent (the bus red, softened to coral `#ff5a4e`, with an indigo secondary), semantic green / amber / red / blue for states; contrast AA on both themes |
| Shell | left sidebar with the nine sections and their icons ([Q121](QUESTIONS.md#q121-icons)), collapsible to icons; glass top bar with release, host, account menu and the palette hint (`⌘K`); a phone gets a bottom bar and a drawer |
| Type | two families: Geist for text and display (headings, landing, KPI figures), tabular figures (`tnum`) in every number; JetBrains Mono for names, addresses and ACL lines. Inter is the alternative shown on the W.2 style guide |
| Surfaces | cards with 12 px radius, 1 px hairline borders, soft inner glow on hover; no heavy shadows |
| Status | pills with a dot (`● Active`, `● Inactive`), severity as a coloured rail on attention items, `INACTIVE` badge kept |
| Data | tables with sticky headers, zebra-free, row hover, right-aligned tabular numbers; stacked cards at ≤ 40 rem |
| Charts | uPlot for the day charts on Activity and record detail (hour ticks, hover crosshair, series toggles); server-drawn SVG for the 144-cell **day ribbon** (heat strip) per record on lists and detail and the sparklines in the Overview tiles |
| Motion | CSS cross-document View Transitions (`@view-transition { navigation: auto }`); sidebar and top bar carry their own `view-transition-name` so only `main` morphs; icon boxes are sized before the icons render, so nothing jumps; 150 ms ease; `prefers-reduced-motion` turns it all off |
| Forms | floating labels, inline help as native popovers, the error summary as a pinned alert card, list fields (allow, subs, maintainers, members) as monospace textareas with line numbers so `Line N:` refusals point at a visible line |
| Danger | the Danger Zone and every confirmation in a red-rimmed card; the confirm button repeats the verb and the name |
| Landing | the bus picture blurred as a full-bleed backdrop under a gradient veil, the crisp 648×432 picture in its own card beside the heading and lede, four feature cards below, sign-in as a floating card; on a phone the sign-in card comes first |
| Overview | attention items as a stack of alert cards; the node strip as KPI tiles with sparklines; the Find row as quick-action chips |

### Interactive features

| Feature | Behaviour |
|---|---|
| Command palette | `⌘K` / `Ctrl-K`: jump to any section, or to any record, user or group the visitor may see; data from a same-origin `GET /palette.json` made with the visitor's session only when the palette opens ([Q117](DECISIONS.md#decisions)). It answers JSON, `401` JSON when signed out (never the sign-in page), refuses a `Sec-Fetch-Site` other than `same-origin`, and costs `/status`, `/ls` and `/groups`; specified in shell in W.9 |
| Shortcuts | `g o` Overview, `g a` Agents, `g s` Services, `g q` Queues, `g p` PubSub, `g u` Users, `g g` Groups, `/` focuses search, `?` lists them |
| Theme toggle | writes `ab_theme`, swaps without reload |
| Submit on change | the spec's `data-submit-on-change`, kept; its `<noscript>` Apply button is dropped (no fallbacks), with the shell spec edited in W.1 |
| Copy | a copy button beside a record, user or group name in page headings and in the first cell of a row |
| Flash | a success toast after a `303`: an `ab_flash` cookie holding one allowlisted message key (`HttpOnly; SameSite=Strict`), rendered by the server on the next page, which clears it (`Max-Age=0`) so back navigation does not repeat it |

## Behaviour changes

The rewrite fixes every item the spec files list as worth fixing
([Q119](DECISIONS.md#decisions)). Each spec file is updated to the new
behaviour in the step that builds it.

| From | Change |
|---|---|
| [shell](../../docs/web-face/shell.md#differences-from-older-docs) | `HEAD` answered as `GET` without a body on every page and asset (`Bun.serve` does not do it by itself); one signed-out answer everywhere (`401`, `sign in to open this page`), `/` excepted; a real `404` page for unknown paths; a status failure never renders a blank account link; sign-out clears the cookie with the attributes it was set with; `Origin` required on every POST; assets cost no daemon call; `405` pages framed; no `Try again` on `404`; fonts and libraries from one pinned, hashed CDN in place of "no external asset" |
| [node](../../docs/web-face/node.md#inconsistencies-worth-fixing-in-the-rewrite) | Diagnostics renders exchanges with `reply_to`; held and loss tables include inactive records; a `/users` failure shows a section notice; an unreachable daemon at sign-in says so; Activity accepts inactive records visible to the caller; one Uptime source per page |
| [records](../../docs/web-face/records.md#differences-from-the-older-specs) | `kind` kept on paging, Back and Clear filters; the confirmation guard **always** applies to transfer and delete; managers see the description; detail addresses redirect to the kind's own path; a user inbox is not offered Remove; no unused daemon reads; valid HTML on Deliver-To |
| [people](../../docs/web-face/people.md#worth-fixing-in-the-rewrite) | "Not visible to you" renders on hidden groups; register refuses an existing group name instead of replacing it; post-create failures say what was saved; no Transfer on `@<user>/…` groups; redirects use the daemon's stored name; one identity pill; Account lists inactive owned records; `/avatar` dropped (no page references it; photos stay inline `data:` URIs); `/users?kind=other` after sign-in |

## Cutover

1. The TS face runs beside the Go one on `127.0.0.1:6781` until every
   [site-map](../../docs/web-face/site-map.md#every-address) row passes its checks.
2. Setup installs the account, the `/var/lib/agent-bus/web` link and the unit;
   the daemon unit stops passing `-web`; the TS face takes `6780`.
3. After the owner accepts the pages, one commit removes the Go face and
   everything that exists only for it ([W.11](TODO.md#steps) lists it).
4. Current docs take the substance: [discovery § dashboard](../../docs/05-discovery.md#dashboard),
   [processes § web authority boundary](../../docs/11-processes.md#web-authority-boundary),
   [setup § the two accounts](../../docs/09-setup.md#the-two-accounts), with decision rows.
   `docs/web-face/` stays the internal page spec, updated to the TS behaviour.
