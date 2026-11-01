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
  - ( ) The owner runs `sudo sysadminctl -deleteUser _agentbusd` at the Mac, before the rerun and after it ⭐
  - ( ) Take MAC.2 out of R1; the macOS client still ships through npm
  > 💡👾 The one-time grant: every later Mac step then runs without the owner.

- ❓ Q299 Move these to R1.1
  - Context: none blocks the release; each waits on a choice, a sign-up or a design
  - [ ] RT23 agent status line — Q191 open; callers work today
  - [ ] M.7 named-host network rules — new capability, mechanism not approved
  - [ ] M.6b edge runners — local and shared runners work
  - [ ] RT11 RW10–RW17 runner web extras (search, logs, install/update) — the page is usable
  - [ ] RT19 root runner — an idea, not designed
  - [ ] MAC.3 macOS sandbox — deferred until asked
  - [ ] B.15 node cleanup — node operations, not a release artifact
  > 💡👾 Move all of them.

- ❓ Q300 When npm publishes `ai-agent-bus` and `ai-agent-bus-client` for the first time
  - Context: names reserved under parf73 (placeholders 0.0.1); publishing needs the owner's npm login
  - ( ) At the release, right after the STABLE commit 🌟
  - ( ) Now, as a 0.9 preview
  > 💡👾 At the release: the first public version is 1.0.0.

## Messaging agents

Q247 and Q248 (Google Chat) moved to [R1.2](../R1.2/QUESTIONS.md#open-questions) with B.27b: the owner, 2026-11-01, "we need corp acct. cant use personal".

## IMAP folders

Choices for the [IMAP folders plan](imap-folders.md#scope) ([B.30](TODO.md#bundled-agents)). Q270 is not blocking. Q269 and Q271–Q275 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q270 One cache budget for all folders, or one per folder
  - Impact: per-folder multiplies the bound by the folder count
  - ( ) One `cache_bytes` shared, with LRU eviction across folders 🌟
  - ( ) `cache_bytes` per folder
  > 💡👾 Shared: memory and disk stay bounded as today.

## IMAP large transfer

B.31 is parked behind [bigfile](bigfile.md#help-text) (owner, 2026-11-01); its open choices Q281–Q284, Q286 and Q287 wait with it in [its plan](imap-large-transfer.md#parked-questions).

## Short prompts

Choices for the [shorter prompts plan](short-prompts.md#proposal), to discuss with the owner.

- ❓ Q183 Where the protocol card lives
  - Impact: inline costs its bytes on every session start, but cannot be skipped
  - ( ) Inline in the MCP instructions, read once at start
  - ( ) A short doc the instructions tell the session to read
  - ( ) Both, with the instructions holding the minimum 🌟
  > 💡👾 Both: the minimum cannot be skipped, the rest costs nothing until read.

- ❓ Q184 How strict the message style is
  - Impact: guidance alone is free but easy to ignore; a face check is code for a style rule
  - ( ) Guidance in the card only 🌟
  - ( ) Also a soft size hint from the face, such as a warning when a body repeats the push header or exceeds a length
  > 💡👾 Guidance only for now; add the hint if long bodies keep appearing.

## Agent capabilities

- ❓ Q191 How an agent says which asks it answers, so callers stop asking agents that cannot answer
  - Context: the web and doctor send `help-json`, `help-config`, `help-secret` and `mcp` to every agent; a plain agent or a live AI session cannot answer some of them and the caller waits out its deadline (owner, 2026-10-26: "decide how to pass what an agent can do, so we do not ask dumb agents what they cannot do"). [RT23](TODO.md#bundled-agents)
  - Minimum (owner): every agent MUST answer `help`
  - Earlier options: the `help` answer carries one machine-readable capabilities line, for example `Asks: help help-json help-config help-secret mcp`, and callers read it before asking further (owner's "read capabilities from --help", recommended, no new ask or record field); or a separate `capabilities` ask returning the list as JSON; or the list stored on the record at registration so callers need no round trip
  - Owner, 2026-10-26: the capabilities line in `help` must be both machine-readable and human-readable. Sketch to refine: one line of its own, a fixed prefix and comma-separated ask names, each with a short gloss in parentheses, for example `Also answers: help-json (methods as JSON), help-config (settings), help-secret (environment names), mcp (MCP tools)`; a reader takes the words before each `(`. Still to settle: the exact prefix, whether `help` itself is listed, how an unknown future ask is ignored, and where the line sits in the help text
  - Owner, 2026-10-26: add an optional `status` the agent may declare in that line; the web shows it and may offer actions based on it. Settled by the owner: `status` is its own ask, like `help`; its answer is a state word followed by short text (`degraded: Slack socket reconnecting`)
  - Industry practice: two separate axes, lifecycle and health. systemd ActiveState has `activating`, `active`, `deactivating`, `inactive` and `failed`; Kubernetes splits startup, liveness and readiness probes from pod phases (`Pending`, `Running`, `Failed`, `Unknown`); health checks use `pass`/`warn`/`fail` (IETF health-check draft), `UP`/`DOWN`/`OUT_OF_SERVICE`/`UNKNOWN` (Spring Boot) or `OK`/`WARNING`/`CRITICAL`/`UNKNOWN` (Nagios); gRPC health has `SERVING`, `NOT_SERVING` and `UNKNOWN`
  - Settled by the owner, 2026-10-26: the answer format is exactly `word: text`
  - Settled by the owner, 2026-10-26: the six words `starting`, `stopping`, `ok`, `degraded`, `failed`, `unknown`; the web asks `status` only while someone views that agent's page, no background polling
  - Settled by the owner, 2026-10-26: expose both to bus MCP users: `status` joins the `ask` values of `ab_send` (next to help, help-json, help-config, help-secret, mcp), and the MCP instructions say to read the help capabilities line before other asks
  - Earlier proposal: one word from six, joining both axes as systemd does: lifecycle `starting` and `stopping` (the owner's "initializing / finishing"; `starting`/`stopping` is the commoner spelling), and health `ok`, `degraded` (Nagios WARNING), `failed` (CRITICAL) and `unknown`. Lowercase, exactly one first word, `:` then the text (`starting: loading 3 subscriptions`); any other first word reads as `unknown`. The web shows `starting` and `stopping` as neutral, `degraded` as a warning and `failed` as an alarm
  - Settled by the owner, 2026-10-26 ([RD116](DECISIONS.md#recorded-decisions)): the runner answers from metadata what it can; any other ask, `status` included, reaches the script as `AGENT_BUS_ASK=<ask>` with empty input. A script that knows nothing of asks runs with `--no-asks` ([RD117](DECISIONS.md#recorded-decisions)). The runner answers `status` itself when it can ([RD118](DECISIONS.md#recorded-decisions))
  - Impact: the help line keeps the daemon out of describing agents (constitution § agent self-description) and needs no new wire field, at the cost of one `help` round trip before any further ask; a record field saves that trip but makes the daemon carry agent self-description
  - Still open: which actions each state may drive, and whether `ab_ls` shows a status
  - [ ] The web offers restart on `failed` only 🌟
  - [ ] `ab_ls` shows each agent's status (asks every listed agent)
  > 💡👾 Restart on `failed` only, and no status in `ab_ls`: listing would ask every agent each time.

## RAG service

- ❓ Q194 Which keyword half of [RAG](rag/README.md#rules) hybrid search wins the trial on the owner's real Russian and English texts: Qdrant's built-in BM25 or the agent's own sparse vectors with stemming
  - Settled by: the [R.5 trial](rag/TODO.md#steps), then the owner
