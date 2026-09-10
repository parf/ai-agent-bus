# Identity

Who is on the bus, and what they may do. How they prove it is
[access](02-access.md).

## Principals

Every **user**, **service** and **instance** is a *principal* — agents,
consumers and publishers included. Only *generic* records (a description pushed
by someone else) have no identity of their own.

## Names

A principal is written **`$user@$realm`**. The realm says *who vouches for the
name*:

| Form | Realm is | Vouched for by | Example |
|---|---|---|---|
| `user@host` | a specific host | that host's `agent-busd` | `parf@localhost`, `parf@om.parf.dev` |
| `user@provider` | an identity provider | the provider — name and public key | `parf@github` |
| `user@team` | a team, a group on the AUTH server | AUTH | `parf@realmo` |

Same shape as a service address
([services § service and instance](03-services-and-topics.md#service-and-instance))
— one syntax for everything on the bus, three sources of authority behind it.

**The name is the identity.** Provider numeric ids are stored, never used as
ids: they are what a re-check compares against. If a name stops resolving to
its stored id the **account is disabled** — a rename or a recycled login is
locked out, not followed. Internal ids may exist if something needs one; they
are never the name.

One human may hold several principals (`parf@github`, `parf@realmo`); a
grouping "person" record is deferred.

## Registration

**The normal way to register is to state the record:**

| Field | |
|---|---|
| username | `user@realm` |
| person name | who the human is |
| public key | Ed25519 |
| optional details | email, avatar, whatever the org wants |

That is the whole thing. It needs no directory, no network and no provider.

**MVP is this plus GitHub** — nothing else. GitHub is an *alternative to typing
the record*: it fills the same fields from a login the person already has,
which is the only reason open public enrolment is practical (a stranger
arrives already holding a name and a key). What it produces is an ordinary
registration record. LDAP/AD is deferred: [future/ldap-ad.md](future/ldap-ad.md).

**After enrolment a provider is out of the picture.** The record is pinned; the
provider is never on the runtime path, never polled, and the bus works
unchanged if it disappears. The one thing that ever goes back is a **deliberate
re-check** of a user's access.

| Source | Fills in from | Re-check compares |
|---|---|---|
| **direct** | you | nothing — there is no upstream to disagree with |
| **GitHub** | `api.github.com/users/<login>` → `id`, `login`, `name`, `avatar_url`, `email` (if public); `/users/<login>/keys` → per key `id`, `key`, `created_at`, `last_used` (verified 2026-09-09). Plain-text fallback `github.com/<login>.keys` | the stored numeric `id` |

- Re-checks are **explicit**, not scheduled. Nothing forces one; run them when
  you want the assurance. Consequence: a key deleted upstream stays valid until
  someone asks.
- GitHub extras when you do look: `created_at` → "new key on privileged
  principal" alert · `last_used` → stale-key pruning (e.g. ignore > 12 months)
  · `id` → rotation vs addition. Our own `last_used` is tracked separately. If
  ever polled: token (5 000 req/h) + ETag.
- **Ed25519 only** (pairwise needs Ed25519→X25519). RSA/ECDSA keys are filtered
  at fetch time and the user is told which were skipped.
- A provider supplies public keys only — it never authenticates anyone.
- **Self-service enrolment** is the same path run by the stranger themselves:
  claim `<login>@github` on first contact, one-time fetch, prove possession →
  an ordinary record with a default role. Per-service policy: **open**
  (auto-enrol, minimal role) or **closed** (queued for admin).
- **Later: Google, LinkedIn, Facebook** and other sign-in providers. They hold
  no SSH keys, so the account proves *who* and the bus issues the Ed25519 key
  at enrolment. Same record, own realm. Not designed.

## Groups and roles

- **Groups** compose from groups with `& | !` (`eng & !contractors`). They
  exist only with the AUTH role on.
- **Roles** — *what a principal may do*: service-defined strings (`admin`,
  `read-only`, …), assigned with the same expression pattern as groups. AUTH
  stores and resolves; it never interprets.
- Access and authority stay two layers. One expression engine.

## ACL

Two layers, tried in that order. Both use the same shape: a subject (user or
group) mapped to access and an optional role.

| Layer | Lives in | Says |
|---|---|---|
| **service ACL** | the service's own config | `service: => users, groups (+ roles)` — who may see and use *this* service |
| **master ACL** | `agent-busd` | `user => role`, `group => role` — holders reach **every** service on the node, with no per-service entry and no per-service token |

- **The service is asked first.** Its own config decides, and if it answers,
  that is the answer.
- **A service may refuse master access** — one flag in its own config, and
  master holders are treated like anyone else. The service, not the node, has
  the last word on itself.
- **`*:` is the wildcard entry**: `*: => users | groups` applies to every
  service with no entry of its own.
- **`allow: *`** inside a service means *anyone who can authenticate* — every
  GitHub user, for instance. That is how a sign-up service opens itself to the
  world: `allow: *`, minimal role, and the newcomer's first request is the
  enrolment.
- **Master ACL replaces per-service setup.** Without it every service needs its
  own list and its own tokens; with it an operator is configured once.

The user who ran setup gets the **`agent-bus-admin`** role in the master ACL:
users and groups, service ACL, and service install / start / stop / reload
([runner](08-runner-role.md)). Owners still own their service *definitions* and
run services without an admin; admin is the escalation path and the node
operator, not a required participant.

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

Every service may carry `principal → {access, roles[, static token]}`. With
AUTH off this map **is** the service's audience and role source.

**Local users need no token in it** — the daemon already knows who they are
from the socket ([access § local socket](02-access.md#local-socket)) — so they
appear as bare principals with access and roles.

Local is consulted first, then AUTH (if on). Populated the same way AUTH does
it: `parf@github` → fetch once → pin. Modes: file only · AUTH only · both
(local overrides for owner, break-glass admin, peer services).

## Ownership

- **Publish** a new service or topic: any authenticated principal.
- **Change / delete**: **owner or owner group** only.
- Owner is an expression. Tiers: `owner` (all, incl. ACL and owners) and
  `maintainer` (definition only). Owners use org groups but cannot create
  groups or grant beyond their own service. Personal services are owned by
  their user.
- Definitions and ownership are **live records** in `agent-busd`, not bundle
  data; a record is **signed by its writer when the writer has a key**
  (static-token writes are unsigned — the token authenticated them). Users run
  their own services without admin; admin's job is identities and org groups.

## Sealed private config

An instance may store its private config (e.g. IMAP credentials) in
`agent-busd`, **sealed to the instance's own key** (age-style box). The daemon
holds opaque bytes + owner + instance id and cannot read them. Boot = key +
binary → config comes back. Versioned, owner-pushed, not bundle data; does not
need the AUTH role. Multi-instance sharing = encrypt to each key or share a
key. A local file remains the default; this is opt-in for portability and
recovery.
