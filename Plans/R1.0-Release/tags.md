# Record tags

## Classification and filtering

Implemented for R1 on the 0.8 line; verification and review evidence belong in
[the implementation checklist](TODO.md#record-tags-acceptance).
The current contract lives in [record tags](../../docs/03-records.md#tags).

Record edits use Tags after Description. The `personal` label retains the
existing audience and authority rules; the legacy flag is a compatibility
mirror. Untagged records expose the synthetic, never stored `no-tags` label.

## Automatic tags

Launchers add `claude`, `codex` or `opencode`. The runner adds `runner`;
package and host inventory can supply additional labels. The dedicated runner
adds no automatic tag and supports explicitly configured tags.
Tags are additive on registration and runner reload; explicit Manage edits
remove them. See [the current update rules](../../docs/03-records.md#tags).

Comma-only parsing, lowercase normalization and visibility-respecting counts
follow the owner's 2026-10-17 instructions. Multiple-tag query matching is settled
as [№RD85](DECISIONS.md#recorded-decisions) and not built
([№RTA1](TODO.md#record-tags-acceptance)); the built UI selects one tag at a
time.
