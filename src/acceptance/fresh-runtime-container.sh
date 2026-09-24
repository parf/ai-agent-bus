#!/bin/bash
# Runs only inside fresh-runtime.sh's disposable real-systemd container.
# Usage: run.sh offline|claude
# offline: no network; Codex and OpenCode with a loopback model fixture, and
#          Claude's MCP minimum under a literal dummy key.
# claude:  network for the real model under a copied claude.ai login.
set -uo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
mode=${1:?offline or claude}
fail() { echo "FAIL $*" >&2; exit 1; }
pass() { echo "PASS $*"; }

# ---- the host is the release, the fixture and the runtimes, nothing else ----
[ ! -e /rd ] || fail "/rd exists on the fresh host"
checkout=$(find / -xdev \( -name smoke.sh -o -name go.mod -o -name '*.go' \) -print -quit 2>/dev/null)
[ -z "$checkout" ] || fail "checkout content on the fresh host: $checkout"
! command -v go >/dev/null || fail "a Go toolchain is on the fresh host"
pass "no /rd, no checkout file and no Go toolchain on the host"

cp /package/agent-bus-*.tar.gz /root/release.tar.gz
cd /root
printf '%s  release.tar.gz\n' "$(awk '{print $1}' /package/agent-bus-*.tar.gz.sha256)" >release.tar.gz.sha256
sha256sum -c release.tar.gz.sha256 >/dev/null || fail "archive checksum"
mkdir valid && tar -xzf release.tar.gz -C valid --strip-components=1 || fail "archive extraction"
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIownerfixture owner@fresh' >/root/owner-key.pub
(cd valid && ./agent-bus-setup --owner owner@fresh --key /root/owner-key.pub) >/evidence/setup.log 2>&1 || fail "package setup: $(tail -3 /evidence/setup.log)"
systemctl is-active --quiet agent-busd || fail "installed unit is inactive"
curl -fsS --max-time 2 http://127.0.0.1:6767/identity >/evidence/identity.json || fail "installed node does not answer"
pass "packaged agent-bus-setup installed a real systemd node ($(cat /root/valid/internal/version/VERSION))"

# The one mapped person who runs every gate. Their account socket is the only
# credential the installed-node gate is given.
useradd --create-home --shell /bin/bash alice
printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIalicefixture alice@fresh' | agent-bus-admin user add alice@fresh - >/evidence/add-alice.log 2>&1 || fail "user add alice"
agent-bus-admin account set alice alice@fresh >/evidence/map-alice.log 2>&1 || fail "account set alice"
systemctl restart agent-busd
for _ in $(seq 1 100); do [ -S /run/agent-bus/user-alice.sock ] && break; sleep .1; done
runuser -u alice -- agent-bus status >/evidence/alice.status 2>&1 || fail "alice cannot use her socket"
grep -q '"you":"alice@fresh"' /evidence/alice.status || fail "alice's socket does not authenticate alice@fresh"
getent passwd agent-bus-runner >/dev/null || fail "setup did not create the second account agent-bus-runner"
printf 'alice ALL=(agent-bus-runner,nobody) NOPASSWD: ALL\n' >/etc/sudoers.d/runtime-gate
chmod 0440 /etc/sudoers.d/runtime-gate
pass "alice@fresh is mapped to the actual account alice; agent-bus-runner and nobody are the second accounts"

# ---- runtimes: copied executables, in the directories a person installs them to
install -m 0755 /runtimes/bun /usr/bin/bun
for r in codex opencode claude; do [ -f "/runtimes/$r" ] && install -m 0755 "/runtimes/$r" "/usr/local/bin/$r"; done
{
  echo "mode=$mode"
  echo "kernel=$(uname -r)"
  echo "os=$(. /etc/os-release; echo "$PRETTY_NAME")"
  echo "systemd=$(systemctl --version | head -1)"
  echo "glibc=$(ldd --version | head -1)"
  echo "hostname=$(cat /proc/sys/kernel/hostname)"
  echo "interfaces=$(ls /sys/class/net | tr '\n' ' ')"
  echo "agent-bus=$(cat /root/valid/internal/version/VERSION) $(agent-busd --version | sed -n 's/^build_info: //p')"
  echo "bun=$(bun --version)"
  for r in codex opencode claude; do command -v "$r" >/dev/null && echo "$r=$(runuser -u alice -- env HOME=/tmp "$r" --version 2>&1 | tail -1)"; done
} >/evidence/host-$mode.txt
cat /evidence/host-$mode.txt
if [ "$mode" = offline ]; then
  [ "$(ls /sys/class/net)" = lo ] || fail "the offline host has an interface besides lo"
  ! curl -sS --max-time 3 https://api.anthropic.com >/dev/null 2>&1 || fail "the offline host reaches the network"
  pass "the offline host has only loopback"
fi

# ---- the gate programs: bundled harnesses, readable by the second account -----
mkdir -p /opt/gate/internal/version
cp -r /fixture/acceptance /opt/gate/
ln -s runtime-isolation.js /opt/gate/acceptance/runtime-isolation.ts
cp /usr/local/lib/agent-bus/current/internal/version/VERSION /opt/gate/internal/version/VERSION
chmod -R a+rX /opt/gate
mkdir -m 0755 /srv/ev && chown alice: /srv/ev
if [ "$mode" = claude ]; then
  install -d -m 0700 -o alice /home/alice/login
  install -m 0600 -o alice /login/.credentials.json /login/.claude.json /home/alice/login/
fi

results=/evidence/results-$mode.txt
: >"$results"
only=${GATES:-} # a space-separated subset of gate names; empty runs them all
gate() { # gate NAME SECONDS HARNESS ARGS...
  local name=$1 seconds=$2 harness=$3; shift 3
  [ -z "$only" ] || [[ " $only " == *" $name "* ]] || return 0
  local started=$SECONDS code
  echo "== $name: bun $harness /usr/local/bin /srv/ev/$name $*" | tee -a /evidence/commands.txt
  timeout --kill-after=20 "$seconds" runuser -u alice -- env -i PATH=/usr/local/bin:/usr/bin HOME=/home/alice USER=alice LOGNAME=alice LANG=C.UTF-8 \
    bun "/opt/gate/acceptance/$harness.js" /usr/local/bin "/srv/ev/$name" "$@" >"/evidence/$name.log" 2>&1
  code=$?
  local summary; summary=$(grep -E '^checks' "/evidence/$name.log" | tail -1)
  printf '%s exit=%s seconds=%s %s\n' "$name" "$code" $((SECONDS - started)) "$summary" | tee -a "$results"
  # Evidence without any runtime profile, login copy or session state.
  mkdir -p "/evidence/$name"
  (cd "/srv/ev/$name" 2>/dev/null && find . \( -path ./home -o -path ./config -o -path '*/home' -o -path '*/claude' \) -prune -o -type f \( -name '*.log' -o -name '*.txt' -o -name '*.json' -o -name '*.jsonl' \) -print0 |
    xargs -0 -r cp --parents -t "/evidence/$name")
}
if [ "$mode" = offline ]; then
  for r in codex opencode; do
    gate "installed-$r" 300 runtime-installed "$r"
    gate "interactive-$r" 600 runtime-interactive "$r"
    gate "launch-$r" 1200 runtime-launch "$r" launch,failure
    gate "names-$r" 900 runtime-launch "$r" names
    gate "recovery-$r" 900 runtime-recovery "$r"
  done
  gate interactive-claude 600 runtime-interactive claude
else
  gate installed-claude 600 runtime-installed claude /home/alice/login
  gate channel-claude 600 claude-channel-live /home/alice/login
  gate launch-claude 1800 runtime-launch claude /home/alice/login launch,failure
  gate names-claude 1200 runtime-launch claude /home/alice/login names
  gate recovery-claude 1200 runtime-recovery claude /home/alice/login
  rm -rf /home/alice/login /srv/ev/*/config
fi
# The node the gates ran beside is still the installed one, unharmed.
systemctl is-active --quiet agent-busd || fail "the installed node stopped during the gates"
runuser -u alice -- agent-bus status >/dev/null 2>&1 || fail "alice lost her socket during the gates"
cat "$results"
! grep -v ' exit=0 ' "$results" | grep -q . || fail "a runtime gate failed"
pass "every $mode runtime gate passed against the installed programs"
