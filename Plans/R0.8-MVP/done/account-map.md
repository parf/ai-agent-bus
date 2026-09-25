# Durable local-account map

📌 **TL;DR:** 0.5.59 lets existing daemon Administrators durably change which
bus principal each local OS account's private socket authenticates.

## Result

Legacy `-user` flags seed the map once. Current snapshots carry an explicit
establishment marker, including for an intentionally empty map, and override
contrary old flags on every later start. The daemon account's implicit socket
is protected outside the editable map.

The existing Owner/Administrator authority serves `account list`, `account set`
and `account remove` through `agent-bus-admin` locally or over its established
SSH forced command. The same operations accept an existing Administrator token
through the API. No credential or settings page was added.

An edit validates authority before host-account lookup, requires an existing OS
account and an active known principal, and checkpoints before success. It changes
durable desired state only: the current listener keeps its identity until a full
daemon restart. The new supervisor applies the stored map, removes retired socket
paths and reports no pending restart once desired and active maps agree.

## Checks

Core tests cover administrative authority, unknown and inactive principals,
durability, a contrary legacy seed, restart state, explicit empty maps, missing
removal and damaged snapshots. API tests cover strict input, host lookup,
authority-before-enumeration, the protected daemon account and removal after an
OS account disappears. Supervisor tests cover seed-versus-stored precedence,
duplicates, missing accounts, protected-account refusal and safe stale-socket
cleanup. CLI tests cover list/set output and the SSH grammar.

The main smoke uses an actual second local account and two bus principals. It
changes the account's mapping, proves the live listener remains the old
principal until restart, restarts with a contrary `-user` seed, and then proves
the stored principal gains its private visibility and send authority while the
unrelated owner socket remains unchanged.

The targeted overlay run caught **16/16** named changes across authority,
principal standing, live-versus-desired state, persistence, first-run seeding,
the establishment marker, host-account lookup, protected and duplicate
accounts, routes, retired sockets and CLI dispatch.

The first mutation run receives no credit. Its SSH-oriented mutant removed
`account` from direct verb recognition while the test exercised entitled
command parsing, so it changed no forced-command behavior and survived. The
admin dispatcher was factored into the tested path; the corrected mutant drops
its `account` case and the behavioral request fails. The retained first log
records the survivor rather than overwriting it.

Final frozen `src/account-map-smoke.local.sh --slow`: **609 passed, 0 failed**,
exit 0; vet and race passed (`tmp/account-map/slow-final.log`, lines 10–11 and
966). All 198 source/version manifest entries matched after the run. The frozen
and tracked scripts both have SHA-256
`aff1dd8ff54820c09129106b5f0b92c264780cf3394c054629cd78421a82fab3`.

## Limits

This is administration through existing CLI/API credentials, not a dashboard
control. A bus-child restart reuses supervisor-owned listeners and therefore
does not apply a map change; restart the full daemon unit. OS accounts are
validated when a mapping is written and again before listeners open.

Downgrading to an older daemon ignores the durable map and returns temporarily
to its configured `-user` flags. Review those flags before rollback.

## Live postflight

Commit `86f0066` was built as 0.5.59 (`parf@parf.us 2026-09-17
14:07:18`) and restarted on `parf.us`. The preflight snapshot had no account
establishment marker. The first current start persisted
`accounts_established: true` with exactly the two configured mappings:
`parf` to `parf@parf` and `agent-bus-runner` to `runner@parf`.

The owner-only `/accounts` answer reported those mappings and
`restart_required: false`. Their sockets and the separate protected
`agent-busd` socket were mode 0600 with their intended OS owners. The public
identity answer reported 0.5.59. The web child remained in its delegated
cgroup with an effective capability mask of zero. No live mapping was edited
during postflight.
