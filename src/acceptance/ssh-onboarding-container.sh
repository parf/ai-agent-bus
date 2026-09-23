#!/bin/bash
# Disposable rootless-container acceptance; invoked by ssh-onboarding.sh only.
set -euo pipefail
export PATH=/opt/bin:/usr/bin:/usr/sbin
fail() { echo "FAIL $*" >&2; exit 1; }
shell_is() {
 local account=$1 expected=$2 label=$3 actual
 actual=$(getent passwd "$account" | cut -d: -f7)
 [ "$actual" = "$expected" ] || fail "$label: shell=$actual, expected=$expected"
 echo "PASS $label"
}
mkdir -p /opt/bin /usr/libexec/openssh /usr/lib/ssh /usr/share/empty.sshd /var/empty /run/sshd /run/agent-bus /evidence
cp /fixture/bin/agent-bus* /opt/bin/
cp /fixture/ssh-tools/{sshd,ssh,ssh-keygen} /usr/bin/
for helper in /fixture/ssh-tools/sshd-{session,auth}; do
 [ -f "$helper" ] || continue
 cp "$helper" /usr/libexec/openssh/
 cp "$helper" /usr/lib/ssh/
done
for lib in /fixture/ssh-libs/*; do [ -e /usr/lib/"${lib##*/}" ] || cp "$lib" /usr/lib/; done
# Setup's account/files/unit generation is real; this container has no PID-1
# systemd. Start the generated command ourselves after verifying its unit.
printf '#!/bin/sh\nexit 0\n' >/opt/bin/systemctl
chmod +x /opt/bin/systemctl
/opt/bin/agent-bus-setup --owner owner --exec /opt/bin/agent-busd >/evidence/setup.log
shell_is agent-busd /bin/sh "daemon shell runs forced commands"
shell_is agent-bus-runner /usr/sbin/nologin "runner remains nologin"
# Exercise upgrade repair, not just fresh account creation.
usermod --shell /usr/sbin/nologin agent-busd
/opt/bin/agent-bus-setup --owner owner --exec /opt/bin/agent-busd >>/evidence/setup.log
shell_is agent-busd /bin/sh "upgrade repairs old nologin shell"
# A deliberately configured shell is not the old default and must survive.
usermod --shell /bin/bash agent-busd
/opt/bin/agent-bus-setup --owner owner --exec /opt/bin/agent-busd >>/evidence/setup.log
shell_is agent-busd /bin/bash "upgrade preserves a custom shell"
usermod --shell /bin/sh agent-busd
chown agent-busd:agent-busd /run/agent-bus
read -r -a daemon_command <<<"$(sed -n 's/^ExecStart=//p' /etc/systemd/system/agent-busd.service | sed 's/ -web//')"
start_daemon() {
 setpriv --reuid agent-busd --regid agent-busd --init-groups --inh-caps +chown --ambient-caps +chown "${daemon_command[@]}" >>/evidence/bus.log 2>&1 &
 bus_pid=$!
 for i in {1..200}; do curl -fsS http://127.0.0.1:6767/identity >/dev/null 2>&1 && return 0; sleep .1; done
 fail "daemon did not become ready"
}
trap 'kill "$bus_pid" ${ssh_pid:-} 2>/dev/null || true; wait || true' EXIT
start_daemon
# The status a credential gets and who it is; never a token on a command line.
whois() {
 curl -sS -H "X-Agent-Bus-Token: $(cat "$1")" -o /evidence/whois.json -w '%{http_code}' http://127.0.0.1:6767/status
 sed -n 's/.*"you":"\([^"]*\)".*/ \1/p' /evidence/whois.json
}
for key in host ordinary operator other twin; do ssh-keygen -q -t ed25519 -N '' -f /evidence/"$key"; done
adm() { runuser -u agent-busd -- env AGENT_BUS_ADDR=/run/agent-bus/user-agent-busd.sock /opt/bin/agent-bus-admin "$@"; }
adm user add ordinary - </evidence/ordinary.pub
adm user add operator - --admin </evidence/operator.pub
adm user add other@ssh - </evidence/other.pub
# ordinary@ssh is a second principal beside ordinary, with its own key.
adm user add ordinary@ssh - </evidence/twin.pub
useradd --system --home-dir /usr/share/empty.sshd --shell /usr/sbin/nologin sshd
# Password authentication is disabled; PAM permits account/session setup only.
cat >/etc/pam.d/sshd <<'PAM'
auth required pam_deny.so
account required pam_permit.so
session required pam_permit.so
PAM
cat >/evidence/sshd_config <<'CONF'
ListenAddress 127.0.0.1
Port 2222
HostKey /evidence/host
PidFile /evidence/sshd.pid
AuthorizedKeysFile .ssh/authorized_keys
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM yes
AllowUsers agent-busd
LogLevel VERBOSE
CONF
sshd -t -f /evidence/sshd_config
/usr/bin/sshd -D -e -f /evidence/sshd_config >/evidence/sshd.log 2>&1 &
ssh_pid=$!
sleep .2
remote() { local key=$1; shift; ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/evidence/known_hosts -o IdentitiesOnly=yes -i /evidence/"$key" -p 2222 agent-busd@127.0.0.1 "$@"; }
remote ordinary token >/evidence/ordinary.token || fail "ordinary key token command"
remote operator token >/evidence/operator.token || fail "operator key token command"
for name in ordinary operator; do
 token=$(cat /evidence/"$name".token)
 curl -fsS -H "X-Agent-Bus-Token: $token" http://127.0.0.1:6767/status > /evidence/"$name".status
 grep -q '"you":"'"$name"'"' /evidence/"$name".status
 echo "PASS $name key receives a working token for its entitled identity"
done
remote operator user list | grep -q $'^ordinary\tagent-bus-token$' || fail 'operator user list lacks ordinary'
if remote ordinary 'token operator' >/evidence/wrong-name.out 2>&1; then echo 'FAIL ordinary key obtained another identity'; exit 1; fi
if remote operator 'token ordinary' >/evidence/wrong-operator.out 2>&1; then echo 'FAIL operator token delegated without its entitlement'; exit 1; fi
grep -q 'this key may ask for ordinary, not operator' /evidence/wrong-name.out
grep -q 'this key may ask for operator, not ordinary' /evidence/wrong-operator.out
echo 'PASS both key classes explicitly refuse another identity'
# A realm is part of the name: ordinary@ssh is another principal, not a spelling of ordinary.
if remote ordinary 'token ordinary@ssh' >/evidence/realm-spelling.out 2>&1; then fail 'ordinary key obtained ordinary@ssh'; fi
grep -q 'this key may ask for ordinary, not ordinary@ssh' /evidence/realm-spelling.out || fail "realm spelling refusal: $(cat /evidence/realm-spelling.out)"
remote twin token >/evidence/twin.token
[ "$(whois /evidence/twin.token)" = "200 ordinary@ssh" ] || fail "ordinary@ssh key did not get its own identity"
echo 'PASS a realm-less entitlement does not extend to the realm spelling'
for key in ordinary operator; do
 if remote "$key" 'touch /var/lib/agent-bus/daemon/escaped' >/evidence/"$key"-shell.out 2>&1; then echo 'FAIL shell accepted'; exit 1; fi
 test ! -e /var/lib/agent-bus/daemon/escaped
 echo "PASS $key command treated as data, no shell"
done
# Real SSH channel requests: a forced command alone does not prohibit these.
for key in ordinary operator; do
 remote "$key" -tt token >/evidence/"$key"-pty.out 2>/evidence/"$key"-pty.err || true
 grep -q 'PTY allocation request failed' /evidence/"$key"-pty.err || { echo 'FAIL key permitted PTY'; exit 1; }
 if printf 'GET /identity HTTP/1.0\r\n\r\n' | remote "$key" -W 127.0.0.1:6767 >/evidence/"$key"-forward.out 2>/evidence/"$key"-forward.err; then echo 'FAIL key permitted forwarding'; exit 1; fi
 grep -q 'administratively prohibited' /evidence/"$key"-forward.err
 echo "PASS $key key refuses PTY and TCP forwarding"
done
# New keys supplied through the operator grammar become entitled users too.
ssh-keygen -q -t ed25519 -N '' -f /evidence/newcomer
remote operator user add newcomer - </evidence/newcomer.pub
remote newcomer token >/evidence/newcomer.token
curl -fsS -H "X-Agent-Bus-Token: $(cat /evidence/newcomer.token)" http://127.0.0.1:6767/status | grep -q '"you":"newcomer"'
echo 'PASS operator adds a usable ordinary key over SSH'
for key in ordinary operator; do
 remote "$key" 'token --rotate' >/evidence/"$key"-rotated.token
 test "$(cat /evidence/"$key".token)" != "$(cat /evidence/"$key"-rotated.token)"
 [ "$(whois /evidence/"$key"-rotated.token)" = "200 $key" ] || fail "$key rotated credential"
 [ "$(whois /evidence/"$key".token)" = "200 $key" ] || fail "$key previous credential stopped at the first rotation"
 remote "$key" 'token --rotate' >/evidence/"$key"-rotated2.token
 [ "$(whois /evidence/"$key"-rotated2.token)" = "200 $key" ] || fail "$key second rotation"
 [ "$(whois /evidence/"$key"-rotated.token)" = "200 $key" ] || fail "$key previous credential stopped at the second rotation"
 [ "$(whois /evidence/"$key".token)" = 401 ] || fail "$key credential before the previous still authenticates"
 echo "PASS $key SSH rotation keeps current and previous; the one before stops"
done
# Credentials live in the database: a restarted daemon answers the same ones.
kill -TERM "$bus_pid"; wait "$bus_pid" || true
start_daemon
for key in ordinary operator; do
 [ "$(whois /evidence/"$key"-rotated2.token)" = "200 $key" ] || fail "$key current credential after restart"
 [ "$(whois /evidence/"$key"-rotated.token)" = "200 $key" ] || fail "$key previous credential after restart"
 remote "$key" token >/evidence/"$key"-after-restart.token
 test "$(cat /evidence/"$key"-after-restart.token)" = "$(cat /evidence/"$key"-rotated2.token)" || fail "$key asking again after restart issued another"
done
echo 'PASS SSH-issued credentials survive a daemon restart; asking again is a read'
remote other token >/evidence/other-before.token
remote operator user remove ordinary
if remote ordinary token >/evidence/removed-key.out 2>&1; then echo 'FAIL removed key authenticated'; exit 1; fi
remote other token >/evidence/other-after.token
test "$(cat /evidence/other-before.token)" = "$(cat /evidence/other-after.token)"
echo 'PASS key removal preserves unrelated key'
