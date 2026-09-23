#!/bin/bash
# Runs only inside reinstall-install.sh's disposable real-systemd container.
# A real stopped-to-be 0.6 installation, with an operator drop-in overriding
# ExecStart, is replaced by `agent-bus-setup --reinstall`; then the fresh 0.7
# installation is itself reinstalled. See Plans/MVP/0.7-cutover.md#procedure.
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
fail() { echo "FAIL $*" >&2; exit 1; }
pass() { echo "PASS $*"; }
home=/var/lib/agent-bus/daemon
db=$home/agent-bus.db
dropins=/etc/systemd/system/agent-busd.service.d
wait_api() {
  local want=$1
  for _ in $(seq 1 150); do
    got=$(curl -fsS --max-time 2 http://127.0.0.1:6767/identity 2>/dev/null | sed -n 's/.*"version":"\([^"]*\)".*/\1/p' || true)
    [ "$got" = "$want" ] && return 0
    sleep .1
  done
  return 1
}
# The HTTP status a credential gets from /status over the shared socket, and
# the answer beside it. Never a token on a command line inside a log.
status_with() {
  curl -sS --max-time 4 --unix-socket /run/agent-bus/bus.sock \
    -H "X-Agent-Bus-Token: $(cat "$1")" -o "$2" -w '%{http_code}' http://bus/status
}
unpack() {
  local source=$1 destination=$2 archive
  archive=$(find "$source" -maxdepth 1 -name 'agent-bus-*.tar.gz' -type f -print -quit)
  [ -n "$archive" ] || fail "no archive in $source"
  mkdir "$destination"
  tar -xzf "$archive" -C "$destination" --strip-components=1
  (cd "$destination" && sha256sum -c MANIFEST.sha256 >/dev/null)
}
asides() { find /var/lib/agent-bus -mindepth 1 -maxdepth 1 -name 'daemon.before-0.7-*' | sort; }

unpack /package/old /root/old
unpack /package/new /root/new
old_version=$(cat /root/old/internal/version/VERSION)
new_version=$(cat /root/new/internal/version/VERSION)
case $old_version in 0.6.*) ;; *) fail "old archive is $old_version, not a 0.6 release" ;; esac
case $new_version in 0.7.*) ;; *) fail "new archive is $new_version, not a 0.7 release" ;; esac

# --- a populated 0.6 installation --------------------------------------------
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIownerfixture owner@fresh' >/root/owner-key.pub
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIpeerfixture peer@fresh' >/root/peer-key.pub
(cd /root/old && ./agent-bus-setup --owner owner@fresh --key /root/owner-key.pub >/evidence/install-old.log 2>&1) ||
  fail "0.6 release did not install"
wait_api "$old_version" || fail "0.6 API did not start"
agent-bus-admin user add peer@fresh - </root/peer-key.pub >/evidence/add-peer-old.log
agent-bus-admin token owner@fresh >/root/old-owner.token
agent-bus-admin token peer@fresh >/root/old-peer.token
agent-bus register backlog@fresh --kind queue --allow owner@fresh >/dev/null
agent-bus send backlog@fresh --topic reinstall --tag old 'queued in 0.6' >/dev/null
[ "$(status_with /root/old-owner.token /evidence/old-owner-before.json)" = 200 ] || fail "0.6 owner credential positive control"
[ "$(status_with /root/old-peer.token /evidence/old-peer-before.json)" = 200 ] || fail "0.6 peer credential positive control"

# The operator's override replaces ExecStart with the 0.6 flags; left in
# place it would start the 0.7 daemon with flags it no longer has.
mkdir -p "$dropins"
{
  printf '[Service]\nEnvironment=AGENT_BUS_OPERATOR_MARKER=from-0.6\nExecStart=\n'
  grep '^ExecStart=' /etc/systemd/system/agent-busd.service | sed 's/$/ -dump-every 30s/'
} >"$dropins/override.conf"
grep -q -- '-token-file' "$dropins/override.conf" || fail "0.6 unit carries no -token-file to override"
systemctl daemon-reload
systemctl restart agent-busd
wait_api "$old_version" || fail "0.6 daemon did not restart under the operator override"
systemctl show agent-busd -p DropInPaths --value | grep -q override.conf || fail "operator drop-in is not active"
agent-bus send backlog@fresh --topic reinstall --tag old 'still queued in 0.6' >/dev/null
for f in dump.json token .ssh/authorized_keys; do
  [ -f "$home/$f" ] || fail "0.6 state has no $f"
done
grep -qF 'peerfixture' "$home/.ssh/authorized_keys" || fail "0.6 authorization has no peer key"
# The snapshot is rewritten by the graceful stop, so it is recognised by its
# content; the credentials and keys must arrive byte for byte.
(cd "$home" && sha256sum token .ssh/authorized_keys) >/evidence/old-state.sha256
(cd "$dropins" && sha256sum override.conf) >/evidence/old-dropins.sha256
old_owner_token=$(cat /root/old-owner.token)
[ -z "$(asides)" ] || fail "a set-aside directory existed before the reinstall"
pass "populated 0.6 installation with credentials, backlog, SSH keys and an ExecStart override"

# --- reinstall over it --------------------------------------------------------
old_pid=$(systemctl show agent-busd -p MainPID --value)
(cd /root/new && ./agent-bus-setup --reinstall --owner owner@fresh --key /root/owner-key.pub >/evidence/reinstall-1.log 2>&1) ||
  { cat /evidence/reinstall-1.log >&2; fail "reinstall over 0.6 failed"; }
[ -z "$(ps -o pid= -p "$old_pid" 2>/dev/null)" ] || fail "the 0.6 supervisor survived the reinstall"
wait_api "$new_version" || fail "0.7 API did not start after the reinstall"

# Old files exist only in the one root-only set-aside directory.
mapfile -t found < <(asides)
[ "${#found[@]}" -eq 1 ] || fail "expected one set-aside directory, found ${#found[@]}"
aside=${found[0]}
[ "$(stat -c '%U:%G %a' "$aside")" = "root:root 700" ] || fail "set-aside directory is not root-only: $(stat -c '%U:%G %a' "$aside")"
if runuser -u agent-busd -- ls "$aside" >/dev/null 2>&1; then fail "the daemon account can list the set-aside directory"; fi
(cd "$aside/home" && sha256sum -c /evidence/old-state.sha256 >/dev/null) || fail "set-aside home is not the old state"
grep -q 'backlog@fresh' "$aside/home/dump.json" || fail "set-aside snapshot is not the old one"
(cd "$aside/agent-busd.service.d" && sha256sum -c /evidence/old-dropins.sha256 >/dev/null) || fail "set-aside drop-ins are not the old ones"
for f in dump.json token; do
  [ ! -e "$home/$f" ] || fail "0.6 $f remains in the daemon home"
done
if grep -qF 'peerfixture' "$home/.ssh/authorized_keys" 2>/dev/null; then fail "0.6 peer key remains authorized"; fi
grep -rlF -D skip "$old_owner_token" /var /etc /usr/local /run 2>/dev/null >/evidence/old-token-hits.txt || true
[ -s /evidence/old-token-hits.txt ] || fail "old credential search found nothing, not even the set-aside copy"
while read -r hit; do
  case $hit in "$aside"/*) ;; *) fail "old credential outside the set-aside directory: $hit" ;; esac
done </evidence/old-token-hits.txt
pass "old files and drop-ins are only in the root-only $aside"

[ -z "$(find "$dropins" -mindepth 1 2>/dev/null)" ] || fail "drop-ins remain in $dropins"
if systemctl show agent-busd -p DropInPaths --value | grep -q override.conf; then fail "systemd still applies the override"; fi
main_pid=$(systemctl show agent-busd -p MainPID --value)
if tr '\0' '\n' <"/proc/$main_pid/environ" | grep -q AGENT_BUS_OPERATOR_MARKER; then fail "the daemon runs with the old drop-in's environment"; fi
# The process title is rewritten at start, so systemd is asked what it ran.
systemctl show agent-busd -p ExecStart --value >/evidence/execstart-after.txt
grep -qF -- "-db $db" /evidence/execstart-after.txt || fail "the daemon does not run the generated 0.7 command line"
if grep -q -- '-token-file' /evidence/execstart-after.txt; then fail "the old ExecStart override still applies"; fi
pass "the unit's drop-ins are gone and the generated unit runs as written"

[ -f "$db" ] || fail "no fresh database"
[ "$(stat -c '%U %a' "$db")" = "agent-busd 600" ] || fail "database owner or mode: $(stat -c '%U %a' "$db")"
for command in agent-busd agent-bus-setup agent-bus-admin; do
  [ "$("/usr/local/bin/$command" --version | head -1)" = "$new_version" ] || fail "$command is not $new_version"
done
agent-bus-admin token owner@fresh >/root/new-owner.token
[ "$(status_with /root/new-owner.token /evidence/new-owner.json)" = 200 ] || fail "new owner credential is refused"
grep -q '"you":"owner@fresh"' /evidence/new-owner.json || fail "new credential is not the installer's"
grep -q '"daemon_owner":true' /evidence/new-owner.json || fail "the installer is not the daemon owner"
[ "$(cat /root/new-owner.token)" != "$(cat /root/old-owner.token)" ] || fail "the reinstall reissued the old credential"
pass "fresh $new_version database; the installer owns the node"

code=$(status_with /root/old-owner.token /evidence/old-owner-after.json)
[ "$code" = 401 ] || fail "old owner credential got $code, not 401"
code=$(status_with /root/old-peer.token /evidence/old-peer-after.json)
[ "$code" = 401 ] || fail "old peer credential got $code, not 401"
curl -sS --max-time 4 --unix-socket /run/agent-bus/bus.sock -H "X-Agent-Bus-Token: $(cat /root/new-owner.token)" \
  http://bus/users >/evidence/users-after.json
grep -q '"name":"owner@fresh"' /evidence/users-after.json || fail "user listing positive control"
if grep -q '"name":"peer@fresh"' /evidence/users-after.json; then fail "0.6 user was imported"; fi
if agent-bus ls backlog@fresh >/evidence/backlog-after.txt 2>&1 && grep -q 'backlog@fresh' /evidence/backlog-after.txt; then
  fail "0.6 record and backlog were imported"
fi
pass "old credentials authenticate nothing (401) and old users, records and backlog are not imported"

agent-bus register fresh-queue@fresh --kind queue --allow owner@fresh >/dev/null
agent-bus send fresh-queue@fresh --topic reinstall --tag new 'sent after reinstall' >/dev/null
got=$(agent-bus consume --inbox fresh-queue@fresh --topic reinstall --tag new --wait 2s)
grep -q 'sent after reinstall' <<<"$got" || fail "fresh node did not complete a send/consume exchange"
pass "fresh node completes a send/consume exchange"

# A database lost afterwards refuses the start; nothing recreates it.
systemctl stop agent-busd
mkdir -m 700 /root/db.saved
mv "$db"* /root/db.saved/
systemctl start agent-busd || true
sleep 5
[ ! -e "$db" ] || fail "a missing database was silently recreated"
if curl -fsS --max-time 2 http://127.0.0.1:6767/identity >/dev/null 2>&1; then fail "the daemon serves without its database"; fi
journalctl --no-pager -u agent-busd >/evidence/missing-db.journal
grep -qF "no database at $db" /evidence/missing-db.journal || fail "missing database refusal was not reported"
systemctl stop agent-busd
mv /root/db.saved/* "$home/"
systemctl reset-failed agent-busd || true
systemctl start agent-busd
wait_api "$new_version" || fail "restored database did not start"
[ "$(status_with /root/new-owner.token /evidence/restored-owner.json)" = 200 ] || fail "restored database lost the owner credential"
pass "a missing database refuses the start instead of being recreated"

# --- reinstall over a 0.7 installation ---------------------------------------
sleep 1.1  # set-aside names carry the second; a second one must be new
(cd /root/new && ./agent-bus-setup --reinstall --owner owner@fresh --key /root/owner-key.pub >/evidence/reinstall-2.log 2>&1) ||
  { cat /evidence/reinstall-2.log >&2; fail "reinstall over 0.7 failed"; }
wait_api "$new_version" || fail "0.7 API did not start after the second reinstall"
mapfile -t found < <(asides)
[ "${#found[@]}" -eq 2 ] || fail "expected two set-aside directories, found ${#found[@]}"
second=${found[1]}
[ "$second" != "$aside" ] || fail "second reinstall reused the first set-aside directory"
[ "$(stat -c '%U:%G %a' "$second")" = "root:root 700" ] || fail "second set-aside directory is not root-only"
[ -f "$second/home/agent-bus.db" ] || fail "the 0.7 database was not set aside"
(cd "$aside/home" && sha256sum -c /evidence/old-state.sha256 >/dev/null) || fail "second reinstall disturbed the first set-aside"
code=$(status_with /root/new-owner.token /evidence/prior-owner-after.json)
[ "$code" = 401 ] || fail "prior 0.7 owner credential got $code, not 401"
agent-bus-admin token owner@fresh >/root/third-owner.token
[ "$(status_with /root/third-owner.token /evidence/third-owner.json)" = 200 ] || fail "owner credential after second reinstall"
grep -q '"daemon_owner":true' /evidence/third-owner.json || fail "installer is not the owner after second reinstall"
if agent-bus ls fresh-queue@fresh >/evidence/fresh-queue-after.txt 2>&1 && grep -q 'fresh-queue@fresh' /evidence/fresh-queue-after.txt; then
  fail "prior 0.7 record survived the reinstall"
fi
pass "reinstall over 0.7 sets its database aside too and issues fresh credentials"

systemctl show agent-busd -p ActiveState -p MainPID -p DropInPaths >/evidence/unit-state.txt
systemctl --version | head -1 >/evidence/host.txt
printf 'old=%s\nnew=%s\naside=%s\nsecond=%s\n' "$old_version" "$new_version" "$aside" "$second" >>/evidence/host.txt
