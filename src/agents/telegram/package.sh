#!/bin/sh
# Package the Telegram agent for a runner's agent.d: one bundled
# telegram.js (the agent, shared subscription engine/inline worker and
# TypeScript client), with agent.json. The sandbox mounts only this package;
# no runtime import reaches the checkout.
#   src/agents/telegram/package.sh /var/lib/agent-bus/agent.d/telegram
# The shared session router is bundled too; no checkout import remains.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/telegram>}
tmp=$(mktemp -d "${dest%/}.new.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
# The client reads the bus's version file beside the checkout; the bundle has
# no checkout around it, so the version is written in at build time.
AGENT_BUS_BUILD_VERSION=$(cat "$here/../../internal/version/VERSION") \
  bun build "$here/telegram.ts" --target=bun --outfile "$tmp/telegram.js" \
  --env 'AGENT_BUS_BUILD_*' >/dev/null
# The bundle keeps telegram.ts's own #! line; one is added only if it is gone.
head -c 2 "$tmp/telegram.js" | grep -q '#!' || { echo '#!/usr/bin/env bun'; cat "$tmp/telegram.js"; } >"$tmp/telegram.js.x"
[ -e "$tmp/telegram.js.x" ] && mv "$tmp/telegram.js.x" "$tmp/telegram.js"
# The runner's account reads it: the code holds no secret.
install -d -m 0755 "$tmp/licenses"
install -m 0644 "$here/../../../licenses/NOTICE.md" "$tmp/licenses/NOTICE.md"
chmod 755 "$tmp" "$tmp/telegram.js"
sed 's|"command": "./telegram.ts"|"command": "./telegram.js"|' "$here/agent.json" >"$tmp/agent.json"
cp "$here/env.dist" "$tmp/env.dist"
chmod 644 "$tmp/agent.json" "$tmp/env.dist"
grep -q '"command": "./telegram.js"' "$tmp/agent.json"
rm -rf "$dest.old"; [ -e "$dest" ] && mv "$dest" "$dest.old"
mv "$tmp" "$dest"; trap - EXIT
rm -rf "$dest.old"
echo "packaged the Telegram agent into $dest"
