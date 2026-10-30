# DONE — R1

`RR` labels the original unnumbered result rows; existing KV IDs are unchanged.

Completed R1 work, built ahead of the release on the 0.8 line. Remaining work
is in [TODO](TODO.md).

- ✅ B.7 [Server-info](../../src/agents/standalone/server-info/README.md#server-info-agent)
  - Result: all accepted host-fact sections, with a shared standalone systemd starter for №B.2; fixed section calls in text and JSON, root-only sections absent when unprivileged
  - Evidence: `agent_server_info` under `--slow`: real host interfaces, argument refusal, install idempotence, restart and removal after graceful stop or crash; sandbox and argument-forwarding mutants

- ✅ RR1 [Assigned roles](roles.md#role-syntax)
  - Result: Built in 0.8.110; [current contract](../../docs/03-records-agent.md#roles)
  - Evidence: [verification](done/assigned-roles.md#verification)

- ✅ RR2 [Client libraries](done/client-libraries.md#scope)
  - Result: Built in 0.8.100, 2026-10-01: Python, TypeScript, Go, Rust, Java and PHP clients, MIT, each reviewed by another author, with [native examples](../../src/clients/examples/README.md) that call each other
  - Evidence: each client's suite and compiling mutants through the `--slow` client shards; 929/0 at 0.8.101

- ✅ RR3 [Roles are for Agents](roles.md#roles-are-for-agents)
  - Result: Built in 0.8.97–0.8.99, 2026-09-30: a delivered message carries the generated `owner` and `maintainer` roles, worked out at `original_to` and passed through forwards; a script gets them as `AB_ROLE_*`; a role crossing to a record its source's Owner does not answer for fails loudly; an Agent lists the roles it understands. Assigned roles are now built ([contract](../../docs/03-records-agent.md#roles))
  - Evidence: `core` roles_test.go, `agent-bus` TestAChildHoldsOnlyTheRolesItWasHanded under sh and dash, smoke script_done and script_confined roles checks; reviewed by ag-bus and Codex

- ✅ RR4 [Key-value store](done/kv.md#per-record-storage)
  - Result: Built in 0.8.77, 2026-09-30: [contract](../../docs/03-records-kv.md#key-value-store)
  - Evidence: below

- ✅ KV.1 Store and authority
  - Result: String, int and JSON values round-trip, the same name in two kinds staying two values; a write answered before a SIGKILL survives it; an inactive record's store is no such entity and comes back with it; a name removed and registered again starts empty; the Owner, a Maintainer through a group and the record's own Agent use it, a caller on the allow list is refused
  - Evidence: `core` TestKVIsTheRecordsManagersAlone, TestKVStoreIsNoSuchEntityWhileItsRecordIsInactive, TestKVNameRemovedAndRegisteredAgainStartsEmpty, TestKVKindsAreSeparateNamespaces; `sqlite` TestKVRoundTripsAndSurvivesAReopen, TestKVGoesWithItsRecord, TestKVWritesOnlyForAStoredRecord, TestSchemaSixMigratesToKeepValues; smoke `restart` (SIGKILL) and the key-value section. Each mutant — the allow list let in, status ignored, the drop skipped, the stored-record check or the commit removed, the kind ignored on a read — failed an assertion

- ✅ KV.2 `set` modes
  - Result: `add` refuses a present name and `replace` an absent one, each saying so; fifty concurrent `add`s of one name, one success
  - Evidence: `core` TestKVSetModes, TestKVConcurrentAddsHaveOneWinner; mutants treating `add` as `set` or skipping `replace`'s check failed

- ✅ KV.3 Increment and JSON operations
  - Result: 1,000 concurrent increments end at the exact sum; each operation does what its row says; 200 elements shifted by ten workers go out once each; a list with one refused op changes nothing
  - Evidence: `core` TestKVIntIncIsExactUnderConcurrency, TestKVJSONOperationsDoWhatTheirRowSays, TestKVConcurrentShiftsHandEachElementOnce, TestKVJSONListIsAllOrNone, TestKVJSONEqualityIgnoresKeyOrderAndNumberSpelling, TestKVLimits; a read-modify-write outside the store's lock, a partly applied list, text equality and each op's own mutant failed

- B.29 / B.28 step 1a: shared Python subscriptions, language-neutral vectors and runtime inventory wiring completed on `task/eventlog`, pending QA/landing; [verification](agent-events/DONE.md#shared-subscriptions). Eventlog, the TypeScript twin and consumer migrations remain in TODO.

- ✅ P.2 [Container image](../../src/container/README.md#container-image)
  - Result: `docker.io/parf/agent-bus:0.9.179` and `:latest`, digest `sha256:b6b7ebc48d5a9dfad21f868288eac37f749723358b1743c7e28102d0b1a08089`, built from the stamped 0.9.179 release archive, 2026-10-30
  - Evidence: pulled by digest into an empty rootless podman store on this node, `acceptance.sh --image=` passed 10/10: both roles, a request/reply, a recreated daemon keeping the service and both credentials, sandbox refusal. Mutants daemon-entry, runner-entry, storage-outside and sandbox-downgrade each failed on assertions
