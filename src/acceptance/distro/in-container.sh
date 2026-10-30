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
# The archive changed, its signed hash did not.
refused tampered "does not match its signed hash"
# The archive changed and so did SHA256SUMS: only the signature is left to say no.
refused forged "SHA256SUMS is not signed by a release key"

install_from good >/evidence/install.log 2>&1 || { tail -40 /evidence/install.log >&2; fail "install.sh did not install the good release"; }
version=$(cat /fixture/VERSION)
[ "$(readlink /usr/local/lib/agent-bus/current)" != "" ] || fail "no current release link"
/usr/local/bin/agent-busd --version | grep -qx "$version" || fail "agent-busd is not $version"
systemctl is-active --quiet agent-busd || fail "agent-busd is not running"
for _ in $(seq 1 100); do curl -fsS -o /dev/null http://127.0.0.1:6780/healthz 2>/dev/null && break; sleep .1; done
curl -fsS http://127.0.0.1:6780/healthz >/evidence/web-healthz.out || fail "the compiled web face does not answer"
pass "curl | sh installed $version with sudo: daemon and compiled web face running"

you=$(as_alice agent-bus status | python3 -c 'import json,sys; print(json.load(sys.stdin)["you"])')
[ "$you" = alice ] || fail "alice is $you on the bus"
# The node's realm is its host name, as check-deploy.sh falls back to.
realm=$(hostname -f)
# By its path: RHEL's sudo secure_path leaves /usr/local/bin out, so the
# documented "sudo agent-bus-setup" is not found on AlmaLinux.
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
