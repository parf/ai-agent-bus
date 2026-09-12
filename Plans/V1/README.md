# V1 — agent-bus V2

What a developer must know to work on V1 correctly. The active plan will be
[TODO.md](TODO.md); the stage is not started.

**V1 here is the stage after the MVP** — the one
[stages § release 1](../../docs/12-stages.md#release-1) describes. It is not
the NATS JetStream system at `/rd/service/agent-bus/`, which this repo calls
V1 as *legacy* and consults for facts, never for a bar
([CLAUDE.md](../../CLAUDE.md)). Where the two could be confused, the docs say
**Release 1** and mean this one.

**[stages § release 1](../../docs/12-stages.md#release-1) fixes the scope**;
the rest of [docs/](../../docs/00-overview.md) is the design of record and
governs *how* anything in that scope is built. The MVP's knowledge, still
true, is [Plans/MVP/README.md](../MVP/README.md), and what it finished is
[Plans/MVP/DONE.md](../MVP/DONE.md).

## Purpose

**A team or a company can run it, and it can be exposed.** That sentence
decides every argument in this stage the way *install, other people, shared,
safely* decided the MVP's. The MVP put several people on **one host they all
trust**; V1 takes away both halves — more than one host, and a host somebody
outside the team can reach.

Three consequences fall out of that:

- **The bus stops being trusted with bodies.** The MVP struck end-to-end
  encryption because the daemon issues the token a session key would derive
  from, so the claim could not be true ([stages § MVP](../../docs/12-stages.md#mvp)).
  V1 is where the pairwise or derived keys arrive that make it true, and
  until they do the sentence stays off the box
  ([access § encrypted sessions](../../docs/02-access.md#encrypted-sessions)).
- **Starting a service stops being a command you sit in front of.** The MVP
  publishes one with a single foreground command line and keeps no installed
  state. V1 adds `agent-bus-runner` — a second account, outside the daemon,
  that holds each instance's configuration and never hands it back, and starts
  it on boot or on demand ([runner role](../../docs/08-runner-role.md)).
- **Identity stops being one node's.** AUTH, groups and roles exist so that no
  service holds its own user list, and the bundle is offline-signed in git so
  that a compromised replica can serve stale config and never forge it
  ([AUTH role](../../docs/06-auth-role.md)).

## What it inherits

| From | Still true |
|---|---|
| the design | protocol → ports → core, adapters and faces outside; `cmd/` assembles ([modules § the rule](../../docs/10-modules.md#the-rule)) |
| the MVP | one program per privilege ([setup § the programs](../../docs/09-setup.md#the-programs)); the supervisor/bus split, and that **nothing the daemon starts may exec** ([processes § nothing the daemon runs may exec](../../docs/11-processes.md#nothing-the-daemon-runs-may-exec)) |
| the PoC | how work is accepted — nothing is believed until it has been watched failing ([PoC README § mutation first, then belief](../PoC/README.md#mutation-first-then-belief)) |

## How work is accepted here

Unchanged, and it has now caught defects in three stages running:

- **`src/smoke.sh --slow` green.** It runs `go vet` and `go test -race`
  itself. The bare `./smoke.sh` is the fast subset for the edit-run loop and
  is a signal, not a proof.
- **Every fix is broken again and watched turning a named check red.** A
  check that cannot be falsified by any single substitution is falsified by
  hand, and *that* is written down rather than left implied.
- A reviewer's finding is **reproduced before it is accepted**, and reported
  honestly when it turns out not to be a bug.

Two things change with the scope, and they are the ones to get right early:

- **A federated check needs two nodes**, so the harness grows a second daemon
  and a git remote it can push to. A single-node check that claims to prove
  chaining is the hollow shape this stage will produce most easily.
- **A crypto check is about what CANNOT be read**, not about what round-trips.
  An exact plaintext round trip passes with base64; the criterion is that the
  daemon's own key material cannot decrypt a captured body
  ([Plans/MVP/TODO.md](../MVP/TODO.md), wave D, kept there as the shape this
  stage inherits).

## What is deliberately not here

**Billing** is designed and deferred to no stage at all
([future/billing.md](../../docs/future/billing.md)). **LDAP/AD** likewise
([future/ldap-ad.md](../../docs/future/ldap-ad.md)). Neither becomes V1 work
by being written down.
