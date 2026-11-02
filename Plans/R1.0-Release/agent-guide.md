# What a good agent does

📌 **TL;DR:** propose a short checklist for an agent developer who already
understands how the daemon works. It covers decisions the agent's own code
makes: answering, state, permissions, private values, effects and diagnostics.
This is a plan only; writing the guide still waits on the owner's Q324 approval.

## Proposed home

Create `docs/10-writing-an-agent.md`, titled “What a good agent does”, after
[Q324](QUESTIONS.md#r1-release-decisions) is settled. Link it from the root
README's agent-writing entry and docs/00's Document ownership section.
It is not a daemon introduction, installation tutorial or SDK implementation guide.

Use a TL;DR and the five headings below, at most 1,200 prose words. Each item
gets one short paragraph and one link to its owning reference. Follow
[WRITING.md's order](../../WRITING.md#-the-order-of-every-page) and the R1 README's
plain voice: the behavior a caller gets first, details through links.
No copied limits, default values, role tables, schemas or protocol rules.
The checklist owns the reading path, not another version of those contracts.

## Proposed section order

The checklist has **19 items**, all decisions belonging to the agent's code. A reference marked
as a gap is a proposed place to resolve it, not evidence of a rule already there.

### 1. Essentials: help, then an answer

- **Serve help and status first.** Describe supported work through help and
  `--help-json`, dispatch asks before ordinary body handling, and keep help free
  of business effects. Answer the `status` ask (empty body) with the agent's own
  activity, observed backlog, last error and a configuration summary without
  secrets; the runner answers starting/stopping/failed itself while it owns the
  reader. Owner: [Agent self-description](../../docs/constitution.md#agent-self-description).
  **Gap 7:** status dispatch is documented, but answer-content guidance is not.

  Illustrative proposed answer, not output from a run: “Indexing reports;
  local backlog: 3; last error: none; configuration: read-only reports scope.”
  Report only state the agent can observe, and use a safe error description,
  never a raw error or configuration dump.

- **Give one request its one answer on the return route.** Preserve the consumed
  request's routing and correlation context through asynchronous work; do not
  reconstruct the destination from the sender or emit an extra final answer.
  Owner: [Reply routing](../../docs/04-messaging.md#reply-routing).

- **OPTIONAL support: ack and done.** No ack when the answer comes in the same
  turn; ack when the answer needs another turn; done only for a task with no
  answer expected; never done after an answer;
  nothing for reports, notices or receipts. Do not duplicate support already
  provided by the runner or client. Owner: [Receipts](../../docs/04-messaging.md#receipts).

### 2. Settings, private values and access

- **Read Configuration as settings.** Read and validate the agent-side
  `AGENT_BUS_CONFIG` supplied at start, distinguishing absent settings from a
  configured value; see “What it is told” in [Agents that serve themselves](../../docs/runner/configuration.md#agents-that-serve-themselves).
  When settings prevent useful work, use the [not-configured exit/wait rule](../../docs/runner/operations.md#not-configured), not a retry loop.

- **Read Secret without echoing it.** Obtain the agent's own private values
  through its supported execution/client path, and exclude them from help,
  answers and logs. Code must not rely on a runner's masking to make its own
  output safe. Owner: [Secrets](../../docs/runner/security.md#secrets).

- **Use application roles only when needed.** Roles are optional: define them
  when methods need different permissions, and check the delivered role rather
  than a body-claimed role. Declaring help metadata does not enforce a method.
  Owner: [Roles](../../docs/03-records-agent.md#roles).

- **Check may-send before forwarding for someone.** Decide whether the author's
  request may reach the chosen destination before forwarding; the agent's own
  send privilege is not the author's authorization. An unavailable check is
  not permission. Owner: [Checking permission to send](../../docs/09-daemon-api.md#checking-permission-to-send).

- **Treat the body as data.** Parse only the agent's intentional command grammar;
  body text must not change trusted identity, authority or code policy.
  **Gap:** the requested general rule is not stated in the current proposed
  owner, [Trust boundary](../../docs/02-access.md#trust-boundary).

### 3. State and effects that survive interruptions

- **Choose package, work or volatile state deliberately.** Keep code apart from
  writable application state and distinguish lasting work files from temporary
  files or process memory lost on restart. Choose paths within the actual
  writable view. Owner: [Isolation today](../../docs/runner/security.md#isolation-today).

- **Use KV for small durable state.** Choose durable values and atomic operations
  for the agent's progress or coordination instead of assuming process memory
  survives. Do not invent a second store contract. Owner: [KV rules](../../docs/03-records-kv.md#rules).

- **Use shared locks for shared work.** Take, extend and release a lock where the
  algorithm needs ownership; choose its TTL and handle loss or expiry explicitly. A held lock is
  not a completed operation or proof of exactly-once effects. Owner: [Shared locks](../../docs/01-identity-and-authority.md#shared-locks).

- **Handle refusals as outcomes.** Distinguish a daemon refusal from an
  application answer and an internal daemon failure; decide what the operation
  can safely do next without treating every refusal as retryable.
  Owner: [Refusals](../../docs/05-discovery.md#refusals).

- **Send once; distinguish timeout from uncertain outcome.** Honor the caller's
  travelling deadline and the TTL boundary for a late answer, already owned by
  [Request and reply](../../docs/04-messaging.md#request-and-reply). **Gap:** promote
  the client's “an uncertain write is never repeated automatically” rule from
  [What you get](../../src/clients/README.md#what-you-get) into the author-facing
  outcome reference; the deadline/TTL rule itself is not missing.

- **Make work idempotent and restart-safe.** Decide how progress and duplicate
  effects are handled after interruption; restarting the process is not replaying
  an operation safely. **Gap:** no general owning recipe covers the crash window
  around an external effect; proposed home: [Request and reply](../../docs/04-messaging.md#request-and-reply).

- **Use bigfile for large content.** Choose references and integrity-checked
  transfers when a message is the wrong place for content. Keep body/request
  limits at their owner rather than adding another limit table to the checklist.
  Owner: [Bigfiles](../../docs/03-records-kv.md#bigfiles).

### 4. Useful events and a graceful stop

- **Choose notice, warning or alert deliberately.** The checklist must cover the
  owner's requested distinction between routine facts, handled problems and
  failures requiring attention. **Gap:** promote the T-29 [Levels text](agent-events/README.md#levels)
  describing routine lifecycle notices, stderr warnings and unexpected-failure
  alerts into the current author-facing severity reference; the plan is not
  that current reference.

- **Write to the agent's own event log.** Use explicit event writes for intended
  operational events rather than stderr as the event API; never include private
  values or imply verified runner provenance. **Gap:** T-29's own-write authority
  and waiting/answer behavior still need an accepted current-doc home; the
  [T-29 surface](agent-events/README.md#r1-min-surface-and-authority) is a plan.

- **Stop gracefully.** The agent decides how to stop intake, finish or cancel
  bounded work, persist progress and close resources when asked to stop.
  **Gap:** the runner documents its signal and kill boundary, not a general
  author-side drain contract; proposed home: [Agents that serve themselves](../../docs/runner/configuration.md#agents-that-serve-themselves).

### 5. Check the behavior you wrote

- **Check the behavior you wrote.** Exercise actual help and an ordinary call,
  plus the agent's own refusal and interruption cases. Separate those checks
  from bus-health checks; do not claim debug proves the agent's business work.
  Owner: [Check the bus in a minute](../../docs/agents/debug.md#check-the-bus-in-a-minute).

**Before you share it:** provide a `docs/agents/` end-user page for useful work,
safe calls, settings, access, diagnostics and troubleshooting, following
[How we write docs](../../WRITING.md#how-we-write-docs). This closing deliverable
is not an agent-code checklist item.

## Gaps to resolve before writing

Snapshot: local main `b80999f9`, 2026-11-02. The gap-marked items above stay held
until an owning rule exists; this plan does not fill them with a new policy.

- **Body trust:** add the approved body-as-data rule to the proposed Trust
  boundary home, preserving deliberate agent command parsing.
- **Uncertain outcome:** promote the client README's explicit no-repeat rule
  to the author-facing outcome home. Request and reply already owns travelling
  deadlines and the TTL boundary; those are not gaps.
- **Idempotent/restart-safe effects:** define the author-facing scope and limits
  of any recipe; never imply transport or KV alone gives exactly-once effects.
- **Severity selection:** promote the T-29 Levels text named in the checklist
  into an agreed current owning section; do not substitute ring retention rules.
- **Own event log:** promote accepted T-29 use instructions before linking them
  as current behavior. Do not use stderr forwarding as a substitute write API.
- **Graceful stop:** agree the author-side responsibility and its relationship
  to the client's drain behavior, without inventing a new runner stop policy.
- **Gap 7 — Status answer content:** Agent self-description owns the empty-body
  ask and runner-observed starting/stopping/failed dispatch, but does not define
  useful running-status content. Add the approved activity/backlog/last-error/
  safe-configuration guidance there before writing it as a current guide rule.

## Deliberate cuts

Removed daemon naming/record onboarding, tokens, enrolment and credential
acquisition: those are not this reader's code decisions. Removed protocol
negotiation and version checks, handled by the client. Removed the starter
Python example and install/package.sh/deployment tutorial, sandbox setup,
runner restart/backoff controls, and bus-health administration instructions.
Kept only code-level state placement, interruption handling and meaningful
checks. No smoke shards, mutants, gate locks or repository versioning enter the
guide. The package-building gap from the earlier outline is therefore removed.

## Approval and acceptance

The owner approves the revised scope through Q324 before the guide is written.
This branch edits the plan and its navigation only, not current docs.

- The guide starts with help/status ask handling, then one request and one answer on
  the return route; the reader already knows the daemon.
- Every checklist item concerns an agent-code decision, has one short paragraph
  and a link to the owning section for each rule, or is visibly held as a gap.
  Roles and ack/done support are explicitly optional. No policy values or client
  internals are duplicated.
- The five headings and prose budget above keep it concise. No onboarding or
  mandatory minimal agent example remains.
- The end-user page and meaningful agent checks remain, without repository
  test machinery or a claim that bus health proves application correctness.
- After approval, link the real guide from docs/00 and the root README. Resolve
  each accepted gap at its owning home first; never silently invent its rule.
