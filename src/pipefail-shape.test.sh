#!/bin/bash
# J93: under pipefail, `producer | grep -q` and `producer | head` stop the producer with SIGPIPE once the
# match or first line is seen, and a correct answer then carries a failed pipeline status. The scripts
# that run under pipefail use the shapes below instead; this shows each failing the old way and passing
# the new, and a real producer failure still failing.
set -uo pipefail
fails=0
check() { # check NAME WANT-STATUS COMMAND...
  local name=$1 want=$2; shift 2
  "$@" >/dev/null 2>&1; local rc=$?
  if { [ "$want" = ok ] && [ "$rc" = 0 ]; } || { [ "$want" = fail ] && [ "$rc" != 0 ]; }; then echo "ok   $name"
  else echo "FAIL $name (status $rc, wanted $want)"; fails=$((fails + 1)); fi
}
# A producer with far more output than a pipe holds, whose first line is the one wanted, and one that fails.
big() { python3 -c 'import sys; sys.stdout.write("match\n" + "padding\n" * 2000000)'; }
broken() { echo match; return 3; }
old_grep() { big | grep -q match; }
new_grep() { big | grep match >/dev/null; }
old_head() { local first; first=$(big | head -1) && [ "$first" = match ]; }   # an assignment keeps the substitution's status
new_head() { local first; first=$(big | sed -n 1p) && [ "$first" = match ]; }
from_text() { local text; text=$(big); grep -q match <<<"$text"; }
check "grep -q after a large producer is misreported as a failure (the old shape)" fail old_grep
check "a draining grep after the same producer reports the match" ok new_grep
check "head -1 after a large producer is misreported as a failure (the old shape)" fail old_head
check "a draining line selector after the same producer reports the line" ok new_head
check "captured text through a here-string has no producer to kill" ok from_text
# release.sh --rollback with only the current release: grep -vx selects nothing and exits 1, which pipefail
# turns into a dead script before it can say there is no earlier release.
only_current() { printf '/r/1\n'; }
old_rollback() { local prev; prev=$(only_current | grep -vx /r/1 | sed -n 1p); }
new_rollback() { local prev; prev=$(only_current | { grep -vx /r/1 || :; } | sed -n 1p) && [ -z "$prev" ]; }
two_releases() { local prev; prev=$(printf '/r/2\n/r/1\n' | { grep -vx /r/2 || :; } | sed -n 1p) && [ "$prev" = /r/1 ]; }
missing_releases() { local prev; prev=$(ls /nonexistent-releases-dir 2>/dev/null | { grep -vx /r/1 || :; } | sed -n 1p); }
check "release --rollback with only the current release kills the script on the old line" fail old_rollback
check "the tolerated grep reaches the message: an empty previous release" ok new_rollback
check "the tolerated grep still picks the earlier release when there is one" ok two_releases
check "a listing that really fails (no releases directory) still fails" fail missing_releases
real_failure() { broken | grep match >/dev/null; }
check "a producer that really fails still fails the pipeline" fail real_failure
[ "$fails" = 0 ] && echo "pipefail shapes: all checks passed" || { echo "pipefail shapes: $fails failed"; exit 1; }
