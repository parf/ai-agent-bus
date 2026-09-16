# Access

## Status

| MVP | Scope |
|---|---|
| Built | Principal tokens, key-possession token requests, per-account sockets, rotation and dashboard sessions. |
| Pending | None in this topic. |

## What a call carries

**A call carries one thing: a token.** There is no second field on the wire and
no per-service setup. Authentication is always on — no token, no access — and
the AUTH role is not required for any of it.

**The token is the identity.** It backs exactly one principal and the daemon
reads the caller out of it, so every `from`, every owner and every ACL subject
means what it says: **you cannot be somebody else**, because there is nothing on
the wire to be somebody else *with*.

**A name beside it is not asked for, because it proves nothing.** Whoever holds
the credential is the principal. A name sent alongside can only agree — in which
case it said nothing — or disagree, in which case it is a typo in somebody's
config; and catching that typo is not worth a second parameter in every call,
every client library and every deployment, forever.

**The name is spent once, to get the token**: `user@realm` plus a key, a socket
or an ssh line, in exchange for a credential
([getting a token](#getting-a-token)). From then on the credential carries it,
and over ssh the name is not typed at all ([token scope](#token-scope)).

**Built: holding a credential is not being somebody.** The token says which
name is calling; whether that name is anybody is a second question, and the
daemon asks it on every call. A name it has no profile and no record for is not
a principal it knows, and it is refused everything — not because of a state it
is in, but because there is nobody there. Unknown had been reading as *active*,
since a name with no profile has no state and no state passes for the ordinary
case; that is [settled](decisions.md#settled) and fixed.

**The two refusals are not the same, and must not read alike.** A name the
daemon does not know is told `401`, *who are you*: the same answer a bad token
gets, and [counted as the same reason](05-discovery.md#refusals), because what
was presented does not make the caller anybody. A user that is paused or banned
is known, and is told `403 suspended`, which is a state and says so. Nothing
could be granted to the first; the second is waiting on somebody.

**There is no exception, and nothing to bootstrap.** An unregistered name
cannot register itself either: it never had a credential to try it with, because
[one is not issued to nobody](#getting-a-token). A name that holds a credential
and nothing else is a leftover from an older store rather than a newcomer, and
letting it register its way in would only bring leftovers back. Enrolment is the
path that creates a name and its credential together, and it proves a key to do
it ([proving possession](01-identity.md#proving-possession)).

**Built: the answer is taken where it is used, not at the door.** Asking once
on the way in answers about a moment that has passed: the gate lets go of the
registry before the operation takes it, and in that gap the caller can stop
being a principal, be paused or banned, or lose the authority the operation is
about. So the same two questions are asked again under the hold the operation
writes under, and every predicate that decides — who may see a record, who may
manage one, who may edit a user — asks them there. The gate stays, because
refusing at the door is cheaper and says the same thing, but nothing rests on
it.

**What that gap was worth**, on the three shapes reproduced before it was
closed: a caller whose record was removed while its request was in flight could
register itself back into existence; a caller that was paused after the gate was
served anyway; and asking for a token established who owned the target *before*
the credential was written, so a record changing hands in between handed the
former owner the credential of the owner it now has — not a stale read but a
credential the caller was never entitled to. An operation that writes to two
places closes the same way: removing an address and dropping its credential are
one operation under one hold, and so are deciding who may have a credential and
minting it.

**`AGENT_BUS_NAME` survives and authenticates nothing.** It is what a process is
serving as — for its own use and its children's ([runner § what the child is
told](08-runner-role.md#what-the-child-is-told)) — and the daemon does not read
it. A client that does not know its own name asks: `status` answers with it.

## Getting a token

The first two need you to already have access to the machine; the third
needs only the key.

| Path | Command | For |
|---|---|---|
| over SSH | `export AGENT_BUS_TOKEN=$(ssh agent-busd@<node> token)` | anyone with SSH to the node; sshd authenticates you with the key you already have, **so you do not name yourself** — the `authorized_keys` line already does ([token scope](#token-scope)) |
| on the box | `agent-bus-token <user@realm>` | server access, no SSH key on the bus |
| with a key | `agent-bus-token <user@realm> --key <ed25519>` | **no sshd anywhere** — the daemon sets the challenge and the key answers it |

**Built: a credential is issued to somebody, never to nobody.** Every path
above asks for a name the daemon already holds something for — a profile, or a
record of its own. Minting one for a name it knows nothing about is refused,
whoever asks, the daemon owner included. That operation is how this node came to
hold two hundred and thirty-one credentials answering for nothing: the
credential was their only trace, and on its own it let them call. A name becomes
real first — somebody registers it, or a maintainer creates it as a user — and
then it may hold a credential. **There is no self-service**: an unregistered
name cannot do anything at all, registering itself included
([what a call carries](#what-a-call-carries)).

**Whose credential you may ask for is decided while it is written.** The daemon
owner may ask for any, a name may ask for its own, and the owner of a record may
ask for that record's — and which of those is true is settled inside the same
hold that writes the credential, because a record can change hands. Asked
beforehand, the answer described the moment the caller asked rather than the
moment the credential was made ([what a call carries](#what-a-call-carries)).

**Not every host runs sshd**, so the key path does not go through one: the
daemon hands out a nonce, the holder signs it, and a signature that checks
against a key the realm publishes is worth a token. That is the same challenge
the bus already uses to let somebody into a vouched realm
([identity § proving possession](01-identity.md#proving-possession)) — one
mechanism, two things asked of it.

**One program every way**, `agent-bus-token`, and it is the *only* thing an
ordinary user reaches over SSH — the forced command behind their key, where an
operator's key has the admin program instead
([setup § the programs](09-setup.md#the-programs)). Setting it up is one
`authorized_keys` line per person, written by
[`agent-bus-admin user add`](09-setup.md#the-programs).

## Token scope

**Built.** A token authenticates one principal to the daemon. It is not scoped
to a target service. Over SSH, the forced command fixes which principal a key
may request; the caller cannot replace that entitlement.

For bus-routed messages, the daemon checks the caller's token and delivers the
verified sender identity with the message; it does not forward that token to
the receiving service. The service reads and sends using its own credential
([delivery source](../src/internal/api/server.go)). Possession of a user's
token would allow impersonation within that user's permissions, but receiving
a message does not grant that possession.

The daemon owner may request a credential for any name. Other callers may
request their own credential or one for a name they own. Registration in an
unvouched realm is first-come; claiming a name there and obtaining its token
makes the caller that principal. Directory-backed realms require enrolment.

## Token lifetime

**All versions: principal tokens never expire or rotate automatically.**
Invalidation or rotation requires an explicit manual action; elapsed time,
inactivity and daemon restart do not retire a token.

The reason is queued-message recovery: when a token supplies message-encryption
key material, losing that material can make unprocessed ciphertext unreadable.
Manual invalidation also needs an explicit decision about the remaining backlog;
it does not make old ciphertext decryptable with the replacement token. The
[future key lifecycle](../Plans/R1/access.md#key-modes) must account for this.
The MVP currently carries plaintext bodies ([trust boundary](#encrypted-sessions)).

| Built behavior | Meaning |
|---|---|
| Persistence | Tokens survive daemon restarts behind the store port |
| Rotation | Current and previous tokens authenticate; the one before them does not |
| Retrieval | Asking again returns the current token; `agent-bus-token <name> --rotate` asks for a new one |
| Issued time | Durable alongside the token |
| Last use | In-memory state for this run; no disk write per authenticated call |
| Listing credentials | A caller sees only credentials it holds, represented by keyed fingerprints rather than token bytes |
| Removal | Unregistering an address takes its credential with it ([identity § unregistering](01-identity.md#unregistering)); a person's own credential is not a record's to drop, so it stays |
| Lifetime | A person's credential lasts **as long as that person is registered**. Nothing else ends it: no clock, no inactivity, no restart. MVP has no way to deregister a person at all — only [pause and ban](01-identity.md#user-lifecycle), which keep the credential — so today the bound exists and is never reached |
| Collection | **Pending.** At daemon start, a credential whose name answers to nobody — no record, no registered user, and owning no records — is dropped ([ownerless credentials](#ownerless-credentials)) |


### Ownerless credentials

**Built.** A credential belongs to nobody when its name
has **no record of its own and no registered user**, and the daemon drops it
**at start**. That is a sweep at a known moment, not expiry: the rule above still
holds, and no credential is retired for being old or idle.

**Owning records does not save one, because those records do not survive the
same start.** A name with neither a profile nor a record of its own leaves
every record it owns [answering for nobody, and they are
deleted](01-identity.md#when-the-owner-is-gone) first, in the same start — so by the time this sweep asks, there is nothing left for
the credential to answer for, and the two say the same thing about one name.

**That order is the whole of it.** Sweeping a credential whose records were
still there would manufacture exactly the orphans the other rule is for; running
the deletion first means it cannot. There is no third condition and no guard:
the two sweeps are one decision made twice, about the record and then about the
credential.

It is the other half of *as long as the user is registered*. A person keeps
theirs while they are a registered user, whatever they register or unregister
underneath it; a name that is nobody's keeps nothing.

**So what it collects is names, never people.** MVP cannot deregister a person,
and pausing or banning one deliberately keeps their credential
([user lifecycle](01-identity.md#user-lifecycle)) — a banned person who lost
their credential could not be unbanned into anything. Every entry this sweep
can reach is therefore a name with no record and no person behind it: what a
credential outliving its address used to leave.

The daemon owner's credential is minted by the credential store itself rather
than by a record, but **it is not ownerless**: starting the daemon writes the
owner a user profile, so it is a registered user like any other and survives by
the ordinary rule rather than by an exemption. That is worth a check of its own
anyway — a sweep that looked only for a record, and not for a registered user,
would take the owner's own credential on the first start.

A credential asked for before its name is registered is the same shape, briefly:
the sweep sees one moment, so such a credential lasts until the next start — or
until somebody takes it by hand, below — and goes then **only if the name still
answers to nobody by both tests**.
Register it first and it is kept like any other. What the sweep does take it
cannot give back — a name swept needs a credential again the ordinary way
([getting a token](#getting-a-token)).

**Built: it can also be done by hand, by the daemon owner or a maintainer.**
The automatic sweep stays and runs at every start; this is the same rule
applied through an explicit removal action, so a name that turns up between restarts does not
have to wait for one. Maintainers are included because they already administer
users and this is the user directory — an ownerless credential is nobody's, so
the [order](01-identity.md#groups-and-maintainers) does not settle it on its
own and this was chosen.

Eligibility is checked again at the moment of removal, not taken from the list
that was rendered: a record or a profile created in between makes the name
somebody's, and the answer then is a refusal rather than a deletion.

So **support for this category is permanent; its occupancy is not.** Anything
listing it must read as well empty as populated, and against what the caller may
actually see, which is not the whole store
([dashboard](05-discovery.md#dashboard)).

Removal already takes a credential with its address, on unregistering and on
deleting a service ([unregistering](01-identity.md#unregistering)). The sweep is
for what the *old* rule left: a credential was once kept on purpose when its
address went, to hold the name against a stranger, and that is what filled a
person's list with names nothing answers on.

Browser session credentials have their own lifetime and are not persisted;
see [discovery § signing in](05-discovery.md#signing-in).

## Local socket

On the daemon's own host there is nothing to supply at all.

The CLI resolves its connection in this order: global `--addr` before the
command, `AGENT_BUS_ADDR`, then local socket discovery. Discovery checks the
login runtime directory before the installed directory below; without a token
it selects the account's socket, and with a token it selects the shared socket.
An explicit address that fails is reported, never replaced by another bus.
The daemon's default bind path is independent of client discovery.
The token helper shares that discovery rule when `AGENT_BUS_ADDR` is unset.

| | |
|---|---|
| Path | `/run/agent-bus/user-<account>.sock` |
| Owner | `chown <account>` — the local account it belongs to |
| Mode | `chmod 600` — that account and nobody else |
| The directory | `chmod 711` — everyone walks through it to their own socket, nobody reads what else is there |

`agent-busd` creates `/run/agent-bus/` itself, so nothing is written into
anyone else's home or runtime directory. It knows the caller
from *which socket a connection arrived on* and hands that name to the rest of
the system as if a token had carried it. The account running the daemon gets a
socket like everybody else — it is a user of the bus too.

**A client on its own socket never states a name**, so it may not know one:
`status` answers with it.

The sibling `bus.sock` is the shared, token-authenticated listener, with mode
`666` so sessions under other local accounts can reach it. It never supplies
an identity: missing or invalid tokens are refused. A launcher uses its private
user socket to obtain a session token, then uses the shared socket as that session.

**The socket is a credential, not an exemption from having one.** It says who
is calling exactly as a token does — which is why one daemon can serve **many
users on a host** and know which is calling on every request, and why service
ACLs apply per user with nothing for anyone to configure.

Chowning a socket to another account needs **`CAP_CHOWN`** and nothing more,
granted declaratively (`AmbientCapabilities=CAP_CHOWN` under systemd), not by
running as root. It is held by the supervisor alone, which then passes the
listening fds down, so no long-running child has it
([processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown)).

## The three doors

| Door | What authenticates |
|---|---|
| Token | The credential on the API request |
| Local socket | The principal mapped to that account's listener |
| SSH token or admin command | The key's forced-command entitlement |

SSH exposes the [installed command grammar](09-setup.md#ssh-admin), not an
arbitrary remote bus-command proxy. A remote client can use an SSH tunnel to
the loopback listener. The current daemon refuses public-interface binds.

## Encrypted sessions

**MVP trust boundary: plaintext bodies, trusted host.** No peer handshake,
session-key derivation or message encryption is built. The daemon can read
bodies and its stored configuration; removing bodies from a dashboard feed
is a separate, enforced disclosure boundary.

The future design lives in [R1 § encryption](../Plans/R1/access.md#encrypted-sessions).
