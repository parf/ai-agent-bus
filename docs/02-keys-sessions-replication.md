# Access Keys, Sessions, Replication, Admin

## Access keys — pluggable source, one wire protocol

| Mode | Source of `access_key` | Expiry | Needs AUTH |
|---|---|---|---|
| **derived** | `HKDF(master_secret, "ak" \| user \| service \| epoch)`, `epoch = floor(now/3600)` | 60 min | yes, once |
| **pairwise** | `HKDF(X25519(my_priv, their_pub), "pairwise" \| sorted(fp_a, fp_b))` | never | no |
| **static** | pre-shared key in both configs | never | no |

- Derived keys are deterministic → all AUTH replicas agree; no shared token
  store. Accept current **and previous** epoch across the boundary. Shrink the
  epoch (e.g. 15 min) if faster revocation is needed — same design.
- Roles/`level` are **not** in the derivation (a role change would break live
  sessions); they travel as metadata. `gen` is not in the derivation either.
- `master_secret` lives only on AUTH replicas. Rotate via a `key_version`
  prefix in the HKDF label; accept both for one epoch.
- **Static is the minimal mode**: token specified manually on both sides —
  client: `ENV AGENT_BUS_USER_TOKEN` (or config); service: local mapping
  file — no AUTH calls ever. Auth itself is never skipped: no token, no
  access. Also the fallback for keyless parties (scripts, webhooks).
- Pairwise is the standalone/personal mode for key-holding parties and a
  break-glass path independent of AUTH.
- A service may accept several modes; the handshake carries `key_mode` +
  identifiers (`user_id`/pubkey fp, `service`, `epoch` or none).
- Ed25519→X25519: libsodium `crypto_sign_ed25519_pk_to_curve25519`,
  Go `filippo.io/edwards25519`.
- Roles and access, like keys, take effect on the next epoch.

## Encrypted sessions

Handshake: client `{user, service, c_nonce}` → server `{s_nonce}` (server does
the one-time AUTH lookup in between if the user is unknown).
`session_key = HKDF(access_key, "sess" | c_nonce | s_nonce)` → AEAD per message
(XChaCha20-Poly1305 / AES-256-GCM), per-message counter in associated data for
replay protection. Never use `access_key` raw as the cipher key.
Integrity is free: a message that decrypts is from a party AUTH (or the local
file) vouched for. Transport: anything direct — TCP, WebSocket, unix socket.
No TLS/PKI. **No forward secrecy** (decided): no ephemeral exchange; a leaked
access key exposes recorded sessions. Payload encoding: **JSON**, optional
negotiated **msgpack**.

## Replication — signed generations

```
bundle { gen, prev_gen, created_at, payload, payload_hash }
signature = Ed25519(offline_signing_key, gen | prev_gen | created_at | payload_hash)
```

Enforced by replicas **and** services: valid signature; `gen > current`
(anti-rollback; identical gen+hash = no-op). Gaps (`prev_gen != current`):
**newer generation wins**, gap logged.

- **Signing key is offline** (admin machine, `authctl`), never on servers →
  any replica can be master; failover is a pointer flip; a compromised replica
  can only serve stale-but-valid.
- Workflow: edit → `authctl sign --gen N` → `git push`. **The bundle lives
  in a git repo accessed over SSH**: config-as-code, git history = audit
  trail. Replicas **pull on start** (and on poll); master pushes. **Default
  topology: master/slave.**
- **Reference deployment**: a **local AUTH server** (master) + a **git remote
  as the bundle backup**: a GitHub repo (private *suggested*, not required —
  the bundle holds nothing secret) **or the user's own SSH account on another
  server** (any git-over-SSH remote). Master pushes each signed generation;
  slaves and a rebuilt master pull from it. The remote cannot forge (no
  signing key) and holds no `master_secret`. Lose the box → clone + drop in
  `master_secret` file → AUTH is back. Works offline: the local master keeps
  serving; the remote is the off-site copy, not a dependency.
- **Master/slave, pull**: slaves `GET /bundle?since=<gen>` every 30–60 s (304
  if unchanged), optional push on change. Reads (key issuance, lookups) from
  **any** replica; writes only via master. Master down → reads continue;
  promotion is a manual flag flip. Run **2+ replicas**.
- Every response carries `gen`; services refetch config when they see a newer one.
- **Payload**: principals, pubkeys, groups, ACL/role expressions,
  identity-source config, optional `next_signing_pubkey` for rotation.
  **No admin keys** — those live only in `authorized_keys` on the box.
  Nothing in the bundle is secret: the pubkeys are already public on GitHub,
  which is where they came from.
  **Not** in payload: service definitions and ownership (live in
  `agent-busd`), instance health/stats, queues, encrypted private configs.
- `master_secret`: **out-of-band file** on each replica (not in the bundle).
- **Consistency window**: revoked access honored up to poll interval + one
  epoch. Optional `revoked_users` list in the bundle, checked on every session
  start, for immediate effect on new sessions.

## Admin access — SSH forced commands

Core services run as a dedicated user (`agent-bus`/`auth`): nologin shell,
no sudo, home 0700. Admins SSH in with their own keys; identity is bound to
the key.

- `authorized_keys`: `restrict,command="/…/auth-admin <admin-name>" ssh-ed25519 …`
  (optionally `from=`).
- `sshd_config`: `Match User auth` → `ForceCommand`, `PermitTTY no`,
  `AllowTcpForwarding no`, `AllowAgentForwarding no`, `X11Forwarding no`,
  `PermitUserEnvironment no`, `PasswordAuthentication no`;
  `ExposeAuthInfo yes` to log the key fingerprint.
- `auth-admin`: parses `$SSH_ORIGINAL_COMMAND` against a fixed verb grammar
  (`bundle show|push|history`, `user list`, `service list`, `status`,
  `replica-sync`); reads signed bundles from **stdin** and still verifies
  signature + gen (SSH gates who may talk; signature gates what config is
  real); append-only audit log `ts admin fp verb gen result`.
- Admin pubkeys live **only** in `authorized_keys` (not in the bundle);
  never remove the last admin key; keep one **break-glass key offline**.
- Master→slave sync can itself be SSH with a `replica-sync` forced command.
- Test the lockdown: `ssh auth@host bash`, `-L`, `-A`, `-t` must all fail.
