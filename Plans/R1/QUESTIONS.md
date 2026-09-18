# R1 questions

## Open questions

Only unresolved choices. IDs retain their migration identity; missing numbers belong to another plan.

| ID | Question | Settled by | Context |
|---|---|---|---|
| Q4 | `authorized_keys` regeneration would drop the setup-installed token key | owner | [AUTH role § SSH admin](auth.md#ssh-admin) |
| Q5 | Peer sync trusts unsigned records; no clock authority for "newer wins" | owner | [services § registry sync](registry.md#registry-sync) |
| Q6 | How an absent receiver obtains decryption material, and how manual credential changes affect retained keys and queued bodies | owner, with R1 | [access § key modes](access.md#key-modes) |
| Q12 | Whether start-on-demand is built beside the wrapped call, and what idle stops a service that was started that way | owner, in R1 | [runner § on demand](runner.md#on-demand) |
| Q13 | Whether one kept child may have several messages in flight | owner, when a service asks | [runner § long-lived services](runner.md#long-lived-services) |
| Q15 | Who vouches for a runner's name on a host that runs no daemon | owner, with the runner | [runner § where it runs](runner.md#where-it-runs) |
| Q16 | How a per-service token argument is told apart from asking for a name you own | owner, with R1 | [access § token scope](access.md#token-scope) |
| Q17 | How `protocol` is specified for five client languages | owner, with data models | [future clients](modules.md#modules) |
| Q18 | Final R1 scope | owner | [R1 scope](README.md#scope) |
| Q19 | What happens to a running service when its configuration changes | owner | [services § configuring a template](../../docs/03-services-and-topics.md#configuring-a-template) |
| Q20 | A chaining namespace and a service template both want the `/` | owner, with chaining | [overview § chaining](federation.md#chaining) |
| Q33 | Whether backup is a runner verb, a bundled service, or neither | owner | [context](runner.md#backing-it-up) |
| Q35 | How authorization caches observe policy changes and explicit revocations, including disconnected peers and live sessions | owner, with R1 | [AUTH consistency](auth.md#consistency-window) |

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

## Runner context

❓ **Several messages in flight inside one child.** One at a time needs no
correlation; letting a child work on several would, and the envelope already
carries what that costs — a reply matches on topic and tag
([messaging § request and reply](../../docs/04-messaging.md#request-and-reply)), so the
child would echo the tag and replies could come back in any order. Left open
because nothing needs it yet and adding it later breaks nothing.
*Settled by:* owner, when a service asks for it.

❓ **A second kind of on demand** ([runner § on demand](runner.md#on-demand)).
The wrapped call is settled: no consumer, and the daemon hands the call to the
record's fallback channel for the runner to execute. Open is whether the other
kind is built beside it — the call starting the **service**, which then reads
its own inbox the ordinary way — and, if so, what idle stops it again. They
answer different questions, how rare against how expensive to start, so the
second is not a replacement for the first. *Settled by:* owner, in R1.

❓ **What a backup is driven by** — a runner verb, a bundled service, or
neither. It is one encrypted archive either way, which is why the shape is
settled here and the trigger is not. *Settled by:* owner, with R1.

❓ **Who vouches for `runner@<edge>` when that host runs no daemon.** A
`user@host` realm is vouched for by that host's `agent-busd`
([identity § names](../../docs/01-identity-and-roles.md#names)), and an edge box has none — so the
name it registers under is the one case the realm rule does not already
answer. *Settled by:* owner, with the runner.

## Modules context

❓ **How `protocol` is specified for five languages** — a document, a shared
schema, or a generator? Nothing can be reimplemented consistently until this is
answered, and it is the gate on the client libraries. *Settled by:* owner, when
data models are taken up.

## Registry context

❓ **Peer sync trusts unsigned records and has no clock authority** — a peer can
push an unsigned record for any name, and "newer wins" compares clocks that are
not synchronised. *Settled by:* owner.

## Federation context

❓ **A namespace and a service template both want the `/`.** A name holds at
most one, and it already means *template* / *instance*
([identity § names](../../docs/01-identity-and-roles.md#names)), so `team/ci@realm` parses as
template `team`. Either a chaining namespace *is* the template part, or
chaining needs a separator of its own. *Settled by:* the owner, when chaining
is designed.
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
