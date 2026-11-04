# Remove prerelease compatibility before R1

📌 **TL;DR:** R1 supports clean installation, current-format updates and one canonical form for each bus input.
Remove prerelease readers, aliases, migrations and generated recommendations together.
Keep current authority, delivery uncertainty, transactional safety and supported external standards.

## Scope

The owner requires clean R1 inputs, with no preservation or conversion of prerelease
formats. Current-format installation, idempotent configuration and code updates remain
supported through the same setup entry. Current-operation recovery preserves transaction
safety; it must validate the complete current manifest and current schema, never accept an
old package/layout adapter. No one-off conversion is required or shipped. No
`Plans/R1.0.1/` deferral applies.

The initial inventory was read at `01dbbb77`; implementation starts from `562de7d8`.
“Current use” below identifies source callers and fixtures, not observed live installations.
This file records scope and acceptance, not completed runtime verification or deployment.

## Removal inventory

Every row includes all first-party callers, package inventories, scripts, source examples,
help, goldens and current guides. Reject retired input explicitly; do not silently discard
an unknown field or translate it into another supported form.

| Family and code/test evidence | Current use and disposition | Removal acceptance |
|---|---|---|
| Personal boolean: `src/internal/protocol/{envelope,tags}.go`, core `{bus,manage,commit,snapshot,transfer}.go`; `personal_test.go`, `tags_test.go`, owner isolation tests | Remove `Personal`, JSON `personal` and `--personal`; use the `personal` tag. Launcher, web, CLI and fixture producers must change together. | Tag predicates preserve owner-only classification, private group/user rules and self-refresh. Reject boolean inputs; positive tag-only create/manage/import tests. No boolean-only record conversion. |
| Scalar Maintainer: protocol `MaintainerList.UnmarshalJSON`; core record tests; MCP `Record_` | Remove scalar decoding; arrays are canonical. | Reject scalar and wrong-cased fields; accept ordered canonical arrays; update request and listing goldens. |
| Empty tags and old snapshot/account payloads: core `snapshot.go`, `tags.go`, `account.go`, `transfer.go`; snapshot/account tests | Remove prerelease defaults/conversions and old import representations. | Fresh current-format round trips retain authority, counters, keys and queues. Reject incompatible formats without creating empty replacement state. |
| SQL schema migration: `src/internal/store`, store migration tests | Remove prerelease ALTER/copy/adoption paths. Fresh schema is the only installation schema. | Fresh creation succeeds; incompatible schema is refused without conversion or record restoration. Current transaction/commit recovery remains covered. |
| CLI spellings: `src/cmd/agent-bus`, `src/cmd/agent-busd`, setup parser tests; help goldens | Remove ignored `--web`, single-dash long flags, `-h`, `--agent`, verb aliases (`get`, `list`, template aliases, channel creation aliases), and implicit runner `#`. | Canonical `--help`, `ls --table`, full `#name` and `--service-addr` appear in every owned caller. Reject retired spellings before effects. Keep `--flag=value`, `--flag value`, legitimate short options and documented positional forms. |
| Setup/upgrade: `src/cmd/agent-bus-setup/{install,upgrade,rollback}.go`, setup tests | Remove old rollback payload readers, syslog conversion and destructive --reinstall cutover. Keep current-format --upgrade and --recover. | Clean install, current-format update/configuration and interrupted-operation recovery succeed. Incompatible packages/schema refuse before changes; foreign paths remain fail-closed. |
| Launcher bindings: `src/launchers/{sessions,identity,launcher}.ts`, `local.test.ts`, `smoke.ts` | Remove base-less inference, automatic dot-name conversion and fixed `pid` reclamation. Keep current session rename, numbered claims and realm-bound resume. | Current `{name,base}` resumes; missing base refuses; fixed PID state is not reclaimed. UUID lock race tests still protect another live claimant. |
| Launcher source bundles: four `src/launchers/ab-*` wrappers, source entrypoint tests | Remove `launcher.js` preference. Checkout runs `.ts`; packaged release runs its compiled binary. | A stale `.js` cannot replace source execution; both canonical source and release layouts retain tests. |
| Claude plugin source: `src/launchers/plugin-steps.ts`, `plugin-steps.test.ts` | Keep source replacement for current-format deployment; remove assumptions about old bundle formats. | Fresh setup and current-format source replacement work under the plugin lock; no obsolete bundle path is installed. |
| SDK/MCP protocol: six `src/clients` protocol implementations; `src/mcp/{protocol,bus,server}.ts`; shared `identity.json` and `refusals.json` | Require protocol 1 and minimum 1 exactly. Remove absent-as-zero, old-daemon warnings APIs and operation-error rewriting. | Every SDK and MCP rejects absent, zero, malformed and unsupported future pairs before application requests. Current pair succeeds; refusal kind/cause remains intact. |
| SDK socket guessing: six SDK discovery functions and discovery tests | Remove `/tmp/agent-bus-<uid>.sock` fallback. Preserve explicit transport/address, current XDG discovery and installed system endpoints. | Missing discovery never targets a world-writable temporary socket; tokenless callers retain user endpoint selection. |
| Record live state: MCP `Record_`, web `ctx.ts`, `agent-status.tsx`, examples `run.sh`, launcher smoke mocks | Remove `reading` boolean; `readers` is the current count. | Readiness and sorting consume the count; no writer/fixture emits `reading`. Reject retired input where it was accepted. |
| Web addresses: `src/web/pages/{index,records}.tsx`, route tests | Remove `/channels`, `/channels/new`, `/channel`, `/channel/edit` redirect aliases. | Canonical record URLs work; removed paths fail; all links/forms/current docs use canonical routes. |
| Subscription migration engine: `src/agents/shared/subscription_migration.py`, subscribers and package manifests/tests | Remove old IDs, journals, checkpoints, conversion helpers and package entries. Includes cron, at, IMAP, logwatch and chat callers. | Current subscription identity/author/selectors remain exact; no migration code ships; retired rows are rejected without sending/replaying. |
| Logwatch migration: `standalone/logwatch/{subscription_migration,logwatch}.py`, `test_logwatch.py` | Remove old fields/IDs/filter conversion and migration checkpoint handling. | Current authors, targets and filters work; malformed/obsolete rows cannot enable delivery; packaging contains no removed helper. |
| Cron/at migration: shared `scheduler.py`, cron/at tests | Remove noncanonical ID moves, `migrated_from`, paused-state adoption and obsolete outcome adoption. Keep current `sending` recovery and settled-claim cleanup. | Current one-shot restart, slot deduplication, ACL recheck and uncertain-send handling pass. Old rows never become fresh scheduled sends. |
| IMAP migration: `forwarding.py`, `progress.py`, `mailbox_session.py`, `imap.py`, cache tests | Remove ledger/ID/journal conversions and cache-to-progress adoption; use one canonical internal state layout. | Current discovery/delivery cursors remain distinct; UIDVALIDITY reset, bounded retry, missing bodies and no redelivery stay covered. No conversion prerequisite. |
| Git-install old service cleanup: `src/git-install.sh`, release/deploy and install tests | Remove old timer/logwatch unit cleanup/adoption and previous-layout suggestions. | Only canonical layouts are produced or updated; foreign and obsolete installations are refused rather than converted. |

## Current behavior that remains supported

| Family | Reason and acceptance boundary |
|---|---|
| Route-correlated replies without `re` | This remains a current message contract, not an obsolete daemon fallback. Six SDKs must still reject late/unmatched replies as work and correlate current replies correctly. Rename misleading “legacy” descriptions without weakening correlation tests. |
| Current durability and uncertainty | SQLite transactions, queue checkpoints, current bigfile reconciliation, in-flight scheduler claims, subscription delivery uncertainty and cleanup after a failed fresh install prevent loss/replay. Removing a prerelease converter does not remove these protections. |
| External standards | ECMAScript regex case semantics, IMAP UTF-7/INBOX/capabilities, HTTP Expect interoperability and current MCP transports are external protocol support. Do not misclassify dependency/standard support as bus compatibility. |
| Runtime support | OpenCode 1 and 2, Claude and Codex are explicit supported external runtimes. Runtime metadata omissions must be evaluated against their current contracts, not deleted on a “legacy” test label alone. |
| Naming and optional values | Explicit valid dot names, numbered suffixes, optional current configuration/help fields, supported default values and selected operating systems remain functional. Automatic names have one slash-based form. |

## Scripts and docs closure

Audit `src/{git-install,release,build,smoke,check-deploy}.sh`, setup/package scripts,
`src/agents/{package,package-tree}.sh`, acceptance fixtures, each SDK check/daemon script,
`src/clients/examples`, launcher/MCP/web smoke scripts and release inventories. Update
source-linked excerpts when their source changes. Validate current docs, source READMEs,
CLI help/manifests/goldens and installation examples against the same final contract.
Historical plans may describe removed behavior only as history; current guides must never
recommend a retired flag, route, field, package layout or conversion operation.

## Sequence and verification

1. Land canonical core wire/storage/parser contracts and their negative fixtures.
2. Land SDKs, launchers, MCP, web and agent callers with shared vectors and source examples.
3. Close install/release/deploy/package scripts and documentation references against the
   integrated tree; remove dead helpers/imports, not just calls to them.
4. Run focused mocked checks per family. For authority, storage, parser, runner and install
   changes, hold the required gate lock and show a compiling mutant fail the owning assertion.
   For other behavior fixes, show the new regression fail before and pass after.
5. Run the required normal gate on the final integrated tree, then release checks as required.
   Report exact commands, counts, failed assertions and unresolved omissions. A static scan or
   mocked handshake does not verify runtime delivery, clean installation or provider behavior.

No live state conversion, deletion, deployment or external-provider call is implied by this
plan. Inventory completeness remains bounded by source inspection: repeat the compatibility
and serialization-default sweep after integration, and report any remaining supported form
with its actual current contract and test evidence.
