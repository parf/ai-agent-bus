# Owners, administrators and maintainers

Current authority specification. Implementation tracking is in
[TODO](../Plans/MVP/TODO.md#authority-model); unresolved choices are in
[QUESTIONS](../Plans/MVP/QUESTIONS.md#authority-model).

## Role names and scopes

| Scope | Highest authority | Delegated management | Basic access |
|---|---|---|---|
| Daemon | Owner | Administrator | User |
| Service or channel | Owner | Maintainer | Member |

**Administrator** manages daemon users and groups. **Maintainer** manages an
explicitly assigned service or channel. Administrative standing does not
implicitly grant service or channel maintenance.

A **User** is a registered person. A resource **Member** has access and may be a
user or another service. Higher roles include the lower role's permissions
within their scope. The daemon owner's root-like override is explicit; it is
not inherited by Administrators.

## Daemon owner

The daemon owner has root-like authority over the node:

* Assign and revoke Administrators; edit, activate, pause and ban them.
* Manage users and all groups.
* Edit any service or channel, including its ACL, Maintainers and owner.
* Transfer daemon ownership to another user.
* Assigned via `agent-bus-setup`.

## Daemon Administrators

Administrators can add and manage ordinary users, including activation,
reactivation, pausing, banning and unbanning. They manage ordinary groups.

They cannot edit the daemon owner or peer Administrators, or grant themselves
those positions. Protected maintenance membership follows the
[owner-control rule](#services).

Administrator standing alone does not permit service/channel management; an
explicit Maintainer assignment is required.

## Users and profile editing

Users can register services and channels and become their owners. They may edit
their email, but not their username, person name or GitHub name. Person names come
from Linux passwd, GitHub or an Administrator. Account state and roles are
managed by Administrators or the Owner according to their authority.

## Services

| Role | Authority |
|---|---|
| Owner | All Maintainer/Member permissions; assign and revoke Maintainers; transfer ownership |
| Maintainer | Edit service settings and ACL; assign/revoke service-defined roles except Maintainer |
| Member | Use the service |

Only the service owner controls Maintainer membership and ownership transfer,
subject to the daemon-owner override. Maintainers cannot remove or replace
other Maintainers through ACL editing.

ACLs can name users, groups and other services. Groups can contain users,
services and other groups. Service-defined role meanings belong to the service;
the daemon stores/resolves the labels without interpreting those meanings.
Maintainer is the reserved management role enforced by the daemon.

## Shared service management

For management across services, create a group and explicitly assign it as
**Service Maintainer** on each relevant service, or **Channel Maintainer** on
each relevant channel. “Global” describes the coverage of those assignments;
it is not a special group name or automatic access to existing or future
records. Administrator membership alone grants none of these assignments.

Assignments belong to each resource's owner, subject to the daemon-owner
override. Shared groups must preserve
[owner-controlled effective membership](#services).

## Channels

A channel is a service-like entity **without an actual service process behind
it**. It has a name, owner, Maintainers, ACL and settings. The daemon provides
its queue or pub/sub behavior.

The creating user owns it. The [service authority rules](#services) apply:
Owner > Maintainer > Member, with the daemon-owner override. Joining,
publishing or reading does not confer ownership.

* Queue mode: authorized callers send to and consume from the shared queue.
* Pub/sub mode: authorized callers publish and manage their own subscription;
  copies land in subscriber inboxes.
* Owning a channel grants no ownership of subscriber services or their inboxes.
  Channel access does not grant access to other subscribers' inboxes.

[Topic behavior](03-services-and-topics.md#topics) owns delivery semantics.

## Display and ACL input

Use the [web and CLI identity labels](05-discovery.md#identity-labels-in-web-and-cli)
for display. [ACL editing](05-discovery.md#acl-editing) uses the project's
plain-text syntax, not Unicode display glyphs.
