# Restricted empty ACLs

📌 **TL;DR:** Empty ACLs restrict visibility and use; explicit sharing survives a metadata refresh.

## Scope

0.5.44 implements the [accepted ACL default](../../../docs/02-access.md#acl).
Existing snapshots adopt it without rewriting their ACLs. Owners and assigned
Maintainers retain access; the resource keeps its own-inbox right. Unrelated
users, services, Administrators and implicit masters lose both visibility and
use of an empty-ACL resource. Explicit names, groups and wildcard grants retain
their meanings. Standing, owner suspension and Disabled checks remain.

[Upgrade guidance](../../../docs/09-setup.md#empty-acl-upgrade) calls out the
visibility change, reply inboxes and the pending daemon-owner override.
The Personal tag and broader authority changes are not implemented here.

## Registration and runners

Review reproduced a related failure: a metadata re-registration erased grants.
Omitting the ACL now preserves grants and master refusal; explicitly supplying
an ACL replaces them, and management can deliberately clear either setting.
An explicit master refusal can tighten an omitted-ACL refresh.
[Registration](../../../docs/01-identity-and-roles.md#registration) owns the rule.

Script runners accept explicit sharing through flags and service JSON. New
registrations never receive an automatic wildcard. Integration fixtures state
the sharing they require, including access to reply inboxes; the MCP helper
that opens fixture inboxes is imported only by smoke fixtures.

## Checks

- Core checks cover new and serialized/restored records: Owner, Maintainer and
  own-principal positives; unrelated user/service, Administrator and master
  negatives; List, Lookup, Send and Consume. Explicit grants and wildcard
  removal have controls; clearing access cancels an outstanding reader.
- Own-inbox checks deliver real messages with empty and other-service ACLs,
  retaining Disabled and owner-suspension refusals.
- HTTP checks cover hidden/missing equivalence, listing exclusion, explicit
  wildcard sharing, own consumption and subsequent grant removal.
- Refresh checks cover retained grants/refusal, explicit replacement, deliberate
  clearing followed by refresh, and explicit tightening of master refusal.
- Runner checks inspect the actual registration request from flags and JSON;
  a fixture response stops execution before launching a child. Web checks pin
  register/edit help against the restrictive default.
- OpenCode reviewed enforcement, runner plumbing, help, fixtures and refresh
  semantics; its refresh finding was reproduced and fixed.

Final frozen `src/empty-acl-smoke.local.sh --slow`: **589 passed, 0 failed**,
exit 0; vet and race passed (`tmp/empty-acl/slow-final.log`, lines 10–11 and
931). All Go packages passed separately. The frozen script and tracked
`src/smoke.sh` both have SHA-256
`be8db7b2dc8f34234bb58f350d49aea8b329a5d95bf956fd111dff9bc94889c3`.
A hash manifest covering tracked and newly added source files confirmed no
source/version changed during the run (`tmp/empty-acl/source-frozen.json`).

## Mutations

Twelve overlays each failed their named test in a targeted package run, not a
full smoke run per mutation. Logs and replay: `tmp/empty-acl/mutations.log`,
`tmp/empty-acl/mutations/` and `tmp/empty-acl/mutate.py`.

| Changed behavior | Catch |
|---|---|
| Restore open-empty access | New/restored visibility and use matrix |
| Let master reopen empty ACLs | Matrix excludes implicit masters |
| Remove own-principal authority | Own-inbox message delivery |
| Remove assigned-Maintainer authority | Matrix retains Maintainer access |
| Bypass Lookup visibility | Matrix refuses hidden lookup |
| Bypass List visibility | Matrix excludes hidden listing |
| Erase omitted grants on refresh | Explicit grant survives refresh |
| Lift master refusal on refresh | Master remains refused |
| Ignore explicit tightening on refresh | Master loses access |
| Ignore an explicitly replaced nonempty ACL | New recipient gains access |
| Drop runner sharing in registration | Both input forms send intended policy |
| Restore open-empty form wording | Both forms explain restrictive access |

The first replacement mutant intercepted the initial grant too. It was caught,
but the refined overlay permits the first grant and fails specifically at the
later replacement. Its earlier output remains in
`tmp/empty-acl/refresh-ignores-replacement-initial.log`.

## Earlier runs and limits

Exploratory red logs are retained under `tmp/empty-acl/`. The first full slow run
was **586 passed / 3 failed**: a process-title runner fixture had no reply-inbox
grant, while two version checks crossed a version edit during that run. The
fixture was corrected and the final source/version frozen for a fresh run;
that earlier run is not green evidence.

This evidence covers disposable fixtures and package tests. It does not claim
that production ACLs were changed or that the live daemon was upgraded.

## Later live deployment

At the owner's restart instruction, the development installation was rebuilt
with `src/build.sh` and `agent-busd.service` restarted on 2026-09-17 at
01:09 EDT. Public identity and the anonymous web header reported **0.5.44**;
build stamp `parf@parf.us 2026-09-17 01:08:55`. The unit was active/running,
while anonymous `/status` still answered 401.

No live ACLs were widened. The owner could still list its five agent records;
its three unrelated empty-ACL fixture records were no longer visible through
that caller. Four agent records, including Codex and OpenCode, reported attached
readers in the post-restart observation. The fifth had no reader at that instant;
this is not a claim about its health. Codex's same-session bus list succeeded
and showed only its own record, exercising the new visibility default live.
Peer messages now require explicit grants. Local observations are retained in
`tmp/empty-acl/deploy/`.
