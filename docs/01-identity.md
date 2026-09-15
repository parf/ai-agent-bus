# Identity

## Status

| MVP | Scope |
|---|---|
| Built | Canonical names, credentials, registration and enrolment, [owner controls](#owner-control), [person records](#person-records), [user lifecycle](#user-lifecycle) and [flat groups](#groups-and-maintainers). |
| Pending | Installed-runtime acceptance; future identity extensions remain in their owning release plans. |

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

Creation can be conditional: claim a canonical name only if it is unregistered.
The daemon checks and inserts under the same registry lock, refusing an existing
name even when the caller owns it. Launchers use this to allocate unique session
names concurrently. Ordinary registration still permits authorized updates;
the HTTP condition is implemented by the [API](../src/internal/api/server.go).

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

## Unregistering

**Built:** `agent-bus unregister <name>` removes an idle registry record,
its empty inbox, configuration and subscriptions. [Record management authority](#groups-and-maintainers) is required. Missing names are errors; queued messages or any waiting
reader block removal. Stop readers and drain the queue before unregistering.

This removes an address and the credential issued for it, not the process
behind it. It disappears from discovery, new messages to it are refused, and
its credential stops authenticating. Existing history remains history.

Nothing is held back for the name. It is reserved to nobody, survives no
restart, and whoever asks for it next gets it — its previous owner included,
with no priority. A person's own identity is the one exception, and not as a
reservation: their credential is how they call at all, so removing a record of
theirs does not take it. A re-registered name starts with an empty inbox and
no configuration or subscriptions.

Protecting a name is a thing its owner asks for, not something a removal
buys: declaring a record [retired](../Plans/R1.1/records.md#down-and-retired)
holds its name, and is R1.1 work.
MVP reserved it, and paid a permanent credential per throwaway address for
protection it did not need.

<a id="pending-person-records"></a>

## Person records

**Built.** Maintainer-vouched profiles are separate from service registrations.
The daemon lists profiles, self-owned records and otherwise unregistered credential
holders; service identities owned by somebody else are not listed as people.
Administrators see the user directory; ordinary callers see their own details.

### Who may write a record

Only a daemon maintainer edits user fields, never the person. A maintainer may
edit below their own level, not a peer maintainer or the owner. The owner is
always a maintainer and may edit every level. A trusted person record comes
from maintainer vouching or successful enrolment. This policy works before AUTH, using the daemon owner, the daemon maintainers
group and ordinary users as the three levels. Profile and membership changes
are stored in the existing restart snapshot.

### User lifecycle

**Built.** Authorized administrators add and edit users and apply these states
through the [dashboard](05-discovery.md#required-tabs), under the
[write hierarchy](#who-may-write-a-record).

| State | Effect |
|---|---|
| Active | The principal may authenticate and use its granted access |
| Paused | Tokens, existing browser sessions, local sockets and enrolment cannot grant bus access; new deliveries to the user's inbox are refused |
| Banned | The same access restriction; only the daemon owner may lift the ban |

An authorized maintainer may reactivate a paused ordinary user. The daemon owner
must remain active. State changes cancel the user's blocked reads and are retained by the existing
[snapshot contract](04-messaging.md#durability); queued work is retained subject
to its existing TTL. Credentials are not
rotated or deleted, so reactivation restores their use. Running services retain
their own identities and are not stopped by a change to their owner's user state.
Already delivered work cannot be recalled.

These are the implementation defaults chosen on 2026-09-13, not automatic token
expiry or process supervision.

### Profile fields

Profiles carry person name, email and `GithubUser`; avatars are generated from
the person name and served locally. `GithubUser` is the GitHub login, including
on records in other realms; a GitHub identity keeps its own username. Proving a cross-provider alias is [later identity work](../Plans/R1.2/QUESTIONS.md#open-questions).

Phone and IM routes belong to [later contact routing](../Plans/R1.1/people.md#how-to-reach-a-person).

### Every identifying field is unique

Every supported identifying field is normalised before writing and unique
across person records. Email addresses and GitHub logins are trimmed and compared
in lower-case ASCII. Email syntax and GitHub login syntax are checked; provider
aliases such as dots and plus-addresses are not merged. Person names are display
text, not unique identifiers. These are the initial implementation defaults;
a field with no supported normalisation is not supported.
The rule extends to identifying contact fields when their owning release adds them.

## ACL

**Built.** Access is enforced in [core](../src/internal/core/acl.go), for every
face. The record's owner and its own principal have access. An empty `allow`
is open to authenticated callers; otherwise a matching principal or `*` grants
access. Master grants access unless the record refuses master. Flat group
membership is also resolved here; nested group expressions and
role-bearing terms remain unbuilt.

`allow` and the master-refusal flag are registry properties, never fields read
from the service's private configuration. The daemon owner holds master, and
additional masters are configured at startup. Master access does not grant
ownership of another principal's record.

No writing verb bypasses its applicable access and ownership checks. Querying,
sending and consuming are checked in the daemon; a face cannot widen access.

## Groups and maintainers

**Built.** Flat groups support daemon administration and service/topic maintainers.
The protected `@maintainers` group identifies daemon maintainers; the configured
daemon owner is always a member. Only the daemon owner changes that group, which
cannot be deleted or have the owner removed.
Groups before AUTH are flat named sets of principals,
local to one daemon. The daemon resolves membership at its existing access check;
nesting, expressions and service-defined roles remain [R1](../Plans/R1/identity.md#groups-and-roles).
Organization group administration belongs to daemon administration; owning a
service alone does not grant permission to create groups or administer users.

| Authority | May change |
|---|---|
| Daemon owner | Administer the daemon and its maintainers group; edit users at every level under the [user write hierarchy](#who-may-write-a-record) |
| Daemon maintainers group | Administer users below their own level; cannot edit a peer maintainer or the daemon owner, or promote themselves through membership changes |
| Service or topic owner | One user, not a group or expression; [full control over owned services and topics](#owner-control), including exclusive authority to transfer ownership |
| Service or topic maintainers group | Change the assigned record's definition, supported run options, availability and ACL, except ownership; membership confers no authority over unrelated records or daemon administration |

The daemon owner is always a daemon maintainer. Daemon maintenance and record maintenance
are distinct scopes, even when they include the same people. Master access is
still [access, not ownership](#acl). The authenticated record itself retains its
existing right to re-register; that does not grant ownership transfer.
Daemon administrators edit ordinary group membership. An assigned or ACL-referenced
group cannot be deleted until its references are removed. Names are available
for service-owner assignment; membership lists are visible to daemon administrators. Runtime start/stop controls remain [R1 runner work](../Plans/R1/runner.md#what-the-runner-does).

### Owner control

**Built for supported daemon operations.** A user has **full control over services
and topics they own**, without daemon owner approval or membership in the daemon maintainers group. This includes
editing the definition, configuring the service, managing access and assigned
maintainers, enabling or disabling it, deleting it and transferring ownership.
Supported runtime lifecycle operations are also available to the owner when
[managed runner controls](../Plans/R1/runner.md#what-the-runner-does) ship.
The dashboard exposes these controls on the user's own records under
[required tabs](05-discovery.md#required-tabs).

Ownership is sufficient authorization; operations still obey their contracts,
including [idle removal](#unregistering) and
[private configuration handling](03-services-and-topics.md#configuring-a-template).
This grants no authority over other users' services or daemon administration.

Disabling refuses new deliveries to the service and inbox reads, including
blocked reads, while preserving queued messages. Enabling permits delivery and
reading again; it does not start an OS process. Already delivered work is not
recalled. A service re-registering cannot undo its availability or maintainer
assignment. Removing access also cancels blocked reads that relied on it.

Transfer requires a registered, self-owned recipient identity; a self-owned
identity itself cannot be transferred. Existing service credentials and copies
already held remain valid under the [credential policy](02-access.md#token-lifetime).
Transfer changes record ownership and who may request its token; it is not
credential revocation.

## Ownership

**Built.** Publishing a name gives it one owner. Existing records may be changed
by their owner, assigned maintainers or the record's own principal. Re-registration preserves
ownership. The same rule protects registry configuration; only the service
itself may read that configuration back.

A self-owned record can identify a person in the [people view](#person-records).
Person-profile writes use the [maintainer hierarchy](#who-may-write-a-record);
service and topic changes use [record management authority](#groups-and-maintainers).
