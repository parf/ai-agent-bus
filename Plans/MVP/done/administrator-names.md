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
