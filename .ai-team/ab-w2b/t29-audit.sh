#!/bin/bash
# T-29 audit on a fresh test daemon. Output = observations; I read them against the docs.
D=/usr/local/src/ai-agent-bus/tmp/ab-w2b-audit/run; PORT=17893; OWNER=parf@localhost
export XDG_STATE_HOME=$D/state; unset AGENT_BUS_NAME AGENT_BUS_TOKEN AGENT_BUS_ADDR
rm -rf $D/bus.db $D/bus.sock $D/state $D/tok.*; mkdir -p $D/state
start() { "$D/agent-busd" --addr 127.0.0.1:$PORT --socket "$D/bus.sock" --owner "$OWNER" --db "$D/bus.db" $1 --flush-every 0 >>"$D/daemon.log" 2>&1 & DPID=$!
  for _ in $(seq 1 100); do [ -n "$(curl -s --max-time 1 --unix-socket $D/bus.sock http://unix/status 2>/dev/null)" ] && return 0; sleep 0.1; done; echo "NO START"; cat $D/daemon.log; exit 1; }
start --create
ACC=$(id -un)
TOKEN=$(AGENT_BUS_ADDR=$D/user-$ACC.sock $D/agent-bus token $OWNER)
tok() { local f="$D/tok.$(printf '%s' "$1" | tr '/@.#' '____')"; [ -s "$f" ] || AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$TOKEN AGENT_BUS_NAME=$OWNER $D/agent-bus token "$1" >"$f"; cat "$f"; }
ab() { AGENT_BUS_ADDR=$D/bus.sock AGENT_BUS_TOKEN=$(tok "$1") AGENT_BUS_NAME=$1 timeout 40 "$D/agent-bus" "${@:2}"; }
body() { jq -r '.body // .' 2>/dev/null || cat; }
h() { echo; echo "### $*"; }
call() { local who=$1; shift; local o; o=$(ab $who call '#bus' --wait=8s "$@" 2>&1); echo "[$who] $*"; echo "  => $(echo "$o" | head -c 700 | tr '\n' ' ')"; }
A='#a@localhost'; P='#priv@localhost'; Q='#sink@localhost'; AL=alice@localhost; BO=bob@localhost
h setup
ab $OWNER register $A --kind agent --allow '*' >/dev/null; ab $OWNER register $P --kind agent --allow "$OWNER" >/dev/null
ab $OWNER register $Q --kind agent --allow '*' >/dev/null; ab $OWNER register '#sinkpriv@localhost' --kind agent --allow "$OWNER" >/dev/null
for u in $AL $BO; do curl -fsS --unix-socket $D/bus.sock -H "X-Agent-Bus-Token: $TOKEN" -d "{\"name\":\"$u\",\"create\":true}" http://unix/user >/dev/null || echo USERFAIL; done
echo ok
h "status/version"; call $OWNER status; call $OWNER version
h "write: no wait vs wait (answer only with wait)"
ab $OWNER send '#bus' "write $A notice quiet" ; echo "(send w/o wait returned $?)"
call $OWNER "write $A notice hello"
call $OWNER "write $A notice hello"
call $OWNER "write $A warning careful"
call $OWNER "write $A alert boom one"
h "read / list"
call $OWNER "read $A"; call $OWNER "read $A alert"; call $OWNER "list"
call $AL "read $A"; call $AL "list"
call $OWNER "read #bus"; call $AL "read #bus"
call $OWNER "write $A critical x"; call $OWNER "write $A notice"; call $OWNER "write nolog@localhost notice x"
h "pull / clean"
call $OWNER "pull $A notice 1"; call $OWNER "read $A notice"; call $OWNER "pull $A warning 501"; call $OWNER "clean $A warning"; call $OWNER "read $A"
call $AL "pull $A"; call $AL "clean $A"
h "subscriptions: add/list/delete"
call $OWNER "sub add $Q $A alert /boom/"
call $OWNER "write $A notice boom no"
call $OWNER "write $A alert boom two"
call $OWNER "write $A alert fine"
call $OWNER "write $A alert boom two"
echo "-- q inbox:"; ab $Q consume --wait=1s 2>&1 | head -c 600; echo
ab $OWNER consume --wait=1s >/dev/null 2>&1
echo "-- copies from q:"; for i in 1 2 3 4; do ab $Q consume --wait=1s 2>&1 | head -c 400 | tr '\n' ' '; echo; done
call $OWNER "sub list"; call $OWNER "sub list $A"; call $AL "sub list"; call $AL "sub add $Q $A"
call $OWNER "sub add nobody@localhost $A"; call $OWNER "sub add $Q $A alert /(a+)+/"; call $OWNER "sub add $Q $A zzz"; call $OWNER "sub add $Q $A $'\t'alert"
ID=$(ab $OWNER call '#bus' --wait=8s "sub add $Q $A" | jq -r '.body|fromjson|.id' 2>/dev/null); echo "new id=$ID"
call $AL "sub delete $ID"; call $OWNER "sub delete $ID"; call $OWNER "sub delete $ID"; call $OWNER "sub delete bad"
h "Q325: author's source right checked at creation only"
ab $OWNER manage $A --add-maintainers=$AL >/dev/null
call $AL "sub add $Q $A alert /q325/"
ab $OWNER manage $A --remove-maintainers=$AL >/dev/null
call $AL "sub list $A"; call $AL "sub list"
call $OWNER "write $A alert q325 after demotion"
echo "-- copies after demotion:"; for i in 1 2 3; do ab $Q consume --wait=1s 2>&1 | head -c 400 | tr '\n' ' '; echo; done
call $OWNER "sub list $A"
h "target privacy still enforced per send (private queue not admitting #bus / creator)"
ab $OWNER manage $A --add-maintainers=$AL >/dev/null
call $AL "sub add #sinkpriv@localhost $A alert"
ab $OWNER manage $A --remove-maintainers=$AL >/dev/null
call $OWNER "sub add #sinkpriv@localhost $A alert /privq/"
call $OWNER "write $A alert privq go"
call $OWNER "sub list $A"
h "J84: private agent asking #bus"
echo "-- #priv allow=$OWNER only; asks status:"; ab $P call '#bus' --wait=5s status 2>&1 | head -c 300; echo
ab $OWNER manage $P --add-allow='#bus' >/dev/null
echo "-- after allowing #bus:"; ab $P call '#bus' --wait=5s status 2>&1 | head -c 300; echo
h "alerts across restart"
call $OWNER "write $A alert persist me"; call $OWNER "write $A notice volatile"; call $OWNER "sub list $A"; call $OWNER "read $A"
kill $DPID; wait $DPID 2>/dev/null; sleep 1; echo "restarting"; start ""
call $OWNER "read $A"; call $OWNER "list"; call $OWNER "sub list $A"
call $OWNER "write $A alert after restart"; call $OWNER "read $A alert"
h "delete source cascades"
ab $OWNER unregister $A 2>&1 | head -c 200; echo
ab $OWNER register $A --kind agent --allow '*' >/dev/null; call $OWNER "sub list"; call $OWNER "read $A"
h "tab handling"
call $OWNER $'sub\tlist'; call $OWNER $'sub add\t'"$Q $A"
kill $DPID; wait $DPID 2>/dev/null; echo done
