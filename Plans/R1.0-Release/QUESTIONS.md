# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

What stands between main (0.9.262) and the R1 release as 1.0.0 ([readiness](TODO.md#objective)).

- ❓ Q296 Freeze the wire protocol before 1.0
  - Context: from 1.0 a breaking wire change bumps MAJOR. Today `docs/09-daemon-api.md` lists 60 routes without their fields, defaults, status codes or shapes; bigfile and export/import are not in it; the daemon reports no protocol version, so a new client on an old daemon sees a bare 404; refusals are text only; the six clients share 7 conformance cases (send and call)
  - Work: the wire contract for the client surface (send/consume, kv, locks, bigfile, status, config) in 09; a `protocol` number with "daemon too old/new" in the CLI, clients and MCP, and a written compatibility policy; a refusal `kind` and a 404 route-or-data split; shared conformance cases for kv, locks, status, refusals and bigfile in all six clients, plus a check that fails when a documented route or field disappears; remove or freeze the one-release aliases
  - Cut either way: admin, people, enrol, debug and export/import specs (marked unstable); bigfile in the Rust, PHP and Java clients; a `/v1` rewrite
  - Impact: about 7–10 worker-days, 3 calendar days; R1 about Nov 9–10 instead of Nov 5–6
  - ( ) Freeze it in R1 🌟
  - ( ) Ship 1.0 without it ⭐
  > ✅ 💡👾 Freeze it in R1: without it the first real wire fix after 1.0 is a breaking change, so a 2.0.
  >
  > 💬 👤parf agree

- ❓ Q297 Correct the key-value docs to the shipped limits
  - Context: `docs/03-records-kv.md` says integers in [-2^62, 2^62-1] and 128 KiB values; the daemon accepts full int64 and 512 KiB, and the native client tests expect int64. The owner, 2026-11-01: "do not touch" the limits
  - (x) Correct the docs to int64 and 512 KiB, no code change 🌟
  - ( ) Leave the docs as they are
  > ✅ 💡👾 Correct the docs only: the behaviour stays untouched, and 1.0 then promises what it ships.
  >
  > 💬 👤parf agree

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
