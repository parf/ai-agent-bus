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
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIownerfixture owner@fresh' >/root/owner-key.pub
if ! ./agent-bus-setup --owner owner@fresh --key /root/owner-key.pub >/evidence/setup.log 2>&1; then
  fail "package setup did not establish the generated service-account unit"
fi
systemctl is-active --quiet agent-busd || fail "installed unit is inactive"
for _ in $(seq 1 100); do
  curl -fsS http://127.0.0.1:6767/identity >/dev/null 2>&1 && break
  sleep .1
done
curl -fsS http://127.0.0.1:6767/identity >/dev/null || fail "API did not start"
status_field() { awk -v key="$2:" '$1 == key { print $2 }' "/proc/$1/status"; }
find_bus_child() {
  local parent=$1 child
  for child in $(pgrep -P "$parent"); do
    if tr '\0' '\n' <"/proc/$child/environ" | grep -qx 'AGENT_BUS_ROLE=bus'; then
      printf '%s\n' "$child"
      return 0
    fi
  done
  return 1
}
assert_process_boundary() {
  local pid=$1 label=$2 expected_cap=$3
  [ -r "/proc/$pid/status" ] || fail "$label process is absent"
  [ "$(status_field "$pid" Uid)" = "$(id -u agent-busd)" ] || fail "$label does not run as agent-busd"
  [ "$(status_field "$pid" CapPrm)" = "$expected_cap" ] || fail "$label permitted capabilities"
  [ "$(status_field "$pid" CapEff)" = "$expected_cap" ] || fail "$label effective capabilities"
  [ "$(status_field "$pid" CapAmb)" = "$expected_cap" ] || fail "$label ambient capabilities"
  [ "$(status_field "$pid" NoNewPrivs)" = 1 ] || fail "$label lacks NoNewPrivs"
}
main_pid=$(systemctl show agent-busd -p MainPID --value)
bus_pid=$(find_bus_child "$main_pid") || fail "bus child was not found"
assert_process_boundary "$main_pid" supervisor 0000000000000001
assert_process_boundary "$bus_pid" bus 0000000000000000
curl -fsS http://127.0.0.1:6780/ >/evidence/web.html || fail "dashboard did not start"
grep -q '<html' /evidence/web.html || fail "dashboard returned no page"
main_uid=$(awk '/^Uid:/{print $2}' "/proc/$main_pid/status")
[ "$main_uid" = "$(id -u agent-busd)" ] || fail "daemon uid is $main_uid, not agent-busd"
[ "$(stat -c '%U:%G %a' /var/lib/agent-bus/daemon)" = "agent-busd:agent-busd 700" ] || fail "daemon state ownership or mode"
pass "real generated systemd unit runs under its service account and starts API plus dashboard"
grep -q 'agent-bus-admin owner@fresh' /var/lib/agent-bus/daemon/.ssh/authorized_keys || fail "first operator key was not installed after socket readiness"
pass "first-user provisioning waits for the daemon account credential socket"

# The installed shared-host boundary needs actual accounts, not numeric fixture
# UIDs or direct calls that bypass socket discovery. Create the people through
# the established owner administration path, persist their local mappings, and
# restart the whole supervisor because it owns the listeners.
useradd --create-home --shell /bin/sh alice
useradd --create-home --shell /bin/sh bob
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIalicefixture alice@fresh' >/root/alice-key.pub
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIbobfixture bob@fresh' >/root/bob-key.pub
agent-bus-admin user add alice@fresh - </root/alice-key.pub >/evidence/add-alice.log
agent-bus-admin user add bob@fresh - </root/bob-key.pub >/evidence/add-bob.log
agent-bus-admin account set alice alice@fresh >/evidence/map-alice.log
agent-bus-admin account set bob bob@fresh >/evidence/map-bob.log
grep -q 'restart agent-busd' /evidence/map-alice.log || fail "alice mapping did not require the listener restart"
grep -q 'restart agent-busd' /evidence/map-bob.log || fail "bob mapping did not require the listener restart"
systemctl restart agent-busd
for _ in $(seq 1 100); do
  curl -fsS http://127.0.0.1:6767/identity >/dev/null 2>&1 && break
  sleep .1
done
curl -fsS http://127.0.0.1:6767/identity >/dev/null || fail "API did not return after account-map restart"
agent-bus-admin account list >/evidence/account-map.txt
grep -q $'^alice\talice@fresh$' /evidence/account-map.txt || fail "alice mapping is absent after restart"
grep -q $'^bob\tbob@fresh$' /evidence/account-map.txt || fail "bob mapping is absent after restart"
if grep -q 'restart required' /evidence/account-map.txt; then fail "persisted account map is not active after restart"; fi

runtime=/run/agent-bus
[ "$(stat -c '%a' "$runtime")" = 711 ] || fail "runtime directory is not 0711"
for account in alice bob; do
  socket="$runtime/user-$account.sock"
  [ -S "$socket" ] || fail "$account socket is absent"
  if ! runuser -u "$account" -- agent-bus status >"/evidence/$account.status" 2>&1; then
    fail "$account could not use its own mapped socket"
  fi
  grep -q '"you":"'"$account"'@fresh"' "/evidence/$account.status" || fail "$account did not authenticate as its mapped principal"
done
pass "two actual accounts discover their own sockets and authenticate as themselves"

if runuser -u alice -- env AGENT_BUS_ADDR="$runtime/user-bob.sock" agent-bus status >/evidence/alice-to-bob.out 2>&1; then
  fail "alice used bob's credential-bearing socket"
fi
if runuser -u bob -- env AGENT_BUS_ADDR="$runtime/user-alice.sock" agent-bus status >/evidence/bob-to-alice.out 2>&1; then
  fail "bob used alice's credential-bearing socket"
fi
if runuser -u alice -- env AGENT_BUS_ADDR="$runtime/bus.sock" agent-bus status >/evidence/alice-shared-without-token.out 2>&1; then
  fail "shared socket accepted alice without a token"
fi
grep -q 'set AGENT_BUS_TOKEN' /evidence/alice-shared-without-token.out || fail "shared-socket refusal did not name the missing credential"
pass "cross-account sockets and the tokenless shared socket are refused"
for account in alice bob; do
  socket="$runtime/user-$account.sock"
  [ "$(stat -c '%U %a' "$socket")" = "$account 600" ] || fail "$account socket is not owned by that account with mode 0600"
done
pass "credential-bearing sockets have their exact owner and mode"

main_pid=$(systemctl show agent-busd -p MainPID --value)
bus_pid=$(find_bus_child "$main_pid") || fail "bus child was not found"
web_pid=$(pgrep -f '^/agent-bus-web([[:space:]]|$)' | head -1 || true)
[ -n "$web_pid" ] || fail "confined web child was not found"
assert_process_boundary "$main_pid" supervisor 0000000000000001
assert_process_boundary "$bus_pid" bus 0000000000000000
assert_process_boundary "$web_pid" web 0000000000000000
{
  printf 'supervisor=%s\nbus=%s\nweb=%s\n' "$main_pid" "$bus_pid" "$web_pid"
  for pid in "$main_pid" "$bus_pid" "$web_pid"; do
    printf '%s ' "$pid"
    grep -E '^(Uid|CapInh|CapPrm|CapEff|CapBnd|CapAmb|NoNewPrivs):' "/proc/$pid/status" | tr '\n' ' '
    printf '\n'
  done
} >/evidence/process-boundary.txt
pass "supervisor alone holds CAP_CHOWN; bus and web hold none; all run as agent-busd with NoNewPrivs"

# F.12's installed-browser foundation uses a real distribution browser against
# this packaged host. Keep the credential inside the disposable container; the
# driver records cookie properties and screenshots, never the token value.
agent-bus-admin token owner@fresh >/root/owner.token
python /fixture/browser.py --base http://127.0.0.1:6780 --token /root/owner.token --supervisor "$main_pid" --evidence /evidence
pass "real installed browser follows sign-in, web-restart, bus-restart and sign-out session semantics"

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
