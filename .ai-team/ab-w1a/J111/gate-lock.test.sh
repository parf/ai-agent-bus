#!/usr/bin/env bash
# J111: the gate's lock wait. A stub agent-bus answers each `lock` from a plan
# (grant, busy, self, acl) and records every call with its time; holders prints
# one line. Run: bash gate-lock.test.sh [helper], default ./gate-lock.sh.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
helper=${1:-$here/gate-lock.sh}
d=$(mktemp -d "${TMPDIR:-/usr/local/src/ai-agent-bus/tmp}/gate-lock-test.XXXXXX")
trap 'rm -rf "$d"' EXIT
mkdir -p "$d/bin"
cat >"$d/bin/agent-bus" <<'STUB'
#!/usr/bin/env bash
echo "$(date +%s.%N) $*" >>"$STUB_CALLS"
case $1 in
  lock)
    n=$(grep -c ' lock ' "$STUB_CALLS"); answer=$(sed -n "${n}p" "$STUB_PLAN"); [[ -n $answer ]] || answer=$(tail -n 1 "$STUB_PLAN")
    case $answer in
      grant) echo '{"record":"smoke-gate@parf.us","name":"slow","holder":"parf"}' ;;
      busy) echo '{"error":"held by #ab-w2b","kind":"busy"}' >&2; exit 1 ;;
      self) echo '{"error":"you already hold it; use extend","kind":"busy"}' >&2; exit 1 ;;
      acl) echo '{"error":"only the record'"'"'s Owner, Maintainers or Agent may lock it","kind":"acl"}' >&2; exit 1 ;;
    esac ;;
  holders) echo 'smoke-gate@parf.us slow parf 11m left' ;;
  *) exit 3 ;;
esac
STUB
chmod +x "$d/bin/agent-bus"
export PATH="$d/bin:$PATH" STUB_CALLS STUB_PLAN
fail=0
bad() { echo "FAIL $1"; fail=1; }
run() { # label bound pause plan...; sets rc, calls, took, holder, error
  local label=$1; GATE_LOCK_BOUND=$2 GATE_LOCK_PAUSE=$3; shift 3
  STUB_CALLS=$d/$label.calls STUB_PLAN=$d/$label.plan; : >"$STUB_CALLS"; printf '%s\n' "$@" >"$STUB_PLAN"
  local t0; t0=$(date +%s.%N)
  ( source "$helper"; take_gate_lock smoke-gate@parf.us slow "$d/$label.log"
    echo "rc=$? holder=${GATE_LOCK_HOLDER:-} error=${GATE_LOCK_ERROR:-}" >"$d/$label.out" )
  took=$(echo "$(date +%s.%N) - $t0" | bc)
  rc=$(sed -n 's/^rc=\([0-9]*\).*/\1/p' "$d/$label.out"); holder=$(sed -n 's/.* holder=\(.*\) error=.*/\1/p' "$d/$label.out"); error=$(sed -n 's/.* error=//p' "$d/$label.out")
  calls=$(grep -c ' lock ' "$STUB_CALLS")
}
gaps_at_least() { # every gap between lock calls is at least $1 seconds
  awk -v p="$1" '/ lock / { if (prev != "" && $1 - prev < p) bad=1; prev=$1 } END { exit bad }' "$STUB_CALLS"
}

# busy, then held by another parf session, then free: it waits between tries and proceeds.
run busy-then-free 30 1 busy self grant
[[ $rc == 0 ]] || bad "busy-then-free: rc=$rc, want 0"
[[ $calls == 3 ]] || bad "busy-then-free: $calls lock calls, want 3"
gaps_at_least 0.9 || bad "busy-then-free: tries came without the pause"
grep -qE ' (extend|release) ' "$STUB_CALLS" && bad "busy-then-free: the gate extended or released a hold"

# held past the bound: it gives up after the bound, naming the holder, not after a burst of tries.
run held-past-bound 3 1 self
[[ $rc == 1 ]] || bad "held-past-bound: rc=$rc, want 1"
[[ $holder == 'smoke-gate@parf.us slow parf 11m left' ]] || bad "held-past-bound: holder '$holder'"
(( calls >= 2 && calls <= 4 )) || bad "held-past-bound: $calls lock calls in ${took}s, want 2-4"
[[ $(echo "$took >= 2" | bc) == 1 ]] || bad "held-past-bound: gave up after ${took}s, before the bound"
grep -qE ' (extend|release) ' "$STUB_CALLS" && bad "held-past-bound: the gate extended or released a hold"

# any other refusal fails at once, without waiting.
run refused 30 1 acl
[[ $rc == 2 ]] || bad "refused: rc=$rc, want 2"
[[ $calls == 1 ]] || bad "refused: $calls lock calls, want 1"
[[ $error == *'"kind":"acl"'* ]] || bad "refused: error '$error' does not carry the answer"

# granted at once: one call, nothing else.
run granted 30 1 grant
[[ $rc == 0 && $calls == 1 ]] || bad "granted: rc=$rc calls=$calls"

((fail)) && exit 1
echo "gate lock wait: busy waits with a pause, the bound names the holder, other refusals fail at once: green"
