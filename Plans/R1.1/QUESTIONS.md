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

## Filescan on-demand sync

Four open questions for the [on-demand sync](filescan.md#on-demand-sync) (T-48).

<a id="Q372"></a>

- ✅ Q372 → [№Q372](resolved.md#Q372)

<a id="Q373"></a>

- ✅ Q373 → [№Q373](resolved.md#Q373)

<a id="Q374"></a>

- ✅ Q374 → [№Q374](resolved.md#Q374)

<a id="Q375"></a>

- ✅ Q375 → [№Q375](resolved.md#Q375)

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
  - Context: the agent pack's version is internal deploy bookkeeping ([№Q331](resolved.md#Q331), revised by [agent versions](agent-versions.md#rules)). The standing image-push OK names "that commit's VERSION and `latest`"; a change to the agents alone does not change the bus VERSION
  - ( ) Keep today's tags: push only when the bus VERSION changes, so agent fixes ship with the next bus bump (several a day) 🌟
  - ( ) Tag `<bus>-agents.<pack>` plus `latest`; never re-push an existing tag ⭐
  - ( ) Re-push the bus tag with the new agents; a tag then no longer names one image
  > 💡👾 Keep today's tags: no new scheme, no change to the standing push OK.


## Agent versions

- ✅ Q393 → [№Q393](resolved.md#Q393)

- ✅ Q394 → [№Q394](resolved.md#Q394)


## Agent images

- ❓ Q395 Where does an agent's image live
  - Context: a bigfile follows the record's visibility, so whoever sees the agent sees the image, but it expires and is put again by the agent ([lifetime](agent-images.md#rules)). The record's key-value store never expires but is read only by the Owner, Maintainers and the agent, so ordinary viewers would see no image
  - ( ) The record's bigfile, put again under its lifetime rule 🌟
  - ( ) The record's key-value store, no refresh, visible to its readers only ⭐
  - ( ) The web's own cache; nothing on the record
  > 💡👾 The bigfile keeps who may see it right; the key-value store is simpler but hides the picture from most users.

- ❓ Q396 When does an agent's generated image get made
  - Context: through `#ai`'s `agent-image` method and its image model (Gemini 3.1 Flash Image, [№Q354](resolved.md#Q354)), paid from the production key, at most 1 a minute and 50 a day ([cost](agent-images.md#cost-and-size))
  - ( ) When an agent first appears with no image, plus a Regenerate button for the Owner and Maintainers 🌟
  - ( ) Only when the Owner or a Maintainer presses Generate ⭐
  > 💡👾 First appearance plus Regenerate gives every agent a picture; the button alone costs nothing unless someone asks.

- ❓ Q397 Who may call `#ai`'s `agent-image`
  - Context: the method is allowed by a label in `#ai`'s ACL, each picture is paid from the production key, at most 50 a day; the default list is `#ai`'s Owner and their agents, who generate automatically. An outside agent's owner can use Regenerate only if `#ai`'s Owner adds them ([spend gate](agent-images.md#identity-and-spend))
  - ( ) `#ai`'s Owner and their agents 🌟
  - ( ) Anyone, within the daily cap ⭐
  - ( ) Nobody
  > 💡👾 `#ai`'s Owner and their agents: no stranger spends the key by registering, and the Owner can add anyone later.

- ❓ Q398 How are our own agents' pre-generated images shown
  - Context: 24 images generated once in the house style, reviewed by you, committed into each agent's folder; nothing is generated until you OK the count and cost ([cost](agent-images.md#cost-and-size))
  - ( ) Ordinary images, no mark 🌟
  - ( ) Marked "AI" like generated ones ⭐
  > 💡👾 Ordinary images: you reviewed them and they ship with the code, so the mark would say nothing true.

- ❓ Q400 What style do generated images follow
  - Context: the prompt is the agent's name and description; Q399 is not a question here: that number appears only in git history, so the set skips it
  - ( ) One house style for every image, fixed in the prompt 🌟
  - ( ) Free per prompt ⭐
  > 💡👾 One style: the list looks like a set, and the same prompt reproduces an image.

- ❓ Q401 What if the image model cannot return a 512×512 picture
  - Context: nothing in the pack scales images and `#ai` returns only a picture within the caps, 256 KiB and 512×512; a picture the provider returns and `#ai` refuses is still paid; the repository does not show which sizes Gemini 3.1 Flash Image returns ([plan](agent-images.md#rules))
  - ( ) `#ai` asks for the smallest size the model offers and returns nothing larger than the caps; the agent keeps the placeholder 🌟
  - ( ) Raise the caps for generated images, for example to 1024×1024 within 256 KiB ⭐
  - ( ) Let `#ai` scale with an image library, which needs [an approved dependency](../../src/MODULES.md#external-tools)
  > 💡👾 Keep the caps and the placeholder: a picture that does not fit is no reason to add a dependency or loosen the rule.


## Face tool approval

- ❓ Q342 Should OpenCode ask before the bigfile file tools, like Claude and Codex
  - Context: the OpenCode launcher pre-approves every agent-bus tool, the bigfile file tools included (accepted in R1). Claude and the Codex pusher leave the file tools to an operator prompt. The docs now state the difference
  - ( ) OpenCode prompts for the file tools too, so every face has one rule; a small launcher change 🌟
  - ( ) Keep OpenCode's allow-all ⭐
  > 💡👾 One rule for every face: file tools read and write local files, so the operator approves them.

## RAG for mdhouse

- ✅ Q353 → [№Q353](resolved.md#Q353)

The T-49 setup gives mdhouse its own stack on the user-account runner ([№Q353](resolved.md#Q353)): `#rag/mdhouse` with its own `#qdrant/mdhouse`, `#voyage/mdhouse` and `#ai/mdhouse`, collection `rag_mdhouse`, nothing borrowed from the root `#rag`. Three things it needs come from the owner.

- ✅ Q376 → [№Q376](resolved.md#Q376)

- ✅ Q377 → [№Q377](resolved.md#Q377)

- ✅ Q378 → [№Q378](resolved.md#Q378)

- ✅ Q391 → [№Q391](resolved.md#Q391)

## Owner items from 2026-11-06

- ✅ Q354 → [№Q354](resolved.md#Q354)

- ✅ Q355 → [№Q355](resolved.md#Q355)

- ✅ Q356 → [№Q356](resolved.md#Q356)

- ✅ Q359 → [№Q359](resolved.md#Q359)

## Deploy refresh

- ❓ Q402 Should `git-install.sh` refresh the bundled agents a person's user runner runs (J128)
  - Context: a deploy restarts each person's user runner, but its package copies in `~/.local/share/agent-bus/agent.d` stay as installed, so `tmux-sessions` ran the 2026-10-29 code after the 1.0.54 deploy ([J128](../issues/2026-11/2026-11-07.md#deploy)). Refreshing them writes code into another account's home
  - ( ) Re-install only the pack packages it can identify (they carry `support/agents/VERSION`), from the same build, as that person through `setpriv` like the runner refresh, with an opt-out flag; a person's own agents are never touched 🌟
  - ( ) Warn only: name each stale pack package and print the command that refreshes it ⭐
  > 💡👾 Re-install the pack packages: the runner is already restarted on new code, so a pack agent left on old code is the worse surprise.

