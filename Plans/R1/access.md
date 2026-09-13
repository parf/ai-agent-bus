# Scoped credentials and encryption

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Token scope

Target: one token per principal per service, so a credential presented to one
service cannot be replayed at another. The proposed SSH request is
`ssh agent-busd@<node> token <service>`; the authenticated key identifies the
caller. This is not the current helper's entitlement grammar.

Current credential issue and scope remain in [access](../../docs/02-access.md#token-scope).
Disambiguating the proposed service argument from asking for a name the caller
owns is an [open question](QUESTIONS.md#access-context).

## Key modes

Three sources, one wire protocol. Static is the minimal mode; the other two
exist for principals that hold a key.

| Mode | `access_key` | Expiry | AUTH role | Identity |
|---|---|---|---|---|
| **static** | the token above, or pre-shared in both configs | by policy; local default never — see [token lifetime](../../docs/02-access.md#token-lifetime) | not needed | the token |
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
  ([target payload](#payload)). ⚠️ Not in the static-token
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
- Payload encoding is the envelope's ([target payload](#payload)).

The [MVP trust boundary](../../docs/02-access.md#encrypted-sessions) remains unchanged
until this design is implemented. The dashboard continues to use the
[body-free feed](../../docs/05-discovery.md#dashboard); that restriction is not
proof of encryption or a claim that the current daemon cannot read bodies.

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


Unresolved details: [questions](QUESTIONS.md#open-questions).

## Enrolment policy

Proposed admission modes: open enrolment with a minimal grant, or closed enrolment
with an approval queue. This is not the current key-proof implementation.

## Streaming

Long answers may stream in R1. The framing and client contract wait for the
owner-approved protocol work; this is a scope candidate, not a new wire format.

## Payload

The earlier target permits JSON with optional msgpack encoding and encrypted
body bytes inside routing metadata. The owner must approve a protocol description
before implementation. Current JSON fields remain defined in
[protocol source](../../src/internal/protocol/envelope.go).
