# Live-runtime recovery

📌 **TL;DR:** H.9.6 passed on 2026-09-23 for Codex, OpenCode and Claude. Each
native TUI stayed open across a graceful daemon restart and a bus-child crash,
and resumed on its own under the same name and session. A killed MCP face is
now reported and recovered; before 0.8.24 its tools died with no word. Every
named mutant fails its check.

## Scope

Development host, disposable daemons only; the live node was not touched.
Codex CLI 0.156.1, OpenCode 1.18.30 (upstream build), Claude Code 2.1.281
with a copied claude.ai login and the real model. The program directory was
built from HEAD plus this change only (`tmp/scripts/rr-tree.sh`), so other
workers' uncommitted edits were excluded. The [current contract](../../../docs/08-runner-role.md#runtime-isolation-and-recovery)
owns the behaviour. The fresh-host run remains in the release gate.

## Defects found

| Before | After, 0.8.24 |
|---|---|
| A killed MCP face left Codex and OpenCode with dead tools. Push went on consuming messages whose replies then failed with `Transport closed`, and nothing was printed | The launcher notices the lost face, stops push so messages stay queued, restarts the face through the runtime, and reports delivery resumed |
| Under Claude the same death was silent | The launcher prints that tools and channel are off, and how to reconnect them from `/mcp` |
| A dead App Server or OpenCode server ended the session with no way back | The exit line prints the exact `ab-<runtime>` resume command for the same session |
| The face told the model it "stops being reachable if the daemon restarts" | It says the face reconnects under the same name; restart and crash both resumed with no action |
| The gate's first crash check accepted a message consumed before the crash and handed over again | Raised as Q115. The owner accepted redelivery under the same `message_id` ([durability](../../../docs/04-messaging.md#durability)), and the gate now asserts that id |

## Checks

[Recovery gate](../../../src/acceptance/runtime-recovery.ts) drives one
launcher-owned TUI on a PTY. Each exchange sends a random value, and the
peer must get back exactly one correlated reply from the same address, with
nothing else in its inbox. For Codex and OpenCode, the model fixture checks
three more things: the message reached the model once, the model turn carries
every earlier exchange (so the conversation is the same one), and the runtime's
own MCP `ab_send` carried the reply. For Claude, the transcript shows one
channel event per exchange, and one transcript holds all of them. After every
exchange the runtime session id, the launcher binding file and the one bus
address must all be unchanged. Every wait has a bound.

| Runtime × scenario | Codex | OpenCode | Claude |
|---|---|---|---|
| Graceful restart (SIGTERM, same `-db`) | resumed, same binding | resumed, same binding | resumed, same binding |
| Bus-child SIGKILL, supervisor restarts it | resumed, same binding | resumed, same binding | resumed, same binding |
| MCP face SIGKILL | reported, reloaded, exchange ok | reported, reconnected, exchange ok | reported; `/mcp` → agent-bus → Reconnect followed; exchange ok |
| Runtime server SIGKILL | session ends, printed `resume <id>` followed, exchange ok | printed `--session <id>` followed, exchange ok | no such sidecar |
| Final log (`tmp/runtime-recovery/final/`) | `codex.log` 68/0, exit 0 | `opencode.log` 68/0, exit 0 | `claude.log` 54/0, exit 0 |

Loss boundaries: a message queued before a graceful stop survives under its
`message_id` and is not delivered twice. After the crash, the message consumed
since the last flush came back once under the same `message_id`, as Q115
allows. The message queued at the crash came back at most once.

## Mutations

`tmp/scripts/mutate-recovery.py` copies the clean source, applies one edit,
rebuilds, and runs the real gate. A mutant counts as caught only when the gate
fails at its named check. Logs: `tmp/runtime-recovery/mutants.out`, `mutants-2.out`
and `mutants-3.out`. **12 gate runs and one unit test, all caught.**

| Mutant | Runtimes | Named failure |
|---|---|---|
| Push drops the first message after a failed read (post-recovery reply dropped) | Codex, OpenCode, Claude | `[graceful-restart] exchange 2: a correlated reply arrives` |
| Recovery binds a new runtime session | Codex, OpenCode | `[face-kill] exchange 4: the model turn carries every earlier exchange` |
| The lost-face report is removed (inactive integration hidden) | Codex, OpenCode, Claude | `timed out: the session reports its bus integration inactive` |
| No reload or reconnect through the runtime | Codex, OpenCode | `timed out: the launcher reports the face back` |
| A dead face's mark counts as live | Codex; `face-mark.test.ts` | inactive report times out; unit test fails |
| An unclean load gives a queued message a new id | Codex | `[bus-crash] a message redelivered after the crash keeps its message_id` |

The first binding mutant on Codex failed an earlier check: the new thread
started a second face. The gate now counts faces after the exchange, and
the rerun fails at the history check.

## Harness notes

- A real model can treat a peer's channel message as untrusted and hold off.
  One run did so on exchange 2 (`tmp/rr/claude-2`). The person at the keyboard
  now asks for the answers first.
- Starting the Codex and Claude gates at the same moment once showed both
  runtimes' folder-trust prompts (`final` was redone serially;
  `tmp/runtime-recovery/*-trust-prompt*`). Mutants run one at a time.
