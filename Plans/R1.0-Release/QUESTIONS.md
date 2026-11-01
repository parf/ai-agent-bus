# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

What stands between main (0.9.262) and the R1 release as 1.0.0 ([readiness](TODO.md#objective)).

- ❓ Q298 How the macOS test account is removed for MAC.2
  - Context: the rerun at 0.9.243 is built. macOS refuses to delete a user from any ssh session, root included (TCC, -14120), so the `_agentbusd` account left by the 10-31 run blocks the rerun, and the rerun leaves one again. Instructions on the Mac: `~/agent-bus-dev/agent-bus-handoff.md`
  - ( ) A one-time Full Disk Access grant to `/usr/libexec/sshd-keygen-wrapper`; #ab-ceo does the rest over ssh 🌟
  - (x) The owner runs `sudo sysadminctl -deleteUser _agentbusd` at the Mac, before the rerun and after it ⭐
  - ( ) Take MAC.2 out of R1; the macOS client still ships through npm
  > ✅ 💡👾 The one-time grant: every later Mac step then runs without the owner.
  >
  > 💬 👤parf agree
  >
  > 💬👾 The picked option is the second one (you run `sysadminctl` at the Mac), but the ✅ is on the one-time grant. Which one?

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
