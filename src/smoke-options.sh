# Shared by smoke.sh and its mode-contract check; no work is started here.
smoke_options() {
  MODE=${MODE:-fast}; TARGETS=${TARGETS:-}; SHARD=""
  while [ $# -gt 0 ]; do
    case $1 in
      --fast) MODE=fast; TARGETS="" ;;
      --normal|--slow)
        MODE=${1#--}; TARGETS=""
        if [ $# -gt 1 ] && [[ $2 != --* ]]; then TARGETS=$2; shift; fi ;;
      --shard) SHARD=${2:-}; shift ;;
      *) echo "usage: smoke.sh [--fast | --normal [A,F] | --slow [L|all]] [--shard NAME]" >&2; return 2 ;;
    esac
    shift
  done
}

# The caller's bus identity is checked before smoke.sh clears its session.
# Children inherit the plan and run shards, so only the driver checks the lock.
smoke_require_gate_lock() {
  [ "$MODE" = normal ] || [ "$MODE" = slow ] || return 0
  [ -z "$SHARD" ] || return 0
  local status caller holders holder
  status=$(timeout 10s agent-bus status) || {
    echo 'smoke-gate: cannot verify caller; no shards started' >&2; return 2;
  }
  caller=$(python3 -c 'import json,sys
try:
 value=json.load(sys.stdin).get("you")
 if not isinstance(value,str) or not value or any(c.isspace() for c in value): raise ValueError()
 print(value)
except (ValueError,TypeError,AttributeError): sys.exit(1)' <<<"$status") || {
    echo 'smoke-gate: invalid caller status; no shards started' >&2; return 2;
  }
  holders=$(timeout 10s agent-bus holders smoke-gate@parf.us) || {
    echo 'smoke-gate: cannot read holder; no shards started' >&2; return 2;
  }
  holder=$(awk '$1 == "smoke-gate@parf.us" && $2 == "slow" {print $3}' <<<"$holders")
  if [ "$holder" != "$caller" ]; then
    echo "smoke-gate: $caller must hold slow; holder: ${holder:-none}; no shards started" >&2
    return 2
  fi
}
