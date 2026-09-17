# Scoped credentials and encryption

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Token scope

Target: one token per principal per service, so a credential presented to one
service cannot be replayed at another. The proposed SSH request is
`ssh agent-busd@<node> token <service>`; the authenticated key identifies the
caller. This is not the current helper's entitlement grammar.

Current credential issue and scope remain in [access](../../docs/02-access.md#what-a-call-carries).
Disambiguating the proposed service argument from asking for a name the caller
owns is an [open question](QUESTIONS.md#access-context).

## Key modes

The proposed sources follow the shared [token lifetime policy](../../docs/02-access.md#token-lifetime).
The earlier clock-based derivation and scheduled retirement are superseded.

| Mode | Proposed source | AUTH role | Identity |
|---|---|---|---|
| **static** | Issued token or explicitly shared credential | Not needed | Token |
| **pairwise** | Derived from the parties' keys | Not needed | Ed25519 key |
| **derived** | AUTH-derived material scoped to the principal and service | Needed for issuance | Ed25519 key |

- **Pairwise** is the standalone mode for key-holding parties, and the path
  that still works with AUTH down.
- **Derived** keys remain deterministic across AUTH replicas, but the clock
  does not select replacement material. The revised derivation and recovery
  of old material after a manual change remain [Q6](QUESTIONS.md#access-context).
- Roles and policy generations travel as metadata rather than changing the
  encryption key. Policy refresh is governed by the [AUTH consistency
  contract](auth.md#consistency-window), separately from credential lifetime.
- A service may accept several modes; identifying the material needed for a
  queued body is part of the unresolved key lifecycle, not a new wire format here.
- Ed25519→X25519: libsodium `crypto_sign_ed25519_pk_to_curve25519`, Go
  `filippo.io/edwards25519`.
Manual rotation or invalidation must account for outstanding ciphertext.
The MVP's limited credential history is not proof that older encrypted backlog
can be recovered; [Q6](QUESTIONS.md#access-context) must settle retention and
recovery, including what happens to backlog when access is deliberately revoked.

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

The [MVP trust boundary](../../docs/02-access.md#trust-boundary) remains unchanged
until this design is implemented. The dashboard continues to use the
[body-free feed](../../docs/05-discovery.md#dashboard); that restriction is not
proof of encryption or a claim that the current daemon cannot read bodies.

## Key confirmation

The first AEAD message after the handshake is the check: if it fails to
decrypt, the key is wrong. Then:

1. **Re-query AUTH once** to check for an explicit credential change or stale
   lookup, then retry the handshake if access is still granted. This lookup
   does not itself rotate or retire a token.
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
