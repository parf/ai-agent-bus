# Durable daemon ownership

📌 **TL;DR:** 0.5.55 persists one daemon Owner, gives that Owner node-wide
resource management and transfers the position without widening message access.

## Result

Setup must state the first Owner explicitly. Legacy state takes that seed once;
current snapshots carry the Owner and reject missing, invalid, inactive or
unknown durable authority instead of silently restoring setup's value.

The active Owner can discover, configure, manage, transfer and remove every
service and channel. Administrators do not inherit this override. ACL use stays
separate: empty ACLs remain closed, while the existing implicit master grant on
non-empty ACLs follows the current Owner and still obeys master refusal.

`POST /owner` transfers to a different active registered User. The recipient
joins `@administrators`; the former Owner remains an Administrator. The
checkpoint completes before success, and identity, status, token issuance,
recent traffic and activity use the stored current Owner after transfer.

## Checks

- Core tests cover service and channel management, configuration, visibility,
  Administrator refusal, empty-ACL non-use and dynamic master access.
- Transfer tests cover current-owner-only authority, self, unknown and inactive
  recipients, both Administrator memberships, old/new visibility, Recent and
  implicit master behavior.
- Restore tests cover a one-time legacy seed, a contrary seed after transfer,
  and fail-closed marked snapshots with absent, invalid, inactive or unknown
  owners and an owner without the establishment marker.
- API tests cover transfer, dynamic public identity and status. The configured
  daemon-account socket remains the former Owner after transfer: it can perform
  Administrator group work but cannot use the former root resource override.
- The administrative durability matrix recovers the transferred Owner from the
  last acknowledged checkpoint.

The targeted overlay run caught all **14 of 14** mutations
(`tmp/daemon-owner/mutations.log`): missing or inherited root authority,
management widening an empty ACL, lost dynamic master, unauthorized and self
transfer, missing Administrator nesting, uncheckpointed or unsnapshotted
transfer, startup reseeding, silent repair of a missing Owner, stale public
identity, a missing route and OS-derived ownership.

Final frozen `src/daemon-owner-smoke.local.sh --slow`: **597 passed, 0 failed**,
exit 0; vet and race passed (`tmp/daemon-owner/slow-final-2.log`, lines 10–11
and 953). The frozen and tracked scripts both have SHA-256
`017c827a9b85242d452cc48de6c1d82a997b688e9a51ac8bec85846024fad05a`.
All 187 source/version manifest entries matched after the run. Documentation
validation checked 143 files and 2,811 local links with zero errors.

## Fixture corrections

The first frozen run stayed red at **565 passed, 4 failed**
(`tmp/daemon-owner/slow-final.log`). Its fixtures encoded four superseded
assumptions: a negative daemon start omitted the now-required Owner; an enrolled
record and a dashboard record used the daemon Owner where the check meant an
unrelated caller; and a hand-damaged current snapshot expected setup to recreate
its missing Owner.

The corrected suite states the Owner for the unrelated listener check, uses an
ordinary caller for the enrolment refusal, keeps the master-refused send as the
access control while expecting Owner management visibility, and proves damaged
current state stays down before explicitly removing the marker to exercise the
legacy one-time seed. The failed run receives no release credit.

## Limits

This slice exposes transfer through the API; it adds no web transfer control.
Transfer does not rewrite the daemon OS account's mapped socket, remove the
former Owner's Administrator standing or revoke credentials. Downgrading to an
older daemon ignores durable ownership and temporarily restores its configured
startup owner. Profile self-editing, Administrator unbanning, nested groups,
service-defined roles and historical live-fixture cleanup remain pending.
