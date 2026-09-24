# Launcher live acceptance

📌 **TL;DR:** H.9 and H.9.3 passed on 2026-09-23 for Codex, OpenCode and
Claude, each in a launcher-owned native TUI from a program dir built by
`src/build.sh`. H.9.1 passed everywhere except Codex's enforced mode: the
thread the TUI runs is workspace-write with on-request approval, whatever the
App Server is given. That fix widens a sandbox and waits for the owner.
Startup diagnostics now carry the dying server's own reason (0.8.27).

## Scope

Development host, disposable daemons and clean runtime profiles; the live node
was not touched. 0.8.27 program dir; Codex CLI 0.156.1, OpenCode 1.18.30
(upstream build), Claude Code 2.1.281. Codex and OpenCode take model decisions
from a loopback fixture that never calls MCP or the bus; Claude runs the real
model under a copied claude.ai login, since only such a profile has channels.
The [launcher contract](../../../docs/08-runner-role.md#smart-launchers) and
[session names](../../../docs/08-runner-role.md#session-names) own the behaviour.
The fresh-host run stays in the [installed stage gate](../TODO.md#installed-stage-gate).

## Defect found

| Before | After, 0.8.27 |
|---|---|
| Codex's enforced mode reached only the App Server. The remote TUI starts and resumes its thread with its own settings, so every launcher-started Codex session ran `workspace-write` with `on-request` approval: a command outside the working directory waits for a prompt | **Open, owner decision.** Passing the same mode to the TUI fixes it in a test build, but widens the session's sandbox beyond the [documented contract](../../../docs/08-runner-role.md#smart-launchers) (enforced on the App Server); not committed |
| An App Server that died at startup was reported as "Codex must support --ws-auth capability-token and --remote-auth-token-env", whatever the cause; OpenCode said only "exited during startup". The server's own reason was in a log that cleanup removed | The diagnostic carries the last lines of the server's log: `App Server exited during startup: Error: error loading default config … invalid type: integer 5, expected a map`; OpenCode's names its `EACCES` |

## Checks

The [launch gate](../../../src/acceptance/runtime-launch.ts) runs three phase
groups; each can run alone. The [interactive gate](../../../src/acceptance/runtime-interactive.ts)
supplies H.9's two concurrent launches ([MCP minimum](mcp-minimum.md#checks)).
Every wait is bounded, and a reply counts only when it comes from the right
address, carries the exchange's random value, arrives once with nothing else in
the peer's inbox, and was produced by the runtime itself (the fixture sees the
runtime's own `ab_send` result; Claude's transcript holds the channel event).

### H.9 smart launchers (`launch`)

| Check | Measured |
|---|---|
| Path with spaces, no history | A conversation in another directory exists first. The launch in `work with spaces` starts a new conversation; the other one is untouched and its history is not in the first turn |
| Exact forwarded arguments | The runtime TUI's own argv holds them once, in order: Codex `-m fixture-forwarded "KEYBOARD forwarded"`, OpenCode `--log-level WARN`, Claude `--model sonnet "<prompt>"`. Codex's provider request carries that model and prompt; Claude's transcript carries a `claude-sonnet` model and the prompt |
| Active integration | The runtime's own `ab_ls` lists the bus; a pushed (Codex, OpenCode) or channel (Claude) message is answered |
| Prior history | A second launch with no arguments continues the same conversation: same runtime session id, the model turn carries the earlier turns and no other directory's, one reading address. OpenCode titles its conversation itself, so its restart takes the address that title derives, as documented |
| Two concurrent launches | Interactive gate: each receives only its own addressed messages and answer |

### H.9.1 failure handling (`failure`)

A separate session stays open through every case and must keep all its
processes and its reader; it completes an exchange after the runtime exit.

| Case | Measured |
|---|---|
| Missing runtime | `PATH` without the runtime: non-zero exit, `<runtime> executable not found; install the runtime or set <RUNTIME>_BIN`, nothing registered, no run directory or lock |
| No bus configuration | Inside a user and mount namespace that hides `/run/agent-bus`, with no address or token: `bus is not configured; starting a plain runtime session`, the TUI answers typed input, no MCP face, no bus identity; SIGTERM exits 143 and leaves no process |
| Helper startup failure | Codex `-c model_providers=5`; OpenCode an unwritable data directory: non-zero exit with the server's own reason, no session reported ready, no process, reader, run directory or lock left. Claude's one helper, the channel registration, fails on a read-only configuration: reported, and the session keeps its tools |
| Runtime exit | The runtime refuses `--no-such-option` and exits on its own: the launcher returns that same status (Codex 2, OpenCode 1, Claude 1), and every process it started, its run directory, locks and reader are gone |
| Contrary mode | An OpenCode profile that asks before every tool: a shell command still writes outside the working directory with no prompt. Claude `--permission-mode plan`: argv carries `--enable-auto-mode` only, and the session shows auto mode. **Codex fails**: with `-a on-request -s read-only` the write outside the working directory never happens, because the thread runs `workspace-write`/`on-request` (above). The first form of this check wrote inside the working directory, which workspace-write allows, so it passed and its mutant survived; that is what exposed the gap |

### H.9.3 session names (`names`)

| Check | Measured |
|---|---|
| Missing-name fallback | Label `<runtime>(<dir>)`, address derived from the directory |
| Assigned runtime name | A title set offline through the runtime's own API (Codex App Server, OpenCode server; Claude's title metadata) labels the session and derives `#<runtime>/assigned-title@fixture`; the idle fallback address is released |
| Rename with a reply pending | The session's own `ab_rename` runs while the message it answers is still unanswered: the reply leaves from `#<runtime>/renamed-live@fixture`, the face's file and credential authenticate as the new name, the old address is released and refuses messages, and a new message to the new address is answered |
| Restart after a title change | A message queued at the old address while the launcher was down stays there, unread and unmoved; the launcher reports the retained address; the new address answers |
| Explicit address | `AGENT_BUS_NAME` stays fixed when the title changes, the title still labels it, and a live rename is refused |
| Colliding titles | Two sessions titled `Same Title` run at once as `Same Title` / `Same Title #2` on `same-title` / `same-title.2`; each answers only its own message |

| Runtime | Evidence (`tmp/a78/ev-launch/`) |
|---|---|
| Codex | `launch-codex.log`: 61 checks, then `[enforced-mode]` fails as described; `names-codex.log` 57/0 for H.9.3 |
| OpenCode | `launch-opencode.log`: 119 checks, 0 failed |
| Claude | `launch-claude.log`: 108 checks, 0 failed (real model) |

## Mutations

`tmp/a78/scripts/mut.py` with `mutcases.py`; logs `tmp/a78/mut-rest-logs/mutations-*.log`.
Runs that first failed at a check other than the named one (an expected-text
pattern too narrow, a mutant whose edit did not apply) were corrected and rerun;
the table holds the reruns. One batch briefly included the uncommitted Codex
mode hunk above; its runs were discarded and repeated on clean source.

| Mutant | Runtimes | Named failure |
|---|---|---|
| Force resume on empty history (unfiltered Codex/OpenCode listing; Claude `--continue`) | all three | `[fresh]` Codex: the other directory's conversation is resumed, so the forwarded prompt is not its first turn; OpenCode `a fresh conversation, not another directory's`; Claude exits at `[other-directory]` (no conversation to continue) |
| Drop the last forwarded argument | all three | `[fresh] the runtime receives the forwarded arguments exactly once, in order` |
| Codex pusher on a second App Server | Codex | `[fresh] the session's inbox has its reader` times out |
| No continuation | all three | `[history] the same conversation continues`; OpenCode: its reader never appears |
| Two launches share one identity | all three | interactive gate: the second has no inbox reader (`already has a reader`), `both native TUIs and pushers ready` times out |
| Two launches share one endpoint | Codex, OpenCode | Codex as above; OpenCode: the second session's input reaches the first's model (`unexpected model input`) |
| Readiness failure suppressed | all three | `[helper-failure] the launcher reports the failed helper` |
| Plain-session notice removed | Codex | `[no-bus]` notice times out |
| Exit status forced to zero | all three | `[missing-runtime] the launcher exits non-zero (0)` |
| Cleanup removed (children and run directory) | all three | `[helper-failure] no run directory or lock is left` |
| Enforcement removed | OpenCode, Claude | `[enforced-mode]` write prompts / `--permission-mode` reaches the runtime |
| Assigned name ignored | all three | `[assigned]` reader for `assigned-title` never appears |
| Fallback label removed | Codex | `[fallback] an unnamed session is labelled by runtime and directory` |
| Bindings merged (locks and conditional registration off) | all three | `[collision]` second session exits or never reads |
| Rename keeps the old inbox reader | Codex, OpenCode | `[rename-pending] a correlated reply to …renamed-live arrives` |
| Rename keeps the old credential | all three | `[rename-pending]` the reply never leaves under the new name |
| Rename not saved | Codex | `[restart] the launcher reports the retained old address` |
| Queued messages discarded on restart | Codex | same |
| Explicit address yields to a title | all three | `[explicit]` reader for `pinned` never appears |

## Limits

- Claude grants the enforced auto mode only where the account and model offer
  it: under an API key, or with Haiku, the same launch shows manual mode.
- A fresh Codex conversation is bound at the launcher's next two-second poll.
  Quitting inside that window restarts under a new address and leaves the old
  one dormant; the gate waits for the binding.
- A Claude profile without channels (such as an API key) still has its face
  read the inbox and hand each message to a channel the runtime ignores, so the
  message is consumed and lost. The dummy-key MCP gate therefore waits for its
  filtered `ab_consume` to stand before the answer is sent.
