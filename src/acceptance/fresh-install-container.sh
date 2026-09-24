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
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIcarolfixture carol@fresh' >/root/carol-key.pub
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIdavefixture dave@fresh' >/root/dave-key.pub
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIevefixture eve@fresh' >/root/eve-key.pub
agent-bus-admin user add alice@fresh - --admin </root/alice-key.pub >/evidence/add-alice.log
agent-bus-admin user add bob@fresh - </root/bob-key.pub >/evidence/add-bob.log
agent-bus-admin user add carol@fresh - </root/carol-key.pub >/evidence/add-carol.log
agent-bus-admin user add dave@fresh - </root/dave-key.pub >/evidence/add-dave.log
agent-bus-admin user add eve@fresh - </root/eve-key.pub >/evidence/add-eve.log
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

# The release the node reports is the package's own build, not only its
# number: an unstamped or foreign build with the same VERSION is refused.
package_version=$(cat /root/valid/internal/version/VERSION)
package_build=$(/root/valid/agent-busd --version | sed -n 's/^build_info: //p')
[ -n "$package_build" ] && [ "$package_build" != "development (unstamped)" ] || fail "package agent-busd is unstamped: $package_build"
identity_field() { sed -n 's/.*"'"$2"'":"\([^"]*\)".*/\1/p' "$1"; }
curl -fsS http://127.0.0.1:6767/identity >/evidence/identity-installed.json || fail "identity after install"
[ "$(identity_field /evidence/identity-installed.json version)" = "$package_version" ] || fail "node does not report $package_version"
[ "$(identity_field /evidence/identity-installed.json build_info)" = "$package_build" ] || fail "node build_info is not the package's $package_build"
for command in agent-bus agent-busd agent-bus-admin agent-bus-setup agent-bus-token agent-bus-web; do
  [ "$("/usr/local/bin/$command" --version | sed -n 's/^build_info: //p')" = "$package_build" ] || fail "$command build_info is not the package's"
done
pass "installed commands and the running node report the package's build_info $package_build"

# Setup over a running node: the durable Owner is reported, not the --owner
# seed, and a changed unit is applied — setup returns only once the daemon
# answers from it with the installed release.
agent-bus-admin token owner@fresh >/root/owner.token
agent-bus-admin token alice@fresh >/root/alice.token
code=$(curl -sS --max-time 4 --unix-socket /run/agent-bus/bus.sock -H "X-Agent-Bus-Token: $(cat /root/owner.token)" \
  -H 'Content-Type: application/json' --data '{"name":"alice@fresh"}' -o /evidence/owner-transfer.json -w '%{http_code}' http://bus/owner)
[ "$code" = 200 ] || fail "daemon ownership transfer got $code: $(cat /evidence/owner-transfer.json)"
curl -fsS http://127.0.0.1:6767/identity >/evidence/identity-transferred.json
[ "$(identity_field /evidence/identity-transferred.json owner)" = alice@fresh ] || fail "transfer positive control: owner is not alice@fresh"
before_pid=$(systemctl show agent-busd -p MainPID --value)
./agent-bus-setup --owner owner@fresh --addr 127.0.0.1:6768 >/evidence/setup-changed-unit.log 2>&1 ||
  { cat /evidence/setup-changed-unit.log >&2; fail "setup over the running node with a changed unit failed"; }
# One look, no retry: setup has already waited.
curl -fsS --max-time 2 http://127.0.0.1:6768/identity >/evidence/identity-changed-unit.json ||
  fail "setup returned before the daemon answered on its changed address"
grep -qF 'owned by alice@fresh,' /evidence/setup-changed-unit.log || fail "setup did not report the durable Owner: $(tail -1 /evidence/setup-changed-unit.log)"
if grep -qF 'owned by owner@fresh' /evidence/setup-changed-unit.log; then fail "setup reported the --owner seed as the Owner"; fi
[ "$(identity_field /evidence/identity-changed-unit.json build_info)" = "$package_build" ] || fail "changed-unit node build_info"
[ "$(systemctl show agent-busd -p MainPID --value)" != "$before_pid" ] || fail "the changed unit was not applied by a restart"
if curl -fsS --max-time 2 http://127.0.0.1:6767/identity >/dev/null 2>&1; then fail "the old unit's address still answers"; fi
pass "setup over a running node applies a changed unit, waits for the release, and reports the durable Owner"
# Put the node back as the checks after this one expect it: owner@fresh owns
# it and the generated unit serves on 6767.
code=$(curl -sS --max-time 4 --unix-socket /run/agent-bus/bus.sock -H "X-Agent-Bus-Token: $(cat /root/alice.token)" \
  -H 'Content-Type: application/json' --data '{"name":"owner@fresh"}' -o /evidence/owner-transfer-back.json -w '%{http_code}' http://bus/owner)
[ "$code" = 200 ] || fail "daemon ownership transfer back got $code"
./agent-bus-setup --owner owner@fresh >/evidence/setup-unit-restored.log 2>&1 ||
  { cat /evidence/setup-unit-restored.log >&2; fail "setup restoring the default unit failed"; }
grep -qF 'owned by owner@fresh,' /evidence/setup-unit-restored.log || fail "setup did not report the transferred-back Owner"
curl -fsS --max-time 2 http://127.0.0.1:6767/identity >/dev/null || fail "the restored unit does not answer on 6767"


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
pass "real installed browser follows sign-in, cookie, 0.8 pages, web-restart, bus-restart and sign-out session semantics"

agent-bus-admin token alice@fresh >/root/alice.token
agent-bus-admin token bob@fresh >/root/bob.token
agent-bus-admin token carol@fresh >/root/carol.token
agent-bus-admin token dave@fresh >/root/dave.token
agent-bus-admin token eve@fresh >/root/eve.token
cat >/root/fresh-echo.sh <<'SH'
#!/bin/sh
printf 'fresh reply: %s\n' "$1"
SH
chmod 0755 /root/fresh-echo.sh
# An agent's name begins with #. The owner is a User: its reply arrives
# because the agent's allow list admits it, with nothing registered for the
# caller, and a User is never registered as an agent.
/usr/local/bin/agent-bus start '#fresh-echo@fresh' --algo=args --allow owner@fresh /root/fresh-echo.sh >/evidence/agent.log 2>&1 &
agent_pid=$!
trap 'kill "$agent_pid" 2>/dev/null || true; wait "$agent_pid" 2>/dev/null || true' EXIT
for _ in $(seq 1 100); do
  /usr/local/bin/agent-bus ls '#fresh-echo@fresh' >/dev/null 2>&1 && break
  sleep .1
done
if /usr/local/bin/agent-bus register owner@fresh --kind agent --allow '#fresh-echo@fresh' >/evidence/user-as-agent.log 2>&1; then
  fail "a User was registered as an agent"
fi
grep -q "an agent's name begins with #, and owner@fresh does not" /evidence/user-as-agent.log || fail "User-as-agent refusal did not give its reason"
reply=$(timeout 15 /usr/local/bin/agent-bus call '#fresh-echo@fresh' --wait 5s 'installation works' 2>&1) ||
  fail "installed agent call got no reply: $reply"
grep -q 'fresh reply: installation works' <<<"$reply" || fail "installed agent call did not return its unique reply"
pass "new user calls a real script agent using only installed programs; a User is refused as an agent"
python /fixture/browser-roles.py --base http://127.0.0.1:6780 \
  --owner-token /root/owner.token --administrator-token /root/alice.token \
  --resource-owner-token /root/bob.token --maintainer-token /root/carol.token \
  --ordinary-token /root/dave.token --stranger-token /root/eve.token --evidence /evidence
pass "real installed browser exercises the five-role service/queue/pubsub/user/group matrix, stranger and origin refusals and activity graph"

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

before=$(readlink /usr/local/lib/agent-bus/current)
./agent-bus-setup --owner owner@fresh >/evidence/reinstall.log 2>&1 ||
  { cat /evidence/reinstall.log >&2; fail "identical package reinstall failed"; }
after=$(readlink /usr/local/lib/agent-bus/current)
[ "$before" = "$after" ] || fail "identical reinstall selected another release"
[ "$(find /usr/local/lib/agent-bus/releases -mindepth 1 -maxdepth 1 -type d | wc -l)" -eq 1 ] || fail "identical reinstall duplicated the release"
pass "identical package reinstall is idempotent"


systemctl show agent-busd -p ActiveState -p User -p FragmentPath >/evidence/unit-state.txt
systemctl --version | head -1 >/evidence/host.txt
printf 'archive=%s\n' "$archive_name" >>/evidence/host.txt
