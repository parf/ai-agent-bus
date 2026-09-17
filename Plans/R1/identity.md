# Identity extensions

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

## Groups and roles

Basic nested membership and service-defined roles are now
[accepted implementation work](../../docs/01-owners-and-maintainers.md#added-implementation-scope).
The expression syntax and role-transport design below remain proposals; accepting
the capabilities does not adopt these specific representations.

- **Groups** compose from groups with `& | !` (`@eng & !@contractors`) when
  AUTH is on. Basic flat groups and maintainers are now
  [required MVP](../../docs/01-identity.md#groups-and-maintainers).
  The expression engine comes with AUTH: `& | !` on the authorization path
  is where a precedence bug grants silently.
- **Roles** — *what a principal may do*: service-defined strings (`admin`,
  `read-only`, …) written in parentheses after the term ([sigils](#sigils)),
  assigned with the same expression pattern as groups. AUTH stores and
  resolves; **it never interprets** — and the one place a role goes is the
  answer to a service asking who its caller is, so there is no code path here
  that could.
- Access and authority stay two layers. One expression engine.


## Sigils

**An ACL entry is a term, and the term says what kind of thing it names** — so
one list holds every kind and nothing needs a type field beside it.

| Term | Is |
|---|---|
| `parf@github` | a **user** — anything that is not one of the three below |
| `@dev` | a **group** |
| `#batcher@srv1` | a **service** ([service to service](../R1.1/access.md#service-to-service)) |
| `*` | **anyone who can authenticate** |

**Roles go in parentheses after the term, and are left out when there are
none**: `parf@github(admin)`, `@dev(deploy, read-only)`, `*(guest)`,
`#batcher@srv1`. A role is a service-defined string ([groups and
roles](#groups-and-roles)) and is handed to the service exactly as written —
it is that service's own vocabulary, not ours.

| | |
|---|---|
| **being in the list is the access** | the entry grants the call; the parentheses say in what capacity. There is no second access level beside the role, because *may call* and *what the service is told* were the only two things there ever were |
| **the sigil disambiguates subjects, and only subjects** | a group has to be told from a user and a service from both — `@dev & !@contractors` reads as an expression over groups where `dev & !contractors` does not say what it is combining. A role needs no mark: the parentheses already say what it is |
| **a user starts alphanumeric** | which is what any name starts with anyway ([names](../../docs/01-identity.md#names)), so this is the rule already there rather than a second one for ACLs. A term beginning with anything else is a group, a service, or nothing at all |

⚠️ `#` **begins a comment** in a shell word, in YAML and in `.env`. Typed as
`--allow #batcher@srv1` it fails loudly — the flag ends up with no value — but
at the start of a line in a config file it disappears without one. Quote it,
and do not put a service first on a line.

**A group is local to one `agent-busd`**: `@dev`, never `@dev@company`. A
group is only ever an ACL subject ([acl](../../docs/01-identity.md#acl)), and every ACL a daemon
enforces is its own — a record it holds, or its master list — so the daemon is
the scope, not the host it happens to share. AUTH may say who is *in* a group,
but the name is resolved where it is used: there is no second `@` to read, and
nothing to disambiguate against a principal's realm.

**An upstream daemon has its own groups, and we do not care.** A group never
travels — a chained call carries the principal
([delegation](#delegation)), and the upstream decides with its own list
([overview § chaining](federation.md#chaining)). So two daemons may both have
`@dev` and mean different people, and neither has to know. That is the whole
benefit of not giving a group a realm: there is no namespace to collide in.


## Delegation

Service A calling B for user U authenticates as **A** and adds an
**on-behalf-of: U** claim. B grants it only if A holds a delegation role for U
or U's group. U's key or token never leaves U. A *personal* service calling out
**is** the user calling — no delegation involved.


## Resolved at login

Services know nothing about groups or mappings. On first contact they ask AUTH
one question, signed with the service key, and cache the answer according to
the [AUTH consistency contract](auth.md#consistency-window):

```
→ who is this user (for me)?
← access: ok | denied · roles: [...] · key: <access-key> · gen: <n>
```


## Ownership

The owner and maintainers model is now
[required MVP](../../docs/01-identity.md#groups-and-maintainers), including flat
groups before AUTH. R1 adds [managed runner controls](runner.md#what-the-runner-does)
and the distributed record behavior below.

- **A record that owns itself is somebody; one owned by another name is
  something they run.** Personal services being owned by their user is what
  makes that read: it is the whole difference the people view needs
  ([discovery § what it shows](../../docs/05-discovery.md#what-it-shows)), and it costs
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
