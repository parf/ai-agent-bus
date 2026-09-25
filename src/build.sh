#!/bin/bash
# One invocation, one build stamp, every Go program. No source rewriting.
set -euo pipefail
cd "$(dirname "$0")"
out=${1:-.}
build_info="$(id -un)@$(hostname) $(date '+%Y-%m-%d %H:%M:%S')"
# A release names the commit it was built from (release.sh passes it).
[ -n "${BUILD_COMMIT:-}" ] && build_info="$build_info $BUILD_COMMIT"
go build -ldflags "-X 'github.com/parf/ai-agent-bus/internal/version.Build=$build_info'" -o "$out/" ./cmd/...
# A self-contained runtime tree; npm publication remains an R1 deliverable.
if [ "$out" != . ]; then
    mkdir -p "$out/mcp" "$out/launchers" "$out/internal/version"
    bun build --target=bun mcp/server.ts --outfile "$out/mcp/server.js"
    bun build --target=bun launchers/launcher.ts --outfile "$out/launchers/launcher.js"
    cp internal/version/VERSION "$out/internal/version/VERSION"
    # The web face is TypeScript run from source by the system bun, under its
    # own account and unit (docs/11-processes.md#the-web-face): its sources and
    # unit, without tests, development scripts or build tooling.
    mkdir -p "$out/web"
    tar -C web --exclude=node_modules --exclude=test --exclude=bun.lock \
        --exclude=install-dev.sh --exclude=probe-unit.sh --exclude='*.local*' -cf - . | tar -xf - -C "$out/web"
    cp launchers/ab-claude launchers/ab-codex launchers/ab-opencode "$out/launchers/"
    ln -sf launchers/ab-claude "$out/ab-claude"
    ln -sf launchers/ab-codex "$out/ab-codex"
    ln -sf launchers/ab-opencode "$out/ab-opencode"
    cp ../LICENSE.md "$out/LICENSE.md"
    cp INSTALL.md "$out/INSTALL.md"
fi
