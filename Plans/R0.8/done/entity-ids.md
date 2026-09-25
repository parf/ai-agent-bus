# Entity IDs

📌 **TL;DR:** 0.7.3 gives every record and user a stable internal ID, persisted
beside it and never on an answer. IDs only grow: the database keeps each
high-water mark, so an ID is never reused, not even after a restart that no
longer has the removed entity. Evidence for [K.2](../0.7.0-TODO.md#storage-and-identity).

## Result

| Area | Built |
|---|---|
| IDs | `registry_id` on every record and `user_id` on every user; an edit keeps them, a new entity takes the next |
| Marks | `next_record_id` and `next_user_id` are committed with the change that moves them, and a failed commit gives them back |
| Exhaustion | past the last ID a new entity is refused with `ErrExhausted`; no wrap |
| Lifecycle | a record's `created_at` beside its `at`; a user's `created_at` and `updated_at`; all the system's |
| Indexes | ID-to-name for records and users, rebuilt at every load; two stored entities claiming one ID are damage — the second is ignored and reported as an alert |
| Privacy | the IDs are `json:"-"` on every answer |

## Checks

| Check | Mutation that fails it |
|---|---|
| `core` TestRecordIDsAreStableAndNeverReused | not committing the high-water mark: a record made after a restart reused the removed record's ID |
| `core` TestRecordIDsAreStableAndNeverReused | giving an edited record a new ID |
| `core` TestIDIndexIsRebuiltOnLoad | skipping the index rebuild at load |
| `core` TestIDExhaustionRefusesRatherThanWraps | handing out the last ID and wrapping |
| `core` TestUserIDsAreStable, TestIDsAreNotPublic | a user's ID survives an edit and a restart; no ID leaves the daemon |
| `store/sqlite` TestCommitThenLoad | IDs and marks round-trip through the database |
