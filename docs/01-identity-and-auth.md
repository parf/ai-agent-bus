# Identity and AUTH

## Principals

Every **user**, **service** and **instance** is a *principal* — agents,
consumers and publishers included. Only *generic* records (a description pushed
by someone else) have no identity of their own.

| Identity | When | How it authenticates |
|---|---|---|
| **token** | minimal / static mode | presented in the handshake; **the token is the whole identity**, no key |
| **Ed25519 key** | pairwise mode, or AUTH role on | signed challenge: single-use nonce (~60 s), signature bound to purpose (`ssh-keygen -Y sign -n agent-bus`-style namespace) |

The `agent-bus` CLI creates the key or takes the token, and joins the bus with it.

- Ids are namespaced: `github:<numeric id>`, `ldap:<entryUUID>`, `svc:<name>`,
  `static:<name>`. One human may hold several principals; a grouping "person"
  record is deferred.
- **Ed25519 only** (pairwise needs Ed25519→X25519). RSA/ECDSA keys are filtered
  at fetch time and the user is told which were skipped.
- Key directories supply public keys only — they never authenticate.

## Key directories (AUTH role on)

- **GitHub is the main user id.** `api.github.com/users/<login>` → `id`,
  `login`, `name`, `avatar_url`, `email` (if public); `/users/<login>/keys` →
  per key `id`, `key`, `created_at`, `last_used` (owner verified 2026-09;
  re-check open, see `00`). Plain-text fallback `github.com/<login>.keys`.
  Principal id = the **numeric `id`** (logins can be renamed).
  - `created_at` → "new key on privileged principal" alert · `last_used` →
    stale-key pruning (e.g. ignore > 12 months) · `id` → rotation vs addition.
    Our own `last_used` is tracked separately.
  - Fetched **only at enrolment or explicit refresh**, then pinned in the
    signed generation. Never on the runtime path; no polling. If ever polled:
    token (5 000 req/h) + ETag.
- **LDAP** (OpenSSH-LPK `sshPublicKey`, plus `displayName`, `mail`,
  `jpegPhoto`); stable id = `entryUUID`, not `uid`. Same principal record.
- **Self-service enrolment**: most developers already have an SSH key on
  GitHub, so a service may let anyone claim `github:<login>` on first contact —
  one-time fetch, prove possession → principal with a default role. Per-service
  policy: **open** (auto-enrol, minimal role) or **closed** (queued for admin).

## Groups, ACL, roles

- **Groups** compose from groups with `& | !` (`eng & !contractors`).
- **ACL** — *who may access*: an expression over users and groups.
- **Roles** — *what they may do*: service-defined strings (`admin`,
  `read-only`, …), assigned with the same expression pattern to users or
  groups. AUTH stores and resolves; it never interprets.
- ACL and roles stay two layers (access vs authority). One expression engine.

## Delegation

Service A calling B for user U authenticates as **A** and adds an
**on-behalf-of: U** claim. B grants it only if A holds a delegation role for U
or U's group. U's key or token never leaves U. A *personal* service calling out
**is** the user calling — no delegation involved.

## Resolved at login

Services know nothing about groups or mappings. On first contact they ask AUTH
one question, signed with the service key, and cache the answer for the epoch:

```
→ who is this user (for me)?
← access: ok | denied · roles: [...] · key: <access-key> · gen: <n>
```

## Local mapping file

Every service may carry `principal → {access, roles[, static token]}`. Local is
consulted first, then AUTH (if on). Populated the same way AUTH does it:
`github:parf` → fetch once → pin. Modes: file only · AUTH only · both (local
overrides for owner, break-glass admin, peer services).

## Ownership — services and topics

- **Publish** a new service or topic: any valid token.
- **Change / delete**: **owner or owner group** only.
- Owner is an expression. Tiers: `owner` (all, incl. ACL and owners) and
  `maintainer` (definition only). Owners use org groups but cannot create
  groups or grant beyond their own service. Personal services are owned by
  their user.
- Definitions and ownership are **live records in `agent-busd`**, not bundle
  data; a record is **signed by its writer when the writer has a key**
  (static-token writes are unsigned). Users run their own services without admin; admin's job is identities
  and org groups.

## Sealed private config

An instance may store its private config (e.g. IMAP credentials) in
`agent-busd`, **sealed to the instance's own key** (age-style box). The daemon
holds opaque bytes + owner + instance id and cannot read them. Boot = key +
binary → config comes back. Versioned, owner-pushed, not bundle data; does not
need the AUTH role. Multi-instance sharing = encrypt to each key or share a
key. A local file remains the default; this is opt-in for portability and
recovery.
