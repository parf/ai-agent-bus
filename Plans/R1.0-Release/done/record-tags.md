# Record tags

## Verification

Built on the 0.8 line at 0.8.143. Current substance: [record tags](../../../docs/03-records.md#tags).

- Full `src/smoke.sh --slow`: 976 passed, 0 failed; `tmp/tags-slow.log`.
- First Claude reviewed normalization, Personal authority, persistence,
  browse counts, both runner paths and stored-value editing; findings fixed.
- Shared JSON grammar cases run in Go and TypeScript. Fixture-backed web checks
  use 32 records and more than 25 distinct labels, with an inaccessible record
  plus an Owner positive control.
- Compiling mutants fail named assertions: page-limited counts, selected-chip
  omission, scope-limited All count, visibility bypass, Personal Owner-check
  omission, lost tag storage, missing dedicated-runner label and omitted
  kept-token refresh. Logs: `tmp/tags-mutant-*.log`.
- Stored ENV/config display and exact large JSON integer roundtrip checked;
  a failed private read leaves unrelated edits usable without reflecting errors.

No-tags is synthetic, Personal rules are retained, and tags grant no access.
Multiple-tag query matching remains Q145.
