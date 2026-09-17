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
