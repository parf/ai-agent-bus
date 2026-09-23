#!/bin/bash
# Real ps output and real traffic; also run by smoke.sh --slow.
set -euo pipefail
cd "$(dirname "$0")"
BIN=$(realpath "${1:?directory containing the built programs}")
VERSION=$(cat internal/version/VERSION)
fail() { echo "  FAIL $*"; exit 1; }
same() { [ "$2" = "$3" ] || fail "$1: wanted [$3], got [$2]"; echo "  ok   $1"; }

[[ "$VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || fail "release version is SemVer"
stamp=$("$BIN/agent-busd" --version | sed -n 's/^build_info: //p')
prefix="$(id -un)@$(hostname) "
[[ "$stamp" == "$prefix"* && "${stamp#"$prefix"}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}$ ]] || fail "build_info records the builder and build time: [$stamp]"
if [ -n "${BUILD_STARTED:-}" ]; then
  built=$(date -d "${stamp#"$prefix"}" +%s)
  [ "$built" -ge "$BUILD_STARTED" ] && [ "$built" -le "$BUILD_FINISHED" ] || fail "build_info dates this build"
fi
for program in cmd/*; do
  name=${program##*/}
  same "$name shares the release version and build_info" "$(timeout 5 "$BIN/$name" --version)" "$(printf '%s\nbuild_info: %s' "$VERSION" "$stamp")"
done
same "MCP shares the release version without a bus" \
  "$(AGENT_BUS_ADDR=/nonexistent timeout 5 bun run mcp/server.ts --version)" "$VERSION"

mkdir -p ../tmp
D=$(mktemp -d "$(realpath ../tmp)/title.XXXXXX")
SUP=""; RUN=""
cleanup() {
  if [ -n "$RUN" ]; then kill "$RUN" 2>/dev/null || :; wait "$RUN" 2>/dev/null || :; fi
  if [ -n "$SUP" ]; then kill "$SUP" 2>/dev/null || :; wait "$SUP" 2>/dev/null || :; fi
  rm -rf "$D"
}
trap cleanup EXIT
export XDG_CACHE_HOME=$D/cache XDG_STATE_HOME=$D/state
export AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_NAME=owner@test
env -u AGENT_BUS_ROLE -u AGENT_BUS_FDS "$BIN/agent-busd" \
  --addr 127.0.0.1:0 --socket "$D/bus.sock" --owner owner@test \
  --db "$D/bus.db" --create --flush-every 0 >"$D/daemon.log" 2>&1 &
SUP=$!
ready=0
for _ in {1..50}; do
  if [ "$(curl -s --max-time 1 --unix-socket "$D/bus.sock" -o /dev/null -w '%{http_code}' http://unix/status)" = 401 ]; then ready=1; break; fi
  sleep 0.1
done
[ "$ready" = 1 ] || { cat "$D/daemon.log"; fail "daemon ready with preserved arguments"; }
export AGENT_BUS_TOKEN=$(AGENT_BUS_ADDR="$D/user-$(id -un).sock" "$BIN/agent-bus-token" owner@test)
BUS=$(pgrep -P "$SUP" -x agent-busd)
title() { ps -ww -o args= -p "$1" | sed 's/[[:space:]]*$//'; }
await_title() {
  local actual=""
  for _ in {1..40}; do
    actual=$(title "$2") || break
    if [ "$actual" = "$3" ]; then echo "  ok   $1"; return; fi
    sleep 0.1
  done
  fail "$1: wanted [$3], got [$actual]"
}
await_title "supervisor identifies its version and role" "$SUP" "agent-busd $VERSION ; supervisor"
# Let every readiness request reach the title before taking the baseline.
sleep 1.1
before=$(title "$BUS" | sed -n 's/.* ; Calls: \([0-9]*\) ; bus$/\1/p')
[[ "$before" =~ ^[0-9]+$ ]] || fail "bus title contains a call count"
for _ in {1..3}; do
  same "refused request reaches the bus" \
    "$(curl -s --max-time 2 --unix-socket "$D/bus.sock" -o /dev/null -w '%{http_code}' http://unix/status)" 401
done
"$BIN/agent-bus" status >/dev/null
AGENT_BUS_ADDR="$D/user-$(id -un).sock" AGENT_BUS_TOKEN= "$BIN/agent-bus" status >/dev/null
await_title "bus counts accepted and refused requests on both socket kinds" "$BUS" "agent-busd $VERSION ; Calls: $((before+5)) ; bus"
sleep 1.1
same "idle bus does not invent calls" "$(title "$BUS")" "agent-busd $VERSION ; Calls: $((before+5)) ; bus"

# Title rewriting must preserve both the command flags and inherited env
# when the supervisor replaces its child. A process lifetime starts at zero.
kill -KILL "$BUS"
NEW=""
for _ in {1..50}; do
  NEW=$(pgrep -P "$SUP" -x agent-busd || :)
  [ -n "$NEW" ] && [ "$NEW" != "$BUS" ] && break
  sleep 0.1
done
[ -n "$NEW" ] && [ "$NEW" != "$BUS" ] || fail "supervisor replaces its bus"
await_title "replacement bus resets its calls" "$NEW" "agent-busd $VERSION ; Calls: 0 ; bus"
timeout 5 "$BIN/agent-bus" status >/dev/null
await_title "replacement bus still serves the inherited socket" "$NEW" "agent-busd $VERSION ; Calls: 1 ; bus"

# The owner is a User; the agent's own ACL admitting it is what lets the reply in.
"$BIN/agent-bus" start '#title@test' --allow owner@test --algo args 'printf "%s"' -4 >"$D/runner.log" 2>&1 &
RUN=$!
await_title "runner identifies its version and service" "$RUN" "agent-bus-runner $VERSION ; Calls: 0 ; #title@test"
for i in {1..5}; do
  same "runner still executes its script and returns its argument" \
    "$(timeout 8 "$BIN/agent-bus" call '#title@test' --wait 5s "hello-$i" 2>>"$D/runner.log" | sed -n 's/.*"body":"\([^"]*\)".*/\1/p')" "hello-$i"
done
await_title "runner counts messages taken" "$RUN" "agent-bus-runner $VERSION ; Calls: 5 ; #title@test"
sleep 1.1
same "blocked runner does not invent calls" "$(title "$RUN")" "agent-bus-runner $VERSION ; Calls: 5 ; #title@test"
same "build_info is build time, not query time" "$("$BIN/agent-busd" --version)" "$(printf '%s\nbuild_info: %s' "$VERSION" "$stamp")"
