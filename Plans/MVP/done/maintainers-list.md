# Maintainers list

📌 **TL;DR:** 0.5.62 replaces the single resource Maintainers group with an
Owner-assigned list of Users, Groups, Agents and Services.

## Result

The resource Owner or daemon Owner replaces the complete list. Direct named
principals gain management authority; group terms use the existing cycle-safe
nested-membership resolver. The resource Owner and its own principal retain
their existing authority. Maintainers cannot replace the list.

Writes validate the complete candidate before storage. Wildcard, Channel,
unknown or credential-only names, missing groups and normalized duplicates are
refused. Personal resources continue to refuse any Maintainers. Removing group
membership revokes effective management on the next operation.

Registration never imports Maintainers from caller metadata. A refresh
preserves the Owner's stored list; an omitted management field preserves it and
an explicit empty list clears it.

## Migration and faces

Legacy string values decode as one entry and an empty string as no entries.
Every non-empty current daemon answer and snapshot emits an array; an empty
list is omitted. The Administrator-name migration rewrites every list term
after legacy decoding. A noncanonical legacy term is retained but inert until
the Owner replaces the list; restore does not silently invent or normalize
authority.

Raw API and CLI JSON, the MCP record type and WEB carry the array. The existing
WEB editor uses the accepted one-plain-term-per-line textarea and round-trips
every entry. The larger service-detail and Danger Zone redesign remains in
[F.13.3](../TODO.md#web-redesign).

## Checks

Core and API tests cover direct User, Service and Agent terms, nested groups,
the explicit Administrator group, resource-principal and daemon-Owner
authority, Maintainer refusal, group-removal revocation, atomic invalid
replacement, normalized duplicates, clear versus omit, registration and
refresh, snapshot restore, legacy decoding before Administrator rename and
array-only current output. CLI, MCP typechecking and WEB checks cover the face
boundary.

The final targeted run caught **16/16** named changes: losing direct, group or
resource-principal authority; widening or narrowing list replacement; accepting
wildcard, Channel or a normalized duplicate; ignoring clear; losing the list on
refresh; importing registration claims; refusing legacy input; writing the old
string form; rewriting only the first migrated term; allowing Personal
Maintainers; and flattening the WEB textarea.

The first targeted run caught the first fifteen changes, then its WEB mutant
survived because it changed the conditional editor branch the fixture did not
render. That log receives no final-set credit. The corrected mutant changes the
rendered owner editor, and its two-line assertion catches the flattening.

Fast smoke passed **488/0**. Final frozen
`src/maintainers-list-smoke.local.sh --slow` passed **609/0**, exit 0; vet and
race passed (`tmp/maintainers-list/slow-final.log`, lines 10–11 and 965). All
201 source/version manifest entries matched after the run. The frozen and
tracked scripts both have SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`.
Documentation validation checked 152 files and 2,841 local links with zero
errors before this measured paragraph was inserted.

## Limits

This slice stores and enforces the accepted Maintainers model. It does not add
service-defined role storage, redesign the service journey, or change
ownership/orphan traversal. An older daemon cannot decode a snapshot after the
new array form has been written; the setup guide records that fail-closed
downgrade boundary.

## Live postflight

Commit `374632b` was pushed and deployed as 0.5.62. The live identity reported
build `parf@parf.us 2026-09-17 18:56:02`; the public identity route answered
200 and the anonymous status route remained 401. All eight caller-visible
records omitted the empty Maintainers field, matching the current wire rule;
the preflight snapshot likewise contained no legacy Maintainers value to
migrate. No live record was changed for this check.

The WEB child remained in its delegated cgroup with the accepted limits:
256 MiB memory, no swap, 64 processes and one CPU. It had zero effective
capabilities and only `AGENT_BUS_ADDR=/bus.sock` and `PWD=/` in its environment.
