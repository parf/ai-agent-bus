#!/bin/bash
# Runs only inside run.sh's disposable systemd container: the person alice
# installs the release the documented way, curl into sh with sudo, after two
# forged releases are refused without touching the host.
set -euo pipefail
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
fail() {
  echo "FAIL $*" >&2
  journalctl -u agent-busd -u 'agent-bus-runner@*' -n 15 --no-pager 2>/dev/null | sed 's/^/  journal: /' >&2 || true
  exit 1
}
pass() { echo "PASS $*"; }

for tool in go bun node npm; do
  ! command -v "$tool" >/dev/null 2>&1 || fail "the clean host has $tool"
done
pristine() {
  [ ! -e /usr/local/lib/agent-bus ] && [ ! -e /var/lib/agent-bus ] &&
    ! getent passwd agent-busd >/dev/null &&
    [ ! -e /etc/systemd/system/agent-busd.service ] &&
    [ -z "$(ls -A /usr/local/bin)" ]
}
pristine || fail "the host is not clean before the install"
pass "clean $(. /etc/os-release && echo "$PRETTY_NAME"): no Go, bun or Node, nothing installed"

# releases.test is this container, trusted the way a public host would be.
echo '127.0.0.1 releases.test' >>/etc/hosts
if command -v update-ca-trust >/dev/null; then
  cp /fixture/tls.crt /etc/pki/ca-trust/source/anchors/releases-test.crt
  update-ca-trust
else
  cp /fixture/tls.crt /usr/local/share/ca-certificates/releases-test.crt
  update-ca-certificates >/dev/null 2>&1
fi
python3 /fixture/https.py /release /fixture/tls.crt /fixture/tls.key 2>/evidence/https.log &
for _ in $(seq 1 50); do
  curl -fsS -o /dev/null https://releases.test/good/install.sh 2>/dev/null && break
  sleep .1
done
curl -fsS -o /dev/null https://releases.test/good/install.sh || fail "the release server did not answer"

useradd -m -s /bin/bash alice
echo 'alice ALL=(ALL) NOPASSWD: ALL' >/etc/sudoers.d/alice
chmod 440 /etc/sudoers.d/alice
as_alice() {
  runuser -u alice -- env -i HOME=/home/alice USER=alice LOGNAME=alice SHELL=/bin/bash \
    PATH=/usr/local/bin:/usr/bin:/bin XDG_RUNTIME_DIR="/run/user/$(id -u alice)" "$@"
}
as_alice ssh-keygen -q -t ed25519 -N '' -f /home/alice/.ssh/id_ed25519
install_from() { # variant: the one documented command, piped into sh
  as_alice sh -c "curl -fsSL https://releases.test/$1/install.sh | sh"
}

refused() { # variant, words install.sh must say
  if install_from "$1" >"/evidence/$1.log" 2>&1; then
    fail "the $1 release installed"
  fi
  grep -qF "$2" "/evidence/$1.log" || { cat "/evidence/$1.log" >&2; fail "the $1 release was not refused for: $2"; }
  pristine || fail "the refused $1 release changed the host"
  pass "$1 release refused before any file was written: $2"
}
# The archive changed, its signed hash did not. Without bubblewrap install.sh
# warns, and installs nothing here either way.
bwrap=$(command -v bwrap) || fail "the image has no bubblewrap"
mv "$bwrap" /root/bwrap.aside
refused tampered "does not match its signed hash"
mv /root/bwrap.aside "$bwrap"
grep -q 'bubblewrap (bwrap) is not installed' /evidence/tampered.log || fail "install.sh did not warn that bubblewrap is missing"
pass "install.sh warns when bubblewrap is missing"
# The archive changed and so did SHA256SUMS: only the signature is left to say no.
refused forged "SHA256SUMS is not signed by a release key"

install_from good >/evidence/install.log 2>&1 || { tail -40 /evidence/install.log >&2; fail "install.sh did not install the good release"; }
! grep -q 'bubblewrap' /evidence/install.log || fail "install.sh warned about bubblewrap on a host that has it"
version=$(cat /fixture/VERSION)
[ "$(readlink /usr/local/lib/agent-bus/current)" != "" ] || fail "no current release link"
/usr/local/bin/agent-busd --version | grep -qx "$version" || fail "agent-busd is not $version"
systemctl is-active --quiet agent-busd || fail "agent-busd is not running"
for _ in $(seq 1 100); do curl -fsS -o /dev/null http://127.0.0.1:6780/healthz 2>/dev/null && break; sleep .1; done
curl -fsS http://127.0.0.1:6780/healthz >/evidence/web-healthz.out || fail "the compiled web face does not answer"
pass "curl | sh installed $version with sudo: daemon and compiled web face running"

you=$(as_alice agent-bus status | python3 -c 'import json,sys; print(json.load(sys.stdin)["you"])') || fail "alice cannot ask the bus who she is"
[ "$you" = alice ] || fail "alice is $you on the bus"
# The node's realm is its host name, as check-deploy.sh falls back to.
realm=$(hostname -f)
# By its path, as the docs give it: RHEL's sudo secure_path leaves
# /usr/local/bin out, so a bare "sudo agent-bus-setup" is not found on AlmaLinux.
as_alice sudo /usr/local/bin/agent-bus-setup --runner install >/evidence/runner.log 2>&1 || { tail -20 /evidence/runner.log >&2; fail "the runner did not install"; }
cp -a /usr/local/lib/agent-bus/current/agents/debug /var/lib/agent-bus/agent.d/debug
# debug serves itself: its Owner registers it and hands its token to the
# runner once (docs/runner/configuration.md#agents-that-serve-themselves).
as_alice agent-bus register "#debug@$realm" --allow=@owner >/evidence/register.log 2>&1 || { cat /evidence/register.log >&2; fail "alice could not register #debug"; }
tokens=/var/lib/agent-bus/alice-ab-runner/tokens
install -d -o alice-ab-runner -m 700 "$tokens"
as_alice agent-bus token "#debug@$realm" | install -o alice-ab-runner -m 600 /dev/stdin "$tokens/%23debug@$realm"
as_alice agent-bus call "#runner/alice@$realm" reload >/evidence/reload.log 2>&1 || { cat /evidence/reload.log >&2; fail "the runner did not reload"; }
for _ in $(seq 1 60); do
  as_alice agent-bus call "#debug@$realm" --wait 2s ping >/dev/null 2>&1 && break
  sleep .5
done
if ! as_alice sh /fixture/check-deploy.sh >/evidence/check-deploy.log 2>&1; then
  cat /evidence/check-deploy.log >&2
  fail "check-deploy.sh failed"
fi
grep -E '^Checks: [1-9]' /evidence/check-deploy.log >/dev/null || fail "check-deploy.sh ran no selftest"
pass "check-deploy.sh: $(grep -E '^Checks: ' /evidence/check-deploy.log) passed against #debug@$realm"

# The client edition, from another account and without root, joins the node.
useradd -m -s /bin/bash bob
as_bob() {
  runuser -u bob -- env -i HOME=/home/bob USER=bob LOGNAME=bob PATH=/home/bob/.local/bin:/usr/bin:/bin \
    AGENT_BUS_ADDR=http://127.0.0.1:6767 AGENT_BUS_TOKEN="$(cat /home/bob/token 2>/dev/null)" "$@"
}
as_bob ssh-keygen -q -t ed25519 -N '' -f /home/bob/.ssh/id_ed25519
as_alice sudo /usr/local/bin/agent-bus-admin user add bob - </home/bob/.ssh/id_ed25519.pub >/evidence/bob-enrol.log 2>&1 || { cat /evidence/bob-enrol.log >&2; fail "bob could not be enrolled"; }
as_alice sudo /usr/local/bin/agent-bus-admin token bob >/home/bob/token
chown bob /home/bob/token && chmod 600 /home/bob/token
bin_before=$(ls -A /usr/local/bin)
as_bob sh -c "curl -fsSL https://releases.test/good/install.sh | sh -s -- --client" >/evidence/client.log 2>&1 || { cat /evidence/client.log >&2; fail "the client edition did not install"; }
[ "$(ls -A /usr/local/bin)" = "$bin_before" ] || fail "the client edition changed /usr/local/bin"
[ "$(readlink /home/bob/.local/bin/agent-bus)" = /home/bob/.local/lib/agent-bus/current/agent-bus ] || fail "the client's agent-bus is not its own"
[ "$(as_bob agent-bus status | python3 -c 'import json,sys; print(json.load(sys.stdin)["you"])')" = bob ] || fail "the client edition is not bob on the bus"
# A request and its reply, from the client, to an agent the runner serves:
# refused before alice allows bob, answered after.
if as_bob agent-bus call "#debug@$realm" --wait 10s ping >/evidence/bob-refused.log 2>&1; then fail "debug answered bob before alice allowed him"; fi
grep -q 'not on that record' /evidence/bob-refused.log || { cat /evidence/bob-refused.log >&2; fail "bob was refused for another reason"; }
as_alice agent-bus manage "#debug@$realm" --add-to-set-allow=bob >/dev/null
as_bob agent-bus call "#debug@$realm" --wait 10s ping >/evidence/bob-ping.log 2>&1 || { cat /evidence/bob-ping.log >&2; fail "debug did not answer bob"; }
grep -q answered_at /evidence/bob-ping.log || { cat /evidence/bob-ping.log >&2; fail "debug's answer to bob is not its ping"; }
pass "client edition installed by bob without root; it calls #debug@$realm and gets the reply"

# An MCP tool call through the client's compiled face.
as_bob python3 /fixture/mcp-call.py ab_ls '{}' /home/bob/.local/bin/agent-bus-mcp >/evidence/mcp-ls.out 2>&1 || { cat /evidence/mcp-ls.out >&2; fail "the MCP tool call failed"; }
grep -q "^#debug@$realm " /evidence/mcp-ls.out || { cat /evidence/mcp-ls.out >&2; fail "the MCP listing does not show #debug@$realm"; }
pass "MCP tool call ab_ls through the compiled agent-bus-mcp lists #debug@$realm"

# The same command again with a newer release out: the node updates, keeps
# its data and the release it replaced; --version takes it back.
next=$(cat /fixture/NEXT)
deploy_checked() { # label
  as_alice sh /fixture/check-deploy.sh >"/evidence/check-deploy-$1.log" 2>&1 || { cat "/evidence/check-deploy-$1.log" >&2; fail "check-deploy.sh failed $1"; }
  [ "$(as_bob agent-bus status | python3 -c 'import json,sys; print(json.load(sys.stdin)["you"])')" = bob ] || fail "bob's credential stopped working $1"
}
release_of() { cat "/usr/local/lib/agent-bus/$(readlink /usr/local/lib/agent-bus/current)/internal/version/VERSION"; }
as_alice sh -c "curl -fsSL https://releases.test/next/install.sh | sh" >/evidence/update.log 2>&1 || { tail -30 /evidence/update.log >&2; fail "install.sh did not update to $next"; }
/usr/local/bin/agent-busd --version | grep -qx "$next" || fail "agent-busd is not $next after the update"
[ "$(release_of)" = "$next" ] || fail "current is not $next after the update"
kept=$({ grep -lx "$version" /usr/local/lib/agent-bus/releases/*/internal/version/VERSION 2>/dev/null || true; } | wc -l)
[ "$kept" = 1 ] || fail "the update kept $kept copies of $version for rollback, not one"
deploy_checked "after the update"
pass "the same command updated the node to $next: registrations, tokens and $version for rollback kept"

as_bob sh -c "curl -fsSL https://releases.test/next/install.sh | sh -s -- --client" >/evidence/client-update.log 2>&1 || { cat /evidence/client-update.log >&2; fail "the client edition did not update"; }
as_bob agent-bus --version | grep -qx "$next" || fail "the client edition is not $next after its update"
as_bob sh -c "curl -fsSL https://releases.test/next/install.sh | sh -s -- --client --rollback" >/evidence/client-rollback.log 2>&1 || { cat /evidence/client-rollback.log >&2; fail "the client edition did not roll back"; }
as_bob agent-bus --version | grep -qx "$version" || fail "the client edition is not $version after its rollback"
pass "the client edition updated to $next and rolled back to $version"

as_alice sh -c "curl -fsSL https://releases.test/next/install.sh | sh -s -- --version $version" >/evidence/rollback.log 2>&1 || { tail -30 /evidence/rollback.log >&2; fail "install.sh --version $version did not take the node back"; }
/usr/local/bin/agent-busd --version | grep -qx "$version" || fail "agent-busd is not $version after the rollback"
[ "$(release_of)" = "$version" ] || fail "current is not $version after the rollback"
deploy_checked "after the rollback"
pass "install.sh --version $version took the node back, data kept"
