#!/bin/bash
set -eu
cd "$(dirname "$0")"
source ./smoke-options.sh
check() {
  local label=$1 want=$2; shift 2
  local got
  got=$(unset MODE TARGETS; smoke_options "$@"; echo "$MODE:$TARGETS:$SHARD")
  [ "$got" = "$want" ] || { echo "$label: $got, want $want"; exit 1; }
}
check default-fast fast::
check fast-alias fast:: --fast
check normal-changed normal:: --normal
check normal-override normal:A,F: --normal A,F
check slow-override slow:L: --slow L
check release slow:all: --slow all
check normal-shard normal::go_checks --normal --shard go_checks
[ "$(MODE=normal TARGETS=A; smoke_options --shard core; echo "$MODE:$TARGETS:$SHARD")" = normal:A:core ]
for flag in --heavy --all --unknown; do
  rc=0; smoke_options "$flag" >/dev/null 2>&1 || rc=$?
  [ "$rc" = 2 ] || { echo "$flag accepted: $rc"; exit 1; }
done
echo 'normal, fast, slow, inherited shard modes and removed flags: green'

# Run the actual driver up to port allocation in a disposable source tree.
# A port marker proves refusal happened before selection, a build or a shard.
mkdir -p ../tmp
d=$(mktemp -d "$(cd .. && pwd)/tmp/smoke-lock-test.XXXXXX")
trap 'rm -rf "$d"' EXIT
mkdir -p "$d/src/clients" "$d/bin"
cp ./smoke.sh ./smoke-options.sh "$d/src/"
: >"$d/src/clients/cargo-target.sh"
printf '%s\n' 'echo reached-port >>"$PROBE_MARKER"; exit 77' >"$d/src/smoke-port.sh"
cat >"$d/bin/agent-bus" <<'STUB'
#!/bin/bash
case $1 in
  status)
    [ "${PROBE_STATUS_ERROR:-0}" = 0 ] || exit 1
    [ "${PROBE_BAD_JSON:-0}" = 0 ] || { echo broken; exit; }
    printf '{"you":"%s"}\n' "${AGENT_BUS_NAME:-parf}" ;;
  holders)
    [ "${PROBE_HOLDER_ERROR:-0}" = 0 ] || exit 1
    if [ -n "$PROBE_HOLDER" ]; then echo "smoke-gate@parf.us slow $PROBE_HOLDER 14m left"; else echo 'smoke-gate@parf.us holds no locks'; fi ;;
  *) exit 3 ;;
esac
STUB
chmod +x "$d/bin/agent-bus"
export PATH="$d/bin:$PATH" PROBE_MARKER="$d/marker" AGENT_BUS_NAME='#worker@test'
export PROBE_HOLDER='' PROBE_STATUS_ERROR=0 PROBE_HOLDER_ERROR=0 PROBE_BAD_JSON=0
probe() {
  local label=$1 want=$2; shift 2
  rm -f "$PROBE_MARKER"
  local rc=0 output
  output=$(bash "$d/src/smoke.sh" "$@" 2>&1) || rc=$?
  [ "$rc" = "$want" ] || { echo "$label: accepted or wrong refusal ($rc): $output"; exit 1; }
  if [ "$want" = 2 ]; then
    [ ! -e "$PROBE_MARKER" ] || { echo "$label: work began before refusal"; exit 1; }
    [[ $output == *smoke-gate* ]] || { echo "$label: unnamed refusal: $output"; exit 1; }
  else
    [ -e "$PROBE_MARKER" ] || { echo "$label: allowed run never reached port allocation: $output"; exit 1; }
  fi
  PROBE_OUTPUT=$output
}
probe no-lock-normal 2 --normal
[[ $PROBE_OUTPUT == *'holder: none'* ]]
probe no-lock-slow 2 --slow all
PROBE_HOLDER=parf
probe another-holder 2 --normal
[[ $PROBE_OUTPUT == *'holder: parf'* ]]
PROBE_HOLDER='#worker@test'
probe own-lock 77 --normal
AGENT_BUS_NAME=parf PROBE_HOLDER=parf probe gate-identity 77 --normal
PROBE_STATUS_ERROR=1 probe status-error 2 --normal
PROBE_BAD_JSON=1 probe bad-status 2 --normal
PROBE_HOLDER_ERROR=1 probe holder-error 2 --normal
PROBE_HOLDER='' probe fast-needs-no-lock 77 --fast
PROBE_HOLDER='' probe shard-needs-no-second-check 77 --normal --shard core
echo 'driver lock refusal, caller identity, gate identity and shard exemption: green'
