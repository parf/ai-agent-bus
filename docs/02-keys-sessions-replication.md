# Access Keys, Sessions, Replication, Admin

## Access keys — three sources, one wire protocol

| Mode | `access_key` | Expiry | AUTH role | Identity |
|---|---|---|---|---|
| **static** | pre-shared token in both configs | never | not needed | the token itself |
| **pairwise** | `HKDF(X25519(my_priv, their_pub), "pairwise" \| sorted(fp_a, fp_b))` | never | not needed | Ed25519 key |
| **derived** | `HKDF(master_secret, "ak" \| user \| service \| epoch)`, `epoch = floor(now/3600)` | 60 min | yes, once per epoch | Ed25519 key |

- **Static is the minimal mode.** Client: `ENV AGENT_BUS_USER_TOKEN` (or
  config); service: local mapping file. No key exists for a static principal;
  no AUTH calls ever. Auth itself is never skipped: no token, no access. Also
  the fallback for keyless parties (scripts, webhooks).
- **Where the token comes from: your SSH key.**
  `export AGENT_BUS_USER_TOKEN=$(ssh agent-bus@localhost static-token)` — the setup
  script put your public key into the `agent-bus` user's `authorized_keys`
  with a forced command (same mechanism as admin access below), so sshd
  authenticates you with the key you already have and the daemon hands back a
  token bound to that principal. No password, nothing to copy by hand; a
  script on another host does the same against `agent-bus@<node>`.
- **Pairwise** is the standalone mode for key-holding parties and a path that
  works with AUTH down.
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
  not between either of them and `agent-busd`: a queued message body is
  ciphertext the daemon stores and forwards without being able to read it.
  The bus sees the envelope only.
- **Opt-out per service.** A service may turn body encryption **off** in its
  own config (`encryption: off`): its messages travel in plaintext and the bus,
  its debug trace and its logs can then show them. Meant for development and
  debugging; the flag is visible on the service's registry record so nobody
  is surprised.
- Integrity is free: a message that decrypts is from a party AUTH or the local
  file vouched for.
- Transport: anything direct — TCP, WebSocket, unix socket. No TLS, no PKI.
- **No forward secrecy** (decided): no ephemeral exchange; a leaked long-term
  key exposes recorded sessions.
- Payload encoding: **JSON**; **msgpack** as an optional negotiated binary form.
- **Key confirmation.** The first AEAD message after the handshake is the
  check: if it fails to decrypt, the key is wrong. Then:
  1. **re-query AUTH once** for a fresh key (epoch boundary, rotation,
     revocation) and retry the handshake;
  2. **if it still fails — alert, loud**: emit a bus event on the caller's
     inbox and the `alerts` topic, mark the pair on the dashboard, log it.
     No further retries. In static/pairwise mode step 1 is skipped: there is
     nothing to re-query, so it goes straight to the alert.

## Replication — signed generations in git

```
bundle { gen, prev_gen, created_at, payload, payload_hash }
signature = Ed25519(offline_signing_key, gen | prev_gen | created_at | payload_hash)
```

**Payload**: principals, pubkeys, groups, ACL/role expressions, **admin keys**
(the SSH pubkeys allowed to administer; `authorized_keys` is regenerated from
them each generation), identity-source config, optional `next_signing_pubkey`
for rotation. Nothing in it is secret — the pubkeys came from GitHub.
**Not in the payload**: service and topic definitions, ownership, instance
health/stats, queues, sealed private configs, `master_secret`.

Enforced by replicas **and** services: valid signature; `gen > current`
(identical gen + hash = no-op). Gaps (`prev_gen != current`): **newer
generation wins**, gap logged.

- **Where it runs.** The AUTH role is a **child process of `agent-busd`**
  (`auth: on`). Only that child holds `master_secret` and verifies/serves the
  bundle; the core reaches it over a unix socket. A node with the role on is an
  AUTH replica; run **2+**.
- **Signing key is offline** (admin machine): `agent-bus auth sign --gen N`.
  Never on a server → any replica can be master, failover is a pointer flip, a
  compromised replica can serve stale-but-valid config (but see `00`
  trade-offs: it does hold `master_secret`).
- **The bundle lives in a git repo over SSH.** Workflow: edit →
  `agent-bus auth sign --gen N` → `git push`. Replicas **pull on start** and on
  poll (30–60 s); master pushes. Git history is the audit trail. Optional fast
  path: `GET /bundle?since=<gen>` from the master, 304 if unchanged.
- **Default topology: master/slave.** Reads (key issuance, lookups) from any
  replica; writes only via master; master down → reads continue; promotion is
  a manual flag flip.
- **Reference deployment.** A local `agent-busd` with `auth: on` as master + a
  **git remote as backup**: a GitHub repo (private suggested, not required) or
  the user's own SSH account on another server. The same repo also receives
  unsigned **registry snapshots** (services, topics) from the core, in a
  separate directory — also how peer nodes sync their registries on start
  (push own, pull others'; newer record wins). Backup and peer sync, never
  authority. The remote cannot
  forge (no signing key) and holds no `master_secret`. Lose the box → clone,
  drop in the `master_secret` file, start. Works offline; the remote is the
  off-site copy, not a dependency.
- `master_secret`: **out-of-band file** on each AUTH replica, never in the
  bundle or the repo.
- Every response carries `gen`; services refetch when they see a newer one.
- **Consistency window**: revoked access honoured up to poll interval + one
  epoch. Optional `revoked_users` list in the bundle, checked on every session
  start, for immediate effect on new sessions.

## Admin access — SSH forced commands

`agent-busd` runs as the dedicated `agent-bus` user: nologin shell, no sudo,
home 0700. Admins SSH in with their own keys; identity is bound to the key; the
forced command talks to the AUTH child.

- `authorized_keys`: `restrict,command="/…/agent-bus auth admin <admin-name>" ssh-ed25519 …`
  (optionally `from=`). Regenerated from the bundle's admin keys each generation.
- `sshd_config`: `Match User agent-bus` → `ForceCommand`, `PermitTTY no`,
  `AllowTcpForwarding no`, `AllowAgentForwarding no`, `X11Forwarding no`,
  `PermitUserEnvironment no`, `PasswordAuthentication no`;
  `ExposeAuthInfo yes` to log the key fingerprint.
- `agent-bus auth admin` parses `$SSH_ORIGINAL_COMMAND` against a fixed verb
  grammar (`bundle show|history`, `user list`, `service list`, `status`,
  `replica-sync`); a bundle arriving on stdin is still verified (signature +
  gen) — SSH gates *who may talk*, the signature gates *what config is real*;
  append-only audit log `ts admin fp verb gen result`.
- **Break-glass is root on the box** — it can always edit `authorized_keys` or
  the bundle pointer by hand. No separate offline admin key.
- Master→slave sync may itself run over SSH with a `replica-sync` forced command.
- Test the lockdown: `ssh agent-bus@host bash`, `-L`, `-A`, `-t` must all fail.
