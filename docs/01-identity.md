# Identity

## Status

| MVP | Scope |
|---|---|
| Built | Canonical names, token-backed principals, manual registration, key-possession enrolment, service ACL and ownership. |
| Pending | Person fields, maintainer editing and identifier uniqueness; see [pending person records](#pending-person-records). |

## Principals

A principal is the name a credential authenticates. A registry record describes
something reachable; registering a description does not prove its subject holds
a key. A name owns an inbox independently of the process serving it.

## Names

Names are `user@realm`, optionally `template/instance@realm`. The realm identifies
the namespace the daemon or a configured directory answers for; it is not proof
of a process's physical location. A complete name is taken whole. The foreground
runner completes a bare service name with the local hostname.

**Both halves are `a-z 0-9 . _ -`, and both start alphanumeric**; the local
part may also hold `+` and `@`. Lowercase, because a name that differs only in
case is two names to a machine and one to a person. Starting alphanumeric is
what leaves the leading characters free to mean something else
([future sigils](../Plans/R1/identity.md#sigils)), and it costs nothing a realm actually uses — a GitHub
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

## Registration

**Built.** Any authenticated principal may publish an unheld name in a realm
without a directory. The caller becomes its owner. A backed realm requires
proof of a key the directory publishes; the resulting record owns itself.
GitHub enrolment fetches public keys, not profile details. Existing tokens
continue working if the provider is unavailable.

The record actually stored is defined in
[protocol source](../src/internal/protocol/envelope.go); there is no second
schema here.

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

## Pending person records

These are accepted MVP requirements, **not implemented** by the current record
or ACL code. The people view depends on them.

### Who may write a record

Only a daemon maintainer edits user fields, never the person. A maintainer may
edit below their own level, not a peer maintainer or the owner. The owner is
always a maintainer and may edit every level. A trusted person record comes
from maintainer vouching or successful enrolment. This pending policy must work
before the future AUTH role; its representation is an [open MVP question](../Plans/MVP/QUESTIONS.md#open-questions).

### Profile fields

The person profile carries person name, email, avatar and `GithubUser`.
The latter is the GitHub login, including on records in other realms; when
registered from GitHub it equals the username. Collection of profile data is
pending. Proving a cross-provider alias is [later identity work](../Plans/R1.2/QUESTIONS.md#open-questions).

Phone and IM routes belong to [later contact routing](../Plans/R1.1/people.md#how-to-reach-a-person).

### Every identifying field is unique

Every supported identifying field is normalised before writing and unique
across person records. A field with no supported normalisation is not supported.
The [normalisation question](../Plans/MVP/QUESTIONS.md#open-questions) remains open.
The rule extends to identifying contact fields when their owning release adds them.

## ACL

**Built.** Access is enforced in [core](../src/internal/core/acl.go), for every
face. The record's owner and its own principal have access. An empty `allow`
is open to authenticated callers; otherwise a matching principal or `*` grants
access. Master grants access unless the record refuses master. There are no
built group expressions or role-bearing terms.

`allow` and the master-refusal flag are registry properties, never fields read
from the service's private configuration. The daemon owner holds master, and
additional masters are configured at startup. Master access does not grant
ownership of another principal's record.

No writing verb bypasses its applicable access and ownership checks. Querying,
sending and consuming are checked in the daemon; a face cannot widen access.

## Ownership

**Built.** Publishing a name gives it one owner. Existing records may be changed
by their owner or by the record's own principal. Re-registration preserves
ownership. The same rule protects registry configuration; only the service
itself may read that configuration back.

A self-owned record can identify a person for the pending people view, but it
does not imply the profile fields are implemented. Maintainer changes to user
records remain the [pending contract](#pending-person-records).
