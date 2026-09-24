# Decisions Web face rewrite

📌 **TL;DR:** Dated owner decisions for the web face rewrite. Each row names
the decision and links its home in the [README](README.md#web-face-rewrite).

## Decisions

| Date | ID | Decision | Home |
|---|---|---|---|
| 2026-09-24 | Q116 | The web face runs under its own systemd unit and account; the supervisor stops running it, and its bubblewrap and cgroup code go at cutover | [process and account](README.md#process-and-account), [cutover](README.md#cutover) |
| 2026-09-24 | Q118 | Use any open-licensed fonts the design needs; a cool look matters more than download size | [stack](README.md#stack), [design language](README.md#design-language) |
| 2026-09-24 | Q119 | Fix every item the web-face spec lists as worth fixing, not exact parity first | [behaviour changes](README.md#behaviour-changes) |
| 2026-09-24 | Q120 | The rewrite stays on the `0.8` line: PATCH bumps per step, no odd development line | [steps](TODO.md#steps), [versioning](../../CLAUDE.md#versioning) |
| 2026-09-24 | — | Pages and structure may change modestly where needed for a clearer logical view; every change is written into the spec | [page structure](README.md#page-structure) |
| 2026-09-24 | Q117 | CSP adds the one pinned CDN for scripts, styles and fonts, `connect-src 'self'`, and drops inline styles | [Content-Security-Policy](README.md#content-security-policy) |
| 2026-09-24 | — | External fonts and popular JS libraries load from the CDN with integrity hashes, never imported into our code | [external assets](README.md#external-assets) |
| 2026-09-24 | — | Internet is required in the visitor's browser; pages rely on the CDN libraries and no offline or no-JavaScript fallback is built | [external assets](README.md#external-assets) |
| 2026-09-24 | — | Development environment kept simple: `/var/lib/agent-bus/web` links straight to the git checkout's `src/web`, run from source by the system bun under a locked-down systemd unit; no compile or release packaging | [process and account](README.md#process-and-account), [the unit](README.md#the-unit) |
