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

None open: Q191 is [settled](DECISIONS.md#recorded-decisions); [RT23](TODO.md#agent-capabilities) follows the [capabilities plan](agent-capabilities.md#proposal).

## Runner extensions

- M.7: choose the named-host filtering mechanism and approve its format before implementation ([proposal](runner.md#named-hosts)).
- RT19: decide whether the root runner replaces the standalone starter or sits beside it, the sandbox under each account, and how one person’s agents stay isolated from another’s ([proposal](runner.md#root-runner)).

## Filescan

Open choices for [filescan](filescan.md#questions); each carries a recommendation (💡👾).

| ID | Question | Settled by |
|---|---|---|
| Q305 | How a subscriber is fed to RAG, and what a rename is: `sub add … as rag <namespace> <table>` sends rag's own upsert and delete (no bridge agent), and a rename is a delete of the old id plus an upsert of the new. 💡👾 Recommend exactly that: rag has no rename and no other agent would translate. | owner |
| Q306 | A file over the inline limit (32 KiB, rag's text cap). Skip it and say so in `status`, or send it by bigfile reference. 💡👾 Recommend skip-and-report for `as rag` (rag cannot take a reference), and bigfile by hash only for a generic subscriber that opts in; separately ask whether rag should accept a bigfile reference, as its own item, if big notes matter. | owner |
| Q307 | A subscriber added later: start from now, or replay the existing files. 💡👾 Recommend from now by default, and `replay:true` capped by `max_per_scan`, because a replay into RAG is paid. | owner |
| Q308 | May the system runner serve a path outside any person's home. 💡👾 Recommend no in this item: the user-account runner only, so a vault is read as its owner; a system path is a later, separate decision. | owner |
| Q309 | Is the read-only bind of one directory (F0) part of this item, as a general runner feature (an agent setting `dir`), or a filescan-only special case. 💡👾 Recommend the general feature, small, with the checks the plan lists (real path, no symlink out, refuse `/`, the home root and the runner state directory). | owner |
| Q310 | The shared subscription rule sends once and never retries; filescan keeps a change pending and sends it again until confirmed. 💡👾 Recommend the difference, with retries on the next tick and an alert after a configured number of failures, because a file index that drops a change is wrong silently. | owner |
