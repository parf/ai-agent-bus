# Personal service core

📌 **TL;DR:** 0.5.50 stores the owner's Personal choice and enforces its assignment limits without changing access or delivery.

## Scope

The core and public record now carry an owner-controlled Personal
classification. A new service may state it; metadata refresh preserves the
stored choice. Management validates the final candidate atomically, so removing
sharing while enabling Personal and adding sharing while disabling Personal
each take one request.

A Personal ACL may name only another directly registered Service. Users,
profile-backed records, Agents, Channels, unknown names, self, groups,
wildcards and Maintainers are refused. The MVP boundary is service-only;
extension to another identity kind requires an owner decision. Web grouping
and filtering remain the next slice.

Personal is not consulted by authorization or delivery. The ordinary ACL,
owner, own-inbox, standing, suspension and Disabled rules remain unchanged.

## Write-time validation

Assignment validity is checked when the record is written. If every named
service is later removed, the Personal record and its stored names remain; the
stale names grant nobody. A later write must remove the stale names or restore
their services. This matches group retirement: removal does not silently
rewrite other records.

## Checks

- Atomic controls cover both combined directions and both invalid half-changes,
  with failed requests leaving the stored record untouched.
- Assignment controls independently refuse users, a profile on a generic
  backing record, Agents, Channels, unknown and self names, a service-only
  group, wildcard and Maintainers. A direct Service entry succeeds.
- Only the owner toggles Personal. Service refresh preserves the stored choice
  and refuses an explicit invalid replacement ACL rather than clearing the tag.
- JSON snapshot/restore retains Personal. Its ACL name remains stored after
  the only target is removed; while absent, that name matches no principal.
- Real send and consume controls show an allowed service still reaches the
  Personal inbox, an unrelated user remains refused, and the service keeps its
  own-inbox right.
- HTTP checks cover the public field, malformed-policy response and an atomic
  disable-and-share request. CLI checks inspect direct registration and runner
  registration from both flags and JSON.
- OpenCode reviewed the source and approved it without findings.

Final frozen `src/personal-services-smoke.local.sh --slow`: **589 passed, 0
failed**, exit 0; vet and race passed (`tmp/personal-services/slow-final.log`,
lines 10–11 and 936). The frozen script and tracked `src/smoke.sh` both have
SHA-256 `e401546193343b037b16df4c5e408d1f208aa35f6cbe8180a64274a0e6e23b21`.
The 174-entry source/version manifest was unchanged after the run
(`tmp/personal-services/source-frozen.sha256` and `source-verify.log`).

## Mutations

The final targeted run caught all **11 of 11** overlays. Each ran its named Go
test, not the full smoke suite (`tmp/personal-services/mutations.log`).

| Changed behavior | Named catch |
|---|---|
| Skip final-candidate validation in Manage | Invalid half-change lands instead of remaining atomic |
| Skip validation during registration | Forbidden Personal ACL is registered |
| Clear Personal during metadata refresh | Owner choice does not survive restart metadata |
| Accept a profile-backed ACL entry | User on a generic backing record is treated as a Service |
| Accept an Agent or Channel ACL entry | Direct-Service boundary is widened |
| Accept Maintainers | Clean Maintainers-only negative succeeds |
| Let a Maintainer toggle Personal | Owner-only classification is violated |
| Make Personal deny an otherwise allowed caller | Real allowed send no longer reaches the inbox |
| Remove Personal from JSON | Snapshot/restore loses the classification |
| Drop Personal from runner registration | Both runner input forms lose owner intent |
| Drop Personal from direct CLI registration | `register --personal` sends a non-Personal record |

The first mutation run found a hollow assertion: the Maintainers negative also
contained a user ACL, so removing only the Maintainers rejection still failed
for the user. That survivor is retained in
`tmp/personal-services/mutations-initial.log`. The corrected test starts with a
valid direct-Service ACL and makes Maintainers the sole violation; the final
mutant fails it. The group fixture was also tightened to a service-only group,
so Q71's composition case is explicit rather than inferred from the `@` syntax.

One attempted frozen smoke invocation placed the copy under `tmp/`; the script
changes into its own directory and immediately failed to find `build.sh`.
It ran no check and receives no credit (`slow-wrong-location.log`). The final
copy sits beside the tracked script, as the harness requires.

## Limits

This slice does not add dashboard tabs or filter existing pages. It does not
alter any live record, infer Personal from ownership, or migrate existing
services into the classification.
