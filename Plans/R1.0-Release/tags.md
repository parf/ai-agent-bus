# Record tags

## Classification and filtering

Planned for R1; not implemented.

| Item | Direction |
|---|---|
| Classification | Replace the dedicated Personal switch with record tags |
| Input | Accept a space- or comma-delimited list of tags |
| Personal | `personal` is a tag that retains the existing Personal rules |
| Untagged records | Expose them under the synthetic `no-tags` tag |
| Queries | Allow limiting record queries by tag |
| Web | Show clickable `tag(count)` filters on registry lists |
| Visibility | Filtering and counts respect the caller's existing record visibility |

These are record classification tags, separate from message correlation `tag`.
Migrate existing Personal records to the `personal` tag; shared records with
no assigned tags appear under `no-tags`.

The existing [Personal rules](../../docs/03-records.md#personal-and-shared)
switch from the dedicated flag to membership of the `personal` tag. Preserve
their ACL and Maintainers restrictions, owner-only classification changes,
User-record requirement and default web visibility. Adding or removing
`personal` follows the same validation as changing Personal today; other
tags do not weaken those rules. A tag filter grants no access.

## Automatic tags

| Started by | Tag added to its agent record |
|---|---|
| `ab-claude` | `claude` |
| `ab-codex` | `codex` |
| `ab-opencode` | `opencode` |
| AgentBusRunner (a managed task) | `runner` |

Tags are additive: keep existing tags, including `personal`. An agent started
through both a launcher and the runner carries both applicable tags. These
labels describe how it runs; they grant no authority.

Parsing details, matching multiple tags, and counting with active filters
remain open in [Q145](QUESTIONS.md#record-tags). No storage or wire shape is
chosen yet.
