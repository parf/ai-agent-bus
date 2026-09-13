# MVP — agent-bus V2

What a developer must know to work on the MVP correctly. The active plan is
[TODO.md](TODO.md).

**[stages § MVP](../../docs/12-stages.md#mvp) fixes the scope**; the rest of
[docs/](../../docs/00-overview.md) is the design of record and governs *how*
anything in that scope is built. The PoC's knowledge, still true, is
[Plans/PoC/README.md](../PoC/README.md).

## Purpose

**Somebody other than the author can install it and use it safely on a shared
host.** That sentence decides every argument in this stage: if a change does
not move one of *install*, *other people*, *shared*, or *safely*, it belongs
to R1.

**It runs as its own account.** `agent-busd` owns the daemon, homed where a
daemon's state belongs ([setup § the two accounts](../../docs/09-setup.md#the-two-accounts));
nothing runs as the person who installed it and nothing runs as root. A bus
that only works when its author starts it has not met *install*, *shared* or
*safely* — so this is a gate on the stage, not a line item in its last wave.

## What changes shape from the PoC

The PoC's simplifications were deliberate and are now the work. The right-hand
column links the section that owns each; this table states no values of its
own.

| PoC | MVP | Why it cannot stay |
|---|---|---|
| one master token, any name | principals and per-user sockets ([access § local socket](../../docs/02-access.md#local-socket)) | *other people* — two users on one host must not be each other |
| master reaches everything | service ACL, then master ([identity § acl](../../docs/01-identity.md#acl)) | *safely* — and a service may refuse master |
| memory only | a durable store and a queue dump ([setup § storage](../../docs/09-setup.md#storage)) | a restart that loses the backlog is not something to hand someone |
| one process | supervisor and children ([processes § the rule](../../docs/11-processes.md#the-rule)) | *safely*, on a host that is not yours alone |
| run the binary | packaged and set up ([setup § install](../../docs/09-setup.md#install)) | *install* |
| runs as whoever built it | runs as its own account ([setup § the two accounts](../../docs/09-setup.md#the-two-accounts)) | *shared* — a developer's daemon is not an installation |

## Invariants this stage must not break

Carried from the PoC, and now load-bearing rather than convenient. Each is a
claim; the link owns the rule.

- **A configuration is private to its service**, owner included
  ([services § configuring a template](../../docs/03-services-and-topics.md#configuring-a-template)).
- **No token, no serve**, on every route
  ([access § what a call carries](../../docs/02-access.md#what-a-call-carries)).
- **A caller states a record; it never states what the daemon observes**
  ([discovery § what a listing answers](../../docs/05-discovery.md#what-a-listing-answers)).
- **A refusal is never reported as silence** ([messaging § verbs](../../docs/04-messaging.md#verbs)).
- **`consume` is at-most-once** and the daemon keeps no reply state
  ([messaging § reply routing](../../docs/04-messaging.md#reply-routing)).
- **No message reaches two readers**, and competing consumers take turns
  ([messaging § one reader per inbox](../../docs/04-messaging.md#one-reader-per-inbox)).
  What that section additionally restricts is *blocked* reads on an empty
  inbox — one at a time — which is the part the MVP scope pushes on; see
  [TODO § blockers](TODO.md#blockers).
- **Names** ([identity § names](../../docs/01-identity.md#names)).

## How work is accepted here

Unchanged from the PoC, and it earned its place —
[PoC README § mutation first, then belief](../PoC/README.md#mutation-first-then-belief):

- **`src/smoke.sh --slow` green.** It runs `go vet` and `go test -race`
  itself, because a harness that does not run them is blind to everything
  they cover — which happened. The bare `./smoke.sh` is the fast subset for
  the edit-run loop; it skips every check that costs over a second, so it is
  a signal, not a proof, and it says so when it finishes.
- **Every fix is broken again and watched turning a named check red.** Three
  PoC checks passed for reasons unrelated to what they claimed; mutation is
  what found them. Two of this plan's first-draft criteria passed on PoC code
  before a line of MVP work existed, and were rewritten for that reason.
- A reviewer's finding is **reproduced before it is accepted**, and reported
  honestly when it is not a bug.

## The cut this stage wants

[stages § MVP](../../docs/12-stages.md#mvp) says MVP is *proposed* and wants
the owner's cut, and that cut is itself an indexed open decision
([decisions](../../docs/decisions.md)). So [TODO.md](TODO.md) plans the whole
documented scope in dependency order and says at each wave what dropping it
would cost.

The smallest **dependency-closed** set is A–D, **F.1**, **H**, and the slice of
G.1 that B.2 needs — not A–D alone, because a catalog left unfiltered after C
is an ACL leak, and per-user sockets need the capability only a supervisor may
hold. Everything else is a usable bus that nobody else can install.

**The rest of G is the largest single cut available**; E and F.3 are the
cheapest.
