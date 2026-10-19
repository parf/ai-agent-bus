#!/bin/bash
# Source from Cargo entrypoints: all worktrees share one build cache unless
# the caller chose another. Cargo serializes concurrent use of the directory.
if [ -z "${CARGO_TARGET_DIR:-}" ]; then
  cargo_tree=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
  if cargo_common=$(git -C "$cargo_tree" rev-parse --path-format=absolute --git-common-dir 2>/dev/null); then
    cargo_tree=$(dirname "$cargo_common")
  fi
  export CARGO_TARGET_DIR="$cargo_tree/tmp/cargo-target"
  unset cargo_tree cargo_common
fi
