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
| Collection | **Pending.** At daemon start, a credential whose name has neither a record nor a registered user is dropped ([ownerless credentials](#ownerless-credentials)) |


### Ownerless credentials

**Pending, not built.** A credential whose name has neither a record nor a
registered user belongs to nobody, and the daemon drops it **at start**. That
is a sweep at a known moment, not expiry: the rule above still holds, and no
credential is retired for being old or idle.

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
the sweep sees one moment, so such a credential is there until the next restart
and collected at it **if the name is still neither registered nor a registered
user by then**. Register it first and it is kept like any other. What the sweep
does take it cannot give back — a name swept needs a credential again the
ordinary way ([getting a token](#getting-a-token)).

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
