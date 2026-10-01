#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export AGENT_BUS_NAME='#stats-rust@localhost'
export AGENT_BUS_TOKEN=$(bash "$here/../tokens.sh" "$AGENT_BUS_NAME")
export CARGO_TARGET_DIR=${CARGO_TARGET_DIR:-"$EXAMPLES_ROOT/tmp/example-rust-target"}
: "${STATS_ROOT:?set STATS_ROOT to the permitted directory}"
exec cargo run --quiet --locked --manifest-path "$here/Cargo.toml"
