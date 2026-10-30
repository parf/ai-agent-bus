#!/bin/bash
# Container image acceptance (Plans/R1.0-Release/TODO.md P.2), rootless podman.
# Usage: acceptance.sh <directory holding one release archive and its .sha256>
# Builds the image from the archive, runs the daemon and a runner in separate
# containers on a private network and completes a request/reply from a third.
# Then: the daemon recreated on its volume keeps the registered service and
# the issued credential, and an agent that asks for a sandbox is refused.
# Everything it creates is removed on exit, the image included.
# MUTANT=daemon-entry|runner-entry|storage-outside|sandbox-downgrade breaks
# the entry point in the built copy; each must fail a check.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
dist=$(realpath "${1:?release archive directory required}")
mutant=${MUTANT:-}
id="agent-bus-acc-$$-$RANDOM"
image="localhost/$id"
net="$id-net"
dvol="$id-daemon"
rvol="$id-runner"
owner=owner@node
mkdir -p "$here/../../tmp"
work=$(mktemp -d "$here/../../tmp/container-acceptance.XXXXXX")

cleanup() {
  podman rm -f -t 0 "$id-d" "$id-r" >/dev/null 2>&1
  podman volume rm -f "$dvol" "$rvol" >/dev/null 2>&1
  podman network rm -f "$net" >/dev/null 2>&1
  podman image rm -f "$image" >/dev/null 2>&1
  rm -rf "$work"
}
trap cleanup EXIT

passed=0 failed=0
pass() { echo "PASS $*"; passed=$((passed + 1)); }
fail() { echo "FAIL $*"; failed=$((failed + 1)); }
finish() { echo "checks: $passed passed, $failed failed${mutant:+ (MUTANT=$mutant)}"; [ "$failed" = 0 ] && [ "$passed" -gt 0 ]; exit $?; }

# ---- the image, from the archive and a copy of this directory ----------------
cp -r "$here/Containerfile" "$here/agent-bus-container" "$here/echo" "$work/"
entry="$work/agent-bus-container"
mutate() { # mutate FROM TO: the mutant must change the entry point, or it proves nothing
  grep -qF -- "$1" "$entry" || { echo "mutant $mutant: no line to change" >&2; exit 2; }
  FROM=$1 TO=$2 python3 -c 'import os,sys; p=sys.argv[1]; s=open(p).read(); open(p,"w").write(s.replace(os.environ["FROM"], os.environ["TO"], 1))' "$entry"
}
case $mutant in
  "") ;;
  daemon-entry) mutate '--addr "${AGENT_BUS_LISTEN:-0.0.0.0:6767}"' '--addr 127.0.0.1:6767' ;;
  runner-entry) mutate '--agents="$agents"' '--agents=/var/lib/agent-bus/none' ;;
  storage-outside) mutate 'db=$daemon_home/agent-bus.db' 'db=/run/agent-bus/agent-bus.db' ;;
  sandbox-downgrade) mutate 'if [ ! -e "$runner_home/agents.json" ]; then' 'if :; then' ;;
  *) echo "unknown MUTANT $mutant" >&2; exit 2 ;;
esac
if podman build -q --pull=never --build-context "release=$dist" -t "$image" "$work" >"$work/build.log" 2>&1; then
  pass "the image builds from the release archive"
else
  fail "image build: $(tail -3 "$work/build.log")"; finish
fi
archive=$(ls "$dist"/agent-bus-[0-9]*.tar.gz)
release=$(basename "$archive" .tar.gz)
version=$(tar -xzOf "$archive" "$release/internal/version/VERSION" | tr -d '[:space:]')
got=$(podman run --rm "$image" agent-busd --version 2>&1 | head -1)
[[ -n $version && $got == "$version" ]] && pass "the image runs release $version" || fail "agent-busd --version said '$got', the archive '$version'"

podman network create "$net" >/dev/null
podman volume create "$dvol" >/dev/null
podman volume create "$rvol" >/dev/null

daemon() {
  podman run -d --name "$id-d" --network "$net" --network-alias agent-busd --hostname node \
    -e AGENT_BUS_OWNER="$owner" -v "$dvol:/var/lib/agent-bus/daemon" "$image" daemon >/dev/null
  for _ in $(seq 1 100); do
    podman exec -u agent-busd "$id-d" agent-bus status >/dev/null 2>&1 && return 0
    sleep .1
  done
  return 1
}
runner() {
  podman run -d --name "$id-r" --network "$net" -e AGENT_BUS_ADDR=http://agent-busd:6767 \
    -e AGENT_BUS_TOKEN="$rtoken" -v "$rvol:/var/lib/agent-bus/runner" "$image" runner >/dev/null
}
# A person on another container, with only the port and a token.
bus() { podman run --rm --network "$net" -e AGENT_BUS_ADDR=http://agent-busd:6767 -e AGENT_BUS_TOKEN="$otoken" "$image" agent-bus "$@" 2>&1; }
# One call to #echo; the answer must come from #echo and carry exactly the body.
echoes() { # echoes BODY WAIT
  bus call --wait="$2" '#echo' "$1" | python3 -c 'import json,sys
m=json.loads(sys.stdin.read()); sys.exit(0 if m.get("from")=="#echo" and m.get("body")==sys.argv[1] else 1)' "$1" 2>/dev/null
}
echo_within() { # echo_within BODY SECONDS: the runner may still be connecting
  local end=$((SECONDS + $2))
  while [ $SECONDS -lt $end ]; do echoes "$1" 3s && return 0; sleep 1; done
  return 1
}
echo_state() { # the runner's own view of #echo: state and reason
  bus call --wait=10s '#runner/owner' status | python3 -c 'import json,sys
b=json.loads(json.loads(sys.stdin.read())["body"])
print(*[a.get("state","")+"|"+a.get("reason","") for a in b["agents"] if a["name"]=="#echo"])' 2>/dev/null
}

# ---- the two roles and a request/reply -------------------------------------
daemon && pass "the daemon role starts on its volume" || { fail "the daemon did not answer on its socket: $(podman logs "$id-d" 2>&1 | tail -3)"; finish; }
podman exec -u agent-busd "$id-d" agent-bus register '#runner/owner' --descr "the container runner" --allow @owner >/dev/null 2>&1 ||
  fail "register #runner/owner"
rtoken=$(podman exec -u agent-busd "$id-d" agent-bus token '#runner/owner' 2>/dev/null)
otoken=$(podman exec -u agent-busd "$id-d" agent-bus token 2>/dev/null)
[[ $rtoken =~ ^[0-9a-f]+$ && $otoken =~ ^[0-9a-f]+$ && $rtoken != "$otoken" ]] && pass "the Owner and the runner each have a token" || fail "tokens were not issued"
runner
body="delivered across containers $RANDOM"
echo_within "$body" 30 && pass "a request to #echo through the port is answered by the runner container" ||
  fail "no answer from #echo: $(podman logs "$id-r" 2>&1 | tail -3)"

# ---- the daemon recreated on its volume keeps the registry and credentials -
bus register svc-check --service-addr https://example.invalid --protocol http --descr "kept across containers" >/dev/null ||
  fail "register the service svc-check"
podman rm -f -t 5 "$id-d" >/dev/null
daemon && pass "a new daemon container starts on the same volume" || fail "the recreated daemon did not start: $(podman logs "$id-d" 2>&1 | tail -3)"
bus ls svc-check | python3 -c 'import json,sys
r=json.loads(sys.stdin.read()); r=r[0] if isinstance(r,list) else r
sys.exit(0 if r.get("name")=="svc-check" and r.get("kind")=="service" else 1)' 2>/dev/null &&
  pass "the registered service survives the recreated daemon" || fail "svc-check is gone after recreating the daemon"
bus status | grep -q "\"you\":\"$owner\"" && pass "the issued credential still answers for $owner" || fail "the Owner's token no longer works"
body="after the daemon was replaced $RANDOM"
echo_within "$body" 30 && pass "the runner's credential works again and #echo answers" || fail "#echo no longer answers after the daemon was replaced"

# ---- a sandbox the container cannot give is refused, never downgraded ------
podman exec -u agent-bus-runner "$id-r" sh -c 'printf "{\"echo\": {\"sandbox\": \"on\"}}\n" >/var/lib/agent-bus/runner/agents.json'
# Restarted, so the entry point's start path is exercised as well as the runner.
podman restart -t 5 "$id-r" >/dev/null
state=""
for _ in $(seq 1 20); do state=$(echo_state); [ -n "$state" ] && break; sleep 1; done
[[ $state == "refused|the sandbox is on and this host cannot give one"* ]] && pass "an agent that asks for a sandbox is refused, with the reason" ||
  fail "an agent that asks for a sandbox is '$state', not refused"
echoes "unsandboxed $RANDOM" 3s && fail "#echo answered although it asked for a sandbox" || pass "nothing answers for the refused agent"
finish
