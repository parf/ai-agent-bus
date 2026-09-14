# Launcher implementation

## Scope

Implementation evidence for H.8 and H.9–H.9.3, recorded 2026-09-13.
The [launcher contract](../../../docs/08-runner-role.md#smart-launchers) and
[MCP minimum](../../../docs/05-discovery.md#mcp-minimum) own behavior;
[remaining acceptance](../TODO.md#remaining-work) still owns the release gates.

| Work | Result |
|---|---|
| Shared launcher | Runtime-specific shell entry points use one TypeScript implementation and the existing bus and App Server clients |
| Distribution | The build produces a relocatable launcher/MCP tree alongside the stamped Go binaries, release version and license |
| Session identity | Runtime names and directory fallback feed persistent session bindings; conditional registration prevents concurrent claims from overwriting each other |
| Runtime lifecycle | Exit status, startup failure and termination are propagated; shutdown waits for cleanup and MCP EOF stops an orphaned inbox reader |
| Tool delivery | Both runtimes load the MCP face; Claude authorizes the bus tools, and Codex tools and pusher share the selected App Server thread |

## Automated verification

The full slow smoke runs the [launcher harness](../../../src/launchers/smoke.ts)
against a real daemon and bundled MCP face, with deterministic Claude and Codex
runtime fixtures. It checks listing, service responses, ACL refusals, credential
separation, mapped sockets, session continuation and renaming, concurrent names,
missing executables, startup failures, exit status, termination and surviving siblings.

The final `src/smoke.sh --slow` run passed with zero failures, including Go
vet/race checks and the launcher harness. TypeScript type checking also passed.

The [conditional registration test](../../../src/internal/api/register_test.go)
races claims for the same normalized name and requires exactly one winner,
while preserving ordinary authorized updates. Shared
[message tests](../../../src/mcp/messages.test.ts) check return routes and receipt handling.

## Mutation evidence

Each mutation below was made in an isolated copy and failed its named check.

| Mutation | Check that failed |
|---|---|
| Ignore an assigned session name | Uses its assigned title |
| Remove Claude automatic execution | Enforces automatic execution |
| Remove Codex automatic execution | Enforces automatic execution despite contrary options |
| Remove duplicate-label numbering on allocation and retry | Duplicate labels receive the required suffixes |
| Leave launcher locks behind | Releases its session locks |
| Force a successful exit status | Propagates the runtime exit status |
| Remove continuation | Continues its session |
| Discard the saved identity on resume | Keeps its inbox across a rename |
| Remove channel activation | Receives the service answer |
| Remove conditional-registration enforcement | Concurrent claims succeed more than once |
| Remove MCP shutdown on stdin EOF | Termination leaves no runtime or MCP child |
| Return early from concurrent cleanup calls | Cleanup releases locks and stops children |
| Remove Claude's bus-tool authorization | Loads bus tools and lists the real bus |
| Ignore the requested return route | Codex replies follow the requested return route |
| Ask for replies to receipts | Codex receipts never ask for a reply |
| Forward the original relative directory after changing directory | Codex resolves a relative directory exactly once |
| Create an empty headless thread and resume it in the TUI | Codex lets the fresh TUI create its own thread |
| Remove Codex's bus-tool authorization | Codex loads bus tools and lists the real bus |

## Live checks and limits

| Check | Evidence |
|---|---|
| Claude Code 2.1.270 | A launcher-started session with isolated configuration called the real bus listing tool successfully; saved session-title metadata was verified |
| Codex CLI 0.154.0 | A fresh authenticated TUI created its own thread; after restart the launcher resumed that thread and preserved its bus identity. Relative working-directory arguments worked |
| Interactive exchange | Claude sent request `2d35591ecc7a395f`; the shared App Server delivered it into the live Codex TUI. Codex replied with `4ce32f8d65065589` without a tool prompt; Claude received a channel notification and printed `CHANNEL_PUSH_OK` without polling |
| Tool permission regression | Both runtimes initially prompted or refused bus tools despite automatic execution. Explicit namespace grants fixed this; Codex uses the documented [MCP tool approval setting](https://learn.chatgpt.com/docs/extend/mcp?surface=cli#other-configuration-options) |
| Still pending | Renaming while a reply is in flight, and installation on an independent host under the installed service account |

The deterministic fixtures establish launcher wiring and failure behavior;
they do not substitute for these remaining live and installed gates.

## Installed socket correction

The first installed launch reached Codex's App Server but failed when the
session token switched to the daemon-owned shared socket. Same-account test
daemons had hidden that permission failure. The correction shipped locally as
0.5.2 on 2026-09-13; [local access](../../../docs/02-access.md#local-socket)
and [launcher discovery](../../../docs/08-runner-role.md#smart-launchers) own the contracts.

| Verification | Result |
|---|---|
| Full slow suite | 499 passed, zero failures, including vet/race and launcher socket discovery; TypeScript checks passed |
| Mutation checks | Removing TypeScript discovery, Go discovery, explicit CLI precedence, endpoint diagnostics, shared socket access or runner token handoff failed its corresponding check |
| Installed CLI | Address and token unset: status and listing succeeded as the mapped account; all installed programs report the shared release |
| Installed launcher | Address and token unset: the installed Codex launcher discovered the system daemon, registered its own identity and opened the actual TUI |
| Cross-account access | A different OS account reached the shared listener: a valid token authenticated its principal; absent and invalid tokens both returned unauthorized |
| Dashboard | Enabled as a supervised child; local HTTP page and authenticated sign-in verified |

This is acceptance on the development host using the installed service account;
the independent-host installation gate remains pending.

## Token helper follow-up

The standalone token helper still used the old default path after the CLI and
launcher correction. Version 0.5.3 uses the same Go client discovery function.
The old installed helper failed the existing-credential check with no address;
the corrected helper passed, including the exact bare command in fish on the
installed host. Explicit addresses remain authoritative, and a supplied token
cannot borrow the mapped account's authority. The full slow suite passed after
isolating its dashboard port and shortening its test-only Unix socket paths.
