# Today's loop (.ai-team/gate.sh:89-95), as a function, for proving the test fails on it.
take_gate_lock() {
  local rec=$1 name=$2 log=$3 n=0
  until agent-bus lock "$rec" "$name" --ttl=900s --wait=60s >>"$log" 2>&1; do
    ((++n < 30)) || return 1
  done
}
