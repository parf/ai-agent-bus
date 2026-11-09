#!/usr/bin/env bash
# J66: with a PRIVATE empty GOCACHE, the gate's warm step makes the timed go_unit fast command finish under 15 s.
# Takes no lock itself: run it under the smoke-gate slow lock (timing run).
set -uo pipefail
GATE=${J66_GATE:-/usr/local/src/ai-agent-bus/.ai-team/gate.sh.j66}
WT=${J66_WT:-/usr/local/src/ai-agent-bus}
priv=$(mktemp -d); trap 'rm -rf "$priv"' EXIT
log=$priv/log
now() { date +%s%N; }
ms() { echo $(( ($(now) - $1) / 1000000 )); }
warm=$(awk '/^  # Warm Go/{f=1} f&&/^  local t0=/{exit} f' "$GATE")
[[ $warm == *'go test -run'* ]] || { echo "FAIL: $GATE has no warm step"; exit 1; }
timed() { # the go_unit fast command, first entry of smoke-map.json
  (cd "$WT/src" && jq -r '.. | objects | select(.argv? and .argv[0]=="go" and .argv[1]=="test" and (.argv|index("./internal/core"))) | .argv | @sh' smoke-map.json | head -1 | GOCACHE=$priv/cache bash -c "$(cat)") >>"$log" 2>&1
}
export GOCACHE=$priv/cache
t=$(now); (WT=$WT log=$log; eval "$warm"); w=$(ms "$t")
t=$(now); timed; rc=$?; f=$(ms "$t")
echo "cold warm step ${w} ms; timed fast go_unit ${f} ms rc=$rc"
((rc == 0 && f < 15000 && f * 2 < w)) || { echo "FAIL: timed fast step ${f} ms (rc=$rc) must be under 15000 and under half the warm step ${w} ms"; exit 1; }
echo "PASS"
