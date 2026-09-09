# Identity and AUTH / Config

## Principals

- Every **user**, **service** and **instance** is a *principal* with an
  Ed25519 key. Ids are namespaced: `github:<id>`, `ldap:<uuid>`, `svc:<name>`.
- Authentication is always a signed challenge. Key directories only supply
  public keys — they never authenticate.
- Ed25519 keys only (required for pairwise mode).

## Key directories

- **GitHub is the main user id**: `api.github.com/users/<login>` (name, avatar)
  and `/users/<login>/keys` (keys with `id`, `created_at`, `last_used`).
  Fetched **only at enrollment or explicit key refresh**, then pinned in the
  signed generation. GitHub is never on the runtime path.
- **LDAP** (`sshPublicKey`) is a second directory, normalized to the same record.
- Because any developer already has a GitHub key, services can offer
  **self-service enrollment to anyone on the internet**: prove possession of a
  listed key on first contact → principal created with a default role.

## Users & groups

Groups compose from other groups with `& | !`.

## Per-service policy

All written as expressions over users and groups, evaluated by one engine:
- **ACL** — *who may access* (`group1 & group2`, `alice | ops`).
- **Roles** — *what they may do*; vocabulary is **service-defined**
  (`admin`, `manager`, `read-only`…). AUTH stores and resolves, never interprets.
- Keep ACL and roles as two separate layers (access vs. authority).

## Resolved at login

Services know nothing about groups or mappings; on first contact they ask one
question and cache the answer for the epoch:

```
→ who is this user (for me)?
← access: ok | denied · roles: [...] · key: <access-key>
```

## Local mapping file

Every service may carry its own `principal → {access, roles}` file. Local is
consulted first, then AUTH.
Modes: file only (standalone) · AUTH only (org) · both (org + local overrides).

## Ownership

Owner is an expression too. Owners edit the service definition, ACL and roles
(using org groups; they cannot create groups or grant beyond their own service).
Two tiers: `owner` (all, incl. ACL/owners) and `maintainer` (definition only).
Personal services are owned by their user.

## Encrypted private config

An instance may store its private config (e.g. IMAP credentials) in
AUTH/Config, **sealed to the instance's own key**. AUTH holds opaque bytes it
cannot read. Boot = key + binary → config comes back.
Local file remains the default; this is opt-in for portability/recovery.
Not part of the signed generation (owner-pushed, no signing key needed).
