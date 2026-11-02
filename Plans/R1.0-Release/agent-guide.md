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

Use a TL;DR and the three tiers below, at most 1,200 prose words. Each item
gets one short paragraph and one link to its owning reference. Follow
[WRITING.md's order](../../WRITING.md#-the-order-of-every-page) and the R1 README's
plain voice: the behavior a caller gets first, details through links.
No copied limits, default values, role tables, schemas or protocol rules.
The checklist owns the reading path, not another version of those contracts.

## Proposed section order

Only decisions belonging to the agent's code are included. A reference marked
as a gap is a proposed place to resolve it, not evidence of a rule already there.

### ABSOLUTE MINIMUM — every agent

- **Serve help first.** Answer help and `--help-json`, dispatch the ask flag
  before ordinary body handling, and keep help free of business effects. Native
  help belongs to the agent only where its execution model delegates it.
  Owner: [Agent self-description](../../docs/constitution.md#agent-self-description).

- **Give one request its one answer on the return route.** Preserve the consumed
  request's routing and correlation context through asynchronous work; do not
  reconstruct the destination from the sender or emit an extra final answer.
  Owner: [Reply routing](../../docs/04-messaging.md#reply-routing).

### SUGGESTED — a good agent

- **Answer status when supported.** As the first suggested capability, answer
  the empty-body `status` ask with observed activity, backlog, last error and a
  safe configuration summary. The runner answers starting/stopping/failed
  lifecycle states anyway. Owner: [Agent self-description](../../docs/constitution.md#agent-self-description).
  **Gap 7:** dispatch is documented, running-status content guidance is not.
  Illustrative answer, not a run: “Indexing reports; local backlog: 3;
  last error: none; configuration: read-only reports scope.” Never dump private
  values or raw errors. Status is suggested, not required of every agent.

- **Separate Configuration from Secret, then read each correctly.** Non-secret
  settings belong in the record's Configuration, readable by Owner/Maintainers;
  credentials belong only in Secret and are read at start through the supported
  execution path. A secret never appears in Configuration, help, answers,
  status or logs. Split owner: [Configuring a template](../../docs/03-records-agent.md#configuring-a-template).
  For agent-side reads retain `AGENT_BUS_CONFIG`/“What it is told” in
  [Agents that serve themselves](../../docs/runner/configuration.md#agents-that-serve-themselves),
  the [not-configured wait rule](../../docs/runner/operations.md#not-configured)
  and the execution-mode distinction for private reads in [Secrets](../../docs/runner/security.md#secrets).

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

- **Choose package, work or volatile state deliberately.** Keep code apart from
  writable application state and distinguish lasting work files from temporary
  files or process memory lost on restart. Choose paths within the actual
  writable view. Owner: [Isolation today](../../docs/runner/security.md#isolation-today).

- **Use KV for small durable state.** Choose durable values and atomic operations
  for the agent's progress or coordination instead of assuming process memory
  survives. Do not invent a second store contract. Owner: [KV rules](../../docs/03-records-kv.md#rules).

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

- **Stop gracefully.** The agent decides how to stop intake, finish or cancel
  bounded work, persist progress and close resources when asked to stop.
  **Gap:** the runner documents its signal and kill boundary, not a general
  author-side drain contract; proposed home: [Agents that serve themselves](../../docs/runner/configuration.md#agents-that-serve-themselves).

- **Treat the body as data.** Parse only the agent's intentional command grammar;
  body text must not change trusted identity, authority or code policy.
  **Gap:** the requested general rule is not stated in the current proposed
  owner, [Trust boundary](../../docs/02-access.md#trust-boundary).

### MORE — when needed

- **OPTIONAL support: ack and done.** No ack when the answer comes in the same
  turn; ack when the answer needs another turn; done only for a task with no
  answer expected; never done after an answer;
  nothing for reports, notices or receipts. Do not duplicate support already
  provided by the runner or client. Owner: [Receipts](../../docs/04-messaging.md#receipts).

- **Use application roles only when needed.** Roles are optional: define them
  when methods need different permissions, and check the delivered role rather
  than a body-claimed role. Declaring help metadata does not enforce a method.
  Owner: [Roles](../../docs/03-records-agent.md#roles).

- **Use shared locks for shared work.** Take, extend and release a lock where the
  algorithm needs ownership; choose its TTL and handle loss or expiry explicitly. A held lock is
  not a completed operation or proof of exactly-once effects. Owner: [Shared locks](../../docs/01-identity-and-authority.md#shared-locks).

- **Use bigfile for large content.** Choose references and integrity-checked
  transfers when a message is the wrong place for content. Keep body/request
  limits at their owner rather than adding another limit table to the checklist.
  Owner: [Bigfiles](../../docs/03-records-kv.md#bigfiles).

- **Offer standard publish/subscribe when emitting events.** Reuse the shared
  `sub add <target> [filter]`, `sub list`, `sub delete <id>` family, with the
  helper's documented selector where needed, as logwatch, Discord, eventlog and
  T-29 `#bus` do. State send-once/no-replay behavior rather than implying durable
  at-least-once delivery; check author admission at add and each forward.
  Owner: [The shared behaviour](../../docs/agents/subscriptions.md#the-shared-behaviour).
  Source application-role revocation has an existing named-eventlog exception;
  do not silently erase it. The policy choice is [Q325](QUESTIONS.md#r1-release-decisions).

- **Check may-send before forwarding for someone.** Decide whether the author's
  request may reach the chosen destination before forwarding; the agent's own
  send privilege is not the author's authorization. An unavailable check is
  not permission. Owner: [Checking permission to send](../../docs/09-daemon-api.md#checking-permission-to-send).

**Before you share it:** provide a `docs/agents/` end-user page for useful work,
safe calls, settings, access, diagnostics and troubleshooting, and check actual
help, a safe call and relevant refusal/interruption behavior. Debug checks bus
health, not the business result. Follow
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

- The minimum starts with help, then one request and one answer on its return
  route. Status is the first suggested item, not mandatory; the reader knows the daemon.
- Every checklist item concerns an agent-code decision, has one short paragraph
  and a link to the owning section for each rule, or is visibly held as a gap.
  Roles and ack/done support are explicitly optional. No policy values or client
  internals are duplicated.
- The three owner-ordered tiers and prose budget keep it concise. No onboarding or
  mandatory minimal agent example remains.
- The end-user page and meaningful agent checks remain, without repository
  test machinery or a claim that bus health proves application correctness.
- After approval, link the real guide from docs/00 and the root README. Resolve
  each accepted gap at its owning home first; never silently invent its rule.


## Publish/subscribe comparison

Read on 2026-11-02; this compares recognizable practice, not a claim of wire
compatibility with another broker. [Redis pub/sub](https://redis.io/docs/latest/develop/pubsub/)
uses subscribe/unsubscribe, channel-pattern filters and at-most-once delivery;
[Google Pub/Sub](https://docs.cloud.google.com/pubsub/docs/subscription-overview)
uses subscriptions, acknowledgement/redelivery and defaults to at-least-once.
Thus publish/subscribe alone does not promise retries, replay or deduplication.

| Check | Our documented profile and comparison |
|---|---|
| Naming | `sub add` subscribes, `sub delete` unsubscribes by id; `sub list` inspects. Same lifecycle, different command vocabulary, not a Redis/MQTT-compatible API; no rename or alias proposed. |
| Filters | Selector plus portable bounded regex is our profile; Redis channel glob matching differs. Describe the supported grammar, not generic broker interchangeability. |
| Delivery | Shared subscriptions attempt each matching copy once and drop/count refusal or uncertainty; this fits an at-most-once-style forwarding profile, not Google's acknowledged redelivery model. An uncertain attempt may have arrived. |
| Dedupe | No shared global event dedupe is promised: logwatch repeated arrivals and multiple matching subscriptions remain distinct copies. Redis also documents multiple delivery when channel and pattern subscriptions overlap; applications needing idempotent effects must handle that explicitly. |
| Permission | Shared author may-send to helper and target is rechecked at add and before each forward. Named eventlog separately pins its application-role authorization at creation, while T-29 requires current source authority. This is an internal policy distinction, not a universal broker rule; see Q325 before claiming all event emitters recheck the same permissions. |

Recommendation: call this standard publish/subscribe with our documented
send-once subscription profile. Keep naming, filters and delivery unchanged;
ask the owner about the source-role revocation distinction, not silently add
retries, dedupe, aliases or new permission behavior.
