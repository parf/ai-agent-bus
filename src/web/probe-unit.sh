#!/bin/bash
# Runs test/sandbox-probe.ts under every [Service] directive of
# agent-bus-web.service, as its account: each wall it expects to hold must.
# Run with sudo; --without NAME drops one directive, to see its line fail.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd -P)
drop=${2:-NONE}; [ "${1:-}" = --without ] || drop=NONE
args=()
while IFS= read -r l; do args+=(-p "$l"); done < <(sed -n '/^\[Service\]/,/^\[Install\]/p' "$here/agent-bus-web.service" \
  | grep -E '^[A-Za-z]+=' | grep -vE "^(ExecStart|Type|Restart|RestartSec|WorkingDirectory|$drop)=")
out=$(systemd-run --wait --pipe --quiet --collect "${args[@]}" -p WorkingDirectory="$here" /usr/bin/bun run "$here/test/sandbox-probe.ts" 2>&1)
echo "$out"
! grep -q '^FAIL' <<<"$out"
