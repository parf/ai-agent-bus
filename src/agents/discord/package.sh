#!/bin/sh
# A self-contained Bun agent; the shared engine and router are bundled in.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/discord>}
tmp=$(mktemp -d "${dest%/}.new.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
AGENT_BUS_BUILD_VERSION=$(cat "$here/../../internal/version/VERSION") \
  bun build "$here/discord.ts" --target=bun --outfile "$tmp/discord.js" --env 'AGENT_BUS_BUILD_*' >/dev/null
install -d -m 0755 "$tmp/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$tmp/licenses/NOTICE.md"
install -m 0644 "$here/../../../LICENSE.md" "$tmp/LICENSE.md"
sed 's|"command": "./discord.ts"|"command": "./discord.js"|' "$here/agent.json" > "$tmp/agent.json"
install -m 0644 "$here/env.dist" "$tmp/env.dist"
chmod 0755 "$tmp" "$tmp/discord.js"
chmod 0644 "$tmp/agent.json"
rm -rf "$dest.old"; [ ! -e "$dest" ] || mv "$dest" "$dest.old"
mv "$tmp" "$dest"; trap - EXIT
rm -rf "$dest.old"
