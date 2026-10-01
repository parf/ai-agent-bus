#!/bin/bash
# Crawls the client sources across every [stats] agent on the bus, then sends
# a message round every [relay] agent. Runs as the harness's caller, #demo@localhost.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
cp=$(bash "$here/build-java.sh" "$EXAMPLES_ROOT/tmp/codestats-crawler.$$") || exit 1
trap 'rm -rf "$EXAMPLES_ROOT/tmp/codestats-crawler.$$"' EXIT
java -cp "$cp" Crawler "${CRAWL_DIR:-$EXAMPLES_ROOT/src/clients}" "${MAX_FILES:-1500}"
