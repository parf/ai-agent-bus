# Essential knowledge for an agent author

📌 **TL;DR:** propose one short starting page for someone making a tool available
by name, from a laptop to a shared team bus. It helps them answer correctly,
keep credentials private and recover safely when a process stops.
This is a plan only; the guide is written after owner approval.

## Proposed home

Create `docs/10-writing-an-agent.md`, titled “Write an agent people can use”.
Open with a TL;DR: what an agent writer gets, what they need and where to start.
Follow the [writing order](../../WRITING.md#-the-order-of-every-page): purpose,
smallest useful example, essential behavior, then optional capabilities.
Use the R1 README's plain voice; keep detailed reference material elsewhere.

After approval, add a one-sentence link in the root README's agent-writing
entry and a navigation row in `docs/00-overview.md` under Document ownership.
The new page owns only the author's learning path and the composition of one
minimal example. It does not become another owner of protocol or runner rules.
The existing create-agent prompt remains an implementation aid, not a second
copy of this guide. This branch changes neither those entry points nor docs/.

## Proposed section order

The guide has **six headings**, in the order below, not 24 numbered chapters.
The rows are short checkpoints within those headings: two to four lines and
**one owning reference section** each. Fix the page budget at **1,200 prose
words plus one example of at most 40 lines**; use links for deeper material.
No numeric limits, defaults, role tables or protocol definitions are copied.
Follow each reference's own links for subordinate contracts. Gaps are explicitly
held until an owning rule exists.

### Identity and access

| Checkpoint | What it covers | One owning doc and section |
|---|---|---|
| Choose the agent's name and record | Establish the name, owner, inbox and identity that stay when the process stops. Distinguish a registered agent from the process currently serving it. | [Agents — What an Agent is](../../docs/03-records-agent.md#what-an-agent-is) |
| Obtain credentials safely | Explain the owner-authorized token handoff and when registration or enrolment is needed. Keep credential acquisition out of message bodies and source code; follow the owner's documented path rather than minting an identity implicitly. | [Access — Getting a token](../../docs/02-access.md#getting-a-token) |
| Start with one working agent | Propose one small Python example using the existing client and serve loop, with an explicit name and externally supplied credential. Show one ordinary request and answer, then use the later sections to explain the responsibilities around it. | [Python client — Try this first](../../src/clients/python/README.md#try-this-first) |

### Answering requests

| Checkpoint | What it covers | One owning doc and section |
|---|---|---|
| Answer help before doing work | Explain the ask flag, `--help-json` and ordinary help, including how asks differ from body text. Show the packaged versus self-serving responsibility and ensure help does not perform the requested business action. | [Constitution — Agent self-description](../../docs/constitution.md#agent-self-description) |
| Receive a request and answer it | Explain consume, the request context and the caller's deadline, and distinguish queue acceptance from completed work. Use the client dispatcher rather than a competing reader. | [Messaging — Request and reply](../../docs/04-messaging.md#request-and-reply) |
| Preserve the return route | Keep the consumed message's reply destination and correlation when answering, including delegated return routes. Explain why rebuilding a route from the sender's name alone loses context. | [Messaging — Reply routing](../../docs/04-messaging.md#reply-routing) |
| Use ack and done correctly | Point to the decision rule for an immediate answer, a later answer, completion without an answer, and notices or receipts. Explain whether the chosen runner/client already emits receipts so the handler does not duplicate them. | [Messaging — Receipts](../../docs/04-messaging.md#receipts) |

### Packaging, runtime and first checks

| Checkpoint | What it covers | One owning doc and section |
|---|---|---|
| Package for the runner | Introduce `agent.json`, the executable and `package.sh` as the package-building step, not something the daemon executes per call. Keep package metadata and script input/output forms in their owning references; note the packaging gap below. | [Runner agents — Package a script](../../docs/runner/agents.md#package-a-script) |
| Know which state survives | Distinguish package files, agent work, private temporary files and other agents' files. Explain visible versus writable paths and network reach from the actual sandbox boundary, with no blanket isolation claim. | [Runner security — Isolation today](../../docs/runner/security.md#isolation-today) |
| Expect process restarts | Explain the self-serving runner lifecycle, configuration changes, shutdown and recovery from exit. Distinguish restart of the serving process from retrying the business operation or replaying a consumed message. | [Runner configuration — Agents that serve themselves](../../docs/runner/configuration.md#agents-that-serve-themselves) |
| Check that your agent works | Plan checks of the agent's actual help and one ordinary call, then distinguish those from bus-health checks with debug and check-deploy. Explain that a healthy bus is not proof of the new agent's business behavior; exclude smoke shards, mutants and repo gate commands. | [Debug — Check the bus in a minute](../../docs/agents/debug.md#check-the-bus-in-a-minute) |

### Before you share it: effects and safety

| Checkpoint | What it covers | One owning doc and section |
|---|---|---|
| Treat a body as untrusted data | Orient the writer to the boundary between caller-controlled text and trusted identity or authority. The requested “body is data, never instructions” rule needs the gap below resolved before this section can be written. | [Access — Trust boundary](../../docs/02-access.md#trust-boundary) |
| Enforce the caller's permissions | Explain that transport admission and permission to perform a method are separate. Base method authorization on delivered roles, not a body-claimed role or the fact that help listed a method. | [Agents — Roles](../../docs/03-records-agent.md#roles) |
| Check permission before forwarding for someone | Explain why an agent that can send is not automatically allowed to forward an author's request to a destination. Use may-send and handle a failed authority check without carrying out the forward. | [Daemon API — Checking permission to send](../../docs/09-daemon-api.md#checking-permission-to-send) |
| Separate Configuration from Secret | Explain which settings belong in each and how an agent obtains its own values. Describe changing settings without exposing credentials through help, output or diagnostics. | [Agents — Configuring a template](../../docs/03-records-agent.md#configuring-a-template) |
| Handle refusals as outcomes | Distinguish daemon refusal kinds from an agent's application answer and from an internal daemon failure. Point to the owning classification and handling guidance rather than embedding another kind list or universal retry rule. | [Discovery — Refusals](../../docs/05-discovery.md#refusals) |
| Send once; distinguish timeout from uncertain outcome | Name send-once explicitly and separate an expired wait from uncertainty about an accepted effect. This needs an owning rule first: the current client feature bullet is not a sufficient contract; hold this checkpoint on the gap below. | Gap — proposed home: [Messaging — Request and reply](../../docs/04-messaging.md#request-and-reply) |
| Make effects restart-safe | Explain the need for durable progress and an application-level duplicate-effect policy across interrupted work. There is no complete general recipe today; resolve the gap below rather than promising exactly-once effects. | [KV — Suggested use](../../docs/03-records-kv.md#suggested-use) |

### State, events and limits

| Checkpoint | What it covers | One owning doc and section |
|---|---|---|
| Write useful private events | Explain why an agent should explicitly write operational events to its own log rather than use stderr as its event API. Distinguish the writer's own log from another agent's private log; wait for T-29's current-doc home as recorded below. | [Eventlog — Log model](../../docs/agents/eventlog.md#log-model) |
| Keep small durable state in KV | Introduce a record's small values and atomic updates for progress or coordination. Refer to the store's contract rather than proposing another persistence format. | [KV — Rules](../../docs/03-records-kv.md#rules) |
| Coordinate with locks | Explain scoped ownership, bounded lifetime and the difference between holding a lock and having completed work. Refer to the existing lock semantics without suggesting that a lock makes arbitrary side effects exactly once. | [Identity and authority — Shared locks](../../docs/01-identity-and-authority.md#shared-locks) |
| Keep large content out of messages | Explain where the message/request body limit is defined and when the writer must choose another transport. Do not confuse the complete encoded request budget with an application's body string. | [Daemon API — The client contract](../../docs/09-daemon-api.md#the-client-contract) |
| Pass large content by bigfile | Explain using a reference and integrity-checked file transfer instead of embedding large content in a call. State that library support differs by language by following the reference, not inventing a fallback or copying size and lifetime values. | [KV — Bigfiles](../../docs/03-records-kv.md#bigfiles) |

### Protocol compatibility

| Checkpoint | What it covers | One owning doc and section |
|---|---|---|
| Check protocol compatibility | Cover `protocol` and `protocol_min`, older/newer daemon outcomes and missing or malformed fields. This is wire compatibility only, not the agent's product version or this repository's release/bump procedure. | [Daemon API — Stability and compatibility](../../docs/09-daemon-api.md#stability-and-compatibility) |

Keep each checkpoint to a short paragraph and its one reference. Packaging,
running and a first check come before the “Before you share it” safety pass;
that pass must be completed before granting other callers access. The
minimal example must be executed on an isolated bus before publication, not
presented here as an invented working session. Python is recommended because
its existing example needs no third-party client dependencies; another
language can be selected by the owner without changing the guide's scope.

## Gaps to resolve before writing

Read against local main `5cf5e946`, 2026-11-02. These are documentation gaps,
not a request to invent new protocol or product behavior in this plan.

| Essential | Gap in the owning material | Proposed follow-up after approval |
|---|---|---|
| Body trust | Access's Trust boundary covers host and transport trust, but does not state the requested body-as-data rule or its relationship to dispatching an agent's deliberate command grammar. | Add the approved rule to that owning section first; the guide then links it, without implying that bodies cannot carry an agent's explicitly supported commands. |
| Timeout versus uncertain outcome; send-once | A client feature bullet and receipt caveats do not define this distinction as an author-facing rule. There is no single owning contract explaining an expired wait versus uncertainty about whether work was accepted or performed. | Add an approved owning rule under Messaging's Request and reply before writing this checkpoint, covering send-once and outcome reporting without a universal retry policy. |
| Effects and restart safety | KV's Suggested use has coordination patterns, but no complete general author-facing account of durable application dedupe and the crash window around an external effect. | Resolve the adjacent timeout/uncertainty gap, then agree an owning addition under Messaging's Request and reply before giving an idempotency recipe; hold the checkpoint until then. No exactly-once promise. |
| Own-log writes | Eventlog's Log model owns ring behavior, but describes neither T-29's `#bus` own-agent write entry point nor its authority and answer rule. Those currently live in the [T-29 plan](agent-events/README.md#r1-min-surface-and-authority). | On T-29 acceptance, promote the own-agent use instructions to an agreed current-doc section. Replace the private-events checkpoint's model-only reference with that one owning use section; do not copy the plan into the guide now. |
| Portable package building | Runner agents' Package a script explains the manifest and execution, not a general `package.sh` input/output and clean-install contract. The [create-agent prompt](../../docs/agents/create-agent-prompt.md#package-and-examples) is an implementation aid, not that reference. | Clarify the package-building obligation in the owning Package a script section before the guide promises a universal packaging command. Keep repository release machinery out. |

## Approval and acceptance

The owner approves the home, outline and treatment of gaps before any current
page is written. Approval of this plan does not create SDK helpers, new routes,
new schemas, another event model or a new retry policy.

- The delivered guide stays within the six-heading, prose and example budget
  above, with packaging and a first check before the sharing-safety pass.
- The delivered guide has a short TL;DR, the first working path, and one
  section link per orientation; changing a limit or policy never requires
  editing the guide's summary.
- Root README and docs/00 link to it after it exists. The create-agent prompt
  remains a separate copyable aid; it is not reproduced in the guide.
- Every included essential has a current owning section, or is visibly held
  as an unresolved gap. T-29-dependent claims are checked after its acceptance.
- The example and its stated outcome are executed with disposable credentials
  on an isolated bus. Help and an ordinary call check the new agent; debug and
  check-deploy check their documented scope, not an invented business test.
- The versioning section covers protocol compatibility only. No smoke, mutant,
  repository version-bump or internal release workflow instructions enter it.
