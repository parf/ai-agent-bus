# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q304, Q311 to Q313, Q321, Q322, Q324 and Q325 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q327 Does "normal and fast runs write nothing to the host journal" allow systemd's own one-line scope notice
  - Context: the repo rule ([verification](../../CLAUDE.md#verification)) says normal and fast test runs write nothing to the host journal or syslog. A runner test uses `systemd-run --scope`, and systemd itself then logs one "Started run-….scope" line to your user journal per run. Our programs write nothing, and a test checks that. The existing `script_confined` test already does the same at normal. Meanwhile the new guide test runs its runner part only in the slow set, so nothing waits
  - ( ) The rule means our programs write nothing; systemd's own scope line is allowed. The runner check runs in the normal set, and the rule gets a one-line clarification 🌟
  - ( ) Literal: anything that makes systemd log runs only in the slow set; `script_confined` moves to slow too ⭐
  > 💡👾 Allow it: one systemd line per run is not our output, and the normal set catches runner breaks before landing instead of at the release gate.

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
