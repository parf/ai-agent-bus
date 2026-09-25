#!/bin/bash
# Runs only inside upgrade-install.sh's disposable real-systemd container.
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
fail() { echo "FAIL $*" >&2; exit 1; }
pass() { echo "PASS $*"; }
wait_api() {
  local want=$1
  for _ in $(seq 1 150); do
    got=$(curl -fsS http://127.0.0.1:6767/identity 2>/dev/null | sed -n 's/.*"version":"\([^"]*\)".*/\1/p' || true)
    [ "$got" = "$want" ] && return 0
    sleep .1
  done
  return 1
}
# A credential authenticates as exactly its principal over the shared socket.
authenticates() {
  local file=$1 who=$2 answer
  answer=$(curl -sS --max-time 4 --unix-socket /run/agent-bus/bus.sock \
    -H "X-Agent-Bus-Token: $(cat "$file")" -w '\n%{http_code}' http://bus/status) || return 1
  [ "$(tail -n1 <<<"$answer")" = 200 ] && grep -q '"you":"'"$who"'"' <<<"$answer"
}
db=/var/lib/agent-bus/daemon/agent-bus.db
unpack() {
  local source=$1 destination=$2 archive
  archive=$(find "$source" -maxdepth 1 -name 'agent-bus-*.tar.gz' -type f -print -quit)
  [ -n "$archive" ] || fail "no archive in $source"
  mkdir "$destination"
  tar -xzf "$archive" -C "$destination" --strip-components=1
  (cd "$destination" && sha256sum -c MANIFEST.sha256 >/dev/null)
}

unpack /package/old /root/old
unpack /package/new /root/new
old_version=$(cat /root/old/internal/version/VERSION)
new_version=$(cat /root/new/internal/version/VERSION)
[ "$old_version" != "$new_version" ] || fail "upgrade archives have the same version"

printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIownerfixture owner@fresh' >/root/owner-key.pub
cd /root/old
./agent-bus-setup --owner owner@fresh >/evidence/install-old.log
wait_api "$old_version" || fail "old API did not start"
agent-bus-admin user add owner@fresh - --admin </root/owner-key.pub >/evidence/add-owner-key.log

printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIpeerfixture peer@fresh' >/root/peer-key.pub
agent-bus-admin user add peer@fresh - </root/peer-key.pub >/evidence/add-peer.log
agent-bus-admin account set nobody peer@fresh >/evidence/account-map.log
systemctl restart agent-busd
wait_api "$old_version" || fail "old API did not restart with the durable account map"

agent-bus register queue@fresh --kind queue --allow owner@fresh >/dev/null
agent-bus send queue@fresh --topic upgrade --tag retained 'queued before upgrade' >/dev/null
agent-bus register protected@fresh --kind queue --allow peer@fresh >/dev/null
nobody_socket="/run/agent-bus/user-nobody.sock"
agent-bus-admin account list >/evidence/accounts-before.txt
for _ in $(seq 1 100); do [ -S "$nobody_socket" ] && break; sleep .05; done
if [ ! -S "$nobody_socket" ]; then
  find /run/agent-bus -maxdepth 1 -printf '%f %y\n' >/evidence/runtime-files.txt
  fail "mapped peer socket was not created"
fi
sudo -u nobody env AGENT_BUS_ADDR="$nobody_socket" agent-bus ls protected@fresh >/evidence/peer-before.json

mkdir -p /etc/systemd/system/agent-busd.service.d
cat >/etc/systemd/system/agent-busd.service.d/operator.conf <<'EOF'
[Service]
Environment=AGENT_BUS_OPERATOR_MARKER=preserve-me
ExecStartPre=/bin/sleep 2
EOF
systemctl daemon-reload
sha256sum /etc/systemd/system/agent-busd.service /etc/systemd/system/agent-busd.service.d/operator.conf >/evidence/config.before.sha256
cp /etc/systemd/system/agent-busd.service /evidence/unit.before
# Credentials live in the database, which the daemon rewrites while it runs,
# so they are held to what they authenticate rather than to a file hash.
agent-bus-admin token owner@fresh >/root/owner.token
agent-bus-admin token peer@fresh >/root/peer.token
authenticates /root/owner.token owner@fresh || fail "owner credential positive control"
authenticates /root/peer.token peer@fresh || fail "peer credential positive control"
[ -f "$db" ] || fail "old release has no database"
for legacy in token dump.json; do
  [ ! -e "/var/lib/agent-bus/daemon/$legacy" ] || fail "0.7 install wrote the 0.6 $legacy"
done
sha256sum /var/lib/agent-bus/daemon/.ssh/authorized_keys >/evidence/credentials.before.sha256
old_current=$(readlink /usr/local/lib/agent-bus/current)
old_pid=$(systemctl show agent-busd -p MainPID --value)
pass "populated old release has credentials, queued state, ACLs, local mappings and operator configuration"

cp -a /root/new /root/incomplete
unlink /root/incomplete/web/server.ts
if /root/incomplete/agent-bus-setup --upgrade >/evidence/incomplete.out 2>&1; then
  fail "incomplete release upgraded"
fi
[ "$(readlink /usr/local/lib/agent-bus/current)" = "$old_current" ] || fail "incomplete release changed current"
[ "$(systemctl show agent-busd -p MainPID --value)" = "$old_pid" ] || fail "incomplete release stopped the old daemon"
grep -q 'web/server.ts' /evidence/incomplete.out || fail "incomplete release did not name the missing component"
pass "incomplete release fails before stopping the old daemon"

/root/new/agent-bus-setup --upgrade >/evidence/config-change.out 2>&1 &
upgrade_pid=$!
for _ in $(seq 1 1000); do
  current=$(readlink /usr/local/lib/agent-bus/current)
  [ "$current" != "$old_current" ] && break
  kill -0 "$upgrade_pid" 2>/dev/null || break
  sleep .01
done
[ "${current:-$old_current}" != "$old_current" ] || fail "configuration mutation missed the selected-release window"
printf '\n# hostile mid-upgrade edit\n' >>/etc/systemd/system/agent-busd.service
if wait "$upgrade_pid"; then fail "upgrade accepted changed operator configuration"; fi
grep -q 'operator configuration' /evidence/config-change.out || fail "changed configuration was not named"
wait_api "$old_version" || fail "configuration-check rollback did not restore the old daemon"
cp /evidence/unit.before /etc/systemd/system/agent-busd.service
systemctl daemon-reload
pass "mid-upgrade operator-configuration change fails and rolls back"

/root/new/agent-bus-setup --upgrade >/evidence/interrupted.out 2>&1 &
upgrade_pid=$!
for _ in $(seq 1 1000); do
  current=$(readlink /usr/local/lib/agent-bus/current)
  [ "$current" != "$old_current" ] && break
  kill -0 "$upgrade_pid" 2>/dev/null || break
  sleep .01
done
[ "${current:-$old_current}" != "$old_current" ] || fail "upgrade completed or failed before its selection boundary could be interrupted"
kill -9 "$upgrade_pid"
wait "$upgrade_pid" 2>/dev/null || true
[ -f /usr/local/lib/agent-bus/upgrade.json ] || fail "interrupted upgrade left no recovery marker"
/root/new/agent-bus-setup --recover >/evidence/recover.out
wait_api "$old_version" || fail "recovery did not restore the old daemon"
[ "$(readlink /usr/local/lib/agent-bus/current)" = "$old_current" ] || fail "recovery did not restore old current"
[ ! -e /usr/local/lib/agent-bus/upgrade.json ] || fail "recovery marker survived successful recovery"
pass "killed upgrade is recovered by the documented command"

cp -a /root/new /root/broken
cat >/root/broken/agent-busd <<'EOF'
#!/bin/sh
printf 'new_release_only database\n' >/var/lib/agent-bus/daemon/agent-bus.db
rm -f /var/lib/agent-bus/daemon/agent-bus.db-wal /var/lib/agent-bus/daemon/agent-bus.db-shm
printf 'new_release_only\n' >/var/lib/agent-bus/daemon/new-release-only
exit 73
EOF
chmod 0755 /root/broken/agent-busd
(
  cd /root/broken
  sha256sum \
    agent-bus agent-busd agent-bus-admin agent-bus-setup agent-bus-token \
    web/server.ts web/agent-bus-web.service \
    mcp/server.js launchers/launcher.js \
    launchers/ab-claude launchers/ab-codex launchers/ab-opencode \
    internal/version/VERSION LICENSE.md INSTALL.md >MANIFEST.sha256
)
if /root/broken/agent-bus-setup --upgrade >/evidence/broken-start.out 2>&1; then
  fail "start-broken release upgraded"
fi
grep -q 'rolled back' /evidence/broken-start.out || fail "failed start did not report rollback"
wait_api "$old_version" || fail "automatic rollback did not restore the old daemon"
(cd / && sha256sum -c /evidence/credentials.before.sha256 >/dev/null) || fail "rollback restored mismatched SSH authorization"
authenticates /root/owner.token owner@fresh || fail "rollback lost the owner credential"
authenticates /root/peer.token peer@fresh || fail "rollback lost the peer credential"
if grep -q 'new_release_only' "$db"; then fail "rollback retained the new release's database"; fi
[ ! -e /var/lib/agent-bus/daemon/new-release-only ] || fail "rollback retained new-release-only state"
agent-bus ls queue@fresh >/dev/null || fail "rollback did not restore the old registry state"
pass "failed new release restores the old release with its matching credential and state tree"

/root/new/agent-bus-setup --upgrade >/evidence/upgrade.log
wait_api "$new_version" || fail "new API did not start"
[ ! -e /usr/local/lib/agent-bus/upgrade.json ] || fail "successful upgrade left a recovery marker"
sha256sum -c /evidence/config.before.sha256 >/dev/null || fail "operator unit configuration changed"
sha256sum /var/lib/agent-bus/daemon/.ssh/authorized_keys >/evidence/credentials.after.sha256
[ "$(cat /evidence/credentials.before.sha256)" = "$(cat /evidence/credentials.after.sha256)" ] || fail "SSH authorization changed across upgrade"
authenticates /root/owner.token owner@fresh || fail "owner credential lost across upgrade"
authenticates /root/peer.token peer@fresh || fail "peer credential lost across upgrade"

sudo -u nobody env AGENT_BUS_ADDR="$nobody_socket" agent-bus ls protected@fresh >/evidence/peer-after.json
grep -q 'protected@fresh' /evidence/peer-after.json || fail "ACL or local mapping was lost"
queued=$(agent-bus consume --inbox queue@fresh --topic upgrade --tag retained --wait 2s)
grep -q 'queued before upgrade' <<<"$queued" || fail "queued message was lost"

current=$(readlink -f /usr/local/lib/agent-bus/current)
case "$current" in */releases/"$new_version"-*) ;; *) fail "current is not the new release: $current" ;; esac
for command in agent-bus agent-busd agent-bus-admin agent-bus-setup agent-bus-token; do
  [ "$("/usr/local/bin/$command" --version | head -1)" = "$new_version" ] || fail "$command is not $new_version"
done
cgroup=$(systemctl show agent-busd -p ControlGroup --value)
daemons=0
while read -r pid; do
  exe=$(readlink "/proc/$pid/exe" 2>/dev/null || true)
  case "$exe" in
    "$current/agent-busd") daemons=$((daemons+1)) ;;
    */releases/*/agent-busd) fail "mixed running release: $exe" ;;
  esac
done < <(find "/sys/fs/cgroup/${cgroup#/}" -name cgroup.procs -type f -exec cat {} + | sort -u)
[ "$daemons" -ge 2 ] || fail "new running set is incomplete: daemon=$daemons"
# The web face is its own unit, following the release its link names.
systemctl is-active --quiet agent-bus-web || fail "agent-bus-web is not active after the upgrade"
curl -fsS http://127.0.0.1:6780/ >/evidence/web.html
# The node summary's own element, not any mention of the version.
grep -qF ">v$new_version</span>" /evidence/web.html || fail "web does not show the new node release"
pass "successful upgrade preserves state and configuration and runs one intended release"

systemctl show agent-busd -p ActiveState -p MainPID -p ControlGroup >/evidence/unit-state.txt
systemctl --version | head -1 >/evidence/host.txt
printf 'old=%s\nnew=%s\n' "$old_version" "$new_version" >>/evidence/host.txt
