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
* Maintainer service role that can only be assigned by owner

## Service Users can

* access service

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

**Owner answer:** _Pending._

### Daemon ownership transfer

**Codex:** This model grants transfer. Current code assigns the daemon owner at
startup through [`api.New`](../src/internal/api/server.go) and
[`Administrator`](../src/internal/core/manage.go); no runtime transfer operation
exists. Existing docs describe the configured owner, not a transfer procedure.
Changing startup configuration is not an implemented ownership-transfer workflow.

**Decision needed:** Build runtime transfer, or define ownership changes as an installation operation.

**Owner answer:** _Pending._

### Users editing themselves

**Codex:** If “edit itself” means profile fields, this conflicts with
[who may write a record](01-identity.md#who-may-write-a-record), which explicitly
forbids self-vouching. [`mayEditUser`](../src/internal/core/users.go) rejects
ordinary-user and maintainer self-edits; only the daemon owner can edit their own
profile. Editing an owned service is already a different, permitted operation.

**Decision needed:** Which self-editable fields or operations should this grant.

**Owner answer:** _Pending._

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

**Owner answer:** _Pending._

### Service-defined roles

**Codex:** This model allows maintainers to assign/revoke roles other than
maintainer. Current implementation has ownership, an assigned maintainers group
and an access list; service-defined roles are [proposed R1 work](../Plans/R1/identity.md#groups-and-roles),
explicitly [unbuilt in the current ACL contract](01-identity.md#acl).

**Decision needed:** Whether this introduces service-defined roles now, or only names existing access and management permissions.

**Owner answer:** _Pending._

## Existing docs disagreeing with implementation

⁉️ These conflicts are inside the existing documentation, independently of which model is chosen above.

### Changing the assigned maintainers group

**Codex:** [Groups and maintainers](01-identity.md#groups-and-maintainers) says
“only ownership transfer is the owner's alone”; its authority table likewise
excludes only ownership. Code in [`Manage`](../src/internal/core/manage.go)
reserves **both** ownership transfer and changing the assigned maintainers group
to the record owner. This model agrees with code on the latter restriction.

**Decision needed:** Confirm the owner-only restriction so the broader existing prose can be corrected.

**Owner answer:** _Pending._

### Does “user” require a user profile

**Codex:** The existing authority table says a service owner is “one user”, but
[the ownership contract](01-identity.md#when-the-owner-is-gone) and
[`mayOwn`](../src/internal/core/users.go) also permit an active record-backed
principal without a user profile to own services. Likewise, assigning an ordinary
service-maintainers group does not create user profiles for its members. The
automatic owner → maintainer → registered-user nesting is implemented for daemon
administration, not for every service principal.

**Decision needed:** Does “User” in this model mean any admitted principal with service access, or specifically a registered user profile at every layer.

**Owner answer:** _Pending._

## Qualifications the model leaves open

❓ These are existing restrictions or mechanisms the model does not explicitly accept or replace.

### Reactivation after a ban

**Codex:** [User lifecycle](01-identity.md#user-lifecycle) and
[`SetUser` / `SetUserState`](../src/internal/core/users.go) let daemon maintainers
reactivate paused regular users, but **only the daemon owner may lift a ban**.
The model's unqualified “Activate” does not state that exception.

**Owner answer:** _Pending — retain or replace the owner-only unban rule._

### Service transfer restrictions

**Codex:** [Owner control](01-identity.md#owner-control) and
[`Manage`](../src/internal/core/manage.go) require an active recipient with a
self-owned record. A self-owned identity cannot itself be transferred. The
model grants transfer without saying whether these restrictions remain.

**Owner answer:** _Pending — retain or change these recipient and record restrictions._

### Owner setup at startup

**Codex:** Startup requires a valid owner name, but
[`defaultOwner`](../src/cmd/agent-busd/main.go) can derive one from the OS account
and hostname. The setup script need not have run. The model's setup-script text
is still a placeholder.

**Owner answer:** _Pending — allow the derived default or require explicit owner setup._

## Already aligned

Daemon owner membership in the protected maintainers group; daemon maintainers
administering ordinary users but not peer maintainers or the owner; users
registering services/channels they own; service maintainers editing their assigned
service and its ACL and inheriting access through `may` → `manages`. The group
membership qualification above still applies.
