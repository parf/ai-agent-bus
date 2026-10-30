#!/bin/bash
# Count this shard's PHP journal entries, including native extension syslog.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../../.." && pwd)
marker_dir=$(mktemp -d "${TMPDIR:-$root/tmp}/php-journal.XXXXXX") || exit 2
trap 'rm -rf "$marker_dir"' EXIT
export AB_PHP_JOURNAL_ID=ab-php-${marker_dir##*/}
start=$(date '+%Y-%m-%d %H:%M:%S.%6N %z') || exit 2
before=$(journalctl -q -t php -t "$AB_PHP_JOURNAL_ID" --since "$start" -o json 2>&1) || {
  echo "FAIL PHP journal read before tests: $before" >&2; exit 2;
}
"$@"
rc=$?
sleep 1 # syslog reaches journald asynchronously
after=$(journalctl -q -t php -t "$AB_PHP_JOURNAL_ID" --since "$start" -o json 2>&1) || {
  echo "FAIL PHP journal read after tests: $after" >&2; exit 2;
}
count() { python3 -B "$here/journal-count.py" "$AB_PHP_JOURNAL_ID"; }
before_count=$(count <<<"$before") && after_count=$(count <<<"$after") || {
  echo 'FAIL PHP journal attribution could not be read' >&2; exit 2;
}
added=$(( after_count - before_count ))
if [ "$added" != 0 ]; then
  echo "FAIL PHP journal gained $added lines since $start" >&2
  printf '%s\n' "$after" >&2
  exit 3
fi
echo 'ok PHP journal gained 0 lines'
exit "$rc"
