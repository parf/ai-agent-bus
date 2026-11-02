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
  >
  > 💬 👤parf elaborate
  >
  > 💬 👾 Two gaps on the Runners page, both display only:
  > - RW4: when the runner does not answer, the page shows its agents as "not known". The plan wanted the last answer kept and shown as stale (for example "running, as of 10:42"). The web face keeps nothing between requests, so this needs a small cache.
  > - RW8: the page explains that a saved setting takes effect at the next runner reload, but it does not mark which settings are saved and not yet applied. That needs a per-setting comparison between the record and the running process.
  > Nothing is wrong or unsafe today; these are clarity only. Still R1.1.

- ❓ Q312 Agent events (RT34): per-agent notice/warning/alert logs in the daemon, shown in the API and web
  - Context: under discussion since 2026-10-29 and unassigned; it waits for you to ask for the API shape ([design](agent-events/README.md#levels)). The `#eventlog` agent already gives named logs
  - ( ) R1.1 🌟
  - (x) R1
  > 💡👾 R1.1: it is a new daemon feature, and adding it now would reopen the frozen protocol.
  >
  > 💬 👤parf review our eventlog; try to unify systems
  > they do almost the same
  >
  > moreover daemon itself "bus" agent should provide logs agent's logs subscriptions similar to eventlog 
  >
  > discuss
  >
  > 💬 👾 Agreed: they are the same thing. #eventlog already has levels (notice, warning, alert), rings with sizes and TTLs, read, pull, clean and `sub add` with a level and a regex. RT34 designed the same rings again inside the daemon. Proposal: one engine in the daemon, with #eventlog as its user.
  > - The daemon (`#bus`) owns the log engine: rings per level, TTLs, subscriptions. The #eventlog code moves into it.
  > - Each agent automatically gets a log under its own name. The runner writes starts, stops and failures to it, and stderr lines as warnings; the daemon writes its own warnings about the record; the agent writes notices itself.
  > - Owner and Maintainers of an agent read it and subscribe: `#bus` `events <agent>`, `sub add <target> <agent> [level] [/regex/]`, the same verbs as #eventlog today.
  > - Named logs that are not agents (`deploys`, CI) stay as today: #eventlog becomes a thin front over the same engine, or its verbs move to `#bus` and #eventlog is retired. Your choice; I recommend the thin front, because existing callers keep working.
  > - Correction to my 🌟 above: this does not reopen the frozen protocol. New `#bus` methods and a new route are compatible changes under the [freeze rules](../../docs/09-daemon-api.md#stability-and-compatibility).
  > Cost: the team's estimate is coming. If it is more than about two hours of agent time, I suggest 1.0.0 ship now and this goes in 1.0.1 or 1.1. Asked #ab-dir for the plan and the estimate.

- ❓ Q313 Do the macOS archives and npm packages ship in 1.0.0
  - Context: the release workflow builds only the two Linux runners. The macOS CLI, daemon (MAC.2) and npm client (P.1c) all passed on your Mac today from archives cross-built here (Go builds darwin from Linux, no Mac runner needed); `release-verify.sh` already checks darwin archives when present
  - (x) Ship them: the workflow cross-builds darwin arm64 and x64, verifies them, attaches them, and npm publishes the macOS client packages; one more Mac check on the published 1.0.0 🌟
  - ( ) Linux only in 1.0.0; macOS in 1.0.1 ⭐
  > ✅ 💡👾 Ship them: all three Mac checks passed today, and a Mac user can then `npm install -g ai-agent-bus-client`.
  >
  > 💬 👤parf agree

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
