#!/bin/bash
# The entry point's own checks, without a container: id, hostname, setpriv, chown and the two programs
# are stubs that record what the entry point asked of them (README.md#run). Run: bash entrypoint_test.sh
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
bin=$work/bin root=$work/root log=$work/log
mkdir -p "$bin" "$root"/{daemon,runner,agent.d}
stub() { printf '#!/bin/bash\n%s\n' "$2" >"$bin/$1"; chmod +x "$bin/$1"; }
stub id 'case $1 in -u) echo ${STUB_UID:-0};; -un) echo ${STUB_UN:-root};; *) echo 0;; esac'
stub getent 'case $2 in agent-bus-runner) echo agent-bus-runner:x:991:991:::;; agent-busd) echo agent-busd:x:990:990:::;; esac'
stub hostname 'echo ${STUB_HOST:-node}'
stub chown 'echo "chown $*" >>"$STUB_LOG"'
stub install 'echo "install $*" >>"$STUB_LOG"'
# setpriv records its arguments up to --, then runs the command (a test or a stub) as the caller.
stub setpriv 'for a in "$@"; do [ "$a" = -- ] && break; printf "%s " "$a"; done >>"$STUB_LOG"; echo >>"$STUB_LOG"; while [ "$1" != -- ]; do shift; done; shift; [ "${STUB_UNUSABLE:-}" = 1 ] && [ "$1" = test ] && exit 1; exec "$@"'
stub agent-busd 'echo "busd $*" >>"$STUB_LOG"; [ "$1" != --init ] || : >"$STUB_DB"'
stub agent-bus-runner 'echo "runner $*" >>"$STUB_LOG"'
fails=0
check() { # check NAME CONDITION...
  local name=$1; shift
  if "$@"; then echo "ok   $name"; else echo "FAIL $name"; fails=$((fails + 1)); fi
}
run() { : >"$log"; env -i PATH="$bin:/usr/bin:/bin" AGENT_BUS_CONTAINER_ROOT="$root" STUB_LOG="$log" STUB_DB="$root/daemon/agent-bus.db" "$@" bash "$here/agent-bus-container" $ENTRY 2>"$work/err"; }
has() { grep -q -- "$1" "$log"; }
errs() { grep -q -- "$1" "$work/err"; }

# Q4: both roles drop privilege with no new privileges and an empty bounding set.
rm -f "$root/daemon/agent-bus.db"
ENTRY=daemon run AGENT_BUS_OWNER=o@node
check "daemon drop: no-new-privs" has '--no-new-privs'
check "daemon drop: empty bounding set" has '--bounding-set=-all'
rm -f "$root/runner/agents.json"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t
check "runner drop: no-new-privs" has "--no-new-privs"
check "runner drop carries both" bash -c "grep 'bounding-set=-all' '$log' | grep -q -- --no-new-privs"

# Q5: a later start says the owner is kept; a first start does not.
: >"$root/daemon/agent-bus.db"
ENTRY=daemon run AGENT_BUS_OWNER=o@node
check "later start names the kept Owner" errs 'Owner is kept from the first start'
rm -f "$root/daemon/agent-bus.db"
ENTRY=daemon run AGENT_BUS_OWNER=o@node
check "first start stays quiet" bash -c "! grep -q 'kept from the first start' '$work/err'"

# Q6: a first start on a container-id hostname is refused, a named one is not; a later start never is.
rm -f "$root/daemon/agent-bus.db"
ENTRY=daemon run AGENT_BUS_OWNER=o@node STUB_HOST=0123456789ab
check "first start on a container id is refused" errs -- '--hostname'
check "and nothing was started" bash -c "! grep -q '^busd' '$log'"
ENTRY=daemon run AGENT_BUS_OWNER=o@node STUB_HOST=node
check "a named host starts" has 'busd --init'
: >"$root/daemon/agent-bus.db"
ENTRY=daemon run AGENT_BUS_OWNER=o@node STUB_HOST=0123456789ab
check "a later start on a container id still runs" has 'busd --db'

# Q7: the inventory lists the agents present.
mkdir -p "$root/agent.d/echo" "$root/agent.d/mine" "$root/agent.d/notes"
: >"$root/agent.d/echo/agent.json"; : >"$root/agent.d/mine/agent.json"
rm -f "$root/runner/agents.json"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t
check "inventory lists echo and mine, not a directory without agent.json" bash -c "python3 -c \"import json,sys; d=json.load(open('$root/runner/agents.json')); sys.exit(0 if d=={'echo':{'sandbox':'off'},'mine':{'sandbox':'off'}} else 1)\""
rm -rf "$root/agent.d/echo"; rm -f "$root/runner/agents.json"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t
check "a mount without echo does not list it" bash -c "python3 -c \"import json,sys; d=json.load(open('$root/runner/agents.json')); sys.exit(0 if d=={'mine':{'sandbox':'off'}} else 1)\""
rm -rf "$root/agent.d"/*; mkdir "$root/agent.d/none"; rm -f "$root/runner/agents.json"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t
check "no agents gives an empty, valid inventory" bash -c "python3 -c \"import json,sys; sys.exit(0 if json.load(open('$root/runner/agents.json'))=={} else 1)\""

# Q8: a directory the account can already use keeps its owner and mode; one it cannot is given over.
mkdir -p "$root/bind"; chmod 0755 "$root/bind"
real_stat=$(command -v stat)
stub stat 'case $2 in %U) echo ${STUB_OWNER:-hostuser};; %a) echo ${STUB_MODE:-755};; esac'
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t
check "a usable foreign directory is not chowned" bash -c "! grep -q '^chown agent-bus-runner' '$log'"
# A foreign directory the account cannot use: a non-empty one holds somebody's data and is refused,
# untouched; an empty one is a fresh volume and is taken over.
chmod 0755 "$root/runner"; touch "$root/runner/data"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_UNUSABLE=1
check "a non-empty unusable foreign directory is refused by name, with the runner's uid" errs 'holds data.*podman unshare chown 991:991'
check "and it is neither chowned nor chmoded" bash -c "! grep -q '^chown' '$log' && [ \"\$($real_stat -c %a '$root/runner')\" = 755 ]"
find "$root/runner" -mindepth 1 -delete
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_UNUSABLE=1
check "an empty unusable foreign directory is chowned to the account" has '^chown agent-bus-runner:agent-bus-runner '
check "and set to 0700" bash -c "[ \"\$($real_stat -c %a '$root/runner')\" = 700 ]"
# The account's own directory is tightened to 0700, whatever it was.
chmod 0777 "$root/runner"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_OWNER=agent-bus-runner STUB_UNUSABLE=1
check "a directory already the account's is not chowned" bash -c "! grep -q '^chown agent-bus-runner:' '$log'"
check "a role-owned 0777 directory ends 0700" bash -c "[ \"\$($real_stat -c %a '$root/runner')\" = 700 ]"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_MODE=700
check "the probe runs as the account" bash -c "grep -q '^--reuid=agent-bus-runner --regid=agent-bus-runner --init-groups \$' '$log'"
STUB_MODE=777 ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_MODE=777
check "a 0777 mount is refused by name" errs 'writable by its group or others'
check "and its hint names the runner's uid, 991" errs 'podman unshare chown 991:991'
check "and the runner is not started" bash -c "! grep -q '^runner' '$log'"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_MODE=700
check "a 0700 mount the account can use is kept" has '^runner'
check "and not chowned" bash -c "! grep -q '^chown agent-bus-runner' '$log'"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_MODE=775
check "a group-writable mount is refused" errs 'writable by its group or others'

# The account's own start, with no root to drop, still takes no new privileges.
STUB_UID=991 STUB_UN=agent-bus-runner ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_UID=991 STUB_UN=agent-bus-runner
check "the non-root start sets no-new-privs" has '^--no-new-privs'

# An agent directory name that would break the inventory's JSON is refused.
mkdir -p "$root/agent.d/we\\ird"; : >"$root/agent.d/we\\ird/agent.json"; rm -f "$root/runner/agents.json"
ENTRY=runner run AGENT_BUS_ADDR=http://d:1 AGENT_BUS_TOKEN=t STUB_MODE=700
check "a backslash in an agent name is refused" errs 'quote or backslash'
check "and leaves no agents.json, whole or partial" bash -c "[ ! -e '$root/runner/agents.json' ] && [ -z \"\$(ls -A '$root/runner' | grep agents.json)\" ]"
rm -rf "$root/agent.d/we\\ird"
[ "$fails" = 0 ] && echo "entry point: all checks passed" || { echo "entry point: $fails failed"; exit 1; }
