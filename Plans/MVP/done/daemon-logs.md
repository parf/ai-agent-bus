# Daemon logs

📌 **TL;DR:** 0.7.2 gives the daemon its [three logs](../../../docs/constitution.md#logs):
an always-on audit log of every administrative action and entity edit, an
error log copied to syslog, and a debug log of every request that is off
unless asked for. Evidence for [K.14 and K.22](../0.7.0-TODO.md#storage-and-identity),
and the infrastructure half of K.21.

## Result

| Area | Built |
|---|---|
| Port | `ports.Journal`: audit entries, reports by severity, request lines, the debug switch; no field can hold a body, token, secret or configuration |
| Adapter | `internal/journal`: `audit.log`, `error.log`, lazily opened `debug.log`, all `0640`; every value quoted so none can start a forged line; error-log lines also to syslog, and a missing syslog said once in the error log |
| Audit | at the API, around every administrative route: register, unregister, manage, owner transfer, account map, user, GitHub refresh, profile, user state, identity removal, group, subscribe, subscriber removal, configure, set secret, enrolment and the debug switch; actor, operation, target, result and the TCP client address |
| Not audited | send, consume, token issue and rotation, sessions and every read |
| Debug | a line per request — time, caller, method, path, status, duration, address — only while on; `-debug-log` at start, or `POST /debug` by the daemon Owner alone |
| Placement | setup makes `/var/log/agent-bus` `agent-busd:adm` `2750` and installs `/etc/logrotate.d/agent-bus` (weekly, eight kept, copy and truncate); a development daemon writes `logs/` beside its database |

## Checks

| Check | Mutation that fails it |
|---|---|
| `api` TestAuditEntriesForEditsAndNoneForTraffic | removing the audit wrapper from `/manage`: five entries for six edits |
| `api` TestDebugLogIsTheOwnersAndOnDemand | dropping the daemon-Owner check on `/debug`: a non-owner's switch answered 200 |
| `api` TestDebugLogIsTheOwnersAndOnDemand | writing request lines whatever the switch: a line appeared with the debug log off |
| `api` TestSuspensionAndReactivationAreAudited | inventing an address for a unix-socket call |
| `journal` TestAValueCannotForgeAnEntry | writing values unquoted: a newline in a target made two lines |
| smoke *the daemon keeps three logs* | entries for a real edit and a refused one, no entry for a send, no canary value or credential in any log, the Owner-only switch |

## Remaining

K.21's exercise of a corrupt token pair beside a refused call needs the typed
token pairs of K.5.
