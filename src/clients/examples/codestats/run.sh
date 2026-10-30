#!/bin/bash
# Starts this example's two workers and serves until SIGTERM.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
tok() { bash "$here/../tokens.sh" "$1"; }
classes=$EXAMPLES_ROOT/tmp/codestats-classes.$$
cp=$(bash "$here/build-java.sh" "$classes") || exit 1
trap 'kill -TERM $(jobs -p) 2>/dev/null; wait; rm -rf "$classes"' TERM INT EXIT
mkdir "$classes/empty-sasl"
SASL_PATH="$classes/empty-sasl" AGENT_BUS_NAME='#stats-php@localhost' AGENT_BUS_TOKEN=$(tok '#stats-php@localhost') \
  php -d log_errors=0 -d display_errors=stderr "$here/stats-worker.php" &
AGENT_BUS_NAME='#stats-java@localhost' AGENT_BUS_TOKEN=$(tok '#stats-java@localhost') java -cp "$cp" StatsWorker &
wait
