#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
root=$(cd "$here/../../.." && pwd)
package=${AB_TYPESCRIPT_CLIENT_PATH:-$here}
export AGENT_BUS_BUILD_VERSION=$(cat "$root/src/internal/version/VERSION")
work=$(mktemp -d "$root/tmp/ts-check.XXXXXX")
trap 'rm -rf "$work"' EXIT
(cd "$package" && bun install --frozen-lockfile && bun run check)
(cd "$here" && bun test tests/client.test.ts)
AB_CANONICAL_VERSION_FILE="$root/src/internal/version/VERSION" node "$package/build.mjs" "$work/package"
node "$here/tests/node-smoke.mjs" "$work/package/index.js"
