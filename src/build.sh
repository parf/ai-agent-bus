#!/bin/bash
# One invocation, one build stamp, every Go program. No source rewriting.
set -euo pipefail
cd "$(dirname "$0")"
out=${1:-.}
build_info="$(id -un)@$(hostname) $(date '+%Y-%m-%d %H:%M:%S')"
go build -ldflags "-X 'github.com/parf/ai-agent-bus/internal/version.Build=$build_info'" -o "$out/" ./cmd/...
