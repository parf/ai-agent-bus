# H.5.9 — `user add` creates the user it adds

Shipped in 0.5.32. Evidence for the acceptance in
[TODO](../TODO.md#remaining-work) as it stood before removal.

## Scope

`agent-bus-admin user add` wrote an `authorized_keys` line and stopped. Strict
issuing ([Q57](../../../docs/decisions.md#settled)) refuses a credential to a
name the daemon holds no profile and no record for, so the `agent-bus-token`
forced command the key reaches answered every newcomer with a refusal. **A
fresh install could not onboard anybody**, and it was the only onboarding path
there is.

## What it does now

| | |
|---|---|
| `user add <name> <key>` | writes the restricted line, then creates the user on the daemon |
| `user add <name> <key> --admin` | the same, forced into `agent-bus-admin`, and adds the name to `@maintainers` |
| daemon unreachable | **refuses**, and takes the key line back out |
| daemon refuses the creation | the same |
| name already known | not an error — adding a second key for somebody already here is the same operation |

**Order is the design.** The key line is written first because it is the half
that can be taken back; a user is never deleted
([user lifecycle](../../../docs/01-identity.md#user-lifecycle)), so the
irreversible half goes last and the reversible one is undone when it refuses.
The outcome is both or neither.

**The offline case was a choice the task left open** — provision at the
daemon's next start, or refuse to half-add. It refuses. Deferring would put
onboarding in two places and need a stored intent nobody has asked for, and
"a key that works before the name exists" is exactly the state being closed.

**Authority is granted only where it was asked for.** `Maintainer` is derived
by the daemon from `@maintainers` membership and is never accepted as a claim,
so `--admin` grants it by reading the current membership and adding to it —
which also keeps the daemon owner in the group, without which the daemon
refuses the whole write.

## Checks

`cmd/agent-bus-admin/provision_test.go`, against a fake daemon that records
calls, so a check can assert the user was created rather than that nothing
failed.

| Check | Asserts |
|---|---|
| `TestUserAddCreatesTheUserItAdds` | the daemon was asked to create the name, the line exists, and no authority was granted |
| `TestAdminFlagGrantsMaintainerAndNothingElseDoes` | `--admin` grants maintainer and the existing members survive |
| `TestAnUnreachableDaemonAddsNobody` | no daemon, no key left behind |
| `TestARefusedCreationLeavesNoKey` | a refused creation leaves no key |
| `TestAnAlreadyKnownNameStillGetsItsKey` | an existing name still gets the key being added |
| `TestAddingTwiceReplacesTheLine` | one line per name survives |
| `TestForcedCommandFollowsTheFlag` | the flag decides which program the key reaches |

## Mutations

Each acceptance failure mode was produced and watched to fail its **named**
check, not merely to fail.

| Mutation | Caught by |
|---|---|
| M1 skip provisioning — the key is added without the user | `TestUserAddCreatesTheUserItAdds`: *"the key was added without the user: the daemon was never asked to create newcomer@h, so its forced command would be refused a credential"* |
| M2 skip the key write — the user is created without the key | `TestUserAddCreatesTheUserItAdds`: *"the user was created without the key: nothing in authorized_keys reaches the token command"* |
| M3 grant maintainer regardless of the flag | `TestUserAddCreatesTheUserItAdds`: *"authority nobody granted: newcomer@h became a maintainer without --admin"* |
| M4 drop the rollback — the key outlives a refused creation | `TestAnUnreachableDaemonAddsNobody`: *"a key that works before the name exists: the line was left behind after the daemon could not be reached"* |

`src/smoke.sh --slow`: 543 passed, 0 failed.

## Not done here

`user remove` takes the key and leaves the user, which is correct — a user is
never deleted — but nothing yet pauses a removed operator's standing. That is
lifecycle administration, already reachable through the dashboard and the API,
and was not part of this task.
