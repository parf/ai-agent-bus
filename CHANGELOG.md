# Changelog

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
