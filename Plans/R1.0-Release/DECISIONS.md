# R1 decisions

## Recorded decisions

Migrated 2026-09-13 and split from the old R1 on 2026-09-30. Related historical rows are consolidated by their owning decision topic; original dates were not recorded consistently. An indexed target may still be pending implementation. The linked substance wins.

| Decision topic | Substance | Earlier rows |
|---|---|---|
| R1's scope is settled, and the old R1 splits into R1 and [R1.1](../R1.1/README.md#scope) (Q18) | [R1 scope](README.md#scope) | 2026-09-30 owner split; tools and exploration renumbered to R1.2 and R1.3 |
| Record-defined roles are R1's first topic (Q98) | [roles](roles.md#record-defined-roles) | 2026-09-22 owner decision; moved out of MVP; D249 |
| Locks belong to a Group, their namespace and ACL; any member may take one, or release another's with `--force` | [shared locks](locks.md#shared-locks) | 2026-09-30 owner decision; pipelines release what an earlier stage took |
| Locks are held in memory and never stored; a set of locks is one grant over several | [shared locks](locks.md#shared-locks), [a set of locks](locks.md#a-set-of-locks) | 2026-09-22 owner decision; D64, D65, D66 |
| A per-name key-value store | [key-value store](kv.md#per-name-storage) | 2026-09-22 owner instruction |
| MCP Resource and Resource Template become registry records | [Resource records](resources.md#resource-records) | 2026-09-23 owner instruction |
| A Resource is an Agent in disguise; the bus switches the read (Q112) | [Resource records](resources.md#resource-records) | 2026-09-23 owner framing |
| One `resource` kind with a template flag, 📄 and 📑 (Q110); every field the MCP spec defines, ours covering access (Q111); a read is answered like an Agent's message (Q114); the daemon serves no content itself (Q113) | [Resource records](resources.md#resource-records) | 2026-09-30 owner decisions, against the 2026-07-28 specification |
| Federation stays R1 scope, on hold | [federation](federation.md#chaining) | 2026-09-30 owner decision |
| Chaining | [definition](federation.md#chaining) | D80 |
| Modules | [definition](modules.md#modules) | D178 |
| What the runner does | [definition](runner.md#what-the-runner-does) | D211, D212 |
| Who it runs as | [definition](runner.md#who-it-runs-as) | D213 |
| Runner unit | [definition](runner.md#runner-unit) | D199, D200, D201 |
| Where it runs | [definition](runner.md#where-it-runs) | D209 |
| Reaching the runner | [definition](runner.md#reaching-the-runner) | D197, D198, D208 |
| What an instance is | [definition](runner.md#what-an-instance-is) | D129, D202, D207, D210 |
| The three env layers | [definition](runner.md#the-three-env-layers) | D203, D204 |
| What the child is told | [definition](runner.md#what-the-child-is-told) | D205, D206 |
| Long lived services | [definition](runner.md#long-lived-services) | D120, D122 |
| One name on many hosts | [definition](runner.md#one-name-on-many-hosts) | D123, D124, D125, D128 |
| On demand | [definition](runner.md#on-demand) | |
| Message routing | [definition](runner.md#many-names-into-one-inbox) | |
| Additional script forms | [definition](runner.md#additional-script-forms) | D61, D118, D119, D121 |
| The list of what is installed | [definition](runner.md#the-list-of-what-is-installed) | D138, D139, D140 |
| What it comes after | [definition](runner.md#what-it-comes-after) | D141, D142, D143 |
| Backing it up | [definition](runner.md#backing-it-up) | D136, D137, D144 |

## Backup encryption choice

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | User-key backup encryption | Reuse the user's existing key and an established tool; owner instruction | [runner § backing it up](runner.md#backing-it-up) |

## Distribution choice

| Date | Decision | Why | Substance |
|---|---|---|---|
| 2026-09-13 | Published release distributions | Owner requests installation and container startup without a source build | [Release artifacts](distribution.md#release-artifacts), [container runtime](distribution.md#container-runtime) |

## Open

Unresolved choices live in [questions](QUESTIONS.md#open-questions).

## Superseded

| Earlier design | Replacement |
|---|---|
| Global lock names in one default set, a set granted by its own ACL, and only the holder releasing | [Locks in a Group](locks.md#shared-locks), 2026-09-30 |

## History

Original wording and superseded choices are preserved in [decision history](../R0.8-MVP/done/decisions-before-rewrite.md#decision-history-before-the-documentation-rewrite). Original row identifiers are mapped in [migration evidence](../R0.8-MVP/done/document-migration.md#decision-mapping).
