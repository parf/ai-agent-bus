#!/bin/bash
# P.1b: npm clean-install acceptance, on local tarballs only (no publish, no npm
# login): the refusals and the omitted-file mutants on the host, then the wrapped
# packages in a clean systemd container with npm and no checkout, Go or bun.
#
#   src/acceptance/npm-clean-install.sh [--host-only]
#
# NPM_DIST=<dir> reuses archives package.sh already built; otherwise they are built.
# The container half runs podman and takes the smoke-gate slow lock like any gate.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
cd "$root"
mkdir -p tmp
work=$(mktemp -d tmp/npm-clean-install.XXXXXX)
trap 'rm -rf "$work"' EXIT
dist=${NPM_DIST:-}
if [ -z "$dist" ]; then
  src/package.sh "$PWD/$work/dist" >/dev/null
  dist=$PWD/$work/dist
fi
export NPM_DIST=$dist
python3 -B src/npm/acceptance.py
[ "${1:-}" = --host-only ] && exit 0
arch=$(uname -m)
full=$(ls "$dist"/agent-bus-*-linux-"$arch".tar.gz | grep -v -- '-client-' | head -1)
client=$dist/$(basename "$full" | sed 's/^agent-bus-/agent-bus-client-/')
sums=$work/SHA256SUMS
(cd "$dist" && sha256sum "$(basename "$full")" "$(basename "$client")") >"$sums"
python3 src/npm/pack.py "$full" "$client" "$sums" "$work/tarballs" >/dev/null
bash src/npm/container-check.sh "$work/tarballs"
