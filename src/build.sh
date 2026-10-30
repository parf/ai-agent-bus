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
    install -m 0755 agent-bus-token "$out/agent-bus-token"
    mkdir -p "$out/mcp" "$out/launchers" "$out/internal/version"
    # The MCP face, the launcher and the web face as one self-contained
    # binary (ts-main.ts): a release needs no bun or Node. BUN_TARGET picks
    # another platform, e.g. bun-linux-arm64 beside GOARCH=arm64.
    AGENT_BUS_BUILD_VERSION=$(cat internal/version/VERSION) AGENT_BUS_BUILD_INFO="$build_info" \
        bun build --compile --target="${BUN_TARGET:-bun}" ts-main.ts --outfile "$out/agent-bus-ts" --env 'AGENT_BUS_BUILD_*' >/dev/null
    cp internal/version/VERSION "$out/internal/version/VERSION"
    mkdir -p "$out/.claude-plugin" "$out/mcp/.claude-plugin" "$out/mcp/commands"
    cp .claude-plugin/marketplace.json "$out/.claude-plugin/"
    # The plugin starts the compiled face by its name on PATH (setup links it).
    sed 's|"command": "bun",|"command": "agent-bus-mcp",|; s|"args": \["${CLAUDE_PLUGIN_ROOT}/channel.ts"\]|"args": ["channel"]|' \
        mcp/.claude-plugin/plugin.json >"$out/mcp/.claude-plugin/plugin.json"
    grep -q '"command": "agent-bus-mcp"' "$out/mcp/.claude-plugin/plugin.json"
    cp mcp/channel.ts mcp/tags.ts "$out/mcp/"
    cp mcp/commands/ls.md mcp/commands/send.md "$out/mcp/commands/"
    # The web face is TypeScript run from source by the system bun, under its
    # own account and unit (docs/08-processes.md#the-web-face): its sources and
    # unit, without tests, development scripts or build tooling.
    mkdir -p "$out/web"
    tar -C web --exclude=node_modules --exclude=test --exclude=bun.lock \
        --exclude=install-dev.sh --exclude=probe-unit.sh --exclude='*.local*' -cf - . | tar -xf - -C "$out/web"
    # The release's unit runs the compiled face; setup fills @ROOT@.
    sed 's|^ExecStart=/usr/bin/bun run @WEB_DIR@/server.ts$|ExecStart=@ROOT@/agent-bus-ts web|; s|^ExecPaths=/usr/bin/bun |ExecPaths=@ROOT@/agent-bus-ts |' \
        web/agent-bus-web.service >"$out/web/agent-bus-web.service"
    grep -q '^ExecStart=@ROOT@/agent-bus-ts web$' "$out/web/agent-bus-web.service"
    grep -q '^ExecPaths=@ROOT@/agent-bus-ts ' "$out/web/agent-bus-web.service"
    cp launchers/ab-claude launchers/ab-codex launchers/ab-opencode launchers/ab-opencode1 "$out/launchers/"
    cp launchers/review-settings.json.dist "$out/launchers/"
    ln -sf launchers/ab-claude "$out/ab-claude"
    ln -sf launchers/ab-codex "$out/ab-codex"
    ln -sf launchers/ab-opencode "$out/ab-opencode"
    ln -sf launchers/ab-opencode1 "$out/ab-opencode1"
    cp ../LICENSE.md "$out/LICENSE.md"
    cp ../NOTICE.md "$out/NOTICE.md"
    cp ../THIRD-PARTY-NOTICES.md "$out/THIRD-PARTY-NOTICES.md"
    cp INSTALL.md "$out/INSTALL.md"
    agents/package.sh "$out" >&2
fi
