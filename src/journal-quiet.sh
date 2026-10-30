#!/bin/bash
# journal-quiet.sh <name pattern> <command>...: run the command, then fail if the user journal gained an entry for a
# container or podman event whose name matches the pattern while it ran. Normal and fast runs write nothing to the
# host journal (CLAUDE.md#verification); J32's leaked fixture wrote 20,000 lines a minute there.
# Exit: the command's own on silence, 3 when the journal gained entries, 2 when the journal cannot be read.
set -u
pattern=${1:?usage: journal-quiet.sh <name pattern> <command>...}
shift
cursor=$(journalctl --user -n0 --show-cursor 2>/dev/null | sed -n 's/^-- cursor: //p')
[ -n "$cursor" ] || { echo "journal-quiet: the user journal cannot be read, so its silence would prove nothing"; exit 2; }
"$@"
rc=$?
sleep 1   # journald takes what it is sent asynchronously
hits=$(journalctl --user -q -o verbose --after-cursor "$cursor" 2>/dev/null |
  grep -E "^ *(CONTAINER_NAME|PODMAN_NAME)=.*($pattern)" | sort | uniq -c)
if [ -n "$hits" ]; then
  echo "journal-quiet: the host journal gained entries for $pattern:"
  echo "$hits" | head -5
  exit 3
fi
echo "journal-quiet: no journal entry for $pattern"
exit "$rc"
