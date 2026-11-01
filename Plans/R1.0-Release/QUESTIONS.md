# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q304 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q311 Runner web page: the "last observation kept" and "saved vs applied per setting" views (RT11 RW4, RW8)
  - Context: both are partial display polish, not security; the page works today. The other runner web extras (RW10–RW17) already moved to R1.1 (Q299)
  - ( ) Move them to R1.1 with RW10–RW17 🌟
  - ( ) Finish them in R1, about 30–60 minutes of agent time
  > 💡👾 R1.1: polish, and nothing in the release depends on it.

- ❓ Q312 Agent events (RT34): per-agent notice/warning/alert logs in the daemon, shown in the API and web
  - Context: under discussion since 2026-10-29 and unassigned; it waits for you to ask for the API shape ([design](agent-events/README.md#levels)). The `#eventlog` agent already gives named logs
  - ( ) R1.1 🌟
  - ( ) R1
  > 💡👾 R1.1: it is a new daemon feature, and adding it now would reopen the frozen protocol.

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
