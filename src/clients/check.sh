#!/bin/bash
# Isolated client conformance checks, also called by smoke.sh --slow.
# Overrides let mutation runs use scratch sources without touching the tree.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
mkdir -p "$root/tmp"
work=$(mktemp -d "$root/tmp/client-check.XXXXXX")
trap 'rm -rf "$work"' EXIT
export TMPDIR=$work
case ${1:-} in
  python|rust)
    (cd "$root/src" && go build -o "$work/fixture" ./clients/fixture)
    export AB_CLIENT_FIXTURE=$work/fixture AGENT_BUS_FIXTURE_BINARY=$work/fixture
    ;;
esac
case ${1:-} in
  python)
    PYTHONPATH=${AB_PYTHON_CLIENT_PATH:-$here/python} python3 -m unittest discover -s "$here/python/tests" -v
    ;;
  rust)
    cargo test --locked --manifest-path "${AB_RUST_CLIENT_PATH:-$here/rust}/Cargo.toml"
    ;;
  *) echo 'usage: clients/check.sh python|rust' >&2; exit 2 ;;
esac
