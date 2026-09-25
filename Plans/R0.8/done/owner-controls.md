# Required dashboard

## Scope

Implemented the owner's 2026-09-13 [required dashboard](../../../docs/05-discovery.md#required-tabs)
and [service ownership clarification](../../../docs/01-identity-and-roles.md#services):
F.6, F.6.1 and F.7–F.11. Installed resource-confinement acceptance remains open.

| Area | Implementation |
|---|---|
| Service and channel pages | My/all and active/inactive filters, record details and daemon-derived control permissions |
| Owner operations | Definition, configuration replacement, ACL, availability, maintainer assignment, transfer and idle deletion; ordinary users need no daemon administration role |
| Flat groups | Daemon administrators edit ordinary groups; only the owner edits the protected maintainers group; service owners assign existing groups to their records |
| Users | Filtered directory, profiles and local avatars; normalized unique email/GitHub identifiers; profile and lifecycle changes enforce the owner/maintainer hierarchy across API, sessions and local sockets |
| Activity | Bus-owned bounded minute samples, filtered graphs and accessible sample values; independent of page visits |
| Channels | Register pub/sub or queue topics, inspect and change settings, subscribe/unsubscribe the caller, and remove subscriptions as a channel manager |
| Availability and access | Disable retains the queue and refuses delivery/read; access removal cancels affected blocked reads; registration preserves assigned maintainers and availability |
| Restart | Registry policy, group membership and backlog travel in the existing snapshot |
| Browser boundary | Forms forward the visitor's session; exact-origin check, no-store responses and no rendering of credentials or stored private configuration |

Optional managed runtime controls and advanced dashboard features remain
[R1](../../R1/discovery.md#dashboard-extensions). State durability follows the
existing [snapshot contract](../../../docs/04-messaging.md#durability); this change
does not add a synchronous persistence transaction per administrative request.

## Verification

`TMPDIR="$PWD/tmp/owner-control" PORT=18041 bash src/smoke.sh --slow` passed
529 checks with zero failures, including vet, race tests, TypeScript checks and
existing daemon/CLI/MCP integration exercises. New named Go checks:

| Check | Evidence |
|---|---|
| `TestUserAdministrationAndLifecycle` | Allowed administration; peer/owner/self-edit refusal; duplicate identifiers; protected membership; pause/ban, sessions, local sockets and restart |
| `TestActivityIsBoundedAndFiltered` | Known accepted/dequeued/dropped/expired/refused counts, hidden-service filtering and history bound |
| `TestRequiredDashboardTabs` | All required pages and live form round trips, profiles, lifecycle, local avatars, channel creation/subscriptions and inline graphs |
| `TestChannelManagersCanRemoveButStrangersCannot` | Channel subscription removal requires record management authority |
| `TestOwnerControlThroughAPI` | Ordinary-owner success; unrelated caller/master refusal; maintainer limits; disable/queue preservation; restart; transfer and token-acquisition authority |
| `TestPolicyChangesCancelBlockedReaders` | Disable, ACL changes and removed group membership stop existing readers |
| `TestManagementRejectsPartialInvalidChanges` | Invalid edits leave the entire record unchanged |
| `TestDashboardOwnerControls` | Real HTTP front end to API, authenticated sessions, forms, filters, maintainer boundaries and secret omission |
| `TestSameOrigin` | Scheme, host, port, credentials in origin and cross-site request rejection |
| `TestCanceledReaderCannotRecreateRemovedInbox` | Reproduced the disable/unregister/cancel race before fixing it; cleanup does not recreate deleted state |

Eighteen mutation checks run against an isolated source copy with `go test -race`.
They cover owner and transfer checks, disabled delivery, profile and group
restoration, reader revocation, form origin enforcement, removed owner/user
controls, user access and hierarchy, protected membership, ban release, identifier
normalization and uniqueness, and graph filtering/bounds. All fail their named
checks when broken. The button mutation first exposed a weak
assertion that matched the heading; the strengthened assertion detects the missing
control. The cancellation cleanup also has a recorded failing-before/passing-after
regression check.

Local command logs and the mutation driver are under ignored
`tmp/owner-control/` and `tmp/scripts/owner-control-mutations.py`. No live daemon
was replaced or restarted by this work.
