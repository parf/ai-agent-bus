# Changelog

## 0.5.27 — 2026-09-15

The daemon drops credentials that answer for nothing at start: no record of their own and no registered user. This node had 232 of them against four records, every one listed as a user. One interim while orphan-service deletion is still pending — a name that owns services keeps its credential, so the sweep cannot leave a service nobody answers for.

## 0.5.26 — 2026-09-15

The supervisor no longer hands the dashboard child an `AGENT_BUS_TOKEN` it inherited: the dashboard is meant to hold no credential of its own, and that held only while nobody started the daemon from a shell that had one exported.

## 0.5.25 — 2026-09-15

A dashboard refusal is now a page rather than a dead end. An anonymous deep link gets the sign-in form at the address it asked for and returns there afterwards; a return address that is not this dashboard's own is refused. A bus that is not answering says so instead of claiming the visitor is not signed in, and an expired session, a permission refusal, an unknown name and a daemon fault each get their own recovery, on the shell, with the navigation still under them.

## 0.5.24 — 2026-09-15

Every dashboard page now shares one shell: a single navigation with the current entry marked, sign out beside the name it signs out, a document language, a viewport, a main landmark and a title of its own. Pages no longer reload themselves — the diagnostics page says when it was built and offers a Refresh link instead, and the activity page's own thirty-second reload is gone. Muted text meets the contrast minimum, and narrow screens get a smaller gutter with only wide tables scrolling.

## 0.5.23 — 2026-09-15

The authority levels are nested rather than side by side: adding somebody to `@maintainers` now makes them a registered user, as starting the daemon already did for the owner. A snapshot written before this rule is repaired on restore. Taking somebody out of the group leaves the person behind, since a user is never deleted.

## 0.5.22 — 2026-09-15

Removing a registration says what it does: no registration, no access. The dashboard offers no group deletion at all — a group is made inactive or banned as a person is, rather than removed out from under the records naming it. The group states themselves are pending.

## 0.5.21 — 2026-09-15

Two dashboard pages stop promising what the daemon refuses. Removing a registration no longer says credentials remain valid: the service credential goes with the address, and only a person's own stays. The maintainers group no longer offers a delete button, because that deletion is refused for everybody including the daemon owner.

## 0.5.20 — 2026-09-15

`agent-bus-web` refuses to start when a certificate was asked for and is not there, instead of logging the fact and serving plain HTTP. Asking means supplying `-cert` or `-key`; either without the other is the same ask and the same refusal. Not asking at all is unchanged and still plain HTTP on loopback.

## 0.5.19 — 2026-09-15

A send refused because the receiver's queue is full answers `429` rather than `503`: a full queue is the sender outrunning the reader, and `503` is left to mean a service that is itself unavailable. The counted refusal reason is unchanged.

## 0.5.18 — 2026-09-15

The dashboard is `http://127.0.0.1:6780` and nothing else: the borrowed `localhost.direct` hostname, the certificate kept under that name and the 443/8443 binding fallback are gone, since the certificate that scheme existed for had expired. HTTPS remains for anyone who supplies `-cert` and `-key`. The API root's redirect to the dashboard is now `301`.

## 0.5.17 — 2026-09-15

A removed name keeps nothing: no reservation for its last owner, and no credential. Whoever asks for a removed name next gets it, its previous owner included. Reserving it cost a permanent entry and a permanent credential per address — one launcher-smoke run left roughly 250 in one person's name list — for protection this stage does not need; restoring it is R1.2's.

## 0.5.16 — 2026-09-15

The name list says whose each credential is and what it is for, so a person can tell their own identity from the services they registered.

## 0.5.15 — 2026-09-15

The dashboard's service and channel listings, and a record's own page, show when the record was last written.

## 0.5.14 — 2026-09-15

The API root sends a browser to the dashboard instead of answering nothing: an exact-root 303 to `-dashboard` / `AGENT_BUS_DASHBOARD`, leaving a mistyped route its 404. One package now owns where the dashboard is reached.

## 0.5.13 — 2026-09-13

Coordinate `ab_rename` through the launcher so runtime titles, inbox readers, MCP credentials and saved session bindings move together; serialize concurrent renames and preserve the launching account’s ownership.

## 0.5.12 — 2026-09-13

Give all smart launchers session-aware terminal titles with exit restoration and rotating Kitty/Konsole tab colors; keep terminal helpers internal and add OpenCode’s violet palette.

## 0.5.11 — 2026-09-13

Add `ab_rename`: a session changes the address it is registered under, taking its own title when given no name. The new address is registered before the old one is released, and a busy old address is kept and reported rather than dropped with its queue.

## 0.5.10 — 2026-09-13

Prefer OpenCode's user-installed wrapper over a system binary earlier in PATH, preserving wrapper settings when launched from fish; explicit `OPENCODE_BIN` still wins.

## 0.5.9 — 2026-09-13

Bind OpenCode's TUI and inbox reader to the same explicit session, including fresh starts; do not wait for a session-selection event that initial startup does not emit.

## 0.5.8 — 2026-09-13

Add `ab-opencode`: an opencode push adapter over the runtime's HTTP server and event stream, and a launcher that owns a password-protected loopback server the session attaches to.

## 0.5.7 — 2026-09-13

Add the required dashboard tabs with service/channel owner controls, protected maintainer groups, user profiles and pause/ban controls, and bounded activity graphs. Persist administration state and cancel blocked reads when access is removed.

## 0.5.6 — 2026-09-13

Use template/instance addresses in `ab-claude` and `ab-codex`; migrate saved dot-form addresses on restart while preserving explicit names and busy old inboxes.
Exclude a session's own previous registration when assigning its label, avoiding a false `#2` during migration or rename.

## 0.5.5 — 2026-09-13

Add `agent-bus unregister` for idle registry entries, preserving credentials and ownership; show the owner in `ls -h`.
The `ab-*` launchers derive a new address after a session rename on restart and remove the old entry only when idle; explicit bus names stay fixed.

## 0.5.4 — 2026-09-13

Add `agent-bus ls -h` for a readable registry table, including filtered and single-record listings.

## 0.5.3 — 2026-09-13

Make the token helper use local socket discovery when no address is configured, preserving explicit addresses and token identity.

## 0.5.2 — 2026-09-13

Discover local bus sockets automatically in the CLI and both launchers; allow local session tokens through the shared socket across OS accounts.
Report the failing bus endpoint before starting runtime sidecars.

## 0.5.1 — 2026-09-13

Add Claude and Codex launchers with bus MCP tools, automatic execution, session continuation and distinct session identities.
Build portable launcher and MCP artifacts alongside the stamped Go programs.

## 0.5.0 — 2026-09-13

Establish one shared MVP version across all programs and MCP handshakes; `--version` also reports build information in Go programs.
Daemon and runner process titles show the version and live call counts in `ps`.
