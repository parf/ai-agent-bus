# R1.1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q5 | Peer sync trusts unsigned records; no clock authority for "newer wins" | owner | [services § registry sync](registry.md#registry-sync) |
| Q4 | `authorized_keys` regeneration would drop the setup-installed token key | owner | [AUTH role § SSH admin](auth.md#ssh-admin) |
| Q6 | How an absent receiver obtains decryption material, and how manual credential changes affect retained keys and queued bodies | owner, with R1 | [access § key modes](access.md#key-modes) |
| Q16 | How a per-service token argument is told apart from asking for a name you own | owner, with R1 | [access § token scope](access.md#token-scope) |
| Q136 | Final R1.1 scope | owner | [R1.1 scope](README.md#scope) |
| Q35 | How authorization caches observe policy changes and explicit revocations, including disconnected peers and live sessions | owner, with R1 | [AUTH consistency](auth.md#consistency-window) |

## Registry context

❓ **Peer sync trusts unsigned records and has no clock authority** — a peer can
push an unsigned record for any name, and "newer wins" compares clocks that are
not synchronised. *Settled by:* owner.

## Access context

❓ **How scoping meets asking for a name you own.** Today the argument names a
*principal*, which is what lets the daemon's owner get a credential for any
name and a runner collect one for a service it started. Once it names a *service*, those two readings of one
argument have to be told apart. *Settled by:* owner, with R1.

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
backlog outcome. *Settled by:* owner, with R1.

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
*Settled by:* owner, with R1.

Q72: the owner has proposed a single front door — one port serving both web and
API, a public homepage describing the service with repository and API links,
sign-in moved to `/admin/`, and the administrative dashboard run as an
on-demand Bun service on a socket rather than an always-running Go child.
Raised 2026-09-18 for discussion. Decide whether to take it, and in what order
against the in-flight dashboard work; the questions it must answer first are in
[one front door](discovery.md#one-front-door). It is four proposals, and they
need not all be accepted.
*Settled by:* owner.
