# Identity extensions

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Groups and roles

Nested membership is [built](../../docs/01-identity-and-roles.md#groups).
**Record-defined roles are R1 work, and the first R1 topic after 0.7**: the
owner moved them out of MVP on 2026-09-22. The expression syntax and
role-transport design below remain proposals; accepting the capability does not
adopt these specific representations.

- **Groups** compose from groups with `& | !` (`@eng & !@contractors`) when
  AUTH is on. Basic flat groups and maintainers are now
  [required MVP](../../docs/01-identity-and-roles.md#groups).
  The expression engine comes with AUTH: `& | !` on the authorization path
  is where a precedence bug grants silently.
- **Roles** — *what a principal may do*: record-defined strings (`admin`,
  `read-only`, …) written in parentheses after the term ([sigils](#sigils)),
  assigned with the same expression pattern as groups. The daemon stores and
  resolves them; **it never interprets** them — the one place a role goes is
  the answer to an agent asking who its caller is, so there is no code path
  here that could. Maintainer stays the reserved management role, and a role
  edit must never let a Maintainer remove or replace another Maintainer.
- Access and authority stay two layers. One expression engine.


## Sigils

**An ACL entry is a term, and the term says what kind of thing it names** — so
one list holds every kind and nothing needs a type field beside it. The terms
themselves are current 0.7 scope, owned by
[actor terms](../../docs/constitution.md#actors-and-ascii-textarea-syntax):
`parf` a User, `#batcher@srv1` an Agent, `@dev` a Group, `*` every active User
and Agent, and the reserved `@owner` and `@agent`. R1 adds only the role part.

**Roles go in parentheses after the term, and are left out when there are
none**: `parf@github(admin)`, `@dev(deploy, read-only)`, `*(guest)`,
`#batcher@srv1`. A role is a service-defined string ([groups and
roles](#groups-and-roles)) and is handed to the service exactly as written —
it is that service's own vocabulary, not ours.

| | |
|---|---|
| **being in the list is the access** | the entry grants the call; the parentheses say in what capacity. There is no second access level beside the role, because *may call* and *what the service is told* were the only two things there ever were |
| **the sigil disambiguates subjects, and only subjects** | a group has to be told from a user and a service from both — `@dev & !@contractors` reads as an expression over groups where `dev & !contractors` does not say what it is combining. A role needs no mark: the parentheses already say what it is |
| **a user starts alphanumeric** | which is what any name starts with anyway ([names](../../docs/01-identity-and-roles.md#names)), so this is the rule already there rather than a second one for ACLs. A term beginning with anything else is a group, an agent, a reserved term or nothing at all |

⚠️ `#` **begins a comment** in a shell word, in YAML and in `.env`. Typed as
`--allow #batcher@srv1` it fails loudly — the flag ends up with no value — but
at the start of a line in a config file it disappears without one. Quote it,
or use the CLI's [`--agent` form](../../docs/constitution.md#actors-and-ascii-textarea-syntax),
and do not put an agent first on a line.

**A group is local to one `agent-busd`**, and its name may carry a realm by the
ordinary [name rules](../../docs/01-identity-and-roles.md#names): `@dev` and
`@dev@company` are both Group names. The realm is part of the name, not a
pointer to another daemon: every ACL a daemon enforces is on a record it holds,
so the daemon is the scope. AUTH may say who is *in* a group, but the name is
resolved where it is used.

**An upstream daemon has its own groups, and we do not care.** A group never
travels — a chained call carries the principal
([delegation](#delegation)), and the upstream decides with its own list
([overview § chaining](federation.md#chaining)). So two daemons may both have
`@dev` and mean different people, and neither has to know: a group's name,
realm included, never has to agree with another daemon's.


## Delegation

Agent A calling B for user U authenticates as **A** and adds an
**on-behalf-of: U** claim. B grants it only if A holds a delegation role for U
or U's group. U's key or token never leaves U. From 0.7 every agent token
already names its Owner ([token](../../docs/constitution.md#-token)), so an
agent acting for its own Owner needs no delegation.


## Resolved at login

Agents know nothing about groups or mappings. On first contact they ask AUTH
one question, signed with the service key, and cache the answer according to
the [AUTH consistency contract](auth.md#consistency-window):

```
→ who is this user (for me)?
← access: ok | denied · roles: [...] · key: <access-key> · gen: <n>
```


## Ownership

The owner and maintainers model is now
[required MVP](../../docs/01-identity-and-roles.md#groups), including flat
groups before AUTH. R1 adds [managed runner controls](runner.md#what-the-runner-does)
and the distributed record behavior below.

- **Every record is owned by a User** from 0.7
  ([registry record](../../docs/constitution.md#-registry-record)); there are no
  self-owned or Agent-owned records. What a person runs is what that User owns,
  and [Personal](../../docs/03-records.md#personal-and-shared) is a separate
  web classification rather than something read from ownership.
- Definitions and ownership are **live records** in `agent-busd`, not bundle
  data; a record is **signed by its writer when the writer has a key**
  (static-token writes are unsigned — the token authenticated them). Users run
  their own agents without admin; admin's job is identities and org groups.


## Sealed private config

A service may store its private config (e.g. IMAP credentials) in
`agent-busd`, **sealed to the service's own key** (age-style box). The daemon
holds opaque bytes + owner + service name and cannot read them. Boot = key +
binary → config comes back. Versioned, owner-pushed, not bundle data; does not
need the AUTH role. Sharing across services = encrypt to each key or share a
key. A local file remains the default; this is opt-in for portability and
recovery.
