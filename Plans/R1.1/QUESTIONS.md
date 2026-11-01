# R1.1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q268 | The Go MCP face's protocol: hand-write the stdio JSON-RPC it needs (initialize, tools, resources, the Claude channel notification), or take a Go MCP SDK that meets the revision the face speaks today; decided at the start of WIN.3 by what the SDK covers (💡 recommend: hand-write the small part, as `mcp/rpc.ts` already does for transport) | owner, at WIN.3 | [windows CLI § what it costs](windows-cli.md#what-it-costs) |
| Q178 | Who adds the `mcp` tag, and when: the runner asks each agent it serves `--mcp` at start and reload; the daemon asks at registration; or the agent states it and nobody checks. 💡👾 Recommend the runner, the way it adds `runner`, with an agent's own registration allowed to state it | owner | [MCP agents](mcp-agents.md#mcp-agents) |
| Q5 | Peer sync trusts unsigned records; no clock authority for "newer wins" | owner | [registry § registry sync](registry.md#registry-sync) |
| Q4 | `authorized_keys` regeneration would drop the setup-installed token key | owner | [AUTH role § SSH admin](auth.md#ssh-admin) |
| Q6 | How an absent receiver obtains decryption material, and how manual credential changes affect retained keys and queued bodies | owner, with R1.1 | [access § key modes](access.md#key-modes) |
| Q16 | How a per-destination token argument is told apart from asking for a name you own | owner, with R1.1 | [access § token scope](access.md#token-scope) |
| Q136 | Final R1.1 scope | owner | [R1.1 scope](README.md#scope) |
| Q35 | How authorization caches observe policy changes and explicit revocations, including disconnected peers and live sessions | owner, with R1.1 | [AUTH consistency](auth.md#consistency-window) |
| Q72 | Whether to take the single front door — one port for web and API, a public homepage, sign-in at `/admin/`, an on-demand dashboard — and in what order | owner | [front door](#front-door) |

## Messaging agents

- ❓ Q244 Discord: who creates and owns the application and bot, for which server
  - Context: Needed before any live test; no sign-up was made. Moved from R1 with the Discord agent ([plan](discord.md#open-for-the-owner))
  - Proposal: The owner names the account and the server
  - Impact: none of the Discord agent can be tried live before this
  - Settled by: the owner

## Registry context

❓ **Peer sync trusts unsigned records and has no clock authority** — a peer can
push an unsigned record for any name, and "newer wins" compares clocks that are
not synchronised. *Settled by:* owner.

## Access context

❓ **How scoping meets asking for a name you own.** Today the argument names a
*principal*, which is what lets the daemon's owner get a credential for any
name and a runner collect one for an agent it started. Once it names a *destination*, those two readings of one
argument have to be told apart. *Settled by:* owner, with R1.1.

❓ **A queued body outlives the session that encrypted it.** The proposed handshake is live between two endpoints, but an inbox belongs to a name and waits
for a reader that may not exist yet
([messaging § inbox queues](../../docs/04-messaging.md#inbox-queues)), and a dump reloads
a backlog into a restarted daemon
([messaging § durability](../../docs/04-messaging.md#durability)). So a stored body needs
a key recoverable without the sender present. Under the settled
[credential lifetime policy](../../docs/02-access.md#token-lifetime), clock-driven
replacement is excluded. The remaining design must cover deterministic
derivation, identifying the needed material and retaining or recovering it
after a manual change. It must also distinguish refusal of new authentication
from the ability to decrypt old work: a manual revocation needs an explicit
backlog outcome. *Settled by:* owner, with R1.1.

## Auth context

❓ **`authorized_keys` is regenerated from the bundle each generation**, which
would drop the key `agent-bus-setup` installed for issuing tokens
([access § getting a token](../../docs/02-access.md#getting-a-token)) the moment AUTH is
switched on. *Settled by:* owner.

## Authorization refresh

Q35: removing the token epoch also removes the earlier bound on stale AUTH
answers. Decide when cached permissions are refreshed, how an explicit
revocation reaches replicas and live sessions, and what a disconnected caller
may do. This is authorization freshness, not a reopened token-expiry decision.
*Settled by:* owner, with R1.1.

## Front door

Q72: the owner has proposed a single front door — one port serving both web and
API, a public homepage describing agent-bus with repository and API links,
sign-in moved to `/admin/`, and the administrative dashboard run as an
on-demand Bun process on a socket rather than an always-running Go child.
Raised 2026-09-18 for discussion. Decide whether to take it, and in what order
against the in-flight dashboard work; the questions it must answer first are in
[one front door](discovery.md#one-front-door). It is four proposals, and they
need not all be accepted. Since 0.8.50 the dashboard is a Bun process under its
own always-running unit, and since 0.8.62 its port serves TLS beside plain HTTP.
*Settled by:* owner.

## Runner hardening

Policy choices for the [M.14 hardening proposal](runner-hardening.md#runner-hardening-proposal), separate from [current isolation](../../docs/runner/security.md#isolation-today). All await the owner; no mechanism or implementation is selected. Q279 and Q280 are **added from T-18 review, owner to confirm**, not part of the owner's original three boundaries.

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

- ❓ Q191 How an agent says which asks it answers, so callers stop asking agents that cannot answer
  - Context: the web and doctor send `help-json`, `help-config`, `help-secret` and `mcp` to every agent; a plain agent or a live AI session cannot answer some of them and the caller waits out its deadline (owner, 2026-10-26: "decide how to pass what an agent can do, so we do not ask dumb agents what they cannot do"). [RT23](TODO.md#agent-capabilities)
  - Minimum (owner): every agent MUST answer `help`
  - Earlier options: the `help` answer carries one machine-readable capabilities line, for example `Asks: help help-json help-config help-secret mcp`, and callers read it before asking further (owner's "read capabilities from --help", recommended, no new ask or record field); or a separate `capabilities` ask returning the list as JSON; or the list stored on the record at registration so callers need no round trip
  - Owner, 2026-10-26: the capabilities line in `help` must be both machine-readable and human-readable. Sketch to refine: one line of its own, a fixed prefix and comma-separated ask names, each with a short gloss in parentheses, for example `Also answers: help-json (methods as JSON), help-config (settings), help-secret (environment names), mcp (MCP tools)`; a reader takes the words before each `(`. Still to settle: the exact prefix, whether `help` itself is listed, how an unknown future ask is ignored, and where the line sits in the help text
  - Owner, 2026-10-26: add an optional `status` the agent may declare in that line; the web shows it and may offer actions based on it. Settled by the owner: `status` is its own ask, like `help`; its answer is a state word followed by short text (`degraded: Slack socket reconnecting`)
  - Industry practice: two separate axes, lifecycle and health. systemd ActiveState has `activating`, `active`, `deactivating`, `inactive` and `failed`; Kubernetes splits startup, liveness and readiness probes from pod phases (`Pending`, `Running`, `Failed`, `Unknown`); health checks use `pass`/`warn`/`fail` (IETF health-check draft), `UP`/`DOWN`/`OUT_OF_SERVICE`/`UNKNOWN` (Spring Boot) or `OK`/`WARNING`/`CRITICAL`/`UNKNOWN` (Nagios); gRPC health has `SERVING`, `NOT_SERVING` and `UNKNOWN`
  - Settled by the owner, 2026-10-26: the answer format is exactly `word: text`
  - Settled by the owner, 2026-10-26: the six words `starting`, `stopping`, `ok`, `degraded`, `failed`, `unknown`; the web asks `status` only while someone views that agent's page, no background polling
  - Settled by the owner, 2026-10-26: expose both to bus MCP users: `status` joins the `ask` values of `ab_send` (next to help, help-json, help-config, help-secret, mcp), and the MCP instructions say to read the help capabilities line before other asks
  - Earlier proposal: one word from six, joining both axes as systemd does: lifecycle `starting` and `stopping` (the owner's "initializing / finishing"; `starting`/`stopping` is the commoner spelling), and health `ok`, `degraded` (Nagios WARNING), `failed` (CRITICAL) and `unknown`. Lowercase, exactly one first word, `:` then the text (`starting: loading 3 subscriptions`); any other first word reads as `unknown`. The web shows `starting` and `stopping` as neutral, `degraded` as a warning and `failed` as an alarm
  - Settled by the owner, 2026-10-26 ([RD116](../R1.0-Release/DECISIONS.md#recorded-decisions)): the runner answers from metadata what it can; any other ask, `status` included, reaches the script as `AGENT_BUS_ASK=<ask>` with empty input. A script that knows nothing of asks runs with `--no-asks` ([RD117](../R1.0-Release/DECISIONS.md#recorded-decisions)). The runner answers `status` itself when it can ([RD118](../R1.0-Release/DECISIONS.md#recorded-decisions))
  - Impact: the help line keeps the daemon out of describing agents (constitution § agent self-description) and needs no new wire field, at the cost of one `help` round trip before any further ask; a record field saves that trip but makes the daemon carry agent self-description
  - Still open: which actions each state may drive, and whether `ab_ls` shows a status
  - [ ] The web offers restart on `failed` only 🌟
  - [ ] `ab_ls` shows each agent's status (asks every listed agent)
  > 💡👾 Restart on `failed` only, and no status in `ab_ls`: listing would ask every agent each time.

## Runner extensions

- M.7: choose the named-host filtering mechanism and approve its format before implementation ([proposal](runner.md#named-hosts)).
- RT19: decide whether the root runner replaces the standalone starter or sits beside it, the sandbox under each account, and how one person’s agents stay isolated from another’s ([proposal](runner.md#root-runner)).
