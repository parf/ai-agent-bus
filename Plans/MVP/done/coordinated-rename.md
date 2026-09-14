# Coordinated rename verification

Historical evidence, 2026-09-13. Current contract:
[explicit session rename](../../../docs/08-runner-role.md#explicit-session-rename).

| Check | Result |
|---|---|
| Reproduction | The installed prior launcher kept its original reader and binding while the MCP face registered a second address; both runtime fixtures lost replies after renaming |
| End-to-end regression | Both runtimes pass concurrent rename, immediate no-argument repeat, reader migration, credential refresh, account ownership, old-address removal and restart without duplication |
| Control boundary | Unauthenticated rename requests return 401; the capability is private to one launcher |
| Actual runtime APIs | Installed Codex accepts `thread/name/set` and returns the updated thread name; installed OpenCode accepts its session update and returns the updated title. No model turn was needed |
| Full suite | `src/smoke.sh --slow`: 530 passed, zero failed, including vet and race checks |
| Mutation | Bypassing launcher coordination in an isolated full slow run fails the named rename checks for both runtimes |
| Installed bundle | Release 0.5.13 installed; the installed-launcher regression passes, daemon is active, dashboard health returns 200 |
| Existing aliases | Idle broken-rename aliases for Codex mdhouse, OpenCode mdhouse and OpenCode ab_list were removed after transferring their reservations back to the launching account. Active readers and queues were left intact |

Old running launcher processes must be restarted to use the coordinator.
Logs and the scoped repair script remain local under `tmp/rename-fix/`.
