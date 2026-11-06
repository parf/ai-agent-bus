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

- ❓ Q268 The Go MCP face's protocol: hand-write the stdio JSON-RPC, or take a Go MCP SDK
  - Context: it needs initialize, tools, resources and the Claude channel notification at the revision the face speaks today; decided at the start of WIN.3 by what the SDK covers ([windows CLI § what it costs](windows-cli.md#what-it-costs))
  - Settled by: the owner, at WIN.3
  > 💡👾 Hand-write the small part, as `mcp/rpc.ts` already does for transport.

## MCP agents

- ❓ Q178 Who adds the `mcp` tag, and when
  - Context: the runner asks each agent it serves `--mcp` at start and reload; or the daemon asks at registration; or the agent states it and nobody checks ([MCP agents](mcp-agents.md#mcp-agents))
  - Settled by: the owner
  > 💡👾 The runner, the way it adds `runner`, with an agent's own registration allowed to state it.

## Messaging agents

- ❓ Q244 Discord: who creates and owns the application and bot, for which server
  - Context: needed before any live test; no sign-up was made. Moved from R1 with the Discord agent ([plan](discord.md#open-for-the-owner))
  - Proposal: the owner names the account and the server
  - Impact: none of the Discord agent can be tried live before this
  - Settled by: the owner

## Registry context

- ❓ Q5 Peer sync trusts unsigned records and has no clock authority
  - Context: a peer can push an unsigned record for any name, and "newer wins" compares clocks that are not synchronised ([registry § registry sync](registry.md#registry-sync))
  - Settled by: the owner

## Access context

- ❓ Q16 How a per-destination token argument is told apart from asking for a name you own
  - Context: today the argument names a *principal*, which is what lets the daemon's owner get a credential for any name and a runner collect one for an agent it started. Once it names a *destination*, those two readings of one argument have to be told apart ([access § token scope](access.md#token-scope))
  - Settled by: the owner, with R1.1

- ❓ Q6 A queued body outlives the session that encrypted it
  - Context: the proposed handshake is live between two endpoints, but an inbox belongs to a name and waits for a reader that may not exist yet ([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)), and a dump reloads a backlog into a restarted daemon ([messaging § durability](../../docs/04-messaging.md#durability)). So a stored body needs a key recoverable without the sender present ([access § key modes](access.md#key-modes))
  - Context: under the settled [credential lifetime policy](../../docs/02-access.md#token-lifetime), clock-driven replacement is excluded. The design must cover deterministic derivation, identifying the needed material, and retaining or recovering it after a manual change. It must also tell refusal of new authentication apart from the ability to decrypt old work: a manual revocation needs an explicit backlog outcome
  - Settled by: the owner, with R1.1

- ❓ Q323 May an answer reach an agent whose Allow does not name the answerer ([J84](../issues/2026-11/2026-11-02.md))
  - Context: a private agent can ask `#bus` (`status`, `help`, agent events), but the answer is refused, because an answer is an ordinary send checked against the asker's Allow and the daemon keeps no reply state. 1.0.0 keeps it and documents "list `#bus` in your Allow"
  - ( ) Keep and document: an agent that asks `#bus` lists it in its Allow ⭐
  - ( ) Only the daemon's own `#bus` may answer any agent that may ask it; agent-to-agent answers stay as today 🌟
  - ( ) Any answer to an accepted ask passes the asker's Allow; the daemon then has to remember each ask
  > 💡👾 Only `#bus`: it is the daemon itself and only answers, so this fixes the trap without new reply state or wider agent reach.

- ❓ Q326 Do `#eventlog`, `#logwatch` and `#discord` also check a subscription's source only at creation
  - Context: [№Q325](../R1.0-Release/DECISIONS.md#recorded-decisions) made the daemon's `#bus` logs check the source once, at creation. These three agents share a helper that rechecks the creator's reach to the source on every delivery ([shared subscriptions](../../docs/agents/subscriptions.md)), so today the two behave differently
  - ( ) Yes, one rule everywhere: changes shipped behaviour, about 20–30 minutes of agent time plus the shared test cases 🌟
  - ( ) No, the agents keep their per-delivery recheck ⭐
  > 💡👾 One rule everywhere is easier to explain in the agent guide; not urgent, after 1.0.0.

- ❓ Q328 May a Personal agent list `#bus` in its Allow, so it can receive copies of agent-log subscriptions
  - Context: a Personal agent's Allow is limited to its owner's own agents, so `#bus` is refused ([final review A3](../issues/2026-11/2026-11-02.md)). For 1.0.0, `sub add` refuses such a target and the bus guide says so; this would widen authority policy
  - ( ) Let the Personal cohort include the daemon's own agent `#bus`, which is nobody's agent and sends only copies a manager subscribed to 🌟
  - ( ) Keep it: a Personal agent is never a subscription target ⭐
  > 💡👾 Allow `#bus`: it is the daemon itself, and a person's own on-call agent is the natural place for their alerts.

## Auth context

- ❓ Q4 `authorized_keys` regeneration would drop the setup-installed token key
  - Context: `authorized_keys` is regenerated from the bundle each generation, which would drop the key `agent-bus-setup` installed for issuing tokens ([access § getting a token](../../docs/02-access.md#getting-a-token)) the moment AUTH is switched on ([AUTH role § SSH admin](auth.md#ssh-admin))
  - Settled by: the owner

## Authorization refresh

- ❓ Q35 How authorization caches observe policy changes and explicit revocations
  - Context: removing the token epoch also removes the earlier bound on stale AUTH answers. Decide when cached permissions are refreshed, how an explicit revocation reaches replicas and live sessions, and what a disconnected caller may do. This is authorization freshness, not a reopened token-expiry decision ([AUTH consistency](auth.md#consistency-window))
  - Settled by: the owner, with R1.1

## Front door

- ❓ Q72 Whether to take the single front door, and in what order
  - Context: one port serving both web and API, a public homepage describing agent-bus with repository and API links, sign-in moved to `/admin/`, and the administrative dashboard run as an on-demand Bun process on a socket rather than an always-running Go child. Raised 2026-09-18; four proposals that need not all be accepted; the questions they must answer first are in [one front door](discovery.md#one-front-door)
  - Context: since 0.8.50 the dashboard is a Bun process under its own always-running unit, and since 0.8.62 its port serves TLS beside plain HTTP
  - Settled by: the owner

## Runner hardening

Policy choices for the [M.14 hardening proposal](runner-hardening.md#runner-hardening-proposal), separate from [current isolation](../../docs/runner/security.md#isolation-today). All await the owner; no mechanism or implementation is selected. №Q279 and №Q280 are **added from T-18 review, owner to confirm**, not part of the owner's original three boundaries.

- ❓ Q276 Which runtime files and shared public code must remain visible after narrowing `/usr`
  - Decide the supported agent/runtime compatibility set and any explicit exception for sibling public code; the goal is to exclude unrelated package contents.
  - Impact: determines the positive and negative read fixtures, without choosing how the view is constructed.

- ❓ Q277 Which bus operations may a managed self-serving agent perform
  - Decide the required inbox/reply and supporting operations, any deliberately allowed broader authority, and whether the same boundary applies over IP as over a local socket.
  - Impact: defines the allowed and denied calls; no transport, token or enforcement mechanism is chosen here.

- ❓ Q278 Where the host's `share` policy must apply
  - Decide whether it governs a self-serving agent's own consume calls as well as runner-issued reads, and what a changed setting means for active readers and in-flight messages on reload.
  - Impact: preserves the approved exclusive/shared semantics while making the scope and transitions explicit; no consumer, daemon or protocol change is selected.

- ❓ Q279 Whether to add live `jsonl` work-size control — **added from T-18 review, owner to confirm**
  - First approve or decline this addition. If approved, choose the limit semantics, maximum response delay, over-limit action and treatment of in-flight work.
  - Impact: supplies the measurable bound and expected outcome for the live-pool test, without selecting a checking or enforcement mechanism.

- ❓ Q280 Whether to give agents a finite CPU default — **added from T-18 review, owner to confirm**
  - First approve or decline this addition. If approved, choose the default aggregate CPU bound, measurement window/tolerance and permitted override/reload policy.
  - Impact: defines a falsifiable busy-agent test and compatibility requirements; no new setting or scheduling mechanism is selected.

## Agent capabilities

None open: [№Q191](DECISIONS.md#recorded-decisions) is [settled](DECISIONS.md#recorded-decisions); [RT23](TODO.md#agent-capabilities) follows the [capabilities plan](agent-capabilities.md#proposal).

## Runner extensions

- ❓ Q335 M.7: which named-host filtering mechanism, and its format
  - Context: chosen and approved before implementation ([proposal](runner.md#named-hosts))
  - Settled by: the owner
- ❓ Q336 RT19: does the root runner replace the standalone starter or sit beside it
  - Context: also the sandbox under each account, and how one person's agents stay isolated from another's ([proposal](runner.md#root-runner))
  - Settled by: the owner

## Filescan

- ❓ Q334 Build FileScan and DBSync with every suggested answer below (№Q305–№Q310, [№Q314–№Q320](#dbsync))
  - Context: both plans are written ([FileScan](filescan.md#steps-and-estimate), [DBSync](dbsync.md#steps-and-estimate)); order: the shared core, then FileScan, then DBSync, about 8–10 hours of agent time after T-35 and T-36. Your steps at the end: the folder and database account to watch, and a live RAG run (one paid embedding per file)
  - ( ) Yes: take every 🌟 option below and build 🌟
  - ( ) Answer the questions one by one below first
  > 💡👾 The suggestions hold. Mind №Q306: notes over 32 KiB do not reach RAG until RAG takes bigger texts; and №Q308: folders under your home only, for now.


Open choices for [filescan](filescan.md#questions).

- ❓ Q305 How a subscriber feeds RAG, and what a rename is
  - Context: `sub add … as rag <namespace> <table>` sends RAG's own upsert and delete, with no bridge agent; RAG has no rename
  - ( ) A rename is a delete of the old id plus an upsert of the new 🌟
  - ( ) Add a rename to RAG first
  > 💡👾 Delete plus upsert: RAG has no rename and no other agent would translate.

- ❓ Q306 A file over the inline limit (32 KiB, RAG's text cap)
  - ( ) Skip it and say so in `status` for `as rag`; bigfile by hash only for a generic subscriber that opts in 🌟
  - ( ) Teach RAG to accept a bigfile reference first, as its own item ⭐
  > 💡👾 Skip and report for RAG (it cannot take a reference); a RAG bigfile item only if big notes matter.

- ❓ Q307 A subscriber added later
  - ( ) Start from now by default; `replay:true` optional, capped by `max_per_scan` 🌟
  - ( ) Replay every existing file by default
  > 💡👾 From now: a replay into RAG is paid.

- ❓ Q308 May the system runner serve a path outside any person's home
  - ( ) No, in this item: the user-account runner only, so a vault is read as its owner 🌟
  - ( ) Yes, system paths too
  > 💡👾 No for now; a system path is a later, separate decision.

- ❓ Q309 The read-only bind of one directory (step F0)
  - ( ) A general runner feature: an agent setting `dir`, with real-path checks, no symlink out, refusing `/`, the home root and the runner state directory 🌟
  - ( ) A filescan-only special case
  > 💡👾 The general feature: small, and other agents need the same bind.

- ❓ Q310 Retry until confirmed, unlike the shared send-once subscription rule
  - ( ) Keep a change pending and resend on the next tick until confirmed; alert after a configured number of failures 🌟
  - ( ) Send once, like other subscriptions
  > 💡👾 Resend: a file index that silently drops a change is wrong.

## DBSync

Open choices for [dbsync](dbsync.md#questions).

- ❓ Q314 How DBSync and FileScan share code
  - Context: both keep one state file, subscriptions with confirm and resend, and delivery
  - ( ) One shared core module that both agents use 🌟
  - ( ) Copy the code into each agent ⭐
  > 💡👾 Shared core: two copies of the resend and authority logic would drift, and the authority part is ⚠️.

- ❓ Q315 Whose DB-agent grant the query runs under
  - Context: the subscription author's rights to the target are checked as in [FileScan](filescan.md#who-may-use-it); the query itself reads data
  - ( ) The DBSync instance's own grant, fixed by its Owner; the author also needs may-send to the DB agent at `sub add` and before every forward, so nobody reads through DBSync what they could not read directly 🌟
  - ( ) The instance's own grant alone, with no author check on the DB agent ⭐
  - ( ) The subscriber's grant, forwarded per query
  > 💡👾 Own grant plus the author's check: a forwarded grant has no way to travel through a timer-driven poll.

- ❓ Q316 How a delete is noticed
  - ( ) Soft delete (a marked row keeps a new `updated`) first; the key diff later if wanted 🌟
  - ( ) Periodic key diff with a mass-delete guard now ⭐
  - ( ) Never; document that deletes are invisible
  > 💡👾 Soft delete is exact and cheap; a key diff costs a full key listing capped by `max_rows`.

- ❓ Q317 The late-commit lookback
  - ( ) Default 5 minutes, configurable, with de-duplication marks for that window 🌟
  - ( ) A long window (an hour) by default ⭐
  - ( ) No lookback: the cursor alone, late commits lost
  > 💡👾 Five minutes covers ordinary transactions at the price of a small re-read; a long one costs state and queries.

- ❓ Q318 Cursor and key types without bind parameters
  - Context: the DB agent takes one statement of text and has no binds
  - ( ) Only numeric or timestamp types in R1.1; the [type guard](dbsync.md#the-cursor) counts the declared cursor and key columns that are not and refuses at configuration and at `sub add`, never tolerant quoting 🌟
  - ( ) Add binds to the DB agent first, as its own item ⭐
  - ( ) Allow text keys through each engine's literal quoting
  > 💡👾 Numeric and timestamp only: no quoting to get wrong; a DB-agent bind feature can widen it later.

- ❓ Q319 ClickHouse in R1.1
  - Context: no transactions, so late-commit semantics differ
  - ( ) Support MySQL and PostgreSQL first; ClickHouse once its lookback behaviour is tested against the fixture ⭐
  - ( ) All three from the start, with the lookback documented per engine 🌟
  > 💡👾 All three: the DB fixture already has ClickHouse, and the cursor logic is the same.

- ❓ Q320 A counter as the `updated` column
  - Context: R1.1 accepts only a timestamp, or epoch seconds or milliseconds, because the lookback needs database time; a version counter has no such unit
  - ( ) Later, a counter mode with its own lookback in counter units (rows with a counter below the high-water mark within a window) 🌟
  - ( ) Never: refuse counters, tell the owner to add a timestamp ⭐
  > 💡👾 Park it: refuse counters in R1.1; a counter mode is its own small step if a table needs it.

## IMAP large transfer

Open choices for [B.31](TODO.md#imap-large-transfer), parked on
2026-11-01 and moved from R1 with its [plan](imap-large-transfer.md#parked-questions).
[№Q285](../R1.0-Release/DECISIONS.md#recorded-decisions) is settled.
The recommendations below are retained parked proposals.

- ❓ Q281 Chunked calls or a file handoff
  - ( ) chunked `part` and `upload` calls 🌟
  - ( ) a file the agent writes and the caller reads
  - ( ) a generic large-body mechanism ⭐
  > 💡👾 chunked calls; the file is unreadable to remote callers and to other accounts, and the generic mechanism is daemon work no other agent needs
  - Impact: decides whether anything outside `src/agents/imap` changes

- ❓ Q282 Staging bounds and lifetime
  - ( ) 100 MB in total, one staging per caller, upload expires after an hour, download staging after 15 minutes 🌟
  - ( ) tighter or looser ⭐
  > 💡👾 those numbers; they fit the default 1 GiB disk check with room for the mail cache
  - Impact: a lower total refuses 25 MB while another caller is mid-transfer

- ❓ Q283 Caller convenience for a hundred calls
  - ( ) a documented shell loop in the guide only 🌟
  - ( ) a CLI verb that loops ⭐
  - ( ) an MCP helper
  > 💡👾 the documented loop now; a verb only if the owner wants it, as client work outside this plan
  - Impact: an AI caller can loop itself; a person at a shell needs the example

- ❓ Q284 Should the agent build the MIME for an upload
  - ( ) the caller sends a finished RFC 5322 mail 🌟
  - ( ) the agent assembles one from a text body and attachment files ⭐
  > 💡👾 the caller sends the finished mail; MIME assembly is a parser surface the agent need not own
  - Impact: building attachments is the caller's job

- ❓ Q286 Is the SHA-256 at `upload begin` required
  - ( ) optional, checked when given 🌟
  - ( ) required ⭐
  > 💡👾 optional; `part` always reports the hash of the whole part
  - Impact: required makes every upload verifiable but forces callers to hash first

- ❓ Q287 Do windowed `part` answers use base64 for text too
  - ( ) base64 for every type when `--offset` or `--limit` is given, and a plain `part` unchanged 🌟
  - ( ) decoded text for text parts in windows as well ⭐
  > 💡👾 base64 in windows; exact reassembly and the hash, and a split character never shows escapes
  - Impact: the other choice cannot promise the whole-part hash for text parts

## Agent pack and runner links

- ❓ Q331 Land W1A's agent-pack branch as the first step of [AS.1](TODO.md#agents-split)
  - Context: branch `ab-w1a/agent-pack` (`01aba112`, 11-03/04, 42 files) gives the bundled agents their own SemVer 1.0.0 and changelog, makes setup require it, and moves the agent checks to `src/agents/smoke.sh`, still in this repository. You asked on 11-02 to split the agents pack with its own repository and SemVer (AS.1), which waits for an approved plan; the branch is that split without the separate repository, and it contradicts the current "Shared SemVer" rule in CLAUDE.md
  - ( ) Keep the branch parked as input to the AS.1 plan; the plan decides repository, versioning and gate together, then the branch is rebased or redone 🌟
  - ( ) Land it now as AS.1's first step: own version inside this repository, CLAUDE.md's versioning rule amended, two readers
  - ( ) Drop it
  > 💡👾 Park it for the AS.1 plan: a second version line now, before the repository question is settled, would likely be redone.
- ❓ Q332 May an `agent.d` package be a symbolic link to its source checkout
  - Context: branch `ab-w1a/agentd-links` (`8029f6fd`, 11-04): the runner loads, digests, runs and serves a package through a link to its checkout; a dangling link or a link to a file is refused, links inside a package stay refused. It widens which paths the runner trusts; today packages are deployed by copy
  - ( ) Yes, as an R1.1 item, landing with two readers and a mutant ⭐
  - ( ) No, keep copy-only packages 🌟
  > 💡👾 Keep copy-only: a link lets anyone who can write the checkout change a running agent without a deploy.
