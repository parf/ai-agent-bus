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

Both paths need you to already have access to the machine.

| Path | Command | For |
|---|---|---|
| over SSH | `export AGENT_BUS_TOKEN=$(ssh agent-bus@<node> static-token)` | anyone with SSH to the node; sshd authenticates you with the key you already have, behind a forced command |
| on the box | `sudo -u agent-bus agent-bus token <user@realm>` | server access, no SSH key on the bus |

The forced command is [`src/static-token`](../src/static-token): it prints the
token file the daemon reads at start and refuses every other request. Setting
it up is one `authorized_keys` line per person, given in that script's header.

**Who may ask for whose.** The daemon's owner — the principal it was started
for — may get a credential for any name. Anyone else may get one only for a
name they own ([identity § ownership](01-identity.md#ownership)), which is
what lets a runner collect the credential for a script service it started
without letting it collect anybody else's.

⚠️ **Claiming a name nobody holds is a way to become it.** Publishing is open
to any authenticated principal and ownership is first-come
([identity § ownership](01-identity.md#ownership)), so until enrolment says
which names are people's, registering an unheld name and then asking for its
credential makes you it. Enrolment
([identity § registration](01-identity.md#registration)) is what closes this.

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
| **Local default: never expires** | there is nothing to rotate against — the socket's owner is the identity, and the OS already gates it |
| Other sources expire by policy | a remote token lives until refreshed or revoked unless the node sets a shorter life |

The two-token window is the same idea as accepting the current *and previous*
epoch for derived keys (see [key modes](#key-modes)): a credential change must
never break traffic that is already in flight.

Re-running either command above **retrieves the same token** — it is a read,
not a rotation. Rotation is asked for: `agent-bus token <user@realm> --rotate`
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

## Local and remote

| | Local | Remote |
|---|---|---|
| What travels | username + token | username + token |
| Who supplies them | the daemon, from the socket | you, from config and env |
| To set up | nothing | get a token once |
| Principal | `parf@github` | the same `parf@github` |

They differ only in *who fills the fields in*, never in who you are — the same
ACLs and roles apply from either side.

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

- **End to end, through the bus.** The session is between sender and receiver,
  not between either of them and `agent-busd`: a queued body is ciphertext the
  daemon stores and forwards ([messaging § envelope](04-messaging.md#envelope)).
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

❓ **A static session is not end-to-end against the daemon.** The token *is* the
`access_key` and `agent-busd` issued it, so it can derive the session key and
read the body; on the local socket it holds the token outright. True end-to-end
needs pairwise or derived keys. Document the exception, or reserve end-to-end
for those two modes? *Settled by:* owner.

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
