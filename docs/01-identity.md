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

**Principal, not merely a credential holder.** The caller must be somebody the
daemon already holds a profile or a record for; a name holding nothing but a
credential is refused, its own name included
([what a call carries](02-access.md#what-a-call-carries)). So a name is created
by somebody who is already here — a maintainer writing a profile, an owner
registering it, or [enrolment](#proving-possession) proving a key — and never by
itself. Registering a record for an owner the daemon does not know is refused
for the same reason: it would leave that record owned by nobody, which is
[wreckage by another route](#when-the-owner-is-gone).

**And there is no self-owned exception.** A name unknown to the daemon could
briefly still create itself, which is the one shape a legitimate caller needs —
a newcomer a realm has vouched for, whose [enrolment](#proving-possession) is
what writes it a record. Separating that caller from an ordinary one asking for
the same thing was not possible while the check ran on the door's stale answer;
now that it runs
[where the operation writes](02-access.md#what-a-call-carries), enrolment says
so for itself and nothing else reaches the shape.
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

**No registration, no access.** This removes an address and the credential
issued for it, not the process behind it. The name disappears from discovery,
new messages to it are refused, and its credential stops authenticating — the
record is what access hangs on, so with the record gone there is nothing left
to hold. Existing history remains history.

**Built: you cannot leave by stranding what you own.** Removing the record that
is a name's whole standing, while that name still owns others, would leave each
of them owned by somebody the daemon no longer knows — the same wreckage
[deletion](#when-the-owner-is-gone) exists to clean up, made by an ordinary
call. It is refused like a queue that is not empty, `409`, and the refusal names
what is in the way: there is somebody here to tell, and what to do about it —
remove them, or hand them to another owner — is theirs to choose. What counts is
what the removal costs *the name*, not who held the record: a record of yours
that a maintainer owns is no safer to strand. A **registered user keeps its
services** when its record goes, because the user is still somebody.

Nothing is held back for the name. It is reserved to nobody, survives no
restart, and whoever asks for it next gets it — its previous owner included,
with no priority. A person's own identity is the one exception, and not as a
reservation: their credential is how they call at all, so removing a record of
theirs does not take it.

**Built: and the name's group membership goes with it.** Because the name is
reclaimable by anybody, a membership left behind is not a dangling row — it is
inherited. Whoever registers the name next would arrive already in every group
the previous holder was in, and reach every record those groups allow. A
registered user is again the exception, and for the reason above: their record
going does not stop them being somebody, so it does not take their standing.

**The address and its credential go together, in one operation.** Whether the
name is a person, and so keeps its credential, is decided on the same facts the
removal is decided on — asked afterwards it was answered about a record that had
already gone, and in that gap the name could be claimed by somebody else, whose
credential was then the one dropped. If the credential store will not take the
write, the removal is abandoned whole: a record gone with its credential still
answering for it is precisely what
[the sweep](02-access.md#ownerless-credentials) exists to clean up, and a
credential dropped for a record that then stays is recoverable by asking for
another. A blocked read the name had elsewhere ends with it, because it was
being served on standing the name no longer has. A re-registered name starts with an empty inbox and
no configuration or subscriptions.

Protecting a name is a thing its owner asks for, not something a removal
buys: declaring a record [retired](../Plans/R1.1/records.md#down-and-retired)
holds its name, and is R1.1 work.
MVP reserved it, and paid a permanent credential per throwaway address for
protection it did not need.

<a id="pending-person-records"></a>

## Person records

**Built.** Administrator-vouched profiles are separate from service registrations.
Administrators see the user directory; ordinary callers see their own details.

**Built.** The directory keeps registered users, self-owned records and
credential-only identities visible, in separate sections. The daemon classifies
from its user profiles and records; neither blank fields nor name spelling
establishes what an identity is. Only registered users have user lifecycle
labels and controls. A profile with blank fields is still a user.

Each non-user entry explains why it is present and links to its record or
owned services where applicable. Eligible credentials have an explicit review
and removal action under the [cleanup rule](02-access.md#ownerless-credentials);
viewing the directory never removes anything. The page shows what the daemon
holds now, which includes names minted since it started; the sweep is what
takes them, and it runs at the next start.

Search, category filtering and pagination preserve the current view on return
from details and cleanup. Counts describe caller-visible entries, not the
whole credential store. The page does not fetch a directory again per avatar.

### Who may write a record

**Current implementation.** Only a daemon Administrator edits user fields,
never the person. An Administrator may edit below their own level, not a peer
Administrator or the owner. The owner may edit every level. A trusted person
record comes from Administrator vouching or successful enrolment. Profile and
membership changes are stored in the existing restart snapshot.

The [accepted model](01-owners-and-maintainers.md#clear-decisions) changes
self-edit permissions and name provenance; those changes remain pending.

### User lifecycle

**Built.** Authorized administrators add and edit users and apply these states
through the [dashboard](05-discovery.md#required-tabs), under the
[write hierarchy](#who-may-write-a-record).

| State | Effect |
|---|---|
| Active | The principal may authenticate and use its granted access |
| Paused | Tokens, existing browser sessions, local sockets and enrolment cannot grant bus access; new deliveries to the user's inbox are refused |
| Banned | The same access restriction; current code permits only the daemon owner to lift the ban |

The [accepted model](01-owners-and-maintainers.md#clear-decisions) also permits
Administrators to lift ordinary-user bans; that revision is pending.
An authorized Administrator may currently reactivate a paused ordinary user. The daemon owner
must remain active. State changes cancel the user's blocked reads and are retained by the existing
[snapshot contract](04-messaging.md#durability); queued work is retained subject
to its existing TTL. Credentials are not
rotated or deleted, so reactivation restores their use. Running services keep
their own identities and no process is stopped, but the state reaches what they
own: while it lasts, their services [refuse calls](#services-of-a-user-who-is-paused-or-banned).
Already delivered work cannot be recalled.

**That last part is pending.** The states themselves are built and enforced for
the person; propagating one to the services they own is not written yet, so a
paused owner's service still answers today.

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

Role terminology and intended authority are defined in the
[owner model](01-owners-and-maintainers.md#role-names-and-scopes); shared
management follows [explicit group assignments](01-owners-and-maintainers.md#shared-service-management).

**Built representation.** Flat groups support daemon administration and
service/topic maintenance. The protected `@maintainers` group currently holds
**Administrators**; its stored name and the code's older terminology have not
been renamed. The configured daemon owner is always a member. Only the daemon
owner changes that group, which cannot be deleted or have the owner removed.
An added Administrator becomes a registered user; removing the assignment keeps
the profile. Ordinary group membership does not create user profiles.

Daemon administration and record maintenance are separate scopes. Current
`manages` admits a record's owner, its own principal and members of its assigned
maintainers group. Both ownership transfer and changing that group assignment
are owner-only. Master access grants access rather than management. The
[daemon-owner override](01-owners-and-maintainers.md#daemon-owner-editing-any-service)
in the accepted model is not yet implemented.

**Implementation gap.** Administrators can currently edit any ordinary group,
including one assigned as a service's maintainers group, and thereby change
who manages that service. The accepted owner-controlled membership rule needs
[protection for this indirect path](01-owners-and-maintainers.md#remaining-membership-interaction).
Merely renaming the administrative role does not close it.

Current groups are flat sets of principal names, local to one daemon. Nested
groups and service-defined roles are accepted as
[new implementation work](01-owners-and-maintainers.md#added-implementation-scope);
expression syntax remains in the [R1 proposal](../Plans/R1/identity.md#groups-and-roles).
Owning a service alone does not grant daemon user/group administration.

**A group is not deleted, and it is given no states either.** Removing a name
other records point at silently changes what every one of them means, so
deletion is out; `@maintainers` could never be deleted and now none of them can.
But *inactive* and *banned* are not what replaces it.

**A group is not a principal.** It holds no credential, authenticates nothing
and does nothing on its own: it is a list of names that records point at.
Pausing or banning it would have to mean *its members stop counting*, and
**emptying the list means exactly that**, with no second concept and no new
verb. A group that confers nothing is a group with nobody in it, and that is
already sayable today.

So a retired group keeps its name and loses its members. Every record naming it
still means what it meant — an ACL still reads `@ops`, `@ops` is now nobody, and
putting a member back brings it back. Nothing about the record changed, which is
the whole reason deletion was refused.

`@maintainers` is the one that cannot be emptied, because the daemon owner stays
in it ([above](#groups-and-maintainers)) — the same rule that stops it being
deleted, from the same direction.

Names are available for service-owner assignment; membership lists are visible
to daemon administrators. Runtime start/stop controls remain
[R1 runner work](../Plans/R1/runner.md#what-the-runner-does).

### Owner control

**Built for supported daemon operations.** A user has **full control over services
and topics they own**, without daemon owner approval or Administrator standing. This includes
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

**A transfer hands the record to somebody who can answer for it now.** The new
owner must be a registered self-owned principal and must be able to act —
handing a record to a paused or banned name is the orphan by another route. Who
owns a record is also what decides who may be given its credential, and that is
asked where the credential is written rather than beforehand, because a record
can change hands ([getting a token](02-access.md#getting-a-token)).

A self-owned record can identify a person in the [people view](#person-records).
Person-profile writes use the [administrative hierarchy](#who-may-write-a-record);
service and topic changes use [record management authority](#groups-and-maintainers).

### When the owner is gone

**Built.** Every owner is somebody the daemon knows — a user, or
a name with a record of its own — and neither is deleted while it owns anything
([the levels are nested](#groups-and-maintainers), [user lifecycle](#user-lifecycle)).
That is **enforced rather than assumed**: registering a record owned by a name
the daemon holds nothing for but a credential is refused
([what a call carries](02-access.md#what-a-call-carries)), so no call leaves one
behind. It had not been, which is what [Q57](decisions.md#settled) settled.

So a service whose owner is not known is not a state to recover from — it is
wreckage from an older store, or from something that went wrong. **It is
deleted, at once, with everything that hung on it.**

| Goes | |
|---|---|
| the record | the name is free again, reserved to nobody, exactly as [unregistering](#unregistering) leaves it |
| its credential | no registration, no access |
| **its queues** | all of them, whatever is in them |
| its subscriptions and its group membership | a freed name is reclaimable, so a membership left behind would be [inherited](#unregistering) rather than merely stale |

**It happens at start, and it runs to a fixed point.** A start is the one moment
the daemon reads a store it did not write, and nothing is serving yet. One pass
is not enough, because deleting is what makes the next orphan: A owns B and B
owns C, all of them records, so taking A away is what leaves B unknown. The
sweep repeats until a pass finds nothing.

**One step, not walked to a person.** A self-owned record with no user behind it
is a principal this daemon supports — it authenticates, it may be handed a
record by transfer, and it may register records of its own. Asking instead
whether a chain of owners ends at a *user* would delete names the daemon had
just accepted. A cycle therefore survives, by the same rule rather than as an
exception: every member's owner has a record. No call can build one — transfer
demands a self-owned principal and re-registration keeps the owner it had — so a
cycle in a store came from outside the daemon.

**The backlog is not a reason to keep it.** Unregistering by hand refuses while
anything is queued or a reader waits, because a person can be told to drain it
first. There is nobody to tell here: the messages are addressed to something no
principal answers for, and holding them only means holding them forever. So this
deletion does not ask.

**There is nothing to reassign, and no name is held.** An earlier design kept
such a record alive, waiting for the daemon owner to give it an owner; that is
[superseded](decisions.md#superseded). Keeping it paid storage and a refusal
path for a case the nesting rule stops from arising — and afterwards *no such
name* is simply true, because there is no such name.

### Services of a user who is paused or banned

**Built.** A user who is paused or banned keeps their record, their
credential and everything they own ([user lifecycle](#user-lifecycle)) — and
**every service they own refuses calls** while that lasts. Lifting the state
brings them back.

**The daemon enforces it, not a label on a page.** The check is on the call, so
a service's own credential is refused too; a page that said *owner banned* while
the service still answered would be describing a rule nobody applied. It answers
`403 suspended`, the same code and reason as a caller who is suspended
themselves ([refusals](05-discovery.md#refusals)): one suspension seen from
either side, and nothing the caller can do about it either way.

**It follows the record's stated owner, and does not walk the chain.** A
service may own a service, so the boundary has to be said rather than assumed:
suspending the person at the top refuses the services they own, and not the
services *those* own in turn. *Every service they own* is the direct relation
the record states. Reaching further would be a larger rule and would need an
answer for a cycle, which nothing here has.

This deletes nothing and stops nothing. The user stays, their services stay,
their credentials are kept rather than rotated or revoked, and no process is
killed — banning somebody is not a way to reap their work, and a ban that
destroyed things could not be lifted. **Kept is not accepted**: while the state
lasts those credentials grant no access, theirs or their services'
([user lifecycle](#user-lifecycle)). They are there to work again when it is
lifted.
