# Launcher terminal verification

Historical evidence, 2026-09-13. Current contract:
[terminal appearance](../../../docs/08-runner-role.md#terminal-appearance).

| Check | Result |
|---|---|
| Internal helper | `src/launchers/terminal.ts`, bundled into the launcher; no helper command installed on PATH |
| PTY behavior | Named and shortened-path titles, home abbreviation, control-character filtering, live Claude rename, normal exit, failed startup and termination covered |
| Terminal controls | Fake Kitty/Konsole commands verify caller targeting, rotating distinct palettes and restoration; graphical rendering was not manually inspected |
| Full acceptance | `src/smoke.sh --slow`: 529 passed, zero failed, including vet and race checks |
| Broken restoration and runtime overrides | Isolated full slow run fails terminal unit and installed-launcher checks for all three runtimes |
| Focused mutations | Ignoring names, shortening paths incorrectly, removing colors and targeting all tabs each fail the terminal checks |
| Other checks | TypeScript compilation, internal documentation links and whitespace checks pass |
| Installation | Shared release 0.5.12 installed; all launchers report it, stamped Go programs installed, daemon restarted; installed launcher and MCP bundles match the checked build |

The initial live-rename fixture wrote a retired session file and failed. It now
uses the actual foreground session ID; the corrected full suite passes.
Detailed run logs remain local under `tmp/terminal-work/`.
