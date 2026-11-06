#!/bin/bash
# git-install-logrotate.sh in a temp logrotate.d: one rule for the log set afterwards, and it is the tracked one.
set -eu
cd "$(dirname "$0")"
mkdir -p ../tmp
d=$(mktemp -d "$(cd .. && pwd)/tmp/git-install-logrotate.XXXXXX")
trap 'rm -rf "$d"' EXIT
mkdir "$d/logrotate.d"
# What `agent-bus-setup` leaves: its weekly rule, at the same path.
sed -n '/^const logrotateConf = `/,/^`$/p' cmd/agent-bus-setup/main.go | sed '1s/.*`//;$d' >"$d/logrotate.d/agent-bus"
grep -q weekly "$d/logrotate.d/agent-bus" || { echo "the fixture is not setup's rule: $(cat "$d/logrotate.d/agent-bus")"; exit 1; }
bash ./git-install-logrotate.sh "$d/logrotate.d/agent-bus" >/dev/null
files=$(ls "$d/logrotate.d" | wc -l)
[ "$files" = 1 ] || { echo "want one rule file, got $files: $(ls "$d/logrotate.d")"; exit 1; }
cmp -s "$d/logrotate.d/agent-bus" maintenance/agent-bus.logrotate || { echo "the installed rule is not the tracked one: $(cat "$d/logrotate.d/agent-bus")"; exit 1; }
[ "$(grep -c '^/var/log/agent-bus/\*\.log {' "$d/logrotate.d/agent-bus")" = 1 ] || { echo "the log set is covered more than once"; exit 1; }
! grep -q weekly "$d/logrotate.d/agent-bus" || { echo "setup's weekly rule is still there"; exit 1; }
grep -q 'maxsize 10M' "$d/logrotate.d/agent-bus" && grep -q 'rotate 3' "$d/logrotate.d/agent-bus" || { echo "the 10 MiB and three-archive bounds are missing"; exit 1; }
# Running it again changes nothing (one file, the same bytes).
bash ./git-install-logrotate.sh "$d/logrotate.d/agent-bus" >/dev/null
[ "$(ls "$d/logrotate.d" | wc -l)" = 1 ] && cmp -s "$d/logrotate.d/agent-bus" maintenance/agent-bus.logrotate || { echo "a second run changed the rule"; exit 1; }
echo 'git-install leaves one log rotation rule for the daemon logs, the tracked one: green'
