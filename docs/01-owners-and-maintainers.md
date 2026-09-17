# Owner > Maintainer > User



^^ generic logic supported on all layers; for daemon and for services

Owner is also maintainer and user

Maintainer is always a user

So Owner can do any maintainer stuff and user stuff too



# Daemon

Always have owner; will not start unless it have one; use ...our-setup-script... to setup

## Daemon Owner can

* Assign/Revoke maintainer role

* Activate/Deactivate/Ban maintainers

* Transfer ownership (assign new owner)

* Edit any service; including its ACL/roles

* Edit any groups

  

## Daemon Maintainers can

* Add regular users
* Activate/Deactivate/Ban regular users
* Edit groups (except maintainers)

### Can not 

* edit owners or other maintainers in any way
* edit service (must be at least service maintainers)



## Daemon Users can

* register service/channel - they'll became `service owner`
* edit itself



# Service

## Service Owners can

* Add/Remove maintainers (assign/revoke maintainer role)
* Transfer ownership (assign new owner)

## Service Maintainers can

* Edit service 
* Edit ACL 
  * add/remove service users/groups (allow existing user to access service)
  * can not remove maintainers
  * assign/revoke roles (except maintainer)
* Service ACL
  * list of users, groups, or **other services** that can use this service
  * group can contain other groups, users & services

* Maintainer service role that can only be assigned by owner

## Service Users can

* access service

# Channels

A channel is a service-like entity **without an actual service process behind
it**. It has a name, owner, maintainers, access list and settings; the daemon
itself provides its queue or pub/sub behavior. In the current implementation
this is a topic record, not a separately running service.

It follows the same Owner > Maintainer > User authority model as a service;
it is not owned collectively by its subscribers. This section states the intended model;
the daemon-owner override and owner-controlled maintainer membership still
need implementation.

## Channel Owner

* The registered user who creates the channel becomes its owner.
* There is one owner; joining, publishing or reading does not confer ownership.
* The owner can do everything a channel maintainer or user can do.
* The owner assigns/revokes channel maintainers and can transfer ownership.
* The daemon owner retains the root-like override, including changing the channel owner.

## Channel Maintainers

* Edit the channel's definition, settings and access list.
* Cannot transfer ownership or assign/revoke channel maintainers.
* Daemon-maintainer status alone grants no channel-management authority.
* Maintainer membership stays owner-controlled, just as for services.

## Channel Users and Subscribers

* Access is granted to users, groups or other services through the channel ACL.
* Queue mode: authorized callers send messages and consume from the shared queue.
* Pub/sub mode: authorized callers publish and manage their own subscription;
  copies go to subscriber inboxes.
* Owning the channel does not make its owner the owner of subscriber services
  or their inboxes. Channel access does not grant access to other subscribers' inboxes.

See [topic behavior](03-services-and-topics.md#topics) for delivery semantics. The
service-transfer recipient restrictions discussed below also need to be
settled for channels.

---

# Review comments for owner answers

Compared with current source and existing contracts on 2026-09-16. These are
comparison notes, not settled policy or instructions to change implementation.
Answer under each comment; no task or question IDs have been allocated.
Markers follow [Glyphs](https://parf.dev/ai-skills/Glyphs.md).

## Conflicts between this model and existing policy or behavior

⁉️ The sources below disagree or implement different authority boundaries. Owner answers will settle which rule should govern.

### Daemon owner editing any service

**Codex:** This model grants it. Existing [record authority](01-identity.md#groups-and-maintainers)
separates daemon administration from service management. In
[`manages`](../src/internal/core/manage.go), only the record owner, record's own
principal, or a member of its assigned maintainers group may manage it; there is
no daemon-owner override. Master access is access, not management.

**Decision needed:** Add the daemon-owner override, or retain separate scopes.

**Owner answer:** YES - daemon owner = root like authority; it can edit anything; including service owner change; (ok to require daemon/service restart to take effect)

### Daemon ownership transfer

**Codex:** This model grants transfer. Current code assigns the daemon owner at
startup through [`api.New`](../src/internal/api/server.go) and
[`Administrator`](../src/internal/core/manage.go); no runtime transfer operation
exists. Existing docs describe the configured owner, not a transfer procedure.
Changing startup configuration is not an implemented ownership-transfer workflow.

**Decision needed:** Build runtime transfer, or define ownership changes as an installation operation.

**Owner answer:** _Yes . (ok to require daemon/service restart to take effect)_

### Users editing themselves

**Codex:** If “edit itself” means profile fields, this conflicts with
[who may write a record](01-identity.md#who-may-write-a-record), which explicitly
forbids self-vouching. [`mayEditUser`](../src/internal/core/users.go) rejects
ordinary-user and maintainer self-edits; only the daemon owner can edit their own
profile. Editing an owned service is already a different, permitted operation.

**Decision needed:** Which self-editable fields or operations should this grant.

**Owner answer:** YES - user can edit their profile fields; (FORBID changing - userName and FullName)

### Who actually grants service-maintainer authority

**Codex:** This model makes the service owner responsible for adding/removing
maintainers. Today the owner selects one existing group through
[`Manage`](../src/internal/core/manage.go), but service ownership alone cannot
edit its members. [Daemon administrators control ordinary group membership](01-identity.md#groups-and-maintainers).

A daemon maintainer can add themselves to an ordinary group already assigned as
a service's maintainers group, then manage that service. `SetGroup` permits the
membership edit and `manages` grants the resulting authority. Thus the **group
assignment is owner-only; effective membership is not**. The model's own grants
of ordinary-group editing and owner-only service-maintainer assignment also need
to account for this interaction.

**Decision needed:** Does the owner delegate membership control to group administrators, or must service-maintainer membership itself remain owner-controlled.

**Owner answer:** service-maintainer membership itself remain owner-controlled

### Service-defined roles

**Codex:** This model allows maintainers to assign/revoke roles other than
maintainer. Current implementation has ownership, an assigned maintainers group
and an access list; service-defined roles are [proposed R1 work](../Plans/R1/identity.md#groups-and-roles),
explicitly [unbuilt in the current ACL contract](01-identity.md#acl).

**Decision needed:** Whether this introduces service-defined roles now, or only names existing access and management permissions.

**Owner answer:** lets do it now, it cost us almost nothing; because daemon do not care about roles (except for maintainer)

## Existing docs disagreeing with implementation

⁉️ These conflicts are inside the existing documentation, independently of which model is chosen above.

### Changing the assigned maintainers group

**Codex:** [Groups and maintainers](01-identity.md#groups-and-maintainers) says
“only ownership transfer is the owner's alone”; its authority table likewise
excludes only ownership. Code in [`Manage`](../src/internal/core/manage.go)
reserves **both** ownership transfer and changing the assigned maintainers group
to the record owner. This model agrees with code on the latter restriction.

**Decision needed:** Confirm the owner-only restriction so the broader existing prose can be corrected.

**Owner answer:** ownership transfer is the owner's alone.

### Does “user” require a user profile

**Codex:** The existing authority table says a service owner is “one user”, but
[the ownership contract](01-identity.md#when-the-owner-is-gone) and
[`mayOwn`](../src/internal/core/users.go) also permit an active record-backed
principal without a user profile to own services. Likewise, assigning an ordinary
service-maintainers group does not create user profiles for its members. The
automatic owner → maintainer → registered-user nesting is implemented for daemon
administration, not for every service principal.

**Decision needed:** Does “User” in this model mean any admitted principal with service access, or specifically a registered user profile at every layer.

**Owner answer:** registered user; in service AVL we can list other services too

## Qualifications the model leaves open

❓ These are existing restrictions or mechanisms the model does not explicitly accept or replace.

### Reactivation after a ban

**Codex:** [User lifecycle](01-identity.md#user-lifecycle) and
[`SetUser` / `SetUserState`](../src/internal/core/users.go) let daemon maintainers
reactivate paused regular users, but **only the daemon owner may lift a ban**.
The model's unqualified “Activate” does not state that exception.

**Owner answer:** daemon maintainers can reactivate & unban (makes sense because they can add new users)

### Service transfer restrictions

**Codex:** [Owner control](01-identity.md#owner-control) and
[`Manage`](../src/internal/core/manage.go) require an active recipient with a
self-owned record. A self-owned identity cannot itself be transferred. The
model grants transfer without saying whether these restrictions remain.

**Owner answer: what is self-owned identity? how we can get one??

**Codex reply:** It is a registry record whose name and owner are the same:
`alice@host` owned by `alice@host`. It represents Alice herself, unlike
`worker@host` owned by `alice@host`, which represents Alice's service.
Creating an ordinary user through `SetUser` creates that self-owned record
alongside the profile when no record exists; successful signed enrolment also
creates one. It is not a separate kind of account you must manually request.
Adding someone only to daemon `@maintainers` creates a profile but does not
itself guarantee that record exists.

Today transfer requires the recipient to have that self-owned record, and
refuses to transfer a self-owned record itself. With your registered-user rule,
my recommendation is to authorize a service/channel transfer by the recipient's
active user profile, without making this storage detail a user-facing condition;
keep transferring a service distinct from transferring a person's identity.
This is a recommendation, not an answered decision.

### Owner setup at startup

**Codex:** Startup requires a valid owner name, but
[`defaultOwner`](../src/cmd/agent-busd/main.go) can derive one from the OS account
and hostname. The setup script need not have run. The model's setup-script text
is still a placeholder.

**Owner answer:**  yes USER who executed setup script is an owner; should NOT start otherwise 

## Already aligned

Daemon owner membership in the protected maintainers group; daemon maintainers
administering ordinary users but not peer maintainers or the owner; users
registering services/channels they own; service maintainers editing their assigned
service and its ACL and inheriting access through `may` → `manages`. The group
membership qualification above still applies.

# Review of owner answers

The answers set new policy; earlier comparison notes describe the implementation
they replace. Channels follow the same authority rules as services, with the
daemon providing the behavior instead of a service process.

## Clear decisions

* Daemon owner is the root authority, including service/channel ownership changes.
  “Owner-only” at record level is subject to this explicit daemon-owner override.
* Service/channel maintainer membership is controlled by its owner.
* Daemon maintainers can lift bans on ordinary users, still not on peer maintainers
  or the daemon owner.
* Users may edit profile information except their identity name and full name.
  Current field names are `Name` and `PersonName`; lifecycle state and authority
  are administrative controls, not self-editable profile information.
* Ownership must be explicitly established through setup; startup must not invent
  an owner from the account running the daemon. A legitimate ownership transfer
  must update the configured authority that startup uses.

## Remaining membership interaction

⁉️ Owner-controlled service/channel maintainers conflict with unrestricted daemon-maintainer edits of ordinary groups when the same group grants maintenance authority.

Nested groups make that indirect path longer, not different: editing a subgroup
can change effective maintainers too. Protecting only the top-level group is
insufficient. Shared groups also raise whose approval applies when two different
owners assign the same group.

**Owner answer:** _Pending — should maintenance authority use owner-controlled membership separate from ordinary access groups, or can groups grant it subject to protection of all membership changes that affect it._

## Added implementation scope

Group nesting is new work: current `member` checks a flat list and `SetGroup`
normalizes members as principal names. Supporting groups inside groups needs
resolution and defined handling of cycles; it is not just accepting another ACL
entry. Users and services as direct ACL principals are already supported.

Service-defined roles may remain uninterpreted by the daemon, but assigning,
storing and returning the labels still needs implementation. The existing
[R1 proposal](../Plans/R1/identity.md#groups-and-roles) keeps role meanings in the
service while the authority layer stores/resolves labels. “Opaque meaning” does
not require hiding roles inside private service configuration. The reserved
maintainer role must retain its owner-only assignment rule.

## Transfer recipient

❓ The self-owned-identity explanation above answers the terminology question; the recipient restriction itself still needs an answer.

**Owner answer:** _Pending — may any active registered user receive service/channel ownership, regardless of whether they currently have a self-owned registry record._
