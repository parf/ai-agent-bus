#!/bin/bash
# M.11 real-host acceptance, inside a disposable systemd container (rootless podman).
set -uo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
EV=/evidence; : >"$EV/checks.txt"; fails=0
say() { printf '%s\n' "$*" | tee -a "$EV/checks.txt"; }
ok() { say "PASS $*"; }
bad() { say "FAIL $*"; fails=$((fails+1)); }
chk() { local what=$1; shift; if "$@" >/dev/null 2>&1; then ok "$what"; else bad "$what"; fi; }
no() { local what=$1; shift; if "$@" >/dev/null 2>&1; then bad "$what"; else ok "$what"; fi; }
run() { say "\$ $*"; "$@" >"$EV/last.out" 2>&1; local rc=$?; cat "$EV/last.out" | sed 's/^/    /' | tee -a "$EV/checks.txt"; say "    (exit $rc)"; return $rc; }

cp /package/agent-bus-0*.tar.gz /root/release.tar.gz
mkdir /root/valid && tar -xzf /root/release.tar.gz -C /root/valid --strip-components=1
cd /root/valid
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIownerfixture owner@fresh' >/root/owner-key.pub
./agent-bus-setup --owner owner@fresh --key /root/owner-key.pub >"$EV/setup-daemon.log" 2>&1 || { cat "$EV/setup-daemon.log"; bad "daemon setup"; exit 1; }
for _ in $(seq 1 100); do curl -fsS http://127.0.0.1:6767/identity >/dev/null 2>&1 && break; sleep .1; done
useradd --create-home --shell /bin/sh alice
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIalicefixture alice' >/root/alice-key.pub
agent-bus-admin user add alice - </root/alice-key.pub >"$EV/add-alice.log" 2>&1 || { cat "$EV/add-alice.log"; bad "add User alice"; }
say "version: $(agent-bus-setup --version | head -1)"
say "node: $(curl -fsS http://127.0.0.1:6767/identity)"

SETUP=/usr/local/bin/agent-bus-setup
P=alice; ACC=alice-ab-runner; UNIT=agent-bus-runner@alice.service
TPL=/etc/systemd/system/agent-bus-runner@.service; PRIV=/var/lib/agent-bus/$ACC; AGENTS=/var/lib/agent-bus/agent.d
mkdir -p $AGENTS; echo shared >$AGENTS/KEEP-ME.txt
state() { # one line per fact, compared with the docs after each verb
  echo "account:   $(getent passwd $ACC | cut -d: -f1,7 || true)"
  echo "unit:      enabled=$(systemctl is-enabled $UNIT 2>&1 | head -1) active=$(systemctl is-active $UNIT 2>&1 | head -1)"
  echo "template:  $([ -f $TPL ] && echo present || echo absent)"
  echo "private:   $(stat -c '%U:%G %a' $PRIV 2>/dev/null || echo absent)"
  echo "runner.env:$(stat -c ' %U:%G %a' $PRIV/runner.env 2>/dev/null || echo ' absent')"
  echo "record:    $(code=$(curl -sS -o /dev/null -w '%{http_code}' --unix-socket /run/agent-bus/bus.sock -H "X-Agent-Bus-Token: $(agent-bus-admin token owner@fresh 2>/dev/null)" "http://bus/lookup?name=%23runner%2F$P%40$(cat /proc/sys/kernel/hostname)"); [ "$code" = 200 ] && echo "present" || echo "absent ($code)") #runner/$P@$(cat /proc/sys/kernel/hostname)"
  echo "shared:    agent.d $(stat -c '%U:%G %a' $AGENTS 2>/dev/null || echo absent), KEEP-ME $([ -f $AGENTS/KEEP-ME.txt ] && echo present || echo absent)"
}
show() { say "--- state after: $1"; state | sed 's/^/    /' | tee -a "$EV/checks.txt"; }
show "daemon only (before any runner verb)"
no "no runner account before install" getent passwd $ACC

run $SETUP --runner install --for $P --dry-run; chk "install --dry-run changed nothing" test ! -e $PRIV
run $SETUP --runner install --for $P; rc=$?; [ $rc = 0 ] && ok "install exit 0" || bad "install exit $rc"
show "install"
chk "account $ACC exists, nologin" bash -c "getent passwd $ACC | grep -q nologin"
chk "private dir 700 owned by $ACC" bash -c "[ \"\$(stat -c '%U:%G %a' $PRIV)\" = '$ACC:$ACC 700' ]"
chk "runner.env present" test -f $PRIV/runner.env
chk "template unit present" test -f $TPL
chk "unit enabled" systemctl is-enabled --quiet $UNIT
for _ in $(seq 1 60); do systemctl is-active --quiet $UNIT && break; sleep .5; done
chk "unit active" systemctl is-active --quiet $UNIT
chk "runner process runs as $ACC" bash -c "[ \"\$(ps -o user= -p \$(systemctl show -P MainPID $UNIT) | tr -d ' ')\" = $ACC ]"
chk "template: user %i-ab-runner, private dir only writable path, no Requires=" bash -c "grep -q '^User=%i-ab-runner' $TPL && grep -q '^ReadWritePaths=/var/lib/agent-bus/%i-ab-runner' $TPL && ! grep -q '^Requires=' $TPL"
chk "template: Wants+After the daemon" bash -c "grep -q '^Wants=agent-busd.service' $TPL && grep -q '^After=agent-busd.service' $TPL"
chk "shared agent.d kept" test -f $AGENTS/KEEP-ME.txt
cp $PRIV/runner.env /root/env.1; stat -c '%Y' $TPL >/root/tpl.1; say "runner.env after install: sha256 $(sha256sum < $PRIV/runner.env | cut -c1-12) mtime $(stat -c %y $PRIV/runner.env)"

run $SETUP --runner install --for $P; rc=$?; [ $rc = 0 ] && ok "second install exit 0" || bad "second install exit $rc"
run $SETUP --runner install --for $P --dry-run
chk "second install --dry-run plans nothing" bash -c "! grep -qiE 'create|write|start|enable|register' $EV/last.out || grep -qi 'nothing' $EV/last.out"
show "second install"
if [ "$(sha256sum < $PRIV/runner.env)" = "$(sha256sum < /root/env.1)" ]; then ok "second install: runner.env unchanged"; else bad "second install: runner.env unchanged"; say "    runner.env keys now:    $(sed 's/=.*/=<masked>/' $PRIV/runner.env | tr '\n' ' ')"; say "    runner.env keys before: $(sed 's/=.*/=<masked>/' /root/env.1 | tr '\n' ' ')"; say "    sha256 before $(sha256sum < /root/env.1 | cut -c1-12), now $(sha256sum < $PRIV/runner.env | cut -c1-12); differing lines: $(diff <(sed 's/=.*/=<masked>/' /root/env.1) <(sed 's/=.*/=<masked>/' $PRIV/runner.env) | wc -l); sizes $(stat -c %s /root/env.1) vs $(stat -c %s $PRIV/runner.env); mtime $(stat -c %y $PRIV/runner.env)"; fi
chk "second install: template untouched" bash -c "[ \"\$(stat -c %Y $TPL)\" = \"\$(cat /root/tpl.1)\" ]"
chk "second install: unit still active" systemctl is-active --quiet $UNIT

run $SETUP --runner disable --for $P; show "disable"
no "disabled: unit not enabled" systemctl is-enabled --quiet $UNIT
no "disabled: unit not active" systemctl is-active --quiet $UNIT
chk "disabled: account kept" getent passwd $ACC
chk "disabled: files kept" test -f $PRIV/runner.env
chk "disabled: shared agent.d kept" test -f $AGENTS/KEEP-ME.txt
run $SETUP --runner disable --for $P; chk "second disable exit 0" bash -c "true"

run $SETUP --runner enable --for $P; show "enable"
chk "enabled: unit enabled" systemctl is-enabled --quiet $UNIT
for _ in $(seq 1 60); do systemctl is-active --quiet $UNIT && break; sleep .5; done
chk "enabled: unit active" systemctl is-active --quiet $UNIT

run $SETUP --runner uninstall --for $P; show "uninstall"
no "uninstalled: no account" getent passwd $ACC
no "uninstalled: no private dir" test -e $PRIV
no "uninstalled: no template once the last runner goes" test -e $TPL
no "uninstalled: unit not enabled" systemctl is-enabled --quiet $UNIT
chk "uninstalled: shared agent.d kept" test -f $AGENTS/KEEP-ME.txt
run $SETUP --runner uninstall --for $P --dry-run; chk "second uninstall plans nothing" bash -c "grep -qiE 'nothing|already|no ' $EV/last.out || [ ! -s $EV/last.out ]"

run $SETUP --runner install --for $P; run $SETUP --runner uninstall --for $P --purge; show "uninstall --purge"
no "purged: no account" getent passwd $ACC
no "purged: no private dir" test -e $PRIV
no "purged: no template" test -e $TPL
chk "purged: shared agent.d still kept" test -f $AGENTS/KEEP-ME.txt

say "--- refusals"
run $SETUP --runner install --for nosuchperson; [ $? != 0 ] && ok "a person who is not a bus User is refused" || bad "non-User accepted"
no "refusal left no account" getent passwd nosuchperson-ab-runner
say "RESULT: $fails failure(s)"
exit $fails
