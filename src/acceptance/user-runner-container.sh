#!/bin/bash
# Runs only inside user-runner.sh's disposable real-systemd container (RT31).
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
fail() { echo "FAIL $*" >&2; exit 1; }
pass() { echo "PASS $*"; }
host=$(cat /proc/sys/kernel/hostname)

mkdir /root/valid
tar -xzf /package/agent-bus-*.tar.gz -C /root/valid --strip-components=1
cd /root/valid
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIownerfixture owner@fresh' >/root/owner-key.pub
./agent-bus-setup --owner owner@fresh --key /root/owner-key.pub >/evidence/setup.log 2>&1 || { cat /evidence/setup.log; fail "setup"; }
for _ in $(seq 1 100); do curl -fsS http://127.0.0.1:6767/identity >/dev/null 2>&1 && break; sleep .1; done
useradd --create-home --shell /bin/bash alice
uid=$(id -u alice)
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIalicefixture alice' >/root/alice-key.pub
agent-bus-admin user add alice - </root/alice-key.pub >/evidence/add-alice.log
agent-bus-admin account set alice alice >/evidence/map-alice.log
systemctl restart agent-busd
for _ in $(seq 1 100); do runuser -u alice -- agent-bus status >/dev/null 2>&1 && break; sleep .1; done
as_alice() { runuser -u alice -- "$@"; }
uctl() { systemctl --user -M alice@ "$@"; }
pass "node installed; alice is a bus User and a login account"

for bad in root nobody ghost; do
  if ./agent-bus-setup --runner install --user-runner --for "$bad" >/evidence/refuse-$bad.out 2>&1; then fail "--for $bad accepted"; fi
done
grep -q 'uid 0' /evidence/refuse-root.out || fail "root refusal reason: $(cat /evidence/refuse-root.out)"
pass "root, a system account and an unknown name are refused"

./agent-bus-setup --runner install --user-runner --for alice --dry-run >/evidence/dry.out 2>&1 || { cat /evidence/dry.out; fail "dry run"; }
grep -q '^would ' /evidence/dry.out || fail "dry run printed no plan"
[ ! -e /home/alice/.local/state/agent-bus ] && [ ! -e /etc/systemd/user/agent-bus-runner.service ] || fail "dry run changed something"
pass "dry run plans and changes nothing"

# A planted link where the state directory goes: never followed, install refused.
as_alice mkdir -p /home/alice/.local/state/agent-bus
as_alice ln -s /root /home/alice/.local/state/agent-bus/runner
before=$(ls -la /root | sha256sum)
if ./agent-bus-setup --runner install --user-runner --for alice >/evidence/planted.out 2>&1; then fail "install through a planted link"; fi
grep -q 'not a directory' /evidence/planted.out || fail "planted link reason: $(cat /evidence/planted.out)"
[ "$(ls -la /root | sha256sum)" = "$before" ] || fail "/root changed through the planted link"
as_alice rm /home/alice/.local/state/agent-bus/runner
pass "a planted link in the person's home is neither followed nor written through"

./agent-bus-setup --runner install --user-runner --for alice >/evidence/install.out 2>&1 || { cat /evidence/install.out; fail "install"; }
for _ in $(seq 1 50); do uctl is-active --quiet agent-bus-runner.service && break; sleep .2; done
uctl is-active --quiet agent-bus-runner.service || { uctl status --no-pager agent-bus-runner.service >&2 || true; journalctl --no-pager -n 40 _UID=$uid >&2 || true; fail "runner unit not active"; }
runner=$(uctl show -p MainPID --value agent-bus-runner.service)
[ "$(awk '/^Uid:/{print $2}' /proc/$runner/status)" = "$uid" ] || fail "runner does not run as alice"
grep -q "user@$uid.service/.*agent-bus-runner.service" /proc/$runner/cgroup || fail "runner is not in alice's user manager: $(cat /proc/$runner/cgroup)"
[ "$(stat -c '%U %a' /home/alice/.local/state/agent-bus/runner/runner.env)" = "alice 600" ] || fail "runner.env owner or mode"
as_alice agent-bus help "#runner/alice-user@$host" >/evidence/runner-help.out 2>&1 || fail "the runner does not answer"
pass "#runner/alice-user runs as alice in her user manager and answers"

./agent-bus-setup --runner install --user-runner --for alice >/evidence/install2.out 2>&1 || fail "second install"
grep -q 'nothing to change' /evidence/install2.out || fail "second install changed something: $(cat /evidence/install2.out)"
pass "a second install plans nothing"

pkg=/home/alice/.local/share/agent-bus/agent.d/tmux-sessions
as_alice /usr/local/lib/agent-bus/current/agents/standalone/tmux-sessions/package.sh "$pkg" || fail "package tmux-sessions as alice"
as_alice sh -c 'umask 077; printf "%s\n" "{\"tmux-sessions\":{\"enabled\":true,\"sandbox\":\"off\"}}" > /home/alice/.local/state/agent-bus/runner/agents.json'
as_alice agent-bus call "#runner/alice-user@$host" --wait 20s reload >/evidence/reload1.out 2>&1 || { cat /evidence/reload1.out; fail "reload"; }
tmux_name=$(as_alice agent-bus ls 2>/dev/null | grep -o '#tmux-sessions@[^"]*' | head -1)
[ -n "$tmux_name" ] || fail "no tmux-sessions record after reload: $(cat /evidence/reload1.out)"
printf '%s' '{"session":"work","windows":[{"name":"w1","cwd":"/home/alice","command":"sleep 100000"}]}' | as_alice agent-bus config "$tmux_name" - >/evidence/config.out 2>&1 || fail "configure"
as_alice agent-bus call "#runner/alice-user@$host" --wait 20s reload >/evidence/reload2.out 2>&1 || fail "reload after config"
as_alice agent-bus call "$tmux_name" --wait 20s start >/evidence/start.out 2>&1 || { cat /evidence/start.out; fail "start"; }
if grep -q '"body":"error' /evidence/start.out; then fail "start answered an error: $(cat /evidence/start.out)"; fi
for _ in $(seq 1 50); do pgrep -u alice -x sleep >/dev/null && break; sleep .1; done
server=$(pgrep -u alice -f 'tmux new-session -d -s work' | head -1 || true); pane=$(pgrep -u alice -x sleep || true)
[ -n "$server" ] && [ -n "$pane" ] || { ps -u alice -o pid,ppid,cgroup:80,args >&2; uctl list-units --no-pager --all 'ab-session*' >&2 || true; journalctl --no-pager -n 30 _UID=$uid | grep -v sd-pam >&2 || true; fail "no tmux server or window process"; }
grep -q 'ab-session-work.scope' /proc/$server/cgroup || fail "tmux server is not in its own scope: $(cat /proc/$server/cgroup)"
if grep -q agent-bus-runner.service /proc/$server/cgroup; then fail "tmux server is inside the runner unit"; fi
as_alice agent-bus help "$tmux_name" >/evidence/tmux-help.out 2>&1 || fail "tmux help through the runner"
pass "tmux-sessions answers through the runner and starts a session in its own scope"

same() {
  [ -d /proc/$server ] && [ -d /proc/$pane ] || fail "$1: a tmux process is gone"
  [ "$(pgrep -u alice -f 'tmux new-session -d -s work' | head -1)" = "$server" ] && [ "$(pgrep -u alice -x sleep)" = "$pane" ] || fail "$1: tmux pids changed"
  uctl is-active --quiet ab-session-work.scope || fail "$1: the session scope is not active"
}
as_alice agent-bus call "#runner/alice-user@$host" --wait 20s reload >/dev/null 2>&1 || fail "reload"
same "runner reload"
uctl stop agent-bus-runner.service; uctl start agent-bus-runner.service
same "runner stop and start"
kill -9 "$(uctl show -p MainPID --value agent-bus-runner.service)"
for _ in $(seq 1 100); do uctl is-active --quiet agent-bus-runner.service && break; sleep .1; done
uctl is-active --quiet agent-bus-runner.service || fail "runner did not come back after kill -9"
same "runner kill -9"
uctl daemon-reload
same "user manager daemon-reload"
pass "tmux server and window pids survive reload, stop/start, kill -9 and daemon-reload"

printf '# stale\n' >>/etc/systemd/user/agent-bus-runner.service
before_runner=$(uctl show -p MainPID --value agent-bus-runner.service)
/usr/local/bin/agent-bus-setup --runner refresh --user-runner >/evidence/refresh.out 2>&1 || { cat /evidence/refresh.out; fail "refresh"; }
grep -q "alice's user-account runner: refreshed" /evidence/refresh.out || fail "refresh did not report alice: $(cat /evidence/refresh.out)"
if grep -q '# stale' /etc/systemd/user/agent-bus-runner.service; then fail "refresh kept a stale template"; fi
for _ in $(seq 1 100); do uctl is-active --quiet agent-bus-runner.service && break; sleep .1; done
[ "$(uctl show -p MainPID --value agent-bus-runner.service)" != "$before_runner" ] || fail "refresh did not restart the runner"
same "refresh"
pass "refresh rewrites a stale template and restarts the runner, tmux untouched"

./agent-bus-setup --runner uninstall --user-runner --for alice >/evidence/uninstall.out 2>&1 || { cat /evidence/uninstall.out; fail "uninstall"; }
if uctl is-active --quiet agent-bus-runner.service; then fail "runner still active after uninstall"; fi
[ ! -e /home/alice/.local/state/agent-bus/runner ] || fail "state left after uninstall"
[ ! -e /etc/systemd/user/agent-bus-runner.service ] || fail "template left after the last uninstall"
agent-bus-admin token owner@fresh >/root/owner.token
code=$(curl -sS --unix-socket /run/agent-bus/bus.sock -H "X-Agent-Bus-Token: $(cat /root/owner.token)" -o /dev/null -w '%{http_code}' "http://bus/lookup?name=%23runner%2Falice-user%40$host")
[ "$code" = 404 ] || fail "record left after uninstall ($code)"
same "uninstall"
pass "uninstall removes unit, state, template and record, and leaves tmux running"
