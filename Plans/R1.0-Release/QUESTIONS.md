# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q301 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q302 Set up RAG on this node so keyword search can be used
  - Context: RAG keyword search (Q301) lands about 14:55, but RAG is not configured here: `#rag@parf.us` answers "configure RAG block names and collection" and `#qdrant` has no collections. Setup is the owner's by design: create one Qdrant collection, grant `#rag` the `#qdrant`, `#voyage` and `#ai` blocks, set its Configuration ([installing it](../../docs/agents/rag.md#installing-it))
  - Cost: embedding every stored record calls Voyage (paid); classification calls the `#ai` block (paid)
  - ( ) Set it up now: collection `notes`, model voyage-3.5 (1024 dimensions), the three blocks granted; #ab-ceo runs it 🌟
  - ( ) Set it up with another collection name or model (say which)
  - ( ) Leave RAG unconfigured until there is data to store ⭐
  > 💡👾 Set it up now with `notes` and voyage-3.5: an empty collection costs nothing, and the keyword search is then tried live before 1.0.

- ❓ Q303 Set an allowed bigfile directory for AI sessions on this node
  - Context: after J1 (~16:00) the MCP bigfile tools work only inside `AGENT_BUS_BIGFILE_DIR` and refuse when it is unset (your rule, 14:53). Anything in that folder can be uploaded, even by a turn a bus message started, so it must be a dedicated exchange folder, never the home or a project tree
  - ( ) Leave it unset: AI sessions have no bigfile tools; people still use the `agent-bus bigfile` CLI 🌟
  - ( ) Set it to a new empty `/home/parf/agent-bus-files` (0700) for every AB-DVP and `work` window ⭐
  > 💡👾 Leave it unset until a session needs it: nothing uses the MCP bigfile tools today.

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
