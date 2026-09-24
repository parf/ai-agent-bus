# MCP minimum in each runtime

📌 **TL;DR:** H.9.2 passed on 2026-09-23 for Codex, OpenCode and Claude. In
each launcher-started native TUI the runtime itself lists an allowed service,
does not see a forbidden one, is refused a direct send to it, and returns a
service answer that exists only after the service consumed the request. All
18 named mutants fail their checks, including a receipt sent in place of the
answer.

## Scope

Development host, disposable daemons and clean runtime profiles; the live node
was not touched. The program dir was built by `src/build.sh` from the 0.8.27
tree, as a package ships it. Codex CLI 0.156.1, OpenCode 1.18.30 (upstream
build), Claude Code 2.1.281. The [MCP minimum](../../../docs/05-discovery.md#mcp-minimum)
owns the contract.

## Checks

The [interactive gate](../../../src/acceptance/runtime-interactive.ts) starts
two launcher-owned TUIs per runtime on PTYs, each in a new home and a working
directory with spaces. The [model fixture](../../../src/acceptance/runtime-model-fixture.ts)
only chooses the next tool call; the runtime executes it and the fixture reads
back the runtime's real tool result:

| Step | What must hold |
|---|---|
| Tools loaded | The model request declares `ab_ls` and `ab_send` from the agent-bus server |
| List | The runtime's `ab_ls` result contains `#echo@fixture` and not `#forbidden@fixture` |
| Refusal | The runtime's `ab_send` to `#forbidden@fixture` is refused, not accepted |
| Call | `#echo@fixture` receives one request per session, with sender, topic and tag |
| Unique response | The service makes a random answer **after** consuming; the peer must receive exactly that body from the session, and the sibling session must never see it |

Codex and OpenCode get the service answer pushed into the TUI. Claude, which
runs against the loopback model under a literal dummy key, has no channel in
such a profile, so it collects the answer with a filtered `ab_consume`: the
contract's other documented path. Claude's channel exchange is H.8's
[live gate](runtime-delivery.md#checks).

| Runtime | Evidence (`tmp/a78/ev-h92/`) |
|---|---|
| Codex | `int-codex.log`: 23 checks, 0 failed |
| OpenCode | `int-opencode.log`: 23 checks, 0 failed |
| Claude | `int-claude.log`: 19 checks, 0 failed (no native endpoint to attack) |

## Mutations

`tmp/a78/scripts/mut.py` (cases in `mutcases.py`) copies the source,
applies one edit, rebuilds, and runs the real gate; a mutant counts only when
the gate fails at its named check. Logs: `tmp/a78/mut-h92-logs/mutations-1.log`
and `mutations.log`.

| Mutant | Runtimes | Named failure |
|---|---|---|
| Remove tool loading (Codex `mcp_servers`, OpenCode `mcp` entry, Claude `--mcp-config` and local-scope name) | all three | `runtime did not load the real MCP tools`; Claude never gets an inbox reader: `both native TUIs and pushers ready` times out |
| `ab_ls` answers "nothing is registered" | all three | `MCP listing absent or leaked hidden service` |
| Drop the response (the pushed answer; Claude's filtered `ab_consume`) | all three | `correlated replies execute through MCP` times out; Claude: `filtered ab_consume returned no service answer` |
| ACL bypass in the listing (`canSee` removed) | all three | `MCP listing absent or leaked hidden service` |
| ACL bypass on send (`may` removed) | all three | `forbidden MCP send did not refuse` |
| The service sends a `done` receipt instead of its answer | all three | same as a dropped response: acceptance is not the service response |

## Harness notes

- A first drop-response mutant counted with `++undefined` and dropped
  nothing; both runtimes passed. The corrected mutant is the one recorded.
- Mutant runs live under a short directory: a Unix socket path over 108
  bytes made the first batch fail at daemon start, which is not a catch.
