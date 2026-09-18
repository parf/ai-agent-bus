#!/bin/bash
# Runs only inside fresh-install.sh's disposable real-systemd container.
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
fail() { echo "FAIL $*" >&2; exit 1; }
pass() { echo "PASS $*"; }

cp /package/agent-bus-*.tar.gz /root/release.tar.gz
cp /package/agent-bus-*.tar.gz.sha256 /root/release.tar.gz.sha256.original
archive_name=$(awk '{print $2}' /root/release.tar.gz.sha256.original)
printf '%s  %s\n' "$(awk '{print $1}' /root/release.tar.gz.sha256.original)" release.tar.gz >/root/release.tar.gz.sha256
cd /root
sha256sum -c release.tar.gz.sha256
mkdir valid
tar -xzf release.tar.gz -C valid --strip-components=1
pass "packaged checksum and archive"

pristine() {
  ! getent passwd agent-busd >/dev/null 2>&1
  ! getent passwd agent-bus-runner >/dev/null 2>&1
  [ ! -e /var/lib/agent-bus ]
  [ ! -e /etc/systemd/system/agent-busd.service ]
  [ ! -e /usr/local/lib/agent-bus/current ]
}
mutate_missing() {
  local label=$1 missing=$2 dir=/root/mutation-$1
  cp -a /root/valid "$dir"
  unlink "$dir/$missing"
  if "$dir/agent-bus-setup" --owner owner@fresh >"/evidence/$label.out" 2>&1; then
    fail "$label package unexpectedly installed"
  fi
  grep -q "$missing" "/evidence/$label.out" || fail "$label did not name $missing"
  pristine || fail "$label changed accounts, state, unit or current"
  pass "$label missing artifact fails before host mutation"
}
mutate_missing binary agent-bus-web
mutate_missing mcp mcp/server.js
mutate_missing launcher launchers/launcher.js

cd /root/valid
./agent-bus-setup --owner owner@fresh >/evidence/setup.log
systemctl is-active --quiet agent-busd || fail "installed unit is inactive"
for _ in $(seq 1 100); do
  curl -fsS http://127.0.0.1:6767/identity >/dev/null 2>&1 && break
  sleep .1
done
curl -fsS http://127.0.0.1:6767/identity >/dev/null || fail "API did not start"
curl -fsS http://127.0.0.1:6780/ >/evidence/web.html || fail "dashboard did not start"
grep -q '<html' /evidence/web.html || fail "dashboard returned no page"
main_pid=$(systemctl show agent-busd -p MainPID --value)
main_uid=$(awk '/^Uid:/{print $2}' "/proc/$main_pid/status")
[ "$main_uid" = "$(id -u agent-busd)" ] || fail "daemon uid is $main_uid, not agent-busd"
[ "$(stat -c '%U:%G %a' /var/lib/agent-bus/daemon)" = "agent-busd:agent-busd 700" ] || fail "daemon state ownership or mode"
pass "real generated systemd unit runs under its service account and starts API plus dashboard"

for command in agent-bus agent-busd agent-bus-admin agent-bus-setup agent-bus-token agent-bus-web; do
  [ -L "/usr/local/bin/$command" ] || fail "$command is not a stable link"
  target=$(readlink "/usr/local/bin/$command")
  [ "$target" = "/usr/local/lib/agent-bus/current/$command" ] || fail "$command targets $target"
  "/usr/local/bin/$command" --version >"/evidence/$command.version"
  grep -q '^build_info: ' "/evidence/$command.version" || fail "$command has no build stamp"
done
for command in ab-claude ab-codex ab-opencode; do
  target=$(readlink "/usr/local/bin/$command")
  [ "$target" = "/usr/local/lib/agent-bus/current/launchers/$command" ] || fail "$command targets $target"
done
[ -f /usr/local/lib/agent-bus/current/mcp/server.js ] || fail "MCP face was not installed"
[ -f /usr/local/lib/agent-bus/current/launchers/launcher.js ] || fail "launcher face was not installed"
(cd /usr/local/lib/agent-bus/current && sha256sum -c MANIFEST.sha256 >/dev/null)
unit=$(cat /etc/systemd/system/agent-busd.service)
[ "$(grep -oE -- ' -web([[:space:]]|$)' <<<"$unit" | wc -l)" -eq 1 ] || fail "unit does not enable web exactly once"
if grep -Eq '/root/valid|/package|/fixture|/home/|/rd/' <<<"$unit"; then fail "unit contains a build-host path"; fi
for command in agent-bus agent-busd agent-bus-admin agent-bus-setup agent-bus-token agent-bus-web ab-claude ab-codex ab-opencode; do
  case $(readlink "/usr/local/bin/$command") in /usr/local/lib/agent-bus/current/*) ;; *) fail "$command escaped the installed tree" ;; esac
done
pass "complete stamped release is installed without build-host paths"

cat >/root/fresh-echo.sh <<'SH'
#!/bin/sh
printf 'fresh reply: %s\n' "$1"
SH
chmod 0755 /root/fresh-echo.sh
/usr/local/bin/agent-bus start fresh-echo@fresh --algo=args --allow owner@fresh /root/fresh-echo.sh >/evidence/service.log 2>&1 &
service_pid=$!
trap 'kill "$service_pid" 2>/dev/null || true; wait "$service_pid" 2>/dev/null || true' EXIT
for _ in $(seq 1 100); do
  /usr/local/bin/agent-bus ls fresh-echo@fresh >/dev/null 2>&1 && break
  sleep .1
done
/usr/local/bin/agent-bus register owner@fresh --kind agent --allow fresh-echo@fresh >/evidence/reply-grant.log
reply=$(/usr/local/bin/agent-bus call fresh-echo@fresh --wait 5s 'installation works')
grep -q 'fresh reply: installation works' <<<"$reply" || fail "installed service call did not return its unique reply"
pass "new user calls a real service using only installed programs"

before=$(readlink /usr/local/lib/agent-bus/current)
./agent-bus-setup --owner owner@fresh >/evidence/reinstall.log
after=$(readlink /usr/local/lib/agent-bus/current)
[ "$before" = "$after" ] || fail "identical reinstall selected another release"
[ "$(find /usr/local/lib/agent-bus/releases -mindepth 1 -maxdepth 1 -type d | wc -l)" -eq 1 ] || fail "identical reinstall duplicated the release"
pass "identical package reinstall is idempotent"

systemctl show agent-busd -p ActiveState -p User -p FragmentPath >/evidence/unit-state.txt
systemctl --version | head -1 >/evidence/host.txt
printf 'archive=%s\n' "$archive_name" >>/evidence/host.txt
