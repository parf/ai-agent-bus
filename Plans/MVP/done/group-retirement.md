# H.5.6 — a group is retired by emptying it

Shipped in 0.5.33. Evidence for the acceptance in
[TODO](../TODO.md#remaining-work) as it stood before removal.

## Scope

The contract was settled and written down
([identity § a group is not deleted](../../../docs/01-identity.md#groups-and-maintainers)):
a group is retired by emptying its membership, because removing a name other
records point at silently changes what every one of them means. The code had
not caught up. `SetGroup` still carried a `remove` parameter and a
`delete(b.groups, name)`, guarded only by *is this group referenced right now*
— so an unreferenced group could still be unmapped, and the record that named
it tomorrow would find nothing there.

## What changed

| | |
|---|---|
| [`core.SetGroup`](../../../src/internal/core/manage.go) | the `remove` parameter and the deletion branch are gone, with the two `ErrBusy` guards that existed only for it |
| [`api` group handler](../../../src/internal/api/manage.go) | still reads `Remove`, and refuses it — `ErrNoRemoval`, `400`, counted `malformed` |
| [`agent-bus-web` `POST /groups`](../../../src/cmd/agent-bus-web/admin.go) | the `delete` action is refused; the forwarding struct no longer carries `Remove` |

**The field is read in order to be refused, and that is the point.** Dropping
it would have been the obvious cut and the wrong one: `encoding/json` ignores
unknown fields and an absent `members` decodes as the empty set, so
`{"name":"@ops","remove":true}` would have become *set `@ops` to no members* —
a different operation, answered `200`. Retirement and deletion are close enough
in effect that silently substituting one for the other is exactly the mistake
worth refusing. The mutation below shows it happening.

`400 malformed` rather than `403` or `409`: the caller asked for a verb that
does not exist, which is not an authority question and not a conflict. The
daemon owner is refused it on the same terms as anybody, so the old `403` for
`@maintainers` and `409` for a referenced group are both gone — those answers
described branches that no longer exist.

## Checks

| Check | What it pins |
|---|---|
| `TestNoPathRemovesAGroup` | refused for the daemon owner, a maintainer and an ordinary user; membership unchanged afterwards; a removal carrying members does not apply them |
| `TestEmptyingAGroupLeavesTheRecordsThatNameIt` | the group survives emptying, the record still names it, its description survives, a former member confers nothing, and putting one back restores it |
| `TestAnEmptiedGroupSurvivesASnapshotRoundTrip` | retirement persists — an emptied group comes back as a group with no members, not as an absent name |
| `TestTheMaintainersGroupIsNeitherEmptiedNorRemoved` | `403` for emptying, `400` for removal: the one group that defines a level is retirable by neither route |
| `TestDashboardOwnerControls` | `POST /groups action=delete` answers `400` from a request that is otherwise entirely in order, with and without members attached |

**The fixture is the check.** `@ops` is nonempty and named by no record, which
is precisely the case the old deletion branch allowed. A referenced group or
`@maintainers` would have been refused with the verb still present, so either
would pass against the code these tests exist to reject.

## Mutation

Each break was made, the named check watched to fail, and the file restored.

| Mutation | Failed |
|---|---|
| the API stops refusing `remove: true` | `TestNoPathRemovesAGroup` — `admin@h` answered `200`, and the members carried alongside **were applied**, which is the silent reinterpretation in full |
| the web handler accepts `delete` again | `TestDashboardOwnerControls` — `POST /groups` answered `303` |
| `Snapshot` omits groups with no members | `TestAnEmptiedGroupSurvivesASnapshotRoundTrip` — the emptied group did not survive |

Restoring the core deletion branch on its own is **not** a mutation that fails
anything, and that is correct rather than a gap: with both callers refusing,
the branch is unreachable, so its absence is not what any behaviour rests on.

`src/smoke.sh --slow`: 543 passed, 0 failed.

## Peer review

codex audited the removal surface independently and found the live ingress this
started from — the dashboard rendered no delete button and its handler accepted
`action=delete` regardless. It also confirmed the core deletion branch was the
only `delete(b.groups, ...)`, that `Restore` merges rather than replaces so an
absent key does not delete, that neither the CLI nor the MCP face carries a
group verb, and asked for the snapshot round-trip and the members-carried case
that are checked above.

## Not done here

*No `inactive` or `banned` is added to a group anywhere* is a requirement to
not build something, and nothing was built. There is no check, because a test
that a concept is absent passes by default and would say nothing.
