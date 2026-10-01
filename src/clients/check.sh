#!/bin/bash
# Isolated client conformance checks, also called by smoke.sh --slow.
# Overrides let mutation runs use scratch sources without touching the tree.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
# A developer's live session never supplies credentials or transport defaults.
for variable in ${!AGENT_BUS_@}; do unset "$variable"; done
mkdir -p "$root/tmp"
work=$(mktemp -d "$root/tmp/client-check.XXXXXX")
trap 'rm -rf "$work"' EXIT
export TMPDIR=$work
case ${1:-} in
  python|rust|typescript)
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
  php)
    "$here/php/tests/daemon.sh" php "${AB_PHP_CLIENT_PATH:-$here/php}/tests/run.php"
    ;;
  typescript)
    bash "$here/typescript/tests/check.sh"
    ;;
  *) echo 'usage: clients/check.sh python|rust|php|typescript' >&2; exit 2 ;;
esac
