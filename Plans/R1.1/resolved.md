# R1.1 resolved questions

📌 **TL;DR:** answered R1.1 questions with their threads verbatim; the rules they set are in [decisions](DECISIONS.md#recorded-decisions).

<a id="Q268"></a>

- ✅ Q268 The Go MCP face's protocol: hand-write the stdio JSON-RPC, or take a Go MCP SDK
  - Context: it needs initialize, tools, resources and the Claude channel notification at the revision the face speaks today; decided at the start of WIN.3 by what the SDK covers ([windows CLI § what it costs](windows-cli.md#what-it-costs))
  - Settled by: the owner, at WIN.3
  > ✅ 💡👾 Hand-write the small part, as `mcp/rpc.ts` already does for transport.
  >
  > 💬 👤parf agree
  >
  > 💬👾 → Hand-write the Go MCP face's small stdio JSON-RPC part.

<a id="Q178"></a>

- ✅ Q178 Who adds the `mcp` tag, and when
  - Context: the runner asks each agent it serves `--mcp` at start and reload; or the daemon asks at registration; or the agent states it and nobody checks ([MCP agents](mcp-agents.md#mcp-agents))
  - Settled by: the owner
  > ✅ 💡👾 The runner, the way it adds `runner`, with an agent's own registration allowed to state it.
  >
  > 💬 👤parf agree
  >
  > 💬👾 → The runner adds the `mcp` tag, as it adds `runner`; an agent's own registration may state it.

<a id="Q326"></a>

- ✅ Q326 Do `#eventlog`, `#logwatch` and `#discord` also check a subscription's source only at creation
  - Context: [№Q325](../R1.0-Release/DECISIONS.md#recorded-decisions) made the daemon's `#bus` logs check the source once, at creation. These three agents share a helper that rechecks the creator's reach to the source on every delivery ([shared subscriptions](../../docs/agents/subscriptions.md)), so today the two behave differently
  - (x) Yes, one rule everywhere: changes shipped behaviour, about 20–30 minutes of agent time plus the shared test cases 🌟
  - ( ) No, the agents keep their per-delivery recheck ⭐
  > ✅ 💡👾 One rule everywhere is easier to explain in the agent guide; not urgent, after 1.0.0.
  >
  > 💬 👤parf agree
  >
  > 💬👾 → One rule everywhere: `#eventlog`, `#logwatch` and `#discord` check a subscription's source at creation only.

<a id="Q328"></a>

- ✅ Q328 May a Personal agent list `#bus` in its Allow, so it can receive copies of agent-log subscriptions
  - Context: a Personal agent's Allow is limited to its owner's own agents, so `#bus` is refused ([final review A3](../issues/2026-11/2026-11-02.md)). For 1.0.0, `sub add` refuses such a target and the bus guide says so; this would widen authority policy
  - (x) Let the Personal cohort include the daemon's own agent `#bus`, which is nobody's agent and sends only copies a manager subscribed to 🌟
  - ( ) Keep it: a Personal agent is never a subscription target ⭐
  > 💡👾 Allow `#bus`: it is the daemon itself, and a person's own on-call agent is the natural place for their alerts.
  >
  > 💬 they can, why not,
  > BUT - do we even have functionality when bus is going to anything there
  >
  > 💬👾 → A Personal agent may list `#bus` in its Allow.

<a id="Q334"></a>

- ✅ Q334 Build FileScan and DBSync with every suggested answer below (№Q305–№Q310, [№Q314–№Q320](QUESTIONS.md#dbsync))
  - Context: both plans are written ([FileScan](filescan.md#steps-and-estimate), [DBSync](dbsync.md#steps-and-estimate)); order: the shared core, then FileScan, then DBSync, about 8–10 hours of agent time after T-35 and T-36. Your steps at the end: the folder and database account to watch, and a live RAG run (one paid embedding per file)
  - (x) Yes: take every 🌟 option below and build 🌟
  - ( ) Answer the questions one by one below first
  > 💡👾 The suggestions hold. Mind №Q306: notes over 32 KiB do not reach RAG until RAG takes bigger texts; and №Q308: folders under your home only, for now.
  >
  > 💬👾 → Build FileScan and DBSync: every 🌟 option, except where the owner picked another below.

<a id="Q305"></a>

- ✅ Q305 How a subscriber feeds RAG, and what a rename is
  - Context: `sub add … as rag <namespace> <table>` sends RAG's own upsert and delete, with no bridge agent; RAG has no rename
  - ( ) A rename is a delete of the old id plus an upsert of the new 🌟
  - ( ) Add a rename to RAG first
  > 💡👾 Delete plus upsert: RAG has no rename and no other agent would translate.
  >
  > 💬👾 → A rename is a delete of the old id plus an upsert of the new (via №Q334).

<a id="Q306"></a>

- ✅ Q306 A file over the inline limit (32 KiB, RAG's text cap)
  - ( ) Skip it and say so in `status` for `as rag`; bigfile by hash only for a generic subscriber that opts in 🌟
  - ( ) Teach RAG to accept a bigfile reference first, as its own item ⭐
  > 💡👾 Skip and report for RAG (it cannot take a reference); a RAG bigfile item only if big notes matter.
  >
  > 💬👾 → A file over the inline limit is skipped for `as rag` and reported in `status` (via №Q334).

<a id="Q307"></a>

- ✅ Q307 A subscriber added later
  - ( ) Start from now by default; `replay:true` optional, capped by `max_per_scan` 🌟
  - (x) Replay every existing file by default
  > 💡👾 From now: a replay into RAG is paid.
  >
  > 💬 u mean when we setup Filescan on exisitng file base -   first we should feed it all files
  >
  > 💬👾 → Replay every existing file by default: a new FileScan feeds the whole existing file base.

<a id="Q308"></a>

- ✅ Q308 May the system runner serve a path outside any person's home
  - ( ) No, in this item: the user-account runner only, so a vault is read as its owner 🌟
  - (x) Yes, system paths too
  > 💡👾 No for now; a system path is a later, separate decision.
  >
  > 💬👾 → System paths too, not only a person's home.

<a id="Q309"></a>

- ✅ Q309 The read-only bind of one directory (step F0)
  - (x) A general runner feature: an agent setting `dir`, with real-path checks, no symlink out, refusing `/`, the home root and the runner state directory 🌟
  - ( ) A filescan-only special case
  > 💡👾 The general feature: small, and other agents need the same bind.
  >
  > 💬👾 → The read-only bind of one directory is a general runner feature, the agent setting `dir`.

<a id="Q310"></a>

- ✅ Q310 Retry until confirmed, unlike the shared send-once subscription rule
  - (x) Keep a change pending and resend on the next tick until confirmed; alert after a configured number of failures 🌟
  - ( ) Send once, like other subscriptions
  > 💡👾 Resend: a file index that silently drops a change is wrong.
  >
  > 💬👾 → Keep a change pending and resend until confirmed; alert after a configured number of failures.

<a id="Q314"></a>

- ✅ Q314 How DBSync and FileScan share code
  - Context: both keep one state file, subscriptions with confirm and resend, and delivery
  - (x) One shared core module that both agents use 🌟
  - ( ) Copy the code into each agent ⭐
  > 💡👾 Shared core: two copies of the resend and authority logic would drift, and the authority part is ⚠️.
  >
  > 💬👾 → One shared core module for DBSync and FileScan.

<a id="Q315"></a>

- ✅ Q315 Whose DB-agent grant the query runs under
  - Context: the subscription author's rights to the target are checked as in [FileScan](filescan.md#who-may-use-it); the query itself reads data
  - (x) The DBSync instance's own grant, fixed by its Owner; the author also needs may-send to the DB agent at `sub add` and before every forward, so nobody reads through DBSync what they could not read directly 🌟
  - ( ) The instance's own grant alone, with no author check on the DB agent ⭐
  - ( ) The subscriber's grant, forwarded per query
  > 💡👾 Own grant plus the author's check: a forwarded grant has no way to travel through a timer-driven poll.
  >
  > 💬👾 → DBSync queries under its own grant; the author also needs may-send to the DB agent at `sub add` and before every forward.

<a id="Q316"></a>

- ✅ Q316 How a delete is noticed
  - (x) Soft delete (a marked row keeps a new `updated`) first; the key diff later if wanted 🌟
  - ( ) Periodic key diff with a mass-delete guard now ⭐
  - ( ) Never; document that deletes are invisible
  > 💡👾 Soft delete is exact and cheap; a key diff costs a full key listing capped by `max_rows`.
  >
  > 💬👾 → Soft delete first; the key diff later if wanted.

<a id="Q317"></a>

- ✅ Q317 The late-commit lookback
  - (x) Default 5 minutes, configurable, with de-duplication marks for that window 🌟
  - ( ) A long window (an hour) by default ⭐
  - ( ) No lookback: the cursor alone, late commits lost
  > 💡👾 Five minutes covers ordinary transactions at the price of a small re-read; a long one costs state and queries.
  >
  > 💬👾 → Late-commit lookback defaults to 5 minutes, configurable, with de-duplication marks.

<a id="Q318"></a>

- ✅ Q318 Cursor and key types without bind parameters
  - Context: the DB agent takes one statement of text and has no binds
  - (x) Only numeric or timestamp types in R1.1; the [type guard](dbsync.md#the-cursor) counts the declared cursor and key columns that are not and refuses at configuration and at `sub add`, never tolerant quoting 🌟
  - ( ) Add binds to the DB agent first, as its own item ⭐
  - ( ) Allow text keys through each engine's literal quoting
  > 💡👾 Numeric and timestamp only: no quoting to get wrong; a DB-agent bind feature can widen it later.
  >
  > 💬👾 → Only numeric or timestamp cursor and key types, refused otherwise by the type guard.

<a id="Q319"></a>

- ✅ Q319 ClickHouse in R1.1
  - Context: no transactions, so late-commit semantics differ
  - (x) Support MySQL and PostgreSQL first; ClickHouse once its lookback behaviour is tested against the fixture ⭐
  - ( ) All three from the start, with the lookback documented per engine 🌟
  > 💡👾 All three: the DB fixture already has ClickHouse, and the cursor logic is the same.
  >
  > 💬👾 → MySQL and PostgreSQL first; ClickHouse once its lookback is tested.

<a id="Q320"></a>

- ✅ Q320 A counter as the `updated` column
  - Context: R1.1 accepts only a timestamp, or epoch seconds or milliseconds, because the lookback needs database time; a version counter has no such unit
  - (x) Later, a counter mode with its own lookback in counter units (rows with a counter below the high-water mark within a window) 🌟
  - ( ) Never: refuse counters, tell the owner to add a timestamp ⭐
  > 💡👾 Park it: refuse counters in R1.1; a counter mode is its own small step if a table needs it.
  >
  > 💬👾 → A counter mode later, with its own lookback in counter units.

<a id="Q281"></a>

- ✅ Q281 Chunked calls or a file handoff
  - (x) chunked `part` and `upload` calls 🌟
  - ( ) a file the agent writes and the caller reads
  - ( ) a generic large-body mechanism ⭐
  > 💡👾 chunked calls; the file is unreadable to remote callers and to other accounts, and the generic mechanism is daemon work no other agent needs
  - Impact: decides whether anything outside `src/agents/imap` changes
  >
  > 💬👾 → IMAP large transfer uses chunked `part` and `upload` calls.

<a id="Q282"></a>

- ✅ Q282 Staging bounds and lifetime
  - (x) 100 MB in total, one staging per caller, upload expires after an hour, download staging after 15 minutes 🌟
  - ( ) tighter or looser ⭐
  > 💡👾 those numbers; they fit the default 1 GiB disk check with room for the mail cache
  - Impact: a lower total refuses 25 MB while another caller is mid-transfer
  >
  > 💬👾 → Staging: 100 MB in total, one per caller; uploads expire after an hour, download staging after 15 minutes.

<a id="Q283"></a>

- ✅ Q283 Caller convenience for a hundred calls
  - (x) a documented shell loop in the guide only 🌟
  - ( ) a CLI verb that loops ⭐
  - ( ) an MCP helper
  > 💡👾 the documented loop now; a verb only if the owner wants it, as client work outside this plan
  - Impact: an AI caller can loop itself; a person at a shell needs the example
  >
  > 💬👾 → Caller convenience is a documented shell loop in the guide only.

<a id="Q284"></a>

- ✅ Q284 Should the agent build the MIME for an upload
  - (x) the caller sends a finished RFC 5322 mail 🌟
  - ( ) the agent assembles one from a text body and attachment files ⭐
  > 💡👾 the caller sends the finished mail; MIME assembly is a parser surface the agent need not own
  - Impact: building attachments is the caller's job
  >
  > 💬👾 → The caller sends a finished RFC 5322 mail; the agent builds no MIME.

<a id="Q286"></a>

- ✅ Q286 Is the SHA-256 at `upload begin` required
  - (x) optional, checked when given 🌟
  - ( ) required ⭐
  > 💡👾 optional; `part` always reports the hash of the whole part
  - Impact: required makes every upload verifiable but forces callers to hash first
  >
  > 💬👾 → The SHA-256 at `upload begin` is optional, checked when given.

<a id="Q287"></a>

- ✅ Q287 Do windowed `part` answers use base64 for text too
  - (x) base64 for every type when `--offset` or `--limit` is given, and a plain `part` unchanged 🌟
  - ( ) decoded text for text parts in windows as well ⭐
  > 💡👾 base64 in windows; exact reassembly and the hash, and a split character never shows escapes
  - Impact: the other choice cannot promise the whole-part hash for text parts
  >
  > 💬👾 → Windowed `part` answers are base64 for every type; a plain `part` is unchanged.

<a id="Q331"></a>

- ✅ Q331 Land W1A's agent-pack branch as the first step of [AS.1](TODO.md#agents-split)
  - Context: branch `ab-w1a/agent-pack` (`01aba112`, 11-03/04, 42 files) gives the bundled agents their own SemVer 1.0.0 and changelog, makes setup require it, and moves the agent checks to `src/agents/smoke.sh`, still in this repository. You asked on 11-02 to split the agents pack with its own repository and SemVer (AS.1), which waits for an approved plan; the branch is that split without the separate repository, and it contradicts the current "Shared SemVer" rule in CLAUDE.md
  - ( ) Keep the branch parked as input to the AS.1 plan; the plan decides repository, versioning and gate together, then the branch is rebased or redone 🌟
  - ( ) Land it now as AS.1's first step: own version inside this repository, CLAUDE.md's versioning rule amended, two readers
  - ( ) Drop it
  > 💡👾 Park it for the AS.1 plan: a second version line now, before the repository question is settled, would likely be redone.
  >
  > 💬 so far
  > we have one monorepo
  >  we just use diff semvers for daemon and agent-pack
  >
  > 💬👾 → Land the agent pack with its own SemVer inside this one repository: daemon and agent pack keep separate versions.

<a id="Q332"></a>

- ✅ Q332 May an `agent.d` package be a symbolic link to its source checkout
  - Context: branch `ab-w1a/agentd-links` (`8029f6fd`, 11-04): the runner loads, digests, runs and serves a package through a link to its checkout; a dangling link or a link to a file is refused, links inside a package stay refused. It widens which paths the runner trusts; today packages are deployed by copy
  - (x) Yes, as an R1.1 item, landing with two readers and a mutant ⭐
  - ( ) No, keep copy-only packages 🌟
  > 💡👾 Keep copy-only: a link lets anyone who can write the checkout change a running agent without a deploy.
  >
  > 💬 thats the idea; i place symlink; i do git pull there
  > if needed i'll ask runner to reload
  >
  > elaborate if u still have question
  >
  > 💬👾 → An `agent.d` package may be a symbolic link to its checkout; the owner updates it with git pull and a runner reload.

<a id="Q337"></a>

- ✅ Q337 Which default models does `#ai` use
  - Context: `#ai`'s Configuration is empty, so `models` marks no default and an `ask` without a `model` is refused ("set Configuration model or provide a request model"). `image_model` is unset too
  - Context: Claude Haiku 5.5 costs $0.10 per million input tokens and $0.50 per million output tokens, with a 1M context window; Claude Sonnet 5.5 costs more
  - ( ) `model` = `anthropic/claude-haiku-5.5`: fast and cheap for routine bus asks 🌟
  - ( ) `model` = `anthropic/claude-sonnet-5.5`: stronger, dearer ⭐
  - ( ) Another model: write its id in a reply
  - ( ) No default: every caller names its model
  > 💡👾 Haiku 5.5 as the default; callers that need more name a model per request. `image_model` can be picked once images work.
  >
  > 💬 👤parf (in chat) default: anthropic/claude-sonnet-5.5: stronger and more expensive.
  >
  > 💬👾 → `#ai` Configuration `{"model":"anthropic/claude-sonnet-5.5"}` set 2026-11-05; `models` marks it (default).
  >
  > 💬 👤parf (in chat) can we ask to use latest sonnet?
  >
  > 💬👾 Yes: OpenRouter's alias `~anthropic/claude-sonnet-latest` always routes to the newest Sonnet; set it as the default.
  >
  > 💬 👤parf (in chat) reconsidered - use haiku `anthropic/claude-haiku-5.5`: fast and cheap for routine bus asks
  >
  > 💬👾 → `#ai` Configuration `{"model":"anthropic/claude-haiku-5.5"}` set 2026-11-05; `models` marks it (default).

<a id="Q323"></a>

- ✅ Q323 May an answer reach an agent whose Allow does not name the answerer ([J84](../issues/2026-11/2026-11-02.md))
  - Context: a private agent can ask `#bus` (`status`, `help`, agent events), but the answer is refused, because an answer is an ordinary send checked against the asker's Allow and the daemon keeps no reply state. 1.0.0 keeps it and documents "list `#bus` in your Allow"
  - ( ) Keep and document: an agent that asks `#bus` lists it in its Allow ⭐
  - (x) Only the daemon's own `#bus` may answer any agent that may ask it; agent-to-agent answers stay as today 🌟
  - ( ) Any answer to an accepted ask passes the asker's Allow; the daemon then has to remember each ask
  > ✅ 💡👾 Only `#bus`: it is the daemon itself and only answers, so this fixes the trap without new reply state or wider agent reach.
  >
  > 💬 in order to ack agent must be in ACL
  > bus should have * in ACL;
  >
  > 💬👾 → `#bus` answers any asker, on the asker's own route only; every other answer needs the asker's Allow, and a refused answer names the rule ([answers](../../docs/02-access.md#answers)).

<a id="Q339"></a>

- ✅ Q339 Does `#imap` also check a subscription only at creation (T-41, after [№Q326](#Q326))
  - Context: `#eventlog` and `#logwatch` forward log copies; `#imap` forwards mail through the same helper. Checking only at creation lets an author who has lost access to a mailbox keep receiving its mail. `#cron` does not use the helper; Discord waits for R1.2
  - (x) `#imap` keeps the per-delivery check, for mail confidentiality; only `#eventlog` and `#logwatch` check at creation 🌟
  - ( ) `#imap` checks at creation too: one rule everywhere ⭐
  > ✅ 💡👾 Keep the check for mail. Either answer is a one-line switch.
  >
  > 💬👾 → `#imap` keeps the per-delivery check; only `#eventlog` and `#logwatch` check at creation (owner, 2026-11-06). The code already does this, so nothing changes: only those two agents set `source_at_creation_only`, and the [shared-behaviour table](../../docs/agents/subscriptions.md#source-check) says so.

<a id="Q336"></a>

- ✅ Q336 RT19: does the root runner replace the standalone starter or sit beside it
  - Context: also the sandbox under each account, and how one person's agents stay isolated from another's ([proposal](runner.md#root-runner))
  - Settled by: the owner
  > 💬 replaced it
  >
  > for  root runner (and for real-user runner) we need to come with "default" policy
  > than will protect other root/user agents data (protect other agents secret files we hide)
  >
  > ideally come with idea how to protect env files from user itself in users runner 
  > seems liike we cant (and should not) protect root runners from root itself by its nature
  >
  > 💬 **Serg Parf:** create dedicated readme/todo plan for this section - use combined document
  > in next question file - just point - X open  questions in LINK
  >
  > 💬👾 → The root runner replaces the standalone starter (owner, 2026-11-06). The approach and its open choices are in [root runner](root-runner.md#what-the-starter-is-today).

<a id="Q72"></a>

- 🚫 Q72 Whether to take the single front door, and in what order
  - Context: one port serving both web and API, a public homepage describing agent-bus with repository and API links, sign-in moved to `/admin/`, and the administrative dashboard run as an on-demand Bun process on a socket rather than an always-running Go child. Raised 2026-09-18; four proposals that need not all be accepted; the questions they must answer first are in the [decision archive](../done/decisions-archive.md#web-cli-mcp-and-launchers), where the plan text went
  - Context: since 0.8.50 the dashboard is a Bun process under its own always-running unit, and since 0.8.62 its port serves TLS beside plain HTTP
  - Settled by: the owner
  > 💬 NO - reject this idea; lots of troubles for no gain
  >
  > 💬👾 → Rejected (owner, 2026-11-06): nothing of the single front door is built or planned.

<a id="Q353"></a>

- ✅ Q353 Which #rag endpoint mdhouse uses
  - Context: RAG 0.3.0 runs as `#rag@parf.us`, allowed to the owner only. mdhouse planned `#rag/mdhouse` and asks which is final. Its production caller is `#mdhouse@parf.us`. mdhouse wants no grants to its CTO, CEO, worker or review sessions, no wildcard FileScan identities and no manager role. The docs recommend one instance per trusted application, since any admitted caller can name any namespace
  - ( ) Provision `#rag/mdhouse@parf.us`: a second instance of the bundled agent, its own record, token and Qdrant collection, allowed to `#mdhouse@parf.us` only; #ab-ceo registers it under the standing deploy OK 🌟
  - ( ) Allow `#mdhouse@parf.us` on the existing `#rag@parf.us` ⭐
  > 💡👾 A separate instance: mdhouse's data, prompts and paid usage stay apart from anything else the owner stores, and removing it touches nothing else.
  > 💬 👤parf (in chat to #ab-ceo, 2026-11-06 ~13:50): provision it; mdhouse controls it, not #ab-ceo: "it should control it; not you - it should have grants"; then chose Maintainer grants over a new mdhouse User
  >
  > 💬👾 → `#rag/mdhouse@parf.us`, a separate instance with its own collection. parf stays Owner (a record can be owned only by a User); mdhouse's control identity (`#cx/mdhouse-cto`, and `#mdhouse@parf.us` once registered) is Maintainer: configuration, access list, subscriptions. #ab-ceo does the one-time registration and token handoff only.

<a id="Q355"></a>

- ✅ Q355 Which folders FileScan may read on this node
  - Context: FileScan is installed and refused at start: its `roots` in `runner/agents.json` are empty by default, and system and bus directories are refused whatever the roots say. mdhouse will want one instance per project
  - ( ) None until mdhouse names its projects; then one root per project directory 🌟
  - ( ) A root the owner names now (path)
  > 💡👾 Wait for mdhouse's project list: a root grants read access to everything under it.
  > 💬👾 2026-11-06: the RAG-sink resend fix W1C found is landed (`0f5340c3`, an upsert counts only when RAG echoes the stored record) and deployed with FileScan at 03:38; a root no longer pays for repeat embeds
  > 💬 👤parf (in chat to #ab-ceo, 2026-11-06 ~13:50): as Q353, mdhouse controls it with Maintainer grants
  >
  > 💬👾 → root `/home/parf/src/mdhouse` for `#filescan/mdhouse`, include `*.md`, dotfiles and `node_modules/**` excluded, namespace `mdhouse`, table `docs`. `/home/parf` is 0700, so it runs on parf's user-account runner. No subscription or ingestion until mdhouse starts it.
