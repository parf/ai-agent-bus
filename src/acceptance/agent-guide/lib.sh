#!/bin/bash
# Helpers for the agent-guide check: a private test bus, and a private runner on
# it. Never the node's daemon or runner: every path is under the repository's
# tmp/, every credential is minted from the throwaway daemon's own owner.
# Sourced by guide.sh.
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
EX=$ROOT/src/clients/examples/agent-guide
PKG=$EX/packages
pass=0; fail=0

ok()   { echo "  ok   $1"; pass=$((pass+1)); }
bad()  { echo "  FAIL $1${2:+: $2}"; fail=$((fail+1)); }
# has LABEL TEXT REGEX / lacks LABEL TEXT REGEX: the label names the claim.
has()   { if grep -Eq -- "$3" <<<"$2"; then ok "$1"; else bad "$1" "[$2] lacks [$3]"; fi; }
lacks() { if grep -Eq -- "$3" <<<"$2"; then bad "$1" "[$2] still has [$3]"; else ok "$1"; fi; }
eq()    { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "got [$2], want [$3]"; fi; }
sec()   { echo "== $1"; }

# A stale exported scope must not make our actual unit look foreign.
require_guide_scope() {
  local cgroup
  while IFS= read -r cgroup; do
    [[ $cgroup != */"$1" ]] || return 0
  done < /proc/self/cgroup
  printf 'FAIL not inside %s\n' "$1" >&2
  return 1
}

# Ignore only positively foreign units. Short-lived children can exit before
# journald enriches a record: unknown attribution still counts as stray output.
# A foreign process without unit metadata can therefore still fail this check.
journal_silent() {
  local out
  if out=$(
    set -o pipefail
    journalctl -q -b -t agent-busd -t agent-bus-runner -t agent-bus \
      --since "$JOURNAL_START" -o json |
      jq -r --arg guide "${GUIDE_SCOPE:?the guide needs its own scope}" --arg runner "${RUNNER_SCOPE:-}" '
        # A future agent-bus run --sandbox=on creates run-u*.service units;
        # add those fixture units before using that mode in this guide.
        # user@UID.service alone is only a user-manager ancestor, not proof
        # that a record came from outside our user scopes.
        (._SYSTEMD_USER_UNIT //
          (._SYSTEMD_UNIT // "" | if test("^user@[0-9]+\\.service$") then "" else . end)) as $unit |
        select($unit == "" or $unit == $guide or ($runner != "" and $unit == $runner)) |
        .MESSAGE'
  ); then
    eq "$1" "$out" ""
  else
    bad "$1" "journal query or attribution failed: $out"
  fi
}

# A private daemon, its socket and database under tmp/, and an owner token.
guide_up() {
  GB_BIN=$1
  unset AGENT_BUS_ADDR AGENT_BUS_TOKEN AGENT_BUS_NAME AGENT_BUS_CONFIG AGENT_BUS_TLS_FINGERPRINT AGENT_BUS_SESSION_FILE AGENT_BUS_CONTROL_ADDR AGENT_BUS_CONTROL_TOKEN
  mkdir -p "$ROOT/tmp"
  GB_DIR=$(mktemp -d "$ROOT/tmp/agent-guide.XXXXXX")
  export XDG_STATE_HOME=$GB_DIR/state XDG_CACHE_HOME=$GB_DIR/cache
  GB_OWNER=parf@localhost
  GB_PIDS=()
  local port try
  for try in 1 2 3 4 5; do
    port=$((20000 + RANDOM % 20000))
    rm -f "$GB_DIR/bus.db"
    "$GB_BIN/agent-busd" --addr 127.0.0.1:$port --socket "$GB_DIR/bus.sock" --owner "$GB_OWNER" --db "$GB_DIR/bus.db" --create --flush-every 0 >"$GB_DIR/daemon.log" 2>&1 &
    GB_DPID=$!
    for _ in $(seq 1 20); do [ -S "$GB_DIR/bus.sock" ] && break; kill -0 "$GB_DPID" 2>/dev/null || break; sleep 0.05; done
    kill -0 "$GB_DPID" 2>/dev/null && break
    grep -q "address already in use" "$GB_DIR/daemon.log" || break
  done
  for _ in $(seq 1 100); do
    [ -n "$(curl -s --max-time 1 --unix-socket "$GB_DIR/bus.sock" http://unix/status 2>/dev/null)" ] && break
    sleep 0.1
  done
  GB_TOKEN=$(AGENT_BUS_ADDR="$GB_DIR/user-$(id -un).sock" "$GB_BIN/agent-bus" token "$GB_OWNER") \
    || { echo "FAIL the test bus gave no owner token"; cat "$GB_DIR/daemon.log"; return 1; }
  GB_ADDR=$GB_DIR/bus.sock
}

guide_down() {
  local p
  for p in "${GB_PIDS[@]:-}"; do [ -n "$p" ] && kill "$p" 2>/dev/null; done
  for p in "${GB_PIDS[@]:-}"; do [ -n "$p" ] && wait "$p" 2>/dev/null; done
  [ -n "${GB_DPID:-}" ] && { kill "$GB_DPID" 2>/dev/null; wait "$GB_DPID" 2>/dev/null; }
  [ -n "${GB_DIR:-}" ] && { [ -n "${GUIDE_KEEP:-}" ] && echo "kept $GB_DIR" || rm -rf "$GB_DIR"; }
}

# tok NAME: the credential of NAME, minted by the owner of the test bus.
gtok() { AGENT_BUS_ADDR=$GB_ADDR AGENT_BUS_TOKEN=$GB_TOKEN AGENT_BUS_NAME=$GB_OWNER "$GB_BIN/agent-bus" token "$1"; }
# gb NAME ARGS...: the agent-bus CLI as NAME.
gb() { AGENT_BUS_ADDR=$GB_ADDR AGENT_BUS_TOKEN=$(gtok "$1") AGENT_BUS_NAME=$1 "$GB_BIN/agent-bus" "${@:2}"; }
# gbx: the same, exec'd, for `&`: $! is then the binary, not a subshell.
gbx() { AGENT_BUS_ADDR=$GB_ADDR AGENT_BUS_TOKEN=$(gtok "$1") AGENT_BUS_NAME=$1 exec "$GB_BIN/agent-bus" "${@:2}"; }
# answer NAME FROM ARGS...: what FROM's call to NAME answered, the body only.
answer() { gb "$2" call "$1" --wait 15s "${@:3}" 2>/dev/null | jq -r .body; }
# ask NAME FROM FLAG: an ask (--help, --status) and the answer body.
ask() { gb "$2" call "$1" "$3" 2>/dev/null | jq -r .body; }
users() {
  local name
  for name in "$@"; do
    curl -fsS --unix-socket "$GB_ADDR" -H "X-Agent-Bus-Token: $GB_TOKEN" -d "{\"name\":\"$name\",\"create\":true}" http://unix/user >/dev/null || return 1
  done
}
# wait_reader NAME: until one reader waits on NAME's inbox, bounded.
wait_reader() {
  for _ in $(seq 1 100); do
    gb asker@localhost ls "$1" 2>/dev/null | grep -q '"readers":1' && return 0
    sleep 0.1
  done
  return 1
}
# cli_start NAME SCRIPT [run options...]: `agent-bus run` in the foreground of a background job.
cli_start() {
  local name=$1 script=$2; shift 2
  gbx "$GB_OWNER" run "$name" "$script" --allow '*' "$@" >"$GB_DIR/run.$$.${name//[#@\/]/_}.log" 2>&1 &
  CLI_PID=$!
  GB_PIDS+=("$CLI_PID")
  wait_reader "$name"
}
cli_stop() { kill "$CLI_PID" 2>/dev/null; wait "$CLI_PID" 2>/dev/null; }
# within LABEL VALUE LOW HIGH: LOW <= VALUE <= HIGH, all whole seconds.
within() { if [ "$2" -ge "$3" ] && [ "$2" -le "$4" ]; then ok "$1"; else bad "$1" "$2 is outside $3..$4"; fi; }

# A private runner on the test bus, run by a person (launcher@localhost), with
# its own package folder, host folder and inventory, in a delegated cgroup scope.
# runner_up: needs systemd-run --user and the sandbox the runner picks; a host
# without them fails the check, never skips it.
runner_up() {
  RH=$GB_DIR/rh; mkdir -p "$RH/agent.d" "$RH/runner/tokens"
  users launcher@localhost || return 1
  gb launcher@localhost register '#runner/launcher@localhost' --kind agent --allow '@owner' >/dev/null || return 1
  command -v systemd-run >/dev/null || { echo "FAIL no systemd-run: the runner's cgroup scope needs it"; return 1; }
  RUNNER_LOG=$GB_DIR/runner.log
  RUNNER_SCOPE="agent-guide-runner-$$-$RANDOM.scope"
  AGENT_BUS_ADDR=$GB_ADDR AGENT_BUS_TOKEN=$(gtok '#runner/launcher@localhost') \
    systemd-run --user --scope --quiet --unit="$RUNNER_SCOPE" -p Delegate=yes "$GB_BIN/agent-bus-runner" --agents "$RH/agent.d" --host "$RH/runner" >>"$RUNNER_LOG" 2>&1 &
  RUNNER_PID=$!; GB_PIDS+=("$RUNNER_PID")
  for _ in $(seq 1 100); do grep -q 'is the runner' "$RUNNER_LOG" 2>/dev/null && return 0; sleep 0.1; done
  echo "FAIL the runner did not report that it is serving"; cat "$RUNNER_LOG"; return 1
}
# runner_call VERB...: a call to the runner as its person.
runner_call() { gb launcher@localhost call '#runner/launcher@localhost' --wait 20s "$@" | jq -r .body; }
