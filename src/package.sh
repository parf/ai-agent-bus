#!/bin/bash
# Build the one self-contained MVP installation archive. agent-bus-setup
# inside it is the installer; this script only assembles and checks delivery.
set -euo pipefail
cd "$(dirname "$0")"

out=${1:-../tmp/dist}
mkdir -p "$out" ../tmp
out=$(realpath "$out")
work=$(mktemp -d ../tmp/package.XXXXXX)
trap 'rm -rf "$work"' EXIT

version=$(tr -d '[:space:]' < internal/version/VERSION)
os=$(go env GOOS)
[ "$os" = linux ] || { echo "agent-bus package supports Linux systemd hosts, not $os" >&2; exit 1; }
case $(go env GOARCH) in amd64) arch=x86_64 ;; arm64) arch=aarch64 ;; *) arch=$(go env GOARCH) ;; esac
name="agent-bus-$version-linux-$arch"
root="$work/$name"
mkdir "$root"
./build.sh "$root" >&2

required=(
  agent-bus agent-busd agent-bus-admin agent-bus-runner agent-bus-setup
  agent-bus-token # fixed SSH wrapper, copied by build.sh
  web/server.ts web/agent-bus-web.service
  .claude-plugin/marketplace.json mcp/.claude-plugin/plugin.json mcp/channel.ts
  mcp/commands/ls.md mcp/commands/send.md
  mcp/server.js launchers/launcher.js
  launchers/ab-claude launchers/ab-codex launchers/ab-opencode launchers/ab-opencode1
  internal/version/VERSION LICENSE.md NOTICE.md THIRD-PARTY-NOTICES.md INSTALL.md
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
(
  cd "$out"
  sha256sum "$(basename "$archive")" > "$(basename "$archive").sha256"
)
printf '%s\n%s\n' "$archive" "$archive.sha256"
