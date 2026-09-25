# SQLite store

📌 **TL;DR:** 0.7.1 replaces the JSON snapshot and the token file with one
SQLite database, held exclusively, behind the store ports. A management write
commits only the entities it touched before it is answered, and a failed commit
publishes nothing; queue state is flushed in batches. Evidence for
[K.1.1 and K.1.3](../0.7.0-TODO.md#storage-and-identity).

## Result

| Area | Built |
|---|---|
| Driver | `modernc.org/sqlite`, pure Go, one connection per daemon |
| Durable | daemon Owner, account map, users, records, groups, credentials, queue contents and the four counters |
| Management write | staged in the live maps under the node lock with an undo log, committed as one transaction holding only the touched entities, then answered; a failed commit or a failed validation puts every staged entity back |
| Queue state | flushed every `-flush-every` (default one minute) and at a graceful stop; only queues whose counters moved are written |
| Removal | a removed record's queue rows go in the same transaction |
| Lock | `locking_mode=EXCLUSIVE` taken by a write at open and held until close; the supervisor reads the account map and closes before the bus child opens |
| Creation | only `agent-busd -init` or `-create`; setup runs `-init` as the daemon account and the unit never creates |
| Load | schema version and `quick_check` verified; message rows for a queue with no queue row refuse the load |
| Files | `agent-bus.db` mode 0600; the 0.6 `dump.json` and `token` are not read |

## Checks

| Check | Mutation that fails it |
|---|---|
| `store/sqlite` TestSecondOpenerIsRefused, across two processes | removing `locking_mode(EXCLUSIVE)`: the second opener got the database |
| `store/sqlite` TestMissingDatabaseIsNotCreatedUnasked | opening with create for a missing file |
| `store/sqlite` TestFailedCommitWritesNothing | a trigger aborts the second table's insert; the first table's row must not survive |
| `store/sqlite` TestIncompatibleSchemaIsRefused, TestDamagedDatabaseIsRefused | accepting another `user_version`, or a non-database file |
| `core` TestFailedCommitPublishesNothing | dropping the rollback: a ban, an ACL edit, a removal and a group edit whose commits failed took effect |
| `core` TestOneEditCommitsOneRecord | committing more than the edited record |
| `core` TestAdministrativeSuccessHasAlreadyPersisted | a restart recovers only what was committed, for twelve kinds of administrative change |
| `cmd/agent-busd` TestSupervisorUsesEstablishedAccountMapInsteadOfSetupFlags | a missing database taken for a first run without `-create` |
| smoke *a restart is not a loss* | a SIGKILLed bus reports the unclean stop, keeps a secret written before the kill, and delivers nothing twice |
| smoke *a start clears out the records whose owner it does not know* | the orphan sweep committed before the daemon was killed outright |

The in-place mutations above were run against the named tests; the lock
mutation was seen to fail before the change was kept.

## Not claimed

Detecting a lock lost after start is not required by the
[owner's narrowing](../../../docs/decisions.md#settled). A held database cannot
be made to refuse a write from outside the process, so the smoke no longer
checks a credential the store could not keep; `internal/auth`
TestMintHandsOutNothingItCouldNotWrite covers it in-process.
