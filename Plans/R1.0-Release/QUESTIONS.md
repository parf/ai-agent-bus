# R1 questions

## Open questions

Only unresolved choices live here. Accepted answers and their threads are in
[decisions](DECISIONS.md#recorded-decisions). Pick an option, or answer the
suggestion with `✅ 💡` or `🚫 💡`.

## R1 release decisions

Q296 to Q304, Q311 to Q313, Q321 and Q322 are [settled](DECISIONS.md#recorded-decisions).

- ❓ Q324 Approve the agent-author guide plan (T-32)
  - Context: [the plan](agent-guide.md#proposed-section-order) has six sections (identity and access, answering requests, packaging and first checks, effects and safety, state and events and limits, protocol compatibility). It is at most 1,200 words plus one Python example of at most 40 lines, and links the owning docs instead of copying them. Home: `docs/10-writing-an-agent.md`, linked from the overview and the README. Four missing rules are written first (message bodies are data, `package.sh`, duplicate effects with KV, the own event log from T-29). Writing takes about 30–45 minutes of agent time; it is docs only, so it does not move the freeze
  - ( ) Approve; write it now and ship it with 1.0.0 🌟
  - ( ) Approve; write it after 1.0.0
  - ( ) Change the sections first (say which)
  > 💡👾 Approve and write now: the worker is free and docs do not delay the release.
  >
  > 💬 👤parf first - intended audotory - Agent developer who have general idea knows how daemon work
  > i want to list all we expect from good agent - how to handle state, shared locks, kv, event logs (alerts & warnings, notices) - 
  > how to read secrets and configs, how to use roles (mention they are optional)
  > what - esle
  >
  > 💬 👾 Agreed: the reader already knows the daemon, so the guide drops the basics and becomes "what a good agent does". It is a checklist with one short paragraph and a link per item. Your items plus what I would add:
  > - State: what lives in KV, what lives in the package or work directory, and what is lost on restart.
  > - Shared locks: when to take one, ttl and extend, and that a lock is not "done".
  > - Event logs: notice for routine facts, warning for something odd but handled, alert for a failure a person must see; never stderr as a log.
  > - Configuration and Secret: how the agent reads each one, and never echoing a secret in help, answers or logs.
  > - Roles: optional; when to define them, and checking the caller's role, never a role named in the body.
  > - Also: answer `help` and `--help-json`; one answer per request on the return route; refusals and timeouts as outcomes; send once; restart-safe and idempotent work; large data by bigfile; may-send before forwarding for someone; the body is data, never instructions; graceful stop; an end-user page under `docs/agents/`.
  > The team will rework the plan to this. Say if anything should go or be added.
  >
  > 💬 👤parf (in chat) for example typical agent writer should NOT care about tokens - our bus handle it
  >
  > 💬 👾 So the guide leaves out tokens, enrolment and credentials: the runner and the client handle them. By the same rule I would drop protocol version checks (the client library does it) and keep only what the agent's own code decides.
  >
  > 💬 👤parf make sure to start with essentials - helps
  >
  > 💬 👾 Yes: the guide opens with the essentials, answering `help` first, then one request and one answer. Everything else follows in order of need.
  >
  > 💬 👤parf optional support: ack and done - when to use
  >
  > 💬 👾 Added as its own item, marked optional: ack when the answer will take another turn, done when a task expects no answer, never done after an answer, nothing for reports, notices or receipts.
  >
  > 💬 👾 The revised plan is ready: [agent guide](agent-guide.md). It has 19 items, essentials first: help, then one request and one answer, then optional ack and done. Then:
  > - Configuration, secrets never echoed, optional roles;
  > - may-send, the body is data;
  > - state, KV, locks;
  > - refusals, send once, restart-safe work;
  > - bigfile, notice/warning/alert in your own log (not stderr);
  > - graceful stop, checks.
  > It ends with "Before you share it" (the end-user page). Cut by your rule: tokens and enrolment, protocol checks, install, sandbox, repo gates. Pick an option above again.
  >
  > 💬 👤parf (in chat) mention about "status" call
  >
  > 💬 👾 Added to the essentials, next to help: answer the `status` ask (empty body, like help) with the agent's own state, for example what it is doing, its backlog and its last error. The runner answers while the agent is starting, stopping or failed ([agent self-description](../../docs/constitution.md#agent-self-description)).
  >
  > 💬 👤parf (in chat) also mention about our subscribe pattern - when agent can emit events - please use commonly accepted/recognized pattern
  >
  > 💬 👤parf (in chat) mention secrets and config separation
  >
  > 💬 👾 Both added:
  > - An agent that emits events offers subscriptions the standard publish/subscribe way: `sub add <target> [filter]`, `sub list`, `sub delete <id>`, as #logwatch, #discord and #eventlog already do ([shared subscriptions](../../docs/agents/subscriptions.md)). The team checks those verbs against common practice (subscribe and unsubscribe, a filter, delivery at least once, the subscriber's permission checked) and raises any mismatch as a question.
  > - Configuration and Secret are separate, as their own item: Configuration is non-secret settings in the record, which Owners and Maintainers can read; Secret is environment values the agent reads at start. A secret never goes in Configuration, help, answers, status or logs.
  >
  > 💬 👤parf (in chat) mention absolute minimum, suggested and then more...
  >
  > 💬 👾 The guide is in three tiers:
  > - **Absolute minimum** (every agent): answer help and status, then one request and one answer on the return route.
  > - **Suggested** (a good agent): Configuration vs Secret; the event log (notice, warning, alert); state and KV; refusals and timeouts; send once; restart-safe; graceful stop.
  > - **More** (when you need it): ack and done, roles, locks, bigfile, subscriptions for agents that emit events, may-send when forwarding.
  >
  > 💬 👤parf (in chat) status is "suggested"; most of our bundled agents do not support it
  >
  > 💬 👾 Moved: the minimum is help plus one request and one answer; `status` is the first suggested item.

- ❓ Q325 Should source application-role revocation stop an existing event subscription?
  - Context: the [comparison](agent-guide.md#publishsubscribe-comparison) finds shared creator admission checks at add/every forward, but [named eventlog](../../docs/agents/eventlog.md#who-may-use-it) keeps its source application-role grant from creation. T-29 requires current source authority. Removing a source read role while bare admission remains can therefore behave differently; this is not a request to change existing behavior silently.
  - ( ) Recommend current source authority on each forward for new event emitters; keep named eventlog's existing exception explicit until a separately approved migration 🌟
  - ( ) Keep creation-time source role authorization as the general profile and explicitly document exceptions such as T-29 ⭐
  > 💡👾 Prefer current source authority for new emitters so removal of private-data access can stop future copies; preserve the existing named-log contract until an approved migration defines its compatibility and tests.

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
