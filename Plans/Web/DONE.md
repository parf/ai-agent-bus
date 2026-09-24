# DONE Web face rewrite

📌 **TL;DR:** W.1–W.10 are built in `src/web` (0.8.32, 2026-09-24): every
site-map address, the new design, and a development install running as its
own account on `127.0.0.1:6781` against the live daemon. W.11, the cutover,
waits for the owner. Open checks are named per row.

## Done — Web

| ID | Result | Evidence | Open |
|---|---|---|---|
| W.1 | Server core: JSX runtime that escapes by default, daemon client, headers, origin checks on every POST, 1 MiB cap, `HEAD`, `/healthz`, `--version` | `src/web/test/unit.test.ts`, `contract.test.ts`; mutants of escaping, origin, body cap, cache headers each fail a test | — |
| W.2 | Design system: tokens, dark and light, components, CDN table with hashes, `/_styleguide` behind `AGENT_BUS_WEB_DEV=1` | exact-URL CSP and hash tests; screenshots at 1440 and 390 px, both themes | axe run in the container |
| W.3 | Frame, sidebar, theme toggle, view transitions, landing, sign-in and sign-out, problem pages, `return` rules | contract tests for 401 answers, cookie attributes, `//evil`, ended session, 502 without the socket path | — |
| W.4 | Overview with attention items, node strip, today's ribbon | unit test of item conditions and order | — |
| W.5 | Activity (uPlot day chart), Diagnostics with exchange folding, `reply_to` and consumer receipts | unit tests of folding cases | — |
| W.6–W.7 | Record lists, register, detail, inactive view, settings, deactivate, Danger Zone, both POST handlers; the confirmation guard always applies | contract tests: form retention without the secret, `412` marks `name`, unconfirmed transfer and delete refused, stale confirmation `409` | — |
| W.8 | Users, user, profile editor, own-email form, GitHub refresh, Groups, group, group editor, Account | contract tests: own row only, no Register for an ordinary user, Personal group prefix, existing group name refused | — |
| W.9 | Palette with `/palette.json`, shortcuts, copy buttons, flash toasts | contract tests: palette scope, `401` and cross-site `403`, flash shown once | reduced-motion browser check |
| W.10 | `src/web/install-dev.sh`, `agent-bus-web` account, `agent-bus-web.service` on `6781` | runs as uid `agent-bus-web`; security score 0.9; pages render for a live session | directive-by-directive mutation and the pressure checks |
