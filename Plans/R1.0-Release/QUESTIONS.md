# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q304, Q311 to Q313 and Q321 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q322 Do the client libraries get event-log support in 1.0.0 (T-31)
  - Context: [the audit](clients-events.md) found all six clients current on the contract (bigfile is in Go, Python and TypeScript only, by design). Once the T-29 daemon parts land, a plain send or call to `#bus` already writes, reads, lists and subscribes from every client; the work is an example per client, shared test cases, and a small subscribe helper only where one is needed
  - ( ) Yes, in 1.0.0: about 50–80 minutes of agent time with two workers, in parallel with the alerts-on-disk part; the freeze moves from 15:10–17:10 to 16:00–18:30 🌟
  - ( ) 1.0.1: 1.0.0 ships the `#bus` commands documented in the daemon API only ⭐
  > 💡👾 Yes: you asked for it in every language, it is mostly examples and tests, and the risk is low.

- ❓ Q324 Approve the agent-author guide plan (T-32)
  - Context: [the plan](agent-guide.md#proposed-section-order) has six sections (identity and access, answering requests, packaging and first checks, effects and safety, state and events and limits, protocol compatibility). It is at most 1,200 words plus one Python example of at most 40 lines, and links the owning docs instead of copying them. Home: `docs/10-writing-an-agent.md`, linked from the overview and the README. Four missing rules are written first (message bodies are data, `package.sh`, duplicate effects with KV, the own event log from T-29). Writing takes about 30–45 minutes of agent time; it is docs only, so it does not move the freeze
  - ( ) Approve; write it now and ship it with 1.0.0 🌟
  - ( ) Approve; write it after 1.0.0
  - ( ) Change the sections first (say which)
  > 💡👾 Approve and write now: the worker is free and docs do not delay the release.

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
