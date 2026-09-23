#!/bin/bash
# Runs only inside backup-restore.sh's disposable real-systemd containers.
# Phase "source": populate, refuse an online copy, back up twice (B1, B2) with
# the documented procedure, then restore B1 over newer, uncleanly stopped state.
# Phase "restore": a fresh install of the same archive takes B2, then a
# graceful restart and a bus-child crash. See docs/09-setup.md#backup-and-restore.
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
fail() { echo "FAIL $*" >&2; exit 1; }
pass() { echo "PASS $*"; }
phase=${1:?phase required}
mutant=${BACKUP_MUTANT:-}
home=/var/lib/agent-bus/daemon
db=$home/agent-bus.db
shared=/shared

wait_api() {
  for _ in $(seq 1 150); do
    curl -fsS --max-time 2 http://127.0.0.1:6767/identity >/dev/null 2>&1 && agent-bus status >/dev/null 2>&1 && return 0
    sleep .1
  done
  fail "API did not start"
}
# The HTTP status a credential gets from /status on the shared socket, and
# who it authenticated as. Never a token on a command line inside a log.
auth() {
  local code
  code=$(curl -sS --max-time 4 --unix-socket /run/agent-bus/bus.sock \
    -H "X-Agent-Bus-Token: $(cat "$1")" -o /root/auth.json -w '%{http_code}' http://bus/status)
  if [ "$code" = 200 ]; then
    printf '%s %s\n' "$code" "$(sed -n 's/.*"you":"\([^"]*\)".*/\1/p' /root/auth.json)"
  else
    printf '%s\n' "$code"
  fi
}
install() {
  mkdir /root/release
  tar -xzf /package/agent-bus-*.tar.gz -C /root/release --strip-components=1
  (cd /root/release && sha256sum -c MANIFEST.sha256 >/dev/null)
  printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIownerfixture owner@fresh' >/root/owner-key.pub
  (cd /root/release && ./agent-bus-setup --owner owner@fresh --key /root/owner-key.pub >/evidence/setup.log 2>&1) ||
    { cat /evidence/setup.log >&2; fail "package did not install"; }
  wait_api
}

# Everything a restore must bring back, as listings answer it. Internal IDs
# never leave the daemon; what they hold together is compared instead.
cat >/root/snapshot.py <<'PY'
import hashlib, json, subprocess, sys
def out(*a): return subprocess.check_output(a, text=True)
records = json.loads(out("agent-bus", "ls"))
volatile = {"oldest", "readers", "reading"}
records = sorted(({k: v for k, v in r.items() if k not in volatile} for r in records), key=lambda r: r["name"])
token = open("/root/snapshot-owner.token").read().strip()
users = json.loads(out("curl", "-fsS", "--unix-socket", "/run/agent-bus/bus.sock",
                       "-H", "X-Agent-Bus-Token: " + token, "http://bus/users"))
status = json.loads(out("agent-bus", "status"))
keys = open("/var/lib/agent-bus/daemon/.ssh/authorized_keys", "rb").read()
snap = {
    "records": records,
    "users": sorted(users, key=lambda u: u["name"]),
    "accounts": out("agent-bus-admin", "account", "list"),
    "keys": out("agent-bus-admin", "user", "list"),
    "authorized_keys_sha256": hashlib.sha256(keys).hexdigest(),
    "status": {k: status.get(k) for k in ("services", "queued", "dropped", "expired", "kinds")},
}
json.dump(snap, sys.stdout, indent=1, sort_keys=True)
print()
PY
snapshot() { python /root/snapshot.py >"$1"; }
same() {
  local d="/evidence/${3//[^a-zA-Z0-9]/-}.diff"
  python -c 'import difflib,sys; a,b=(open(f).readlines() for f in sys.argv[1:3]); d=list(difflib.unified_diff(a,b,*sys.argv[1:3])); sys.stdout.writelines(d); sys.exit(1 if d else 0)' "$1" "$2" >"$d" ||
    { cat "$d" >&2; fail "$3: state differs"; }
}
cmp_same() { [ "$(sha256sum <"$1")" = "$(sha256sum <"$2")" ]; }
field() { agent-bus ls "$1" | python -c "import json,sys; print(json.load(sys.stdin).get('$2', 0))"; }

# The supported procedure: a clean stop checkpoints the WAL into the database
# file, so the stopped daemon home is complete; archive it, start again.
backup() {
  local dest=$1
  if [ "$mutant" = hot ]; then
    tar -C /var/lib/agent-bus --numeric-owner -cpf "$dest" daemon
  else
    systemctl stop agent-busd
    [ ! -e "$db-wal" ] || [ ! -s "$db-wal" ] || fail "a clean stop left a non-empty WAL"
    tar -C /var/lib/agent-bus -cpf "$dest" daemon
    systemctl start agent-busd
  fi
  chmod 600 "$dest"
  wait_api
}
# Restore replaces the whole daemon home, so no WAL or key file of the state
# being replaced survives into the restored one.
restore() {
  local src=$1
  systemctl stop agent-busd
  if [ "$mutant" = keepwal ]; then
    tar -C /root -xpf "$src" daemon/agent-bus.db
    cp -p /root/daemon/agent-bus.db "$db"
    rm -rf /root/daemon
  else
    rm -rf "$home"
    tar -C /var/lib/agent-bus -xpf "$src"
  fi
  systemctl reset-failed agent-busd 2>/dev/null || true
  systemctl start agent-busd
  wait_api
}

if [ "$phase" = source ]; then
  install
  agent-bus-admin token owner@fresh >/root/snapshot-owner.token
  cp /root/snapshot-owner.token "$shared/owner.token"
  printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIalicefixture alice@fresh' | agent-bus-admin user add alice@fresh - >/dev/null
  printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIbobfixture bob@fresh' | agent-bus-admin user add bob@fresh - >/dev/null
  agent-bus register '#worker@fresh' --kind agent --allow owner@fresh,alice@fresh --descr 'a worker' >/dev/null
  agent-bus agent-template '#worker@fresh' '{"model":"m1","n":1}' >/dev/null
  agent-bus register backlog@fresh --kind queue --allow owner@fresh,alice@fresh --bound 50 --overflow ring >/dev/null
  agent-bus register tiny@fresh --kind queue --allow owner@fresh --bound 2 --overflow ring >/dev/null
  agent-bus register sink@fresh --kind queue --allow owner@fresh >/dev/null
  agent-bus channel create news@fresh --kind pubsub --descr news >/dev/null
  agent-bus manage news@fresh --add-deliver-to sink@fresh >/dev/null
  agent-bus manage sink@fresh --add-allow news@fresh >/dev/null
  agent-bus group @team@fresh alice@fresh bob@fresh >/dev/null
  agent-bus register mysql@fresh --addr 127.0.0.1:3306 --protocol mysql --allow owner@fresh >/dev/null
  agent-bus secret mysql@fresh 'PASSWORD=first' >/dev/null
  for i in 1 2 3; do agent-bus send backlog@fresh --topic t --tag g "backlog $i" >/dev/null; done
  for i in 1 2 3 4; do agent-bus send tiny@fresh "tiny $i" >/dev/null; done
  agent-bus publish --channel news@fresh 'headline 1' >/dev/null
  got=$(agent-bus consume --inbox backlog@fresh --wait 2s)
  grep -q 'backlog 1' <<<"$got" || fail "first backlog message was not consumed"
  # Name reuse: the second reuse@fresh is a new record whose counters start at zero.
  agent-bus register reuse@fresh --kind queue --allow owner@fresh >/dev/null
  agent-bus send reuse@fresh 'belongs to the first reuse' >/dev/null
  agent-bus consume --inbox reuse@fresh --wait 2s >/dev/null
  agent-bus unregister reuse@fresh >/dev/null
  agent-bus register reuse@fresh --kind queue --allow owner@fresh >/dev/null
  agent-bus manage backlog@fresh --owner alice@fresh >/dev/null
  for who in alice@fresh bob@fresh '#worker@fresh'; do agent-bus-admin token "$who" >"$shared/${who//[#@]/_}.token"; done
  # Positive controls: the populated state is non-trivial where it matters.
  [ "$(field backlog@fresh queued)" = 2 ] && [ "$(field backlog@fresh out)" = 1 ] || fail "backlog counters control"
  [ "$(field tiny@fresh dropped)" = 2 ] || fail "ring overflow control"
  [ "$(field sink@fresh queued)" = 1 ] || fail "pubsub copy control"
  [ "$(field reuse@fresh in)" = 0 ] || fail "a reused name inherited the old record's counters"
  [ "$(field backlog@fresh owner)" = alice@fresh ] || fail "transfer control"
  pass "populated: users, # agent with configuration, queues, pubsub copy, group, service secret, transfer, reuse, credentials"

  if timeout 10 python -c 'import sqlite3; sqlite3.connect("file:'"$db"'?mode=ro", uri=True, timeout=1).execute("select count(*) from records").fetchall()' \
    >/evidence/online-copy.out 2>&1; then
    fail "the database was readable while the daemon held it"
  fi
  grep -q 'database is locked' /evidence/online-copy.out || fail "online copy failed for another reason: $(cat /evidence/online-copy.out)"
  pass "an online SQLite copy is refused while the daemon holds the database"

  snapshot /evidence/s1.json
  backup "$shared/b1.tar"
  snapshot /evidence/s1-after-backup.json
  same /evidence/s1.json /evidence/s1-after-backup.json "B1 stop/start"
  tar -tf "$shared/b1.tar" >/evidence/b1.list
  grep -qx 'daemon/agent-bus.db' /evidence/b1.list || fail "B1 has no database"
  if grep -q 'agent-bus.db-wal' /evidence/b1.list; then fail "B1 carries a WAL; the stop was not clean"; fi
  [ "$(stat -c '%U %a' "$shared/b1.tar")" = "root 600" ] || fail "backup archive is not root-only"
  pass "B1: stop, archive the daemon home, start; the listing is unchanged"

  # Newer state: another user and key, a rotation, traffic, reconfiguration,
  # a new record and a second transfer.
  printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIcarolfixture carol@fresh' | agent-bus-admin user add carol@fresh - >/dev/null
  agent-bus-admin token carol@fresh >"$shared/carol.token"
  agent-bus-admin token alice@fresh --rotate >"$shared/alice-rotated.token"
  agent-bus send backlog@fresh --topic t --tag g 'backlog 4' >/dev/null
  agent-bus agent-template '#worker@fresh' '{"model":"m2","n":2}' >/dev/null
  agent-bus secret mysql@fresh 'PASSWORD=second' >/dev/null
  agent-bus register newer@fresh --kind queue --allow owner@fresh >/dev/null
  agent-bus manage tiny@fresh --owner bob@fresh >/dev/null
  cp "$shared/owner.token" /root/snapshot-owner.token
  snapshot /evidence/s2.json
  if cmp_same /evidence/s1.json /evidence/s2.json; then fail "newer state is not different from B1"; fi
  backup "$shared/b2.tar"
  snapshot /evidence/s2-after-backup.json
  same /evidence/s2.json /evidence/s2-after-backup.json "B2 stop/start"
  cp /evidence/s2.json "$shared/s2.json"
  pass "B2 taken over newer state"

  # Newest state, then an unclean stop that leaves a WAL behind: the restore
  # must not let it replay onto the older database.
  agent-bus register newest@fresh --kind queue --allow owner@fresh >/dev/null
  printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIdavefixture dave@fresh' | agent-bus-admin user add dave@fresh - >/dev/null
  systemctl kill --kill-whom=all --signal=KILL agent-busd
  sleep .5
  systemctl stop agent-busd
  [ -s "$db-wal" ] || fail "precondition: the unclean stop left no WAL to replay"
  restore "$shared/b1.tar"
  snapshot /evidence/s1-restored.json
  same /evidence/s1.json /evidence/s1-restored.json "older B1 over newer state"
  if agent-bus ls newest@fresh >/dev/null 2>&1; then fail "a record from after B1 survived the restore"; fi
  [ "$(auth "$shared/alice_fresh.token")" = "200 alice@fresh" ] || fail "alice's B1 credential"
  [ "$(auth "$shared/alice-rotated.token")" = 401 ] || fail "alice's later rotation survived the restore"
  [ "$(auth "$shared/carol.token")" = 401 ] || fail "carol's later credential survived the restore"
  if grep -q 'carolfixture\|davefixture' "$home/.ssh/authorized_keys"; then fail "a later SSH key survived the restore"; fi
  pass "restoring older B1 over newer, uncleanly stopped state brings back exactly B1"
  exit 0
fi

[ "$phase" = restore ] || fail "unknown phase $phase"
install
[ "$(auth "$shared/owner.token")" = 401 ] || fail "the source node's credential works on a fresh node before restore"
restore "$shared/b2.tar"
cp "$shared/owner.token" /root/snapshot-owner.token
snapshot /evidence/s2-restored.json
same "$shared/s2.json" /evidence/s2-restored.json "B2 on a fresh host"
[ "$(auth "$shared/owner.token")" = "200 owner@fresh" ] || fail "owner credential after restore"
[ "$(auth "$shared/alice_fresh.token")" = "200 alice@fresh" ] || fail "alice's previous credential after restore"
[ "$(auth "$shared/alice-rotated.token")" = "200 alice@fresh" ] || fail "alice's rotated credential after restore"
[ "$(auth "$shared/bob_fresh.token")" = "200 bob@fresh" ] || fail "bob's credential after restore"
[ "$(auth "$shared/carol.token")" = "200 carol@fresh" ] || fail "carol's credential after restore"
[ "$(auth "$shared/_worker_fresh.token")" = "200 #worker@fresh" ] || fail "the agent's credential after restore"
[ "$(agent-bus-admin token alice@fresh)" = "$(cat "$shared/alice-rotated.token")" ] || fail "a read issued another credential"
pass "B2 on a fresh host: listings, owners, config_sha, secret_sha, counters and credentials as backed up"

# The restored node is a working node: its queues deliver in order, and a
# record registered now starts with no restored queue.
agent-bus register after@fresh --kind queue --allow owner@fresh >/dev/null
[ "$(field after@fresh queued)" = 0 ] || fail "a new record inherited a restored queue"
got=$(agent-bus consume --inbox backlog@fresh --topic t --tag g --wait 2s)
grep -q '"body":"backlog 2"' <<<"$got" || fail "restored queue lost its head: $got"
[ "$(field backlog@fresh queued)" = 2 ] || fail "restored backlog count after one read"
pass "restored queue delivers its contents in order"

# Graceful restart: everything, including traffic since the last flush, stays.
agent-bus send backlog@fresh --topic t --tag g 'sent before restart' >/dev/null
in_before=$(field backlog@fresh in)
snapshot /evidence/before-restart.json
systemctl restart agent-busd
wait_api
snapshot /evidence/after-restart.json
same /evidence/before-restart.json /evidence/after-restart.json "graceful restart"
[ "$(field backlog@fresh queued)" = 3 ] && [ "$(field backlog@fresh in)" = "$in_before" ] || fail "graceful restart lost unflushed traffic"
if agent-bus status | grep -q '"unclean":true'; then fail "a graceful restart reported an unclean stop"; fi
pass "graceful restart keeps unflushed traffic and reports a clean stop"

# Bus-child crash: the supervisor restarts it; what the periodic flush saved
# stays, what came after it is the documented loss, and committed
# administrative changes are not traffic and stay.
agent-bus send backlog@fresh --topic t --tag g 'flushed before crash' >/dev/null
sleep 65  # past one -flush-every period (1m)
agent-bus send backlog@fresh --topic t --tag g 'after the last flush' >/dev/null
agent-bus register crash-admin@fresh --kind queue --allow owner@fresh >/dev/null
[ "$(field backlog@fresh queued)" = 5 ] || fail "precondition: backlog holds the unflushed message"
supervisor=$(systemctl show agent-busd -p MainPID --value)
bus=$(pgrep -P "$supervisor" -x agent-busd)
[ -n "$bus" ] || fail "bus child was not found"
kill -KILL "$bus"
for _ in $(seq 1 100); do
  now=$(pgrep -P "$supervisor" -x agent-busd || true)
  [ -n "$now" ] && [ "$now" != "$bus" ] && break
  sleep .1
done
[ -n "$now" ] && [ "$now" != "$bus" ] || fail "the supervisor did not restart the bus child"
[ "$(systemctl show agent-busd -p MainPID --value)" = "$supervisor" ] || fail "the crash took the supervisor down"
wait_api
agent-bus status | grep -q '"unclean":true' || fail "the crash was not reported as an unclean stop"
journalctl --no-pager -u agent-busd >/evidence/crash.journal
grep -q 'did not stop cleanly; queue changes after .* are gone' /evidence/crash.journal || fail "the crash-loss boundary was not reported"
[ "$(field backlog@fresh queued)" = 4 ] || fail "backlog after crash: $(field backlog@fresh queued), not 4"
agent-bus consume --inbox backlog@fresh --topic t --tag g --wait 2s >/dev/null
agent-bus consume --inbox backlog@fresh --topic t --tag g --wait 2s >/dev/null
agent-bus consume --inbox backlog@fresh --topic t --tag g --wait 2s >/dev/null
got=$(agent-bus consume --inbox backlog@fresh --topic t --tag g --wait 2s)
grep -q '"body":"flushed before crash"' <<<"$got" || fail "flushed message lost in the crash: $got"
agent-bus ls crash-admin@fresh >/dev/null || fail "a committed administrative change was lost in the crash"
[ "$(auth "$shared/carol.token")" = "200 carol@fresh" ] || fail "credential after crash"
pass "bus-child crash: supervisor restarts it, flushed traffic and committed changes stay, later traffic is the reported loss"
