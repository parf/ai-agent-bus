# Access Keys, Sessions, Replication, Admin

## Access keys — pluggable source, one wire protocol

| Mode | Source of `access_key` | Expiry | Needs AUTH |
|---|---|---|---|
| **derived** | `HKDF(master_secret, user \| service \| epoch)` | 60 min | yes, once |
| **pairwise** | X25519 ECDH from the two Ed25519 keys | never | no |
| **static** | pre-shared key in both configs | never | no |

- Derived keys are deterministic → all AUTH replicas agree; no shared state.
  `master_secret` lives only on AUTH replicas.
- Pairwise is the standalone/personal mode and a break-glass path independent
  of AUTH. Static is the fallback for keyless parties (scripts, hooks).
- Roles and access, like keys, take effect on the next epoch.

## Encrypted sessions

Exchange nonces → `session_key = HKDF(access_key, nonces)` → AEAD
(XChaCha20-Poly1305 / AES-GCM) with a counter. Integrity is free: a message
that decrypts is from a party AUTH (or the local file) vouched for.
Transport: anything direct — TCP, WebSocket, unix socket. No TLS/PKI.

## Replication — signed generations

`bundle {gen, prev_gen, created_at, payload, hash}` signed with an
**offline** signing key (`authctl`). `gen` strictly increases; replicas and
services reject rollback or bad signatures.

- Signing key is not on any server → any replica can be master; failover is
  a pointer flip.
- Master/slave, **pull** (30–60 s) with optional push. Writes to master,
  reads anywhere. Master down → reads continue.
- Responses carry `gen`; services refetch when they see a newer one.

## Admin access — SSH forced commands

Core services run as a dedicated user. Admins SSH in with their own keys;
`authorized_keys`: `restrict,command="auth-admin <name>"`. `sshd` forces the
command, no PTY/forwarding. `auth-admin` has a fixed verb set, reads signed
bundles from stdin, still verifies the signature, audit-logs every call.
Admin keys can live in the bundle; keep one break-glass key offline.
