#!/bin/sh
# Self-contained runner package; debug.py remains the executable entry.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/debug>}
tmp=$(mktemp -d "${dest%/}.new.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/support/agents/shared" "$tmp/support/clients/python" "$tmp/support/internal/version"
cp "$here/debug.py" "$here/agent.json" "$here/env.dist" "$tmp/"
cp "$here/../shared/metadata.py" "$tmp/support/agents/shared/"
cp -r "$here/../../clients/python/agent_bus" "$tmp/support/clients/python/"
find "$tmp" -type d -name __pycache__ -exec rm -rf {} +
cp "$here/../../internal/version/VERSION" "$tmp/support/internal/version/"
cp "$here/../../clients/LICENSE.md" "$tmp/support/clients/LICENSE.md"
cp "$here/../../../LICENSE.md" "$tmp/LICENSE.md"
chmod -R a+rX "$tmp"
cp "$here/../../../THIRD-PARTY-NOTICES.md" "$tmp/THIRD-PARTY-NOTICES.md"
chmod 644 "$tmp/THIRD-PARTY-NOTICES.md"
chmod 755 "$tmp" "$tmp/debug.py"
rm -rf "$dest.old"
[ ! -e "$dest" ] || mv "$dest" "$dest.old"
mv "$tmp" "$dest"
trap - EXIT
rm -rf "$dest.old"
