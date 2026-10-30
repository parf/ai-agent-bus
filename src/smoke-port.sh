# The ports one smoke run binds, sourced by smoke.sh and its test.
#
# smoke_port <span>: an explicit PORT is used as given. Unset, the run claims
# the first block of <span> ports from SMOKE_PORT_FIRST (10000) in steps of
# SMOKE_PORT_STEP (1000), ending before SMOKE_PORT_LAST (21000), on which
# nothing listens, and holds that block's lock on fd SMOKE_PORT_FD until it
# and every child exits, so a run started at the same moment takes the next.
# The blocks stay under 21000: the agents' pinned PORTs (21000..28000, one
# block each) only show as listeners while a run of theirs has a daemon up.
smoke_port() {
  local span=$1 base end lockdir
  [ -n "${PORT:-}" ] && return 0
  local first=${SMOKE_PORT_FIRST:-10000} step=${SMOKE_PORT_STEP:-1000} last=${SMOKE_PORT_LAST:-21000}
  [ "$span" -le "$step" ] || { echo "smoke: $span ports do not fit a block of $step" >&2; return 2; }
  lockdir=${SMOKE_PORT_LOCKS:-${XDG_RUNTIME_DIR:-$HOME/.cache}/agent-bus-smoke-ports}
  mkdir -p "$lockdir" || return 2
  for ((base = first; base + span <= last; base += step)); do
    end=$((base + span - 1))
    exec {SMOKE_PORT_FD}>"$lockdir/$base.lock" || return 2
    if flock -n "$SMOKE_PORT_FD"; then
      # Locked by nobody else, but another program may still listen there.
      if ! ss -Htlnu 2>/dev/null | awk -v b="$base" -v e="$end" '{n = split($5, a, ":"); p = a[n] + 0; if (p >= b && p <= e) found = 1} END {exit !found}'; then
        PORT=$base
        return 0
      fi
      flock -u "$SMOKE_PORT_FD"
    fi
    exec {SMOKE_PORT_FD}>&-
  done
  echo "smoke: no free block of $span ports between $first and $last" >&2
  return 1
}
