# Identity

Who is on the bus, and what they may do. How they prove it is
[access](02-access.md).

## Principals

Every **user** and **service** is a *principal* — agents, consumers and
publishers included. A service is always a configured one
([services § service and template](03-services-and-topics.md#service-and-template));
an unconfigured template is not a principal, because it does not run and has no
address. Only *generic* records (a description pushed by someone else) have no
identity of their own.

## Names

A principal is written **`$user@$realm`**. The realm says *who vouches for the
name*:

| Form | Realm is | Vouched for by | Example |
|---|---|---|---|
| `user@bus` | a name a daemon answers for | that daemon | `parf@localhost`, `parf@om.parf.dev` |
| `user@provider` | an identity provider | the provider — name and public key | `parf@github` |
| `user@team` | a team, a group on the AUTH server | AUTH | `parf@realmo` |

A **service** is written the same way, and may prefix the **service template**
it was configured from:

| Form | Is | Example |
|---|---|---|
| `service@realm` | a standalone service, with no separate template | `claude@rdvp` |
| `template/instance-name@realm` | a service configured from a template | `imap-mail-reader/billing@rdvp` |

**A realm is the name a daemon answers for, and a hostname is only its
default.** A daemon may be told to hold others, and `image-scaler@pool1` is why:
a service answered by processes on four hosts must be **one name**, and a name
carrying any of those hostnames would be false for the other three
([runner § one name on many hosts](08-runner-role.md#one-name-on-many-hosts)).
A pool realm claims membership instead of a location, which stays true when a
member moves.

**A bare name is completed with the local host, and that completion is a
convenience with no authority in it.** `image-scaler` becomes
`image-scaler@<host>` so the common case is typed short; it asserts nothing
about where the process sits. **A name that arrives complete is taken whole**:
nothing appends to it,
substitutes into it, or checks a hostname against it. So a service that wants a
realm of its own simply says one.

Everything else is unchanged — whoever vouches is still the
daemon you are talking to, and two daemons holding one realm name is the same
locality as two of them holding one group ([sigils](#sigils)).

**Both halves are `a-z 0-9 . _ -`, and both start alphanumeric**; the local
part may also hold `+` and `@`. Lowercase, because a name that differs only in
case is two names to a machine and one to a person. Starting alphanumeric is
what leaves the leading characters free to mean something else
([sigils](#sigils)), and it costs nothing a realm actually uses — a GitHub
handle may begin with a digit, and `0xdead@github` is a name like any other.

**The realm is whatever follows the last `@`.** The local part — an instance
name, or a user — may itself be an address, since the thing that reads a
mailbox is reasonably named after it. So `mail-sender/parf@comfi.com@srv1` is
template `mail-sender`, instance `parf@comfi.com`, realm `srv1`. The bus reads **no meaning** out of it: it is a
name it routes on, never a mailbox it parses.

One syntax for everything on the bus, three sources of authority behind it. The
template part is **optional and part of the identity**, not a lookup: two
services from one template are two names, two inboxes, two configs
([services § service and template](03-services-and-topics.md#service-and-template)).

**One name has one spelling: lower-case, and trimmed — the whole name *and
each component*.** Case and surrounding space are noise, so they go before
anything compares, stores or routes on a name: `PARF@Localhost` and
` parf@localhost ` are the same principal, `mail-sender / parf@comfi.com @
host` is `mail-sender/parf@comfi.com@host`, and a registration cannot land in
one inbox while a send goes to another. Space *inside* a component is not
trimmed away — it is a bad character, and the name is refused.

| | |
|---|---|
| charset | **`a-z 0-9 . _ -`**, every component, starting alphanumeric |
| the local part | wider: also **`+`** and **`@`**, so `parf+alerts@comfi.com` is one — whether it names a service instance or a user. The template and the host take neither |
| at-signs | the **last** one splits off the host. Earlier ones sit inside the instance name, joining non-empty components — `parf@` and `parf@@x` are refused |
| length | **64 characters for the whole name**, `@` and any `/` included — a name is an identifier, not a payload |
| ASCII only | a name spellable two ways in Unicode is a name two people can be tricked by |
| dots | allowed in every part — a realm is often a host (`om.parf.dev`), and a local part may be dotted (`slack.reader`) |
| slashes | **at most one**, and only between template and instance name. A realm never holds one, and never an `@`: a realm is a name, not a path |

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

**"Optional details" is three fields** — person name, email and avatar URL —
because those are what GitHub already fills in, so they cost nothing at
enrolment and a dashboard has something to render
([discovery § what it shows](05-discovery.md#what-it-shows)). They are
description, never enforcement, and the principal writes its own.

Two things that look like fields of the same kind and are not:

| Looks like a field | Is |
|---|---|
| **status: active / inactive / banned** | an access decision. Rendered from something nobody enforces it is a lie — the page says *banned* while the token still works. It belongs where enforcement is: `revoked_users` in the bundle ([AUTH role § consistency window](06-auth-role.md#consistency-window)) |
| **role** | service-defined and never interpreted here ([groups and roles](#groups-and-roles)). What can honestly be shown is **access**: master or not, owner of what, member of which group |

Phone numbers and IM handles are contact routes nothing on the bus uses —
nothing routes on them, nothing checks them, and holding them makes the record
worth protecting for reasons that have nothing to do with the bus. Later, if
an organisation asks.

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

### Proving possession

**Fetching a key is not authentication**, so the proof is a step of its own and
the `directory` port stays a lookup — one function, so that every future
provider is a fetch to write and not a protocol to get right
([modules § modules](10-modules.md#modules)).

| Step | |
|---|---|
| 1 | the newcomer claims `<login>@<realm>` |
| 2 | the bus looks the login up, **keeps the keys it found**, and answers with a nonce — what is checked later is what was published when the question was asked |
| 3 | the newcomer signs the nonce with the private half, using the host's own `ssh-keygen`; nothing but that tool touches a private key |
| 4 | the bus verifies against the kept keys, writes the record **owned by the name itself**, and hands back its credential |

The record being its own owner is what makes this worth doing: nobody else may
re-register over it ([ownership](#ownership)) and nobody else may be handed its
credential ([access § getting a token](02-access.md#getting-a-token)), so the
name is the key-holder's and stays that way.

**A realm with a directory behind it cannot be registered into at all** — only
enrolled into. Otherwise the first caller to ask for `someone@github` would
become them, which is exactly the hole enrolment exists to close. Realms nobody
vouches for stay open, and there the claim is still first-come.

**The exchange carries no credential**, and it is the only thing on the bus
that does not: it is where a credential comes from, so wanting one first would
be a circle. What makes that safe is that the signature *is* the credential,
and only a realm somebody vouches for can be entered this way — everything
else on the API still refuses a caller it cannot name.

The same two steps answer a second question: a name that is already enrolled
can ask for its credential again the same way, which is how a host with no
sshd hands one out ([access § getting a token](02-access.md#getting-a-token)).

An unanswered challenge expires; an answered one is spent.

## Groups and roles

- **Groups** compose from groups with `& | !` (`@eng & !@contractors`). They
  exist only with the AUTH role on. ⚠️ A group that arrives before AUTH does
  is a **flat named set** — a list of principals, a record like any other,
  expanded in the one place `allow` is already checked ([acl](#acl)). The
  expression engine comes with AUTH and not before: `& | !` on the
  authorization path is where a precedence bug grants silently.
- **Roles** — *what a principal may do*: service-defined strings (`#admin`,
  `#read-only`, …), assigned with the same expression pattern as groups. AUTH
  stores and resolves; **it never interprets** — and the one place a role goes
  is the answer to a service asking who its caller is ([sigils](#sigils)), so
  there is no code path here that could.
- Access and authority stay two layers. One expression engine.

## Sigils

**A leading character says what kind of thing an ACL entry is**, so one list
holds every kind and nothing needs a type field beside it.

| | Is | Appears |
|---|---|---|
| `parf@github` | a **user** — anything that is not one of the two below | either side |
| `@dev` | a **group** | only as an ACL subject |
| `#admin` | a **role** | only in what is handed to a service, and `#`-stripped on the way |

Each sigil disambiguates **within its own side** of an entry, which is why
both are needed and neither is decoration:

- the left side holds subjects, where a group has to be told from a user —
  `@dev & !@contractors` reads as an expression over groups, `dev &
  !contractors` does not say what it is combining;
- the right side holds access *and* an optional role, where a role has to be
  told from an access level: `parf@github => rw, #admin`.

**A user starts alphanumeric**, which is what any name starts with anyway
([names](#names)) — so the rule is the one already there rather than a second
one for ACLs, and an entry beginning with anything else is a group, a role, or
nothing at all.

**The `#` never leaves.** It marks a role as ours while it is stored here; the
service asking about its caller is given `admin`, because a role is that
service's own vocabulary and our namespace is not its business.

⚠️ `#` **begins a comment** in a shell word, in YAML and in `.env`. Typed as
`--allow #admin` it fails loudly — the flag ends up with no value — but at the
start of a line in a config file it disappears without one. Quote it, and do
not put a role first on a line.

**A group is local to one `agent-busd`**: `@dev`, never `@dev@company`. A
group is only ever an ACL subject ([acl](#acl)), and every ACL a daemon
enforces is its own — a record it holds, or its master list — so the daemon is
the scope, not the host it happens to share. AUTH may say who is *in* a group,
but the name is resolved where it is used: there is no second `@` to read, and
nothing to disambiguate against a principal's realm.

**An upstream daemon has its own groups, and we do not care.** A group never
travels — a chained call carries the principal
([delegation](#delegation)), and the upstream decides with its own list
([overview § chaining](00-overview.md#chaining)). So two daemons may both have
`@dev` and mean different people, and neither has to know. That is the whole
benefit of not giving a group a realm: there is no namespace to collide in.

## ACL

Two layers, tried in that order. Both use the same shape: a subject — a user
or an `@group` ([sigils](#sigils)) — mapped to access and an optional
`#role`.

| Layer | Lives in | Says |
|---|---|---|
| **service ACL** | the service's own **record** | `allow` — who may see and use *this* service |
| **master ACL** | `agent-busd` | `user => role`, `group => role` — holders reach **every** service on the node, with no per-service entry and no per-service token |

**The record, not the configuration.** A service's configuration is private to
it and the daemon will not read it
([services § configuring a template](03-services-and-topics.md#configuring-a-template)),
so a layer the daemon enforces cannot live there. The record is what the
service already states, the daemon already holds, and its owner already
controls — so `allow` is a field on it, stated like any other.

- **The service is asked first.** Its own record decides, and if it answers,
  that is the answer.
- **A service may refuse master access** — one flag on its own record, and
  master holders are treated like anyone else. The service, not the node, has
  the last word on itself.
- **`*:` is the wildcard entry**: `*: => users | groups` applies to every
  service with no entry of its own.
- **`allow: *`** on a service means *anyone who can authenticate* — every
  GitHub user, for instance. That is how a sign-up service opens itself to the
  world: `allow: *`, minimal role, and the newcomer's first request is the
  enrolment.
- **Master ACL replaces per-service setup.** Without it every service needs its
  own list and its own tokens; with it an operator is configured once.

- **No verb is a side door.** Writing goes through the same two layers as
  reading: a send, a publish, a registration and a consume are all subject to
  them, so a principal that may not see a service cannot enqueue to it either.

The user who ran setup **holds master**: users and groups, service ACL, and
service install / start / stop / restart ([runner § what the runner does](08-runner-role.md#what-the-runner-does)). The
name `agent-bus-admin` belongs to the program an operator runs, not to a role
([setup § the programs](09-setup.md#the-programs)). Owners still own their service *definitions* and
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

## Ownership

- **Publish** a new service or topic: any authenticated principal.
- **Change / delete**: **owner or maintainer** only — and the record itself:
  a service re-registering on every start is not a stranger to its own name,
  and it is the only other principal that can hold that name's credential
  ([access § getting a token](02-access.md#getting-a-token)).
- **The owner is exactly one user; the maintainer is a group.** Not an
  expression and not a tier list: *whose is this?* has to have one answer, and
  an expression can match many people or none. One name is also one person
  accountable for it, which a group is not.

| | May change |
|---|---|
| **maintainer** (a group) | what the service *is* — definition, run options, enable/disable — and the ACL, **except** the owner entry |
| **owner** (one user) | that, and ownership itself |

  So granting somebody use of a service never grants them the record: the ACL
  and this are two lists, and only the owner moves the line between them.
  Owners use org groups but cannot create groups or grant beyond their own
  service. Personal services are owned by their user.
- ⚠️ **With the AUTH role off there are no maintainers**, because there are no
  groups ([groups and roles](#groups-and-roles)) — a service has an owner and
  nothing else, which is the whole of the required minimum's answer.
- **A record that owns itself is somebody; one owned by another name is
  something they run.** Personal services being owned by their user is what
  makes that read: it is the whole difference the people view needs
  ([discovery § what it shows](05-discovery.md#what-it-shows)), and it costs
  no flag that can go stale against the owner field beside it.
- Definitions and ownership are **live records** in `agent-busd`, not bundle
  data; a record is **signed by its writer when the writer has a key**
  (static-token writes are unsigned — the token authenticated them). Users run
  their own services without admin; admin's job is identities and org groups.

## Sealed private config

A service may store its private config (e.g. IMAP credentials) in
`agent-busd`, **sealed to the service's own key** (age-style box). The daemon
holds opaque bytes + owner + service name and cannot read them. Boot = key +
binary → config comes back. Versioned, owner-pushed, not bundle data; does not
need the AUTH role. Sharing across services = encrypt to each key or share a
key. A local file remains the default; this is opt-in for portability and
recovery.
