# Access

How a caller gets in. This document owns the credentials, the socket and the
session; who the caller *is* belongs to [identity](01-identity.md).

## Two parameters

**Every call carries exactly two things: `user@realm` and a token.** There is
no third field on the wire and no per-service setup. Authentication is always
on — no token, no access — and the AUTH role is not required for any of it.

**A client that is not on the socket supplies both.** The token comes from the
environment or a file; the **name comes from the same place**, and it is not
decoration — it is the inbox this process owns, the sender a reply comes back
to, and what another principal addresses. A call carries both, and one without
a name is refused.

**The name is checked, not taken.** The daemon binds it to the credential it
arrived with and refuses one that credential does not back — on the socket
from the account at the other end, remotely from the token's principal. So a
name on the wire is not a claim: **you cannot be somebody else**, and every
`from`, every owner and every ACL subject means what it says.

**Remotely, the credential is the token's principal.** A token backs exactly
one name; a request that states a different one is refused as *that token
belongs to somebody else*, which is a different answer from a bad token and
the one that makes the refusal readable.

**Locally it is the account at the other end**, known from which socket the
connection arrived on ([local socket](#local-socket)) — so a name stated
there is checked the same way, and a caller that states nothing is still
somebody.

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
([setup § the programs](09-setup.md#the-programs)). Today it is the
stand-in [`src/static-token`](../src/static-token): it prints the token file
the daemon reads at start and refuses every other request. Setting it up is
one `authorized_keys` line per person, given in that script's header.

## Token scope

**Over SSH you never state who you are.** sshd has authenticated the key and
the forced command behind it names the principal, so a request naming a second
one is refused as *this key may ask for X* — a different answer from a bad
key, and the one that makes the refusal readable. Naming yourself is at best
redundant and at worst an attempt to be somebody else.

What the argument is *for* is the other half, and it changes by stage:

| | What a token is | What that costs |
|---|---|---|
| **MVP** | **one master token per principal** — the same credential whatever you go on to call | a service you call can replay your token against a *different* service and be treated as you. This stage runs with the bus, and everything on its host, trusted ([encrypted sessions](#encrypted-sessions)) |
| **Release 1** | **one token per principal per service**: `ssh agent-busd@<node> token <service>` | a credential reaches exactly one service, so a malicious service holding your token for itself cannot become you anywhere else ([stages § release 1](12-stages.md#release-1)) |

The grammar is the same in both, so nothing a caller does changes when the
stage does: ask for a token naming the service you mean to call, and in the
MVP the answer does not depend on it yet.

❓ **How scoping meets asking for a name you own.** Today the argument names a
*principal*, which is what lets the daemon's owner get a credential for any
name and a runner collect one for a service it started ([who may ask for
whose](#token-scope)). Once it names a *service*, those two readings of one
argument have to be told apart. *Settled by:* owner, with Release 1.

**Who may ask for whose.** The daemon's owner — the principal it was started
for — may get a credential for any name. Anyone else may get one only for a
name they own ([identity § ownership](01-identity.md#ownership)), which is
what lets a runner collect the credential for a script service it started
without letting it collect anybody else's.

⚠️ **In a realm nobody vouches for, claiming a name is still a way to become
it.** Publishing is open to any authenticated principal and ownership is
first-come ([identity § ownership](01-identity.md#ownership)), so registering
an unheld name and then asking for its credential makes you it. A realm with a
directory behind it is closed to this — it can only be enrolled into, and the
record that results is its own owner
([identity § proving possession](01-identity.md#proving-possession)). Realms
without one are as open as the host they are on.

**`token` gets a credential; `register` states a record.** They were one word
and two unrelated jobs — one hands out the thing you authenticate with, the
other says a service exists ([services § service and
template](03-services-and-topics.md#service-and-template)).

## Token lifetime

**Tokens are persisted, and the previous one is kept.**

| Rule | Why |
|---|---|
| A token **survives a restart** — it is saved, not held in memory | a token is the `access_key` a session key derives from ([encrypted sessions](#encrypted-sessions)). Queues survive a restart too ([messaging § durability](04-messaging.md#durability)), so if the token did not, the reloaded backlog would be ciphertext nobody can read |
| The **previous token is kept alongside the current one** | a refresh must not strand messages already queued under the old one. Two are accepted; the one before that is dropped |
| **When a token was issued is kept; when it was last used is not** | the date is written down beside the token, so a credential that has been sitting unrotated for a year still says so after a restart. A *use* is not written down: recording one would put a disk write on the hot path of every authenticated call, and last-used is live state like uptime — true of this run and gone with it |
| **Local default: never expires** | there is nothing to rotate against — the socket's owner is the identity, and the OS already gates it |
| Other sources expire by policy | a remote token lives until refreshed or revoked unless the node sets a shorter life |

The two-token window is the same idea as accepting the current *and previous*
epoch for derived keys (see [key modes](#key-modes)): a credential change must
never break traffic that is already in flight.

**A caller may ask what they hold, and never what anybody else holds.** The
answer is their own name plus the records they own, each with a **fingerprint
standing in for the token** — a page that renders a credential is a page that
leaks one ([discovery § rules it is built to](05-discovery.md#rules-it-is-built-to)).
Owning a name is not holding a credential for it: one has to be asked for, so a
name with none is simply absent rather than shown as empty. The fingerprint is
keyed, so a leaked one cannot be checked against a guessed token.

Re-running any of the three above **retrieves the same token** — it is a read,
not a rotation. Rotation is asked for: `agent-bus-token <user@realm> --rotate`
issues a fresh one and demotes the current to previous. Both then
authenticate; the one before them stops.

## Local socket

On the daemon's own host the two fields are supplied for you.

| | |
|---|---|
| Path | `/run/agent-bus/user-<account>.sock` |
| Owner | `chown <account>` — the local account it belongs to |
| Mode | `chmod 600` — that account and nobody else |
| The directory | `chmod 711` — everyone walks through it to their own socket, nobody reads what else is there |

`agent-busd` creates `/run/agent-bus/` itself, so nothing is written into
anyone else's home or runtime directory. It knows the username and the token
from *which socket a connection arrived on* and hands them to the rest of the
system as if the client had sent them. The account running the daemon gets
one like everybody else — it is a user of the bus too.

**A client on its own socket never states a name**, so it may not know one:
`status` answers with the name the daemon is using for the caller, which is
how a verb that needs its own identity gets it.

**The socket hides the two fields; it does not replace them.** Same mechanism
as remote — which is why one daemon can serve **many users on a host** and know
which is calling on every request, and why service ACLs apply per user with
nothing for anyone to configure.

Chowning a socket to another account needs **`CAP_CHOWN`** and nothing more,
granted declaratively (`AmbientCapabilities=CAP_CHOWN` under systemd), not by
running as root. It is held by the supervisor alone, which then passes the
listening fds down, so no long-running child has it
([processes § why the supervisor holds CAP_CHOWN](11-processes.md#why-the-supervisor-holds-cap_chown)).

## The three doors

Three ways the two fields arrive, one rule. They differ only in *who fills
them in*, never in who you are — the same ACLs and roles apply through any of
them.

| Door | Who supplies the name | Who supplies the token | To set up |
|---|---|---|---|
| stated | you, from config and env | you | get a token once |
| local socket | the socket's account mapping | the daemon | nothing |
| **ssh** | a forced command in `authorized_keys` | the wrapper, as a trusted local asserter | a key in that account |

`ssh agent-busd@host <command>` is how a **remote** daemon is reached: sshd
authenticates the key, the forced command states the principal, and the client
never gets to choose it. It is the **daemon's** door and the only one — the
runner has no ssh access at all, and is reached as a service on the bus
([runner § reaching the runner](08-runner-role.md#reaching-the-runner)).

That is also why **no port is opened to a network and no TLS appears between
bus citizens** — the transport is one the host already runs and already
secures, and the same Ed25519 key that enrolment proves you hold is the one
that admits you ([identity § registration](01-identity.md#registration)).

| Worth getting right | |
|---|---|
| the forced command **parses**, never prepends | the client's words arrive in `SSH_ORIGINAL_COMMAND`; concatenating them onto a command line is a shell with extra steps. It matches against a closed set of verbs or refuses |
| `no-pty`, `no-port-forwarding`, `no-agent-forwarding`, `no-X11-forwarding` | the key admits you to a verb, not to a host |
| many principals, one account | everyone arrives as the same unix user, so the **socket** shortcut cannot tell them apart. An ssh caller takes the stated-parameters path, with the forced command as the source of the name |

The cost, stated rather than left implicit: the wrapper can assert any
principal its `authorized_keys` names. That is the same trust the local socket
already holds, in a second place.

## Key modes

Three sources, one wire protocol. Static is the minimal mode; the other two
exist for principals that hold a key.

| Mode | `access_key` | Expiry | AUTH role | Identity |
|---|---|---|---|---|
| **static** | the token above, or pre-shared in both configs | by policy; local default never — see [token lifetime](#token-lifetime) | not needed | `user@realm` + the token |
| **pairwise** | `HKDF(X25519(my_priv, their_pub), "pairwise" \| sorted(fp_a, fp_b))` | never | not needed | Ed25519 key |
| **derived** | `HKDF(master_secret, "ak" \| user \| service \| epoch)`, `epoch = floor(now/3600)` | 60 min | yes, once per epoch | Ed25519 key |

- **Pairwise** is the standalone mode for key-holding parties, and the path
  that still works with AUTH down.
- **Derived** keys are deterministic → every AUTH replica computes the same
  key, no shared token store. Accept current **and previous** epoch across the
  boundary. Shrink the epoch (e.g. 15 min) for faster revocation — same design.
  Roles and `gen` are **not** in the derivation (a role change must not break
  live sessions); they travel as metadata. `master_secret` rotates via a
  `key_version` prefix in the HKDF label, both accepted for one epoch.
- A service may accept several modes; the handshake carries `key_mode` plus
  identifiers (token id / pubkey fingerprint, `service`, `epoch` or none).
- Ed25519→X25519: libsodium `crypto_sign_ed25519_pk_to_curve25519`, Go
  `filippo.io/edwards25519`.
- Roles and access, like keys, take effect on the next epoch.

## Encrypted sessions

Handshake: client `{principal, service, c_nonce}` → server `{s_nonce}`; the
server does its one-time AUTH lookup in between if the principal is unknown.

`session_key = HKDF(access_key, "sess" | c_nonce | s_nonce)` → AEAD per message
(XChaCha20-Poly1305 or AES-256-GCM), per-message counter in associated data for
replay protection. Never use `access_key` raw as the cipher key.

- **End to end, through the bus** — in the modes where it is true. The session
  is between sender and receiver, not between either of them and `agent-busd`:
  a queued body is ciphertext the daemon stores and forwards
  ([messaging § envelope](04-messaging.md#envelope)). ⚠️ Not in the static-token
  mode, and not in the MVP — see below.
- **Opt-out per service.** A service may turn body encryption **off** in its
  own config (`encryption: off`): messages travel in plaintext and the bus, its
  debug trace and its logs can then show them. For development; the flag is
  visible on the registry record so nobody is surprised.
- Integrity is free: a message that decrypts is from a party AUTH or the local
  mapping file vouched for.
- Transport: anything direct — TCP, WebSocket, unix socket. No TLS, no PKI.
- **No forward secrecy** (decided): no ephemeral exchange; a leaked long-term
  key exposes recorded sessions.
- Payload encoding: JSON; msgpack as an optional negotiated binary form.

**A static session is not end-to-end against the daemon, so the MVP does not
claim it is.** The token *is* the `access_key` and `agent-busd` issued it, so
it can derive the session key and read the body; on the local socket it holds
the token outright. Rather than ship a claim the code contradicts, the MVP
runs with body encryption off and **the bus trusted on its own host**; end to
end waits for pairwise or derived keys ([key modes](#key-modes)) and is a
Release 1 line ([stages § release 1](12-stages.md#release-1)).

What does *not* change: the bus reads envelopes, and its dashboard shows
nothing else ([discovery § dashboard](05-discovery.md#dashboard)). "The bus
cannot read a body" was the claim that had to go; "the bus has no reason to"
is still how it is built.

❓ **A queued body outlives the session that encrypted it.** The handshake
above is live between two endpoints, but an inbox belongs to a name and waits
for a reader that may not exist yet
([messaging § inbox queues](04-messaging.md#inbox-queues)), and a dump reloads
a backlog into a restarted daemon
([messaging § durability](04-messaging.md#durability)). So a stored body needs
a key derivable without the sender present. *Settled by:* owner, with the MVP.

## Key confirmation

The first AEAD message after the handshake is the check: if it fails to
decrypt, the key is wrong. Then:

1. **Re-query AUTH once** for a fresh key (epoch boundary, rotation,
   revocation) and retry the handshake.
2. **If it still fails — alert, loud**: emit a bus event on the caller's inbox
   and the `alerts` topic, mark the pair on the dashboard, log it. No further
   retries.

In static and pairwise mode step 1 is skipped — there is nothing to re-query,
so it goes straight to the alert.
