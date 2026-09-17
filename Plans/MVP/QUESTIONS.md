# MVP questions

## Open questions

Only unresolved choices appear below. Previously numbered questions are settled
or deferred in the [decision index](../../docs/decisions.md#settled); their IDs
remain reserved.

## Service access

### Service reading its own inbox

❓ The [empty ACL rule](../../docs/02-access.md#acl) names Owner and assigned
Maintainers. A service's own principal is normally distinct from its owner:
`svc@host` consumes with its own credential, while `alice@host` owns it. Today
that principal has implicit access. Decide whether it keeps access to its own
inbox under the new default; do not silently add an exception or break service
consumption. Acceptance must exercise this case separately.

## Authority model

### Effective Maintainer membership

⁉️ The [owner-control requirement](../../docs/01-identity-and-roles.md#services)
needs protection when an Administrator can edit a group granting maintenance
authority. Nested subgroup edits can also change effective membership. Shared
groups can be assigned by several resource owners.

**Owner answer:** _Pending — separate owner-controlled maintenance membership
from ordinary access groups, or protect every group change affecting it; define
whose approval applies to a group shared by different owners._

### Transfer recipient

❓ May any active registered user receive service/channel ownership, regardless
of whether they currently have a self-owned registry record.

A self-owned record has the same name and owner, for example `alice@host` owned
by `alice@host`. Ordinary user creation and enrolment normally create one;
Administrator membership creates a profile without guaranteeing that record.
Current transfer requires the record. Transferring a service is distinct from
transferring a person's own identity.

**Owner answer:** _Pending. Recommendation: authorize by the recipient's active
user profile, without exposing the self-owned-record requirement to the user._
