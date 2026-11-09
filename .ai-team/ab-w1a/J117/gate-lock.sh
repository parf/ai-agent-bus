# Sourced by gate.sh: take the smoke-gate lock, waiting a bounded time (J111).
# The bus answers a hold by another parf session at once ("you already hold it;
# use extend", kind busy), so a try count burns out in a second: the bound is
# elapsed time, with a pause after every busy answer. Any other refusal (no
# right on the record, a bad record, no daemon) fails at once. The gate never
# extends or releases a hold it did not take.
# take_gate_lock <record> <name> <log>: 0 granted; 1 still busy when the bound
# passed, with the holders line in GATE_LOCK_HOLDER; 2 refused for another
# reason, with the answer in GATE_LOCK_ERROR.
# Self-wait: a grant that lands just as the 60 s client wait ends is held by
# this gate, yet the next try reads "already hold" like another session's hold
# and waits on it. It is bounded: that hold lapses with its 15 min ttl, well
# inside the bound, and the next try is then granted.
GATE_LOCK_BOUND=${GATE_LOCK_BOUND:-1800} # seconds, the gate's 30 minutes
GATE_LOCK_PAUSE=${GATE_LOCK_PAUSE:-10}   # seconds between busy tries
take_gate_lock() {
  local rec=$1 name=$2 log=$3 deadline=$((SECONDS + ${GATE_LOCK_BOUND})) out
  GATE_LOCK_HOLDER="" GATE_LOCK_ERROR=""
  while :; do
    out=$(agent-bus lock "$rec" "$name" --ttl=900s --wait=60s 2>&1) && { printf '%s\n' "$out" >>"$log"; return 0; }
    printf '%s\n' "$out" >>"$log"
    if [[ $out != *'"kind":"busy"'* && $out != *'already hold'* ]]; then
      GATE_LOCK_ERROR=$(tr '\n' ' ' <<<"$out" | cut -c1-300)
      return 2
    fi
    GATE_LOCK_HOLDER=$(agent-bus holders "$rec" 2>&1 | awk -v r="$rec" -v n="$name" '$1 == r && $2 == n' | head -n 1)
    echo "== lock busy at $(date +%T); holder: ${GATE_LOCK_HOLDER:-unknown}" >>"$log"
    ((SECONDS + GATE_LOCK_PAUSE < deadline)) || return 1
    sleep "$GATE_LOCK_PAUSE"
  done
}
