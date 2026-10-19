# Profile authority

📌 **TL;DR:** 0.5.56 gives users one email-only self-service operation and lets
Administrators unban ordinary users without widening administrative scope.

## Result

`POST /profile` derives its target from the authenticated caller and accepts
only `email`. It rejects identity, person name, GitHub name, state and unknown
fields instead of ignoring them. Core requires an active existing user profile,
uses the same normalization and cross-profile uniqueness rule as administrative
edits, preserves every protected field and checkpoints before success. Email is
optional, so a user may clear it.

The caller's user view carries `can_set_email` separately from administrative
`can_edit`; no existing face treats the narrow right as full profile authority.
The existing `/user` operation remains Administrator/Owner-only.

Administrators may activate an ordinary banned user through the lifecycle or
administrative profile path. They still cannot edit or unban themselves, peer
Administrators or the daemon Owner. Lifting the ban also restores access for
the user's directly owned services under the existing suspension rule.

## Checks

- Core tests cover normalization, uniqueness, clearing and re-setting email,
  protected-field preservation, capability separation, inactive, unknown and
  record-only callers, checkpoint recovery and no-write refusals.
- API tests require protected fields and identity to fail strict decoding, keep
  ordinary self `/user` forbidden and distinguish self-email from `CanEdit`.
- Unban tests cover both write paths, protected authority levels and a directly
  owned service before and after the lift.

The final mutation set caught 16/16 named changes: widening the self route,
removing it, changing its target, skipping normalization or uniqueness,
forbidding email clearing, admitting record-only or inactive callers, skipping
the checkpoint, narrowing or widening unban authority, hiding either derived
capability, conflating self-service with administrative editing, and restoring
the stale Owner-only unban explanation on the user page. The first
run's self-unban mutant changed no behavior because the peer-Administrator
guard still refused the caller; that run receives no credit. The corrected
mutant actually granted self-unban and failed the named assertion.

The first byte-frozen slow run passed 597 checks, but a later source sweep found
the stale web sentence and superseded that run; it receives no final evidence
credit. After correcting and mutation-pinning that sentence, the final
byte-frozen run passed 597 checks with `go vet` and the race run green. All 189
corrected source hashes remained unchanged, and the tracked and copied smoke
scripts both had SHA-256
`017c827a9b85242d452cc48de6c1d82a997b688e9a51ac8bec85846024fad05a`.
The documentation check resolved 2,814 local links with no error.

## Limits

This slice adds no web form. Administrator-entered person names already have a
trusted writer; importing Linux passwd GECOS and GitHub profile data remains
pending and must be performed by their trusted adapters, never by accepting a
caller-stated provenance label.

## Live postflight

Commit `ac620ef` was pushed before deployment. `src/build.sh` stamped the
daemon and web binaries as **0.5.56**, `parf@parf.us 2026-09-17 12:41:01`; the
live `agent-busd.service` restarted at 12:41 EDT.

The public identity reported 0.5.56 and Owner `parf@parf`. Anonymous `/status`
remained 401 while the Owner's mapped socket received 200. A protected-field
`POST /profile` returned 400 for unknown `person_name`; `/users` was byte-for-byte
unchanged before and after, so no live profile was edited. The Owner's user view
reported `can_set_email: true` separately from `can_edit: true`.

The web child retained zero capabilities, `NoNewPrivs`, its explicit two-value
environment and its delegated limits: one CPU, 256 MiB memory, no swap and 64
tasks. OpenCode replied through the same session after the restart, the
thirteenth consecutive measured peer-path reconnection.
