# R1.1 questions

<!-- AB: #ab-ceo -->

## Open questions

Only unresolved choices, in [agent markup](http://localhost:7777/mdhouse/doc/agent-markup.md). IDs keep their migration identity; missing numbers belong to another plan.

## Scope

- ⏸️ Q136 Final R1.1 scope
  - Context: [R1.1 scope](README.md#scope)
  - Settled by: the owner
  > 💬 👤parf (in chat, 2026-11-05) no open scope for R1.1 yet, maybe later; so far taking missing parts and fixing bugs/issues
  >
  > 💬👾 Deferred. Until it opens, work is 1.0.x: missing parts and fixes. The owner-approved T-35 (tab titles) and T-36 (`#ai` methods) continue; FileScan and DBSync wait on №Q334.

## Windows CLI

- ✅ Q268 → [№Q268](resolved.md#Q268)

## MCP agents

- ✅ Q178 → [№Q178](resolved.md#Q178)

## Messaging agents

[№Q244](../R1.2/QUESTIONS.md#messaging-agents) moved to R1.2 with Discord (owner, 2026-11-05).

## Registry context

[№Q5](../R1.3/QUESTIONS.md#Q5) moved to R1.3 (owner, 2026-11-06), with [№Q367](../R1.3/QUESTIONS.md#Q367), which asks whether the whole peer registry follows it.

## Access context

[№Q16](../R2.0-Future/QUESTIONS.md#Q16) moved with the new auth work (owner, 2026-11-05).

[№Q6](../R2.0-Future/QUESTIONS.md#Q6) moved with the new auth work (owner, 2026-11-05).

- ✅ Q323 → [№Q323](resolved.md#Q323)

- ✅ Q326 → [№Q326](resolved.md#Q326)

- ✅ Q339 → [№Q339](resolved.md#Q339)

- ✅ Q328 → [№Q328](resolved.md#Q328)

## Auth context

[№Q4](../R2.0-Future/QUESTIONS.md#Q4) moved with the new auth work (owner, 2026-11-05).

## Authorization refresh

[№Q35](../R2.0-Future/QUESTIONS.md#Q35) moved with the new auth work (owner, 2026-11-05).

## Front door

- 🚫 Q72 → [№Q72](resolved.md#Q72)

## Runner hardening

Moved to R1.2 with M.14: 5 open questions in [runner hardening](../R1.2/runner-hardening.md#questions) (№Q276–Q280).

> 💬 👤parf (in chat to #ab-ceo, 2026-11-06 ~10:45): "Runner hardening => postpone to R.1.2"

## Agent capabilities

None open: [№Q191](DECISIONS.md#recorded-decisions) is [settled](DECISIONS.md#recorded-decisions); [RT23](TODO.md#agent-capabilities) follows the [capabilities plan](agent-capabilities.md#proposal).

## Runner extensions

- ❓ Q335 M.7: which named-host filtering mechanism, and its format
  - Context: chosen and approved before implementation ([proposal](runner.md#named-hosts))
  - Settled by: the owner
- ✅ Q336 → [№Q336](resolved.md#Q336)

## Root runner

5 open questions (№Q361–№Q365) in the [root runner plan](root-runner.md#questions), the combined README, TODO and questions for RT19 (owner, 2026-11-06).

## Filescan

- ✅ Q334 → [№Q334](resolved.md#Q334)


Open choices for [filescan](filescan.md#questions).

- ✅ Q305 → [№Q305](resolved.md#Q305)

- ✅ Q306 → [№Q306](resolved.md#Q306)

- ✅ Q307 → [№Q307](resolved.md#Q307)

- ✅ Q308 → [№Q308](resolved.md#Q308)

- ✅ Q309 → [№Q309](resolved.md#Q309)

- ✅ Q310 → [№Q310](resolved.md#Q310)

## DBSync

Open choices for [dbsync](dbsync.md#questions).

- ✅ Q314 → [№Q314](resolved.md#Q314)

- ✅ Q315 → [№Q315](resolved.md#Q315)

- ✅ Q316 → [№Q316](resolved.md#Q316)

- ✅ Q317 → [№Q317](resolved.md#Q317)

- ✅ Q318 → [№Q318](resolved.md#Q318)

### Live acceptance on this node

DBSync is proven on fixtures and a fake #db. A live run needs these four; no database server runs here today and #db is disabled. The test plan is in `local/ab-w1a/T-38-S3-live-spec.local.md`.

- ❓ Q344 Which engine the live DBSync test runs on
  - Context: both dialects pass the fixture engines; PostgreSQL's `timestamptz` exercises the time-zone guard and mixed-case names, which the fake #db only imitates
  - ( ) PostgreSQL, a local instance on this node, loopback only 🌟
  - ( ) MySQL/MariaDB, a local instance on this node ⭐
  - ( ) An existing database the owner names (host, engine, schema)
  > 💡👾 PostgreSQL: the zoned `updated` path is the riskiest one, and a loopback-only instance keeps the test off shared data.

- ❓ Q345 The database accounts for the live test
  - Context: #db reads as `dbsync_ro` (SELECT on the probe table only, read-only by default); its password lives in the #db instance's Configuration secret, never in the repo
  - ( ) Two accounts: `dbsync_ro` for #db, `dbsync_rw` for the hand-made test changes 🌟
  - ( ) One read-only account; the owner makes the row changes as a superuser ⭐
  > 💡👾 Two accounts: #db never holds a credential that can write, and the test needs no superuser.

- ❓ Q346 Where the probe table lives
  - Context: one table `dbsync_probe` with a primary key, an `updated_at` set on every update, a note column and a `gone` flag
  - ( ) Its own `probe` schema 🌟
  - ( ) The default schema ⭐
  > 💡👾 Its own schema: the grant covers one table and dropping the schema removes the whole test.

- ❓ Q347 Which #db instance the test uses
  - Context: registering an instance, handing its token to the runner and the reload are owner steps (or #ab-ceo's under the standing deploy OK)
  - ( ) A new `#db/probe` instance for this test only, removed after 🌟
  - ( ) Point the main `#db` instance at the probe database ⭐
  > 💡👾 A separate instance: removing it leaves the main #db untouched.

- ✅ Q319 → [№Q319](resolved.md#Q319)

- ✅ Q320 → [№Q320](resolved.md#Q320)

## IMAP large transfer

Open choices for [B.31](TODO.md#imap-large-transfer), parked on
2026-11-01 and moved from R1 with its [plan](imap-large-transfer.md#parked-questions).
[№Q285](../R1.0-Release/DECISIONS.md#recorded-decisions) is settled.
The recommendations below are retained parked proposals.

- ✅ Q281 → [№Q281](resolved.md#Q281)

- ✅ Q282 → [№Q282](resolved.md#Q282)

- ✅ Q283 → [№Q283](resolved.md#Q283)

- ✅ Q284 → [№Q284](resolved.md#Q284)

- ✅ Q286 → [№Q286](resolved.md#Q286)

- ✅ Q287 → [№Q287](resolved.md#Q287)

## Agent pack and runner links

- ✅ Q331 → [№Q331](resolved.md#Q331)
- ✅ Q332 → [№Q332](resolved.md#Q332)

## AI agent settings

- ✅ Q337 → [№Q337](resolved.md#Q337)

## Container image tags

- ❓ Q341 Which image tag after a change to the agent pack only (T-39)
  - Context: the agent pack gets its own version (№Q331). The standing image-push OK names "that commit's VERSION and `latest`"; a change to the agents alone does not change the bus VERSION
  - ( ) Keep today's tags: push only when the bus VERSION changes, so agent fixes ship with the next bus bump (several a day) 🌟
  - ( ) Tag `<bus>-agents.<pack>` plus `latest`; never re-push an existing tag ⭐
  - ( ) Re-push the bus tag with the new agents; a tag then no longer names one image
  > 💡👾 Keep today's tags: no new scheme, no change to the standing push OK.


## Face tool approval

- ❓ Q342 Should OpenCode ask before the bigfile file tools, like Claude and Codex
  - Context: the OpenCode launcher pre-approves every agent-bus tool, the bigfile file tools included (accepted in R1). Claude and the Codex pusher leave the file tools to an operator prompt. The docs now state the difference
  - ( ) OpenCode prompts for the file tools too, so every face has one rule; a small launcher change 🌟
  - ( ) Keep OpenCode's allow-all ⭐
  > 💡👾 One rule for every face: file tools read and write local files, so the operator approves them.

## RAG for mdhouse

- ✅ Q353 → [№Q353](resolved.md#Q353)

## Owner items from 2026-11-06

- ❓ Q354 Which image model #ai uses
  - Context: `image` is refused until an image model is configured ([Q333](DECISIONS.md)). #ai now runs on the realmo production key …9e4f (no spend limit), so every picture is paid from the site's account
  - ( ) `google/gemini-3.1-flash-image` ($0.5 in / $3 out per million tokens) 🌟
  - ( ) `google/gemini-3.1-flash-lite-image` ($0.25 / $1.5), cheapest ⭐
  - ( ) `openai/gpt-5.4-image-2` ($8 / $15), strongest
  - ( ) None for now: keep `image` refused
  > 💡👾 Gemini 3.1 Flash Image: current, cheap, and good enough for agent use; switch per call if a job needs more.

- ✅ Q355 → [№Q355](resolved.md#Q355)

- ❓ Q356 The dead realmo.com site key
  - Context: `AI.RealmoPropertyProfileOverview.api-key` (…730c) in `realmo.com/config.yaml` answers 401 "User not found", so that feature's AI calls fail; `common.yaml` `AI.OpenRouter.api-key` (…85e6) is dead too. The six working keys share one account ($175 left)
  - ( ) Point both at an existing working key, e.g. RealmoHighlights' …5f45 (unused this month) 🌟
  - ( ) The owner creates new keys with a spend limit
  - ( ) Leave them; the feature is not in use
  > 💡👾 Reuse …5f45 now (no new spend), add limits later. This is the realty repository, so #ab-ceo only proposes; the owner or the realty team edits it.

- ❓ Q359 Turn on rerank in #rag
  - Context: RAG 0.4.0 (R.7, deployed 2026-11-06 02:40) can reorder a search's hits best first, but only when `rerank_model` is set and the caller asks `"rerank":true`. Each reranked search is one paid Voyage call (up to 64 candidate excerpts plus the query), on top of the query embedding semantic and hybrid already make. It is off today
  - ( ) Leave it off until mdhouse is wired and asks for it 🌟
  - ( ) Set `rerank_model` to `rerank-2.5` (Voyage's main rerank model)
  - ( ) Set `rerank_model` to `rerank-2.5-lite` (cheaper, faster)
  > 💡👾 Off for now: nobody searches yet, and callers must opt in per search anyway; turn it on when mdhouse's #rag instance (Q353) exists.
