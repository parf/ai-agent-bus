# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q304 and Q311 to Q313 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q321 Do an agent's alerts survive a daemon restart in 1.0.0 (agent events, T-29)
  - Context: your 10-29 design keeps alerts on disk and notices and warnings in memory ([levels](agent-events/README.md#levels)). The daemon restarts on every daemon deploy, several a day plus the release deploy, and an agent's alert is not in syslog. The build goes ahead either way; disk is an add-on step after subscriptions
  - ( ) Alerts on disk in 1.0.0, as designed: about 25–40 more minutes of agent time and a storage migration (the stricter test rules apply) 🌟
  - ( ) All in memory for 1.0.0, alerts lost on a restart; disk is the first 1.0.x item ⭐
  > 💡👾 On disk: a restart several times a day would otherwise erase exactly the entries you want to see, and the cost is under an hour.

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
