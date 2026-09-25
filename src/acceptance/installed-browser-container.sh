#!/bin/bash
# Runs only inside installed-browser.sh's disposable real-systemd container.
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/usr/sbin
fail() { echo "FAIL $*" >&2; exit 1; }
pass() { echo "PASS $*"; }

cd /root
cp /package/agent-bus-*.tar.gz release.tar.gz
printf '%s  release.tar.gz\n' "$(awk '{print $1}' /package/agent-bus-*.tar.gz.sha256)" >release.tar.gz.sha256
sha256sum -c release.tar.gz.sha256 >/dev/null || fail "archive checksum"
mkdir valid && tar -xzf release.tar.gz -C valid --strip-components=1
cd valid
./agent-bus-setup --owner owner@fresh >/evidence/setup.log 2>&1 || { cat /evidence/setup.log >&2; fail "setup"; }
for _ in $(seq 1 150); do curl -fsS http://127.0.0.1:6780/healthz >/dev/null 2>&1 && break; sleep .1; done
systemctl is-active --quiet agent-bus-web || fail "the agent-bus-web unit is not active"
./agent-bus-setup --samples >/evidence/samples.log 2>&1 || { cat /evidence/samples.log >&2; fail "setup --samples"; }
agent-bus-admin token owner@fresh >/root/owner.token
pass "the release is installed with its web unit and the sample data"
python /fixture/browser.py --base http://127.0.0.1:6780 --token /root/owner.token \
  --version "$(cat internal/version/VERSION)" --evidence /evidence
