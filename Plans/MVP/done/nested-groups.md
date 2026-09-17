# Nested groups

📌 **TL;DR:** 0.5.57 makes ordinary group membership transitive for ACL and
Maintainer authority while keeping Administrator membership direct-only.

## Result

An ordinary group may contain principal names and other group names. Membership
is graph reachability through the stored direct entries: cycles terminate,
unknown groups are inert, and populating a referenced group later makes its
members effective. ACL checks, service Maintainer checks and user-directory
group lists use the same resolver. Removing a nested membership rechecks and
ends blocked reads that no longer have access.

`@administrators` remains a protected direct-membership list. The Owner cannot
place a subgroup in it, and a snapshot containing one fails startup before
serving. An ordinary group may include `@administrators`; that gives its direct
Administrators the ordinary group's resource grant without granting
Administrator standing through another path.

The existing administrative-name migration now rewrites nested group edges as
well as record ACL and Maintainer references. Collision preservation continues
to move the ordinary destination group and every reference to its `-legacy`
name without promoting its members.

## Checks

Core tests cover multi-level ACL and Maintainer grants, user and service
principals, effective directory membership, unknown-then-populated references,
self and two-group cycles, nested blocked-read revocation, the protected-group
boundary, persistence, damaged protected restore and both migration rename
directions.

The final mutation set caught 15/15 named changes: refusing group-valued
members, ignoring nested edges, losing cycle exits, activating unknown groups,
nesting or resolving protected Administrator authority, accepting a damaged
snapshot, manufacturing a group-shaped user, hiding effective directory
membership, retaining revoked readers, leaving migrated edges stale, reusing a
dangling nested name during collision migration, excluding service principals,
and bypassing the shared resolver in either ACL or Maintainer checks.

The first mutation run's manufactured-user change survived because its test
called a public helper that rejects group syntax before consulting the user
map. That run receives no credit. The corrected assertion inspects the restored
invariant directly and catches the same mutation. Fast smoke passed 476 checks;
the byte-frozen slow run passed 597 checks with `go vet` and the race run green.
All 190 source hashes remained unchanged. The tracked and copied smoke scripts
both had SHA-256
`017c827a9b85242d452cc48de6c1d82a997b688e9a51ac8bec85846024fad05a`.
The documentation check resolved 2,818 local links with no error before this
measured line was inserted.

## Upgrade and limits

Earlier flat snapshots could retain `@`-prefixed ordinary-group entries as inert
strings. They become membership edges in this release; the setup guide tells
operators to review them before upgrading. Group resolution walks the daemon's
group graph under the existing operation lock; no cache or second authority
path is introduced.

Personal services still reject group ACL entries and Maintainer assignments.
Service-defined role storage remains separate pending work.
