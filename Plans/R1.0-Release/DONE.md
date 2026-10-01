# DONE — R1

Completed R1 work, built ahead of the release on the 0.8 line. Remaining work
is in [TODO](TODO.md).

| Work | Result | Evidence |
|---|---|---|
| [Key-value store](kv.md#per-record-storage) | Built in 0.8.77, 2026-09-30: [contract](../../docs/01-identity-and-authority.md#key-value-store) | below |
| KV.1 Store and authority | String, int and JSON values round-trip, the same name in two kinds staying two values; a write answered before a SIGKILL survives it; an inactive record's store is no such entity and comes back with it; a name removed and registered again starts empty; the Owner, a Maintainer through a group and the record's own Agent use it, a caller on the allow list is refused | `core` TestKVIsTheRecordsManagersAlone, TestKVStoreIsNoSuchEntityWhileItsRecordIsInactive, TestKVNameRemovedAndRegisteredAgainStartsEmpty, TestKVKindsAreSeparateNamespaces; `sqlite` TestKVRoundTripsAndSurvivesAReopen, TestKVGoesWithItsRecord, TestKVWritesOnlyForAStoredRecord, TestSchemaSixMigratesToKeepValues; smoke `restart` (SIGKILL) and the key-value section. Each mutant — the allow list let in, status ignored, the drop skipped, the stored-record check or the commit removed, the kind ignored on a read — failed an assertion |
| KV.2 `set` modes | `add` refuses a present name and `replace` an absent one, each saying so; fifty concurrent `add`s of one name, one success | `core` TestKVSetModes, TestKVConcurrentAddsHaveOneWinner; mutants treating `add` as `set` or skipping `replace`'s check failed |
| KV.3 Increment and JSON operations | 1,000 concurrent increments end at the exact sum; each operation does what its row says; 200 elements shifted by ten workers go out once each; a list with one refused op changes nothing | `core` TestKVIntIncIsExactUnderConcurrency, TestKVJSONOperationsDoWhatTheirRowSays, TestKVConcurrentShiftsHandEachElementOnce, TestKVJSONListIsAllOrNone, TestKVJSONEqualityIgnoresKeyOrderAndNumberSpelling, TestKVLimits; a read-modify-write outside the store's lock, a partly applied list, text equality and each op's own mutant failed |
