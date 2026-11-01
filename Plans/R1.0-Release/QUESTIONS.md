# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q302 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q303 Set an allowed bigfile directory for AI sessions on this node
  - Context: after J1 (~16:00) the MCP bigfile tools work only inside `AGENT_BUS_BIGFILE_DIR` and refuse when it is unset (your rule, 14:53). Anything in that folder can be uploaded, even by a turn a bus message started, so it must be a dedicated exchange folder, never the home or a project tree
  - ( ) Leave it unset: AI sessions have no bigfile tools; people still use the `agent-bus bigfile` CLI 🌟
  - (x) Set it to a new empty `/home/parf/agent-bus-files` (0700) for every AB-DVP and `work` window ⭐
  > ✅ 💡👾 Leave it unset until a session needs it: nothing uses the MCP bigfile tools today.
  >
  > 💬 👤parf agree
  >
  > 💬👾 The picked option sets `/home/parf/agent-bus-files`, but the ✅ is on "leave it unset". Which one?

- ❓ Q304 Name of the refusal kind for "no such data" before the protocol freezes
  - Context: T-24 froze a closed set of refusal kinds ([daemon API](../../docs/09-daemon-api.md#stability-and-compatibility)). A missing record, key or value answers 404 with kind `unknown`; a missing endpoint answers `no_route`. `unknown` reads like "unknown error". After 1.0 renaming it is a breaking change; before 1.0 it is a quick change in the daemon, the six clients and the shared tests
  - ( ) Rename it to `not_found` before 1.0 🌟
  - ( ) Keep `unknown`
  > 💡👾 Rename to `not_found`: a client author reads the kind name without the docs, and this is the last free chance.

## Messaging agents

Q247 and Q248 (Google Chat) moved to [R1.2](../R1.2/QUESTIONS.md#open-questions) with B.27b: the owner, 2026-11-01, "we need corp acct. cant use personal".

## IMAP folders

None open: Q269 to Q275 are [settled](DECISIONS.md#recorded-decisions) for the [IMAP folders plan](imap-folders.md#scope).

## IMAP large transfer

B.31 is parked behind [bigfile](bigfile.md#help-text) (owner, 2026-11-01); its open choices Q281–Q284, Q286 and Q287 wait with it in [its plan](imap-large-transfer.md#parked-questions).

## Short prompts

None open: Q183 and Q184 are [settled](DECISIONS.md#recorded-decisions) for the [shorter prompts plan](short-prompts.md#proposal).

## Agent capabilities

None open: Q191 is [settled](DECISIONS.md#recorded-decisions); RT23 moved to R1.1.

## RAG service

None open: Q194 is [settled](DECISIONS.md#recorded-decisions).
