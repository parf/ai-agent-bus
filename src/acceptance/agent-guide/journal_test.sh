#!/bin/bash
# Offline attribution checks: journalctl is a function, never the host command.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
source "$here/lib.sh"
GUIDE_JOURNAL=1
GUIDE_SCOPE=guide-fixture.scope
RUNNER_SCOPE=runner-fixture.scope
JOURNAL_START=fixture-start
foreign=1 own="" query_error=0 failures=0
journalctl() {
  [ "$query_error" = 0 ] || return 7
  local args=" $* "
  [[ $args == *" -b "* && $args == *" --since fixture-start "* ]] || return 8
  for tag in agent-busd agent-bus-runner agent-bus; do
    [[ $args == *" -t $tag "* ]] || return 9
  done
  [ "$foreign" = 0 ] || printf '%s\n' '{"_SYSTEMD_USER_UNIT":"foreign.scope","MESSAGE":"#debug@foreign: debug requested failure 7"}'
  case $own in
    guide) printf '%s\n' '{"_SYSTEMD_USER_UNIT":"guide-fixture.scope","MESSAGE":"fixture CLI stray output"}' ;;
    runner-child) printf '%s\n' '{"_SYSTEMD_USER_UNIT":"runner-fixture.scope","MESSAGE":"fixture runner child stray output"}' ;;
    unknown-child) printf '%s\n' '{"_PID":"123","MESSAGE":"short-lived child stray output"}' ;;
    generic-user) printf '%s\n' '{"_SYSTEMD_UNIT":"user@1001.service","MESSAGE":"user-manager ancestor is not foreign proof"}' ;;
    unknown-foreign) printf '%s\n' '{"MESSAGE":"unknown foreign process still fails"}' ;;
  esac
  return 0
}
check() {
  local got
  got=$(fail=0; journal_silent attribution >/dev/null; printf '%s' "$fail")
  if [ "$got" = "$2" ]; then echo "ok   $1"; else echo "FAIL $1: failed checks $got, expected $2"; failures=$((failures+1)); fi
}
check 'foreign output under the same tags is excluded' 0
own=guide
check 'own CLI output remains a failure' 1
own=runner-child
check 'an unrecorded runner descendant remains a failure' 1
own=unknown-child
check 'missing unit metadata never hides a short-lived child' 1
own=generic-user
check 'a shared user-manager ancestor is not foreign attribution' 1
own=unknown-foreign
check 'unattributed foreign output can still fail the gate' 1
own="" query_error=1
check 'journal query failure is never silence' 1
if require_guide_scope "bogus-exported-guide.scope" >/dev/null 2>&1; then
  echo 'FAIL a bogus inherited scope was accepted'; failures=$((failures+1))
else
  echo 'ok   a bogus inherited scope is refused'
fi
# Explicit manager detection accepts degraded managers, not an unavailable bus.
systemctl() { printf '%s\n' "$manager_state"; return "$manager_rc"; }
manager_state=offline manager_rc=1
if guide_user_manager; then echo 'FAIL an absent manager was detected as present'; failures=$((failures+1)); else echo 'ok   an absent user manager is detected'; fi
manager_state=degraded manager_rc=1
if guide_user_manager; then echo 'ok   a degraded user manager is still present'; else echo 'FAIL a degraded manager was skipped'; failures=$((failures+1)); fi
GUIDE_JOURNAL=0
out=$(journal_silent attribution)
if [ "$out" = 'no user systemd manager: journal silence not checked' ]; then echo 'ok   only the journal assertion is skipped with its reason'; else echo 'FAIL no-manager skip lost its reason'; failures=$((failures+1)); fi
[ "$failures" = 0 ]
