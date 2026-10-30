#!/bin/bash
# Isolated client conformance checks, also called by smoke.sh.
# Overrides let mutation runs use scratch sources without touching the tree.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
source "$here/cargo-target.sh"
# A developer's live session never supplies credentials or transport defaults.
for variable in ${!AGENT_BUS_@}; do unset "$variable"; done
mkdir -p "$root/tmp"
work=$(mktemp -d "$root/tmp/client-check.XXXXXX")
trap 'rm -rf "$work"' EXIT
export TMPDIR=$work
export GOCACHE=${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/go-build}
export XDG_CACHE_HOME=$work/cache
case ${1:-} in
  python|rust|typescript|go)
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
    # The host's memcached extension initializes Cyrus SASL even for HTTP
    # tests. Its DIGEST-MD5 plugin logs at shutdown, outside PHP's logging.
    export SASL_PATH=$work/empty-sasl
    mkdir "$SASL_PATH"
    "$here/php/tests/daemon.sh" php -d log_errors=0 -d display_errors=stderr \
      -d "syslog.ident=${AB_PHP_JOURNAL_ID:-php}" "${AB_PHP_CLIENT_PATH:-$here/php}/tests/run.php"
    ;;
  typescript)
    bash "$here/typescript/tests/check.sh"
    ;;
  java)
    bash "${AB_JAVA_CLIENT_PATH:-$here/java}/tests/run.sh"
    ;;
  examples)
    python3 "$here/examples/tests/entrypoints.py"
    (cd "$here/examples/go" && go vet ./... && go test -race ./...)
    MAX_FILES=${MAX_FILES:-12} READY_SECONDS=${READY_SECONDS:-45} bash "$here/examples/run.sh"
    ;;
  go)
    (cd "${AB_GO_CLIENT_PATH:-$here/go}" && go vet ./... && go test -race ./...)
    ;;
  *) echo 'usage: clients/check.sh python|rust|php|typescript|java|go|examples' >&2; exit 2 ;;
esac
