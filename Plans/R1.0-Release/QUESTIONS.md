# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q300 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q301 When RAG gets its keyword search (your Q194 answer)
  - Context: main's RAG has no search handler yet; the keyword field is unused and the rules reject new settings
  - Size: the setting alone (built-in BM25 or external, default built-in) 3–5 hours; a working built-in BM25 search with real Qdrant checks about 2–3 worker-days. RAG only, no daemon change; an external engine is not included
  - Impact: in R1 it competes with the protocol freeze (T-24) for the same workers and puts the Nov 5–6 date at risk
  - ( ) R1.1 🌟
  - ( ) R1, after T-24
  - ( ) R1, alongside T-24
  > 💡👾 R1.1: RAG search is a new feature, not a release blocker, and the protocol freeze is what 1.0 must get right.

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
