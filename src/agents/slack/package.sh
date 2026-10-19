#!/bin/sh
# Package the Slack agent for a runner's agent.d: one bundled slack.js (the
# agent, the TypeScript client and its dependency) and its filter worker,
# with an agent.json naming the bundle. The sandbox mounts the agent's own
# directory alone, so nothing outside it may be imported at run time.
#   src/agents/slack/package.sh /var/lib/agent-bus/agent.d/slack
set -eu
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: package.sh <agent.d/slack>}
tmp=$(mktemp -d "${dest%/}.new.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
# The client reads the bus's version file beside the checkout; the bundle has
# no checkout around it, so the version is written in at build time.
AGENT_BUS_BUILD_VERSION=$(cat "$here/../../internal/version/VERSION") \
  bun build "$here/slack.ts" "$here/filter-worker.ts" --target=bun --outdir "$tmp" \
  --env 'AGENT_BUS_BUILD_*' >/dev/null
# The bundle keeps slack.ts's own #! line; one is added only if it is gone.
head -c 2 "$tmp/slack.js" | grep -q '#!' || { echo '#!/usr/bin/env bun'; cat "$tmp/slack.js"; } >"$tmp/slack.js.x"
[ -e "$tmp/slack.js.x" ] && mv "$tmp/slack.js.x" "$tmp/slack.js"
# The runner's account reads it: the code holds no secret.
chmod 755 "$tmp" "$tmp/slack.js"; chmod 644 "$tmp/filter-worker.js"
sed 's|"command": "./slack.ts"|"command": "./slack.js"|' "$here/agent.json" >"$tmp/agent.json"
chmod 644 "$tmp/agent.json"
grep -q '"command": "./slack.js"' "$tmp/agent.json"
rm -rf "$dest.old"; [ -e "$dest" ] && mv "$dest" "$dest.old"
mv "$tmp" "$dest"; trap - EXIT
rm -rf "$dest.old"
echo "packaged the Slack agent into $dest"
