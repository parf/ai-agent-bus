#!/bin/bash
# Count the system journal's PHP entries, including native extension syslog.
set -uo pipefail
start=$(date '+%Y-%m-%d %H:%M:%S.%6N %z') || exit 2
before=$(journalctl -q -t php --since "$start" -o json 2>&1) || {
  echo "FAIL PHP journal read before tests: $before" >&2; exit 2;
}
"$@"
rc=$?
sleep 1 # syslog reaches journald asynchronously
after=$(journalctl -q -t php --since "$start" -o json 2>&1) || {
  echo "FAIL PHP journal read after tests: $after" >&2; exit 2;
}
count() { if [ -z "$1" ]; then echo 0; else printf '%s\n' "$1" | wc -l; fi; }
added=$(( $(count "$after") - $(count "$before") ))
if [ "$added" != 0 ]; then
  echo "FAIL PHP journal gained $added lines since $start" >&2
  printf '%s\n' "$after" >&2
  exit 3
fi
echo 'ok PHP journal gained 0 lines'
exit "$rc"
