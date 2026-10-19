#!/bin/bash
# Snapshot the canonical release into ignored generated metadata for the crate.
# This packages locally; it never publishes.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
source "$here/../cargo-target.sh"
cp "$here/../../internal/version/VERSION" "$here/VERSION.local"
cargo package --locked --allow-dirty --manifest-path "$here/Cargo.toml" "$@"
