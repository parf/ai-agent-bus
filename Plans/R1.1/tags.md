# Record tags

## Classification and filtering

Owner proposal for R1.1; not implemented.

| Item | Direction |
|---|---|
| Classification | Replace the dedicated Personal switch with record tags |
| Input | Accept a space- or comma-delimited list of tags |
| Personal | `personal` is an ordinary tag |
| Untagged records | Expose them under the synthetic `no-tags` tag |
| Queries | Allow limiting record queries by tag |
| Web | Show clickable `tag(count)` filters on registry lists |
| Visibility | Filtering and counts respect the caller's existing record visibility |

These are record classification tags, separate from message correlation `tag`.
Migrate existing Personal records to the `personal` tag; shared records with
no assigned tags appear under `no-tags`.

The current [Personal rules](../../docs/03-records.md#personal-and-shared)
also constrain ACL and Maintainers, force User records to be Personal, and
restrict who changes classification. Their replacement needs an explicit
decision ([Q144](QUESTIONS.md#record-tags)); a tag filter grants no access.

Parsing details, matching multiple tags, and counting with active filters
remain open in [Q145](QUESTIONS.md#record-tags). No storage or wire shape is
chosen yet.
