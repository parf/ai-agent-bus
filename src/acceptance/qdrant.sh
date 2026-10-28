#!/usr/bin/env bash
# Owner-approved commissioning only: restarts Qdrant and refuses live collections.
set -euo pipefail
setup="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)/agent-bus-setup"
output=$("$setup" --qdrant check) || { printf '%s\n' "$output"; exit 1; }
printf '%s\n' "$output"
for check in \
  'PASS full snapshot recovered with read-only rootfs and writable storage init marker' \
  'PASS imported snapshot ownership 1000:1000' \
  'PASS installed generator accepted every rendered Quadlet key'; do
  if ! [[ "$output" == *"$check"* ]]; then
    printf 'FAIL missing acceptance: %s\n' "$check" >&2
    exit 1
  fi
done
