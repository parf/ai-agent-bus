# Changelog

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
