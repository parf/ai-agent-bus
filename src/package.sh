#!/bin/bash
# Build the release archives for one platform: the full node, whose
# agent-bus-setup is the installer, and the client edition install.sh puts
# under ~/.local. This script only assembles and checks delivery. GOARCH
# picks another CPU; the compiled TypeScript binary follows it.
set -euo pipefail
cd "$(dirname "$0")"

out=${1:-../tmp/dist}
mkdir -p "$out" ../tmp
out=$(realpath "$out")
work=$(realpath "$(mktemp -d ../tmp/package.XXXXXX)")
trap 'rm -rf "$work"' EXIT

version=$(tr -d '[:space:]' < internal/version/VERSION)
os=$(go env GOOS)
case $os in
  linux|darwin) ;;
  *) echo "agent-bus package supports Linux systemd hosts and reduced macOS nodes, not $os" >&2; exit 1 ;;
esac
case $(go env GOARCH) in
  amd64) arch=x86_64; bun_cpu=x64 ;;
  arm64) arch=aarch64; bun_cpu=arm64 ;;
  *) echo "agent-bus package supports amd64 and arm64, not $(go env GOARCH)" >&2; exit 1 ;;
esac
export BUN_TARGET=bun-$os-$bun_cpu
if [ "$os" = darwin ]; then
  # Both editions use Darwin backends. The full edition installs the reduced
  # launchd daemon setup; it does not enable Linux units or a sandbox.
  export CGO_ENABLED=0 PROGRAMS="./cmd/agent-bus ./cmd/agent-busd ./cmd/agent-bus-admin ./cmd/agent-bus-runner ./cmd/agent-bus-setup"
fi
name="agent-bus-$version-$os-$arch"
root="$work/$name"
mkdir "$root"
./build.sh "$root" >&2

required=(
  agent-bus agent-busd agent-bus-admin agent-bus-runner agent-bus-setup
  agent-bus-token # fixed SSH wrapper, copied by build.sh
  web/server.ts web/agent-bus-web.service
  .claude-plugin/marketplace.json mcp/.claude-plugin/plugin.json mcp/channel.ts mcp/tags.ts
  mcp/commands/ls.md mcp/commands/send.md
  agent-bus-ts # the compiled MCP face, launcher and web face
  launchers/ab-claude launchers/ab-codex launchers/ab-opencode launchers/ab-opencode1
  internal/version/VERSION LICENSE.md licenses/NOTICE.md INSTALL.md
)
(
  cd "$root"
  # The web face's whole tree: its sources change with it, so they are
  # listed as they are, each checked like any other artifact.
  mapfile -t webfiles < <(find web -type f ! -path web/server.ts ! -path web/agent-bus-web.service | sort)
  mapfile -t agentfiles < <(find agents clients -type f | sort)
  sha256sum "${required[@]}" "${webfiles[@]}" "${agentfiles[@]}" > MANIFEST.sha256
  sha256sum -c MANIFEST.sha256 >/dev/null
)

archive="$out/$name.tar.gz"
tar -C "$work" -czf "$archive" "$name"

# The client edition: what joins an existing bus without root — the CLI, the
# runner and its agents, the compiled MCP face and launchers, the plugin.
client="agent-bus-client-$version-$os-$arch"
croot="$work/$client"
mkdir "$croot"
(
  cd "$root"
  tar -cf - agent-bus agent-bus-runner agent-bus-ts launchers .claude-plugin mcp/.claude-plugin mcp/commands \
    agents clients internal/version/VERSION LICENSE.md licenses/NOTICE.md | tar -xf - -C "$croot"
)
(
  cd "$croot"
  find . -type f ! -name MANIFEST.sha256 -printf '%P\n' | LC_ALL=C sort | xargs -d '\n' sha256sum > MANIFEST.sha256
  sha256sum -c MANIFEST.sha256 >/dev/null
)
carchive="$out/$client.tar.gz"
tar -C "$work" -czf "$carchive" "$client"
(
  cd "$out"
  if [ -n "$archive" ]; then sha256sum "$(basename "$archive")" > "$(basename "$archive").sha256"; fi
  sha256sum "$(basename "$carchive")" > "$(basename "$carchive").sha256"
)
if [ -n "$archive" ]; then printf '%s\n%s\n' "$archive" "$archive.sha256"; fi
printf '%s\n%s\n' "$carchive" "$carchive.sha256"
