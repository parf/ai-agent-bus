# Service and Channel Danger Zone

📌 **TL;DR:** 0.5.63 moves three dangerous resource actions off ordinary
detail and re-reads transfer and removal facts on server-rendered confirmation
pages.

## Result

The ordinary Service and Channel detail keeps read-only facts and routine
controls. An authorized manager follows a red **Danger Zone** link to replace
private configuration, transfer ownership when permitted, or start removal.
The Danger page repeats the caller-visible lookup and derives authority from
the fresh daemon answer. Existing private configuration and credentials never
enter a page.

Transfer and removal post through the same exact-origin boundary to a
server-rendered confirmation. The confirmation reloads the record, names its
current owner and, for removal, reports the observed queue and Readers count.
Rendering it changes nothing. The final form re-reads those visible facts; a
mismatch stops on a distinct conditions-changed page, while an unchanged
daemon refusal keeps the daemon's own current-state reason. The daemon remains
the final authority and a direct API client gains no capability.

Routine resource actions and configuration replacement return to the affected
detail. After transfer, the former owner returns there only while the record
remains visible; otherwise the pre-transfer kind selects Services or Channels
instead of sending the person to a hidden detail. Successful removal uses the
same kind-specific lists.

## Checks

Package tests cover Service and Channel parity, dangerous-form absence on
ordinary detail, manager-only entry, fresh transfer applicability, exact
origin on both confirmation actions, hidden/missing equivalence, no-cache,
escaping, no secret or session rendering, confirmation non-mutation,
visible-fact changes, unchanged busy refusal, retained-or-lost visibility after
transfer and resource-specific returns.

The final targeted run caught **18/18** named breaks: widening the Danger route;
restoring a dangerous label on ordinary detail; removing configuration or
either confirmation hop; widening fresh transfer applicability; mutating on
either confirmation; dropping exact-origin; ignoring owner, queue or reader
changes; merging stale and ordinary refusals; losing detail, list or return-link
state; sending a former owner to a newly hidden detail after transfer; and
showing transfer to a Maintainer.

Three exploratory runs receive no final-set credit. The first exposed a hollow
heading assertion that still found the same words on a button. The next two
stopped on malformed mutation code: first a wrong return arity, then an unused
parsed value. A subsequent 17/17 run was green before the transfer-return
fallback was added and is superseded by the final 18-case set. The final
mutants perform the intended behavioral changes and all fail their named
checks. Fast smoke passed **488/0**. Documentation validation checked 152 files
and 2,845 local links with zero errors before the final frozen run. The frozen
slow smoke passed **609/0**, including vet and race. All **202** frozen
source/version hashes matched afterwards. The tracked runner and its frozen
copy both had SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.

## Limits

The face can compare facts it can read; it cannot make the web precheck and
the daemon operation atomic. A change after the last read can still produce
the daemon's ordinary refusal. Security does not rely on the confirmation or
the comparison: operation-time daemon authorization remains decisive.

This slice does not implement list search, sorting, paging, owner photos,
activity placement, registration-page moves or the remaining visual redesign.
