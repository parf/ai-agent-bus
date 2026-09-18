#!/bin/bash
# One invocation, one build stamp, every Go program. No source rewriting.
set -euo pipefail
cd "$(dirname "$0")"
out=${1:-.}
build_info="$(id -un)@$(hostname) $(date '+%Y-%m-%d %H:%M:%S')"
go build -ldflags "-X 'github.com/parf/ai-agent-bus/internal/version.Build=$build_info'" -o "$out/" ./cmd/...
# The web filesystem contains its executable, not host libraries. Keep its
# build stamp identical while producing a standalone executable for bubblewrap.
CGO_ENABLED=0 go build -ldflags "-X 'github.com/parf/ai-agent-bus/internal/version.Build=$build_info'" -o "$out/agent-bus-web" ./cmd/agent-bus-web
# A self-contained runtime tree; npm publication remains an R1 deliverable.
if [ "$out" != . ]; then
    mkdir -p "$out/mcp" "$out/launchers" "$out/internal/version"
    bun build --target=bun mcp/server.ts --outfile "$out/mcp/server.js"
    bun build --target=bun launchers/launcher.ts --outfile "$out/launchers/launcher.js"
    cp internal/version/VERSION "$out/internal/version/VERSION"
    cp launchers/ab-claude launchers/ab-codex launchers/ab-opencode "$out/launchers/"
    ln -sf launchers/ab-claude "$out/ab-claude"
    ln -sf launchers/ab-codex "$out/ab-codex"
    ln -sf launchers/ab-opencode "$out/ab-opencode"
    cp ../LICENSE.md "$out/LICENSE.md"
    cp INSTALL.md "$out/INSTALL.md"
fi
