# Identity and AUTH / Config

## Principals

- Every **user**, **service** and **instance** is a *principal* with an
  Ed25519 key. Ids are namespaced: `github:<numeric id>`, `ldap:<entryUUID>`,
  `svc:<name>`, possibly `static:<name>`. One human may hold several
  principals; a grouping "person" record is **deferred**.
- Authentication is always a signed challenge: single-use nonce (~60 s),
  signature bound to purpose (`ssh-keygen -Y sign -n agent-bus`-style
  namespace) so it cannot be replayed in another context.
  Key directories only supply public keys — they never authenticate.
- **Ed25519 only** (pairwise mode needs Ed25519→X25519). RSA/ECDSA keys are
  filtered at fetch time and the user is told which were skipped.

## Key directories

- **GitHub is the main user id**: `api.github.com/users/<login>` → `id`,
  `login`, `name`, `avatar_url`, `email` (if public); `/users/<login>/keys` →
  per key `id`, `key`, `created_at`, `last_used` (verified 2026-09; the public
  endpoint returns them). Plain-text fallback: `github.com/<login>.keys`.
  Principal id is the **numeric `id`** (logins can be renamed).
  - `created_at` → "new key on privileged principal" alert;
    `last_used` → stale-key pruning (e.g. ignore > 12 months);
    `id` → rotation vs addition. Our own `last_used` is tracked separately.
  - Fetched **only at enrollment or explicit key refresh**, then pinned in the
    signed generation. GitHub is never on the runtime path; no polling, no
    rate-limit concern. If ever polled: token (5 000 req/h) + ETag.
- **LDAP** (OpenSSH-LPK `sshPublicKey`, plus `displayName`, `mail`,
  `jpegPhoto`) is a second directory; stable id = `entryUUID`, not `uid`.
  Normalized to the same principal record.
- Because any developer already has a GitHub key, services can offer
  **self-service enrollment to anyone on the internet**: claim `github:<login>`
  on first contact, one-time fetch, prove possession → principal with a default
  role. Per-service policy: **open** (auto-enroll, minimal role) or **closed**
  (queued for admin approval).

## Users & groups

Groups compose from other groups with `& | !` (`eng & !contractors`).

## Per-service policy

All written as expressions over users and groups, evaluated by one engine:
- **ACL** — *who may access* (`group1 & group2`, `alice | ops`).
- **Roles** — *what they may do*; vocabulary is **service-defined**
  (`admin`, `manager`, `read-only`…). AUTH stores and resolves, never interprets.
  Roles are assigned with the same expression pattern — to users **or groups**.
- Keep ACL and roles as two separate layers (access vs. authority).

## Resolved at login

Services know nothing about groups or mappings (admin-only); on first contact
they ask one question, signed with the service key, and cache the answer for
the epoch:

```
→ who is this user (for me)?
← access: ok | denied · roles: [...] · key: <access-key> · gen: <n>
```

## Local mapping file

Every service may carry its own `principal → {access, roles[, static key]}`
file. Local is consulted first, then AUTH. Populated the same way AUTH does
it: `github:parf` → fetch once → pin.
Modes: file only (standalone) · AUTH only (org) · both (org + local overrides
for owner, break-glass admin, peer services).

## Ownership

Owner is an expression too. Owners edit the service definition, ACL and roles
(using org groups; they cannot create groups or grant beyond their own service).
Two tiers: `owner` (all, incl. ACL/owners) and `maintainer` (definition only).
Personal services are owned by their user. Ownership changes are generation data.
Users create and control their own services without admin; admin's job is
identities + org groups.

## Encrypted private config

An instance may store its private config (e.g. IMAP credentials) in
AUTH/Config, **sealed to the instance's own key** (age-style sealed box).
AUTH holds opaque bytes + owner + instance id; it cannot read them.
Boot = key + binary → config comes back. Versioned, owner-pushed, **not** part
of the signed generation (no signing key needed). Multi-instance sharing =
encrypt to each key or share a key.
Local file remains the default; this is opt-in for portability/recovery.
