# Administrator naming

## Scope

The daemon administrative role is Administrator; service/channel Maintainer
assignments remain separate. The [current group contract](../../../docs/01-identity.md#groups-and-maintainers)
and [upgrade guidance](../../../docs/09-setup.md#administrator-name-migration)
own the behavior. Root overrides, profile permissions and protected effective
maintenance membership remain [pending work](../TODO.md#authority-model).

## Migration

Restore renames the old administrative group before rebuilding administrative
profiles. A colliding ordinary group is preserved under an unused name; its
record references follow it without promoting its members. Collision selection
also avoids dangling record references, which must not become new grants.
The caller's snapshot is unchanged. Rejecting recreation of the former name
keeps a serialized second restore from repeating the migration.

The public user role, dashboard and administration CLI moved together. The
service `maintainers` property did not change. Older clients must be upgraded;
rollback limits are recorded in the upgrade guidance.

## Checks

- All Go packages passed; OpenCode independently reviewed the source.
- Migration fixtures cover ordinary-name collision, no collision, serialized
  second restore, queue preservation and non-mutation of the input snapshot.
- Management grants and ACL-only grants are checked independently, including
  denial for the opposite group and unresolved-reference controls.
- API checks assert the administrative flag on actual user rows and reject the
  old group name; web checks assert directory-row and detail labels.
- Eight targeted overlay mutations failed named assertions, not compilation.
  Each runs its named test in the affected package; these are not full smoke
  runs per mutation. Local replay and logs: `tmp/administrator-43/mutate.py`
  and `tmp/administrator-43/mutations.log`.

| Mutation | Named check |
|---|---|
| Skip migration | TestAdministratorMigrationPreservesGrantsWithoutPromotion |
| Promote members of the ordinary colliding group | TestAdministratorMigrationPreservesGrantsWithoutPromotion |
| Lose management-reference rewrite | TestAdministratorMigrationPreservesGrantsWithoutPromotion |
| Lose ACL-reference rewrite | TestAdministratorMigrationPreservesGrantsWithoutPromotion |
| Permit recreation of the legacy trigger | TestRetiredAdministrativeNameCannotBeRecreated |
| Modify source snapshot records | TestAdministratorMigrationPreservesGrantsWithoutPromotion |
| Restore old public role field | TestAdministratorRoleUsesItsOwnPublicName |
| Restore old web label | TestDirectoryShowsJunkWithoutCallingItUsers |

Full frozen smoke: **585 passed, 0 failed**, including vet and race checks
(`tmp/administrator-43/slow.log`, result at line 926). The tracked script and
executed `src/administrator-smoke.local.sh` copy both had SHA-256
`29b89a6a2055c92df7ac8dd2be986e4e53aa8d01aba7dd2e56af5f16b4a0e8e6`.
The full run is separate from the eight targeted mutation runs.

## Live verification and harness correction

The stamped build (`parf@parf.us 2026-09-16 22:28:26`) was deployed. Both existing
Administrators, all four users and all eight records survived; record ownership,
ACL and Maintainer assignments matched the preflight reading. Public identity
and the sign-in page report 0.5.43. OpenCode confirmed same-session reconnection.

The first live group comparison failed: a group absent at preflight appeared
under the collision-preservation name. Investigation traced it to the smoke
admin helper: `AGENT_BUS_HOME` isolated key files, but without `AGENT_BUS_ADDR`
the provisioning calls discovered the live owner socket. This run had created
an ordinary group at the new reserved name while the old daemon was running.
Migration preserved that group separately, as designed. It had no record
references and was retired by emptying after verification.

The 585/0 run above is a measured pass, **not evidence of complete test
isolation**. The helper now binds the disposable daemon's account socket.
Three checks read the fixture daemon to establish ordinary-user provisioning,
Administrator provisioning and preservation of its owner's membership. A
stubbed executable probe exercises the actual helper declaration: the bound
case passes; removing the address binding fails `admin command escaped fixture
address`, without sending the mutant to any daemon.

Earlier `plain@srv1`, `chief@srv1` and `piped@srv1` profiles/records, including
chief's administrative standing, already existed in preflight. They were not
removed as part of this rename; their cleanup needs a separate provenance and
intent review. An unchanged live group map alone would not prove isolation:
reprovisioning an existing identity can be a no-op.

The first isolated rerun finished **587 passed, 1 failed**. Its new profile
assertion found that the old `plain@srv1` fixture name was already registered
as a service earlier in the suite. The admin section now uses distinct
`smoke-admin-*` names. That failed run is retained as
`tmp/administrator-43/slow-isolated.log`; it is not counted as green.

Final isolated full smoke: **588 passed, 0 failed**, including vet and race
(`tmp/administrator-43/slow-final.log`, result at line 930). Tracked script and
executed `src/administrator-final-smoke.local.sh` both had SHA-256
`2af6444ffc4447f62059757f92d1b99ad6bbd0c624df5038aa7e07e8e541669a`.
All three fixture provisioning checks passed. The live group map also remained
byte-identical to the reading taken before the corrected admin section ran.
Runtime source and the deployed stamped build were unchanged by this follow-up.
