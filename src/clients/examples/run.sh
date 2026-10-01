#!/bin/bash
# Runs the client examples together on one throwaway daemon, so agents written
# against different clients, in different languages, call each other.
#
#   src/clients/examples/run.sh [example ...]          # default: every example
#   src/clients/examples/run.sh --live [example ...]   # on this node's own daemon
#
# By default everything runs on a throwaway daemon that is gone afterwards.
# --live registers the agents on the node itself, as the account running this
# (through its own socket, so no credential of the account's is minted or
# rotated), runs the demos, and keeps the workers serving until Ctrl-C: then
# they show in `agent-bus ls --all` and the web face. Their records stay after
# Ctrl-C; --clean (with --live) unregisters them on the way out.
#
# An example is a directory here holding:
#   agents.txt   one agent per line: its name, then optionally a description;
#                // starts a comment line
#                (an agent that says [stats] or [relay] in it answers those topics)
#   run.sh       starts its agents and stays in the foreground until SIGTERM
#   demo.sh      optional: drives the example and prints what it found
# Every script gets AGENT_BUS_ADDR, EXAMPLES_TOKENS (a directory holding one
# file per agent, its name with #, / and @ as _), EXAMPLES_OWNER and
# EXAMPLES_OWNER_TOKEN, EXAMPLES_ROOT (the checkout), STATS_ROOT (where workers
# may read; the checkout), SAMPLES (a directory of sample files) and DEMO_FILE
# (SAMPLES/sample.json). tokens.sh NAME prints one agent's token. A demo.sh
# runs as #demo@localhost: AGENT_BUS_NAME and AGENT_BUS_TOKEN are set for it.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
live= clean=
while [ $# -gt 0 ]; do
  case $1 in --live) live=1 ;; --clean) clean=1 ;; *) break ;; esac
  shift
done
if [ -n "$live" ]; then
  # The account's own socket: the daemon knows who is calling without a token.
  AB_TEST_ADDR=${AGENT_BUS_ADDR:-${XDG_RUNTIME_DIR:+$XDG_RUNTIME_DIR/agent-bus/user-$(id -un).sock}}
  [ -S "${AB_TEST_ADDR:-}" ] || AB_TEST_ADDR=/run/agent-bus/user-$(id -un).sock
  AB_TEST_OWNER_TOKEN=
  AB_TEST_OWNER=$(curl -sS --unix-socket "$AB_TEST_ADDR" http://localhost/status | python3 -c 'import json,sys; print(json.load(sys.stdin)["you"])') \
    || { echo "no daemon answers at $AB_TEST_ADDR" >&2; exit 2; }
  AB_TEST_DIR=$root/tmp/examples-live
  export EXAMPLES_LIVE=1
  mkdir -p "$AB_TEST_DIR"
  echo "== live on $AB_TEST_ADDR as $AB_TEST_OWNER"
elif [ -z "${AB_TEST_ADDR:-}" ]; then
  exec "$root/src/clients/php/tests/daemon.sh" bash "$0" "$@"
fi

examples=("$@")
if [ ${#examples[@]} -eq 0 ]; then
  for d in "$here"/*/; do [ -f "$d/agents.txt" ] && examples+=("$(basename "$d")"); done
fi
export AGENT_BUS_ADDR=$AB_TEST_ADDR EXAMPLES_OWNER=$AB_TEST_OWNER EXAMPLES_OWNER_TOKEN=$AB_TEST_OWNER_TOKEN EXAMPLES_ROOT=$root
export EXAMPLES_TOKENS=$AB_TEST_DIR/tokens
mkdir -p "$EXAMPLES_TOKENS"
# Workers read files only under STATS_ROOT: the checkout, which holds the samples too.
export STATS_ROOT=${STATS_ROOT:-$root}
export SAMPLES=$AB_TEST_DIR/samples DEMO_FILE=$AB_TEST_DIR/samples/sample.json
mkdir -p "$SAMPLES"
cat >"$SAMPLES/sample.json" <<'JSON'
{"service": "agent-bus examples", "agents": ["php", "java", "python", "rust", "ts", "go"], "retries": 2, "enabled": true}
JSON
printf 'line one\n// TODO: first thing\nline three\n# TODO second thing\nlast line, unterminated' >"$SAMPLES/notes.txt"

owner() { curl -sS --fail-with-body --unix-socket "$AGENT_BUS_ADDR" ${EXAMPLES_OWNER_TOKEN:+-H "X-Agent-Bus-Token: $EXAMPLES_OWNER_TOKEN"} \
  -H 'Content-Type: application/json' "http://localhost$2" ${3:+-d "$3"} -X "$1"; }

# Register each example's agents, open to every caller, and give each a token.
# #demo@localhost is the caller each demo.sh runs as.
names=()
register() { # name descr
  local body
  body=$(python3 -c 'import json,sys; print(json.dumps({"kind":"agent","name":sys.argv[1],"descr":sys.argv[2],"allow":["*"]}))' "$1" "$2")
  owner POST /register "$body" >/dev/null || { echo "could not register $1" >&2; exit 1; }
  owner POST /token "{\"name\":\"$1\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' \
    > "$EXAMPLES_TOKENS/$(printf '%s' "$1" | tr '#/@' '___')"
}
register '#demo@localhost' 'the caller every example demo runs as'
for ex in "${examples[@]}"; do
  [ -f "$here/$ex/agents.txt" ] || { echo "no example $ex" >&2; exit 2; }
  while read -r name descr; do
    # Agent names begin with #, so a comment line begins with //.
    [ -z "$name" ] && continue
    [[ $name == //* ]] && continue
    register "$name" "${descr:-}"
    names+=("$name")
  done < "$here/$ex/agents.txt"
done

# Start every example, each in its own process group so stopping it stops all it started.
pids=()
stop() {
  for p in "${pids[@]}"; do kill -TERM -- "-$p" 2>/dev/null; done
  for p in "${pids[@]}"; do
    for _ in $(seq 1 50); do kill -0 -- "-$p" 2>/dev/null || break; sleep 0.1; done
    kill -KILL -- "-$p" 2>/dev/null
  done
}
trap stop EXIT
for ex in "${examples[@]}"; do
  echo "== starting $ex"
  setsid bash "$here/$ex/run.sh" >"$AB_TEST_DIR/$ex.log" 2>&1 &
  pids+=($!)
done

# Ready when every agent has a reader, within READY_SECONDS (default 90: a Java
# or Rust build happens first). An example whose run.sh has exited fails at once.
echo "== waiting for ${#names[@]} agents"
deadline=$(( $(date +%s) + ${READY_SECONDS:-90} ))
waiting=("${names[@]}")
while [ ${#waiting[@]} -gt 0 ]; do
  still=()
  for name in "${waiting[@]}"; do
    q=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))' "$name")
    if owner GET "/lookup?name=$q" 2>/dev/null | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("reading") or (r.get("readers") or 0) > 0 else 1)'; then
      echo "  up   $name"
    else
      still+=("$name")
    fi
  done
  waiting=("${still[@]+"${still[@]}"}")
  [ ${#waiting[@]} -eq 0 ] && break
  for i in "${!pids[@]}"; do
    if ! kill -0 "${pids[$i]}" 2>/dev/null; then
      echo "example ${examples[$i]} exited before its agents were up:" >&2; tail -30 "$AB_TEST_DIR/${examples[$i]}.log" >&2; exit 1
    fi
  done
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "not reading after ${READY_SECONDS:-90}s: ${waiting[*]}" >&2
    for ex in "${examples[@]}"; do echo "--- $ex" >&2; tail -15 "$AB_TEST_DIR/$ex.log" >&2; done
    exit 1
  fi
  sleep 0.3
done

rc=0
for ex in "${examples[@]}"; do
  [ -f "$here/$ex/demo.sh" ] || continue
  echo "== $ex"
  AGENT_BUS_NAME='#demo@localhost' AGENT_BUS_TOKEN=$(bash "$here/tokens.sh" '#demo@localhost') \
    bash "$here/$ex/demo.sh" || rc=1
done
if [ -n "$live" ]; then
  echo "== serving on the node: ${names[*]}"
  echo "   see them with: agent-bus ls --all    (Ctrl-C stops the workers${clean:+ and unregisters them})"
  if [ -n "$clean" ]; then
    trap 'stop; for n in "${names[@]}" "#demo@localhost"; do owner POST /unregister "$(python3 -c "import json,sys; print(json.dumps({\"name\":sys.argv[1]}))" "$n")" >/dev/null 2>&1; done' EXIT
  fi
  wait
fi
exit $rc
