#!/bin/bash
# git-install-agents.sh against a fake unit directory, a recording systemctl stub and a
# fake person's home. No host unit, home or service is touched.
set -eu
cd "$(dirname "$0")"
mkdir -p ../tmp
d=$(mktemp -d "$(cd .. && pwd)/tmp/git-install-agents.XXXXXX")
trap 'rm -rf "$d"' EXIT
mkdir -p "$d/bin" "$d/units" "$d/state"
# The stub keeps each unit's state in a file: try-restart restarts only an active unit,
# restart starts any, the way systemctl does.
cat >"$d/bin/systemctl" <<'FAKE'
#!/bin/bash
printf '%s\n' "$*" >>"$CALLS"
u=${!#}; s=$STATE/$u
case $1 in
  try-restart) [ "$u" != "$FAIL_UNIT" ] || exit 1; [ "$(cat "$s" 2>/dev/null)" != active ] || echo restarted >"$s.restarts" ;;
  restart) [ "$u" != "$FAIL_UNIT" ] || exit 1; echo active >"$s"; echo restarted >"$s.restarts" ;;
esac
FAKE
chmod +x "$d/bin/systemctl"
printf '# agent-bus standalone: #up@example.org\n[Unit]\n' >"$d/units/agent-bus-up.service"
printf '# agent-bus standalone: #down@example.org\n[Unit]\n' >"$d/units/agent-bus-down.service"
# Unmarked: the marker text later in the file does not make it ours.
printf '[Unit]\n# agent-bus standalone: #other@example.org\n' >"$d/units/other.service"
echo active >"$d/state/agent-bus-up.service"; echo inactive >"$d/state/agent-bus-down.service"; echo active >"$d/state/other.service"
# One person: a current pack package, a stale one, and an agent of their own.
home=$d/home/alice; pkgs=$home/.local/share/agent-bus/agent.d
mkdir -p "$pkgs/mine"
agents/standalone/tmux-sessions/package.sh "$pkgs/tmux-sessions" >/dev/null
agents/eventlog/package.sh "$pkgs/eventlog" >/dev/null
echo '# an older build' >>"$pkgs/eventlog/agent.json"
echo '{"description": "her own"}' >"$pkgs/mine/agent.json"
printf 'alice:x:%s:%s::%s:/bin/bash\nsvc:x:990:990::%s:/sbin/nologin\n' "$(id -u)" "$(id -g)" "$home" "$home" >"$d/passwd"
export AB_TEST=1 AB_UNIT_DIR=$d/units AB_SYSTEMCTL=$d/bin/systemctl AB_PASSWD=$d/passwd AB_AS_PERSON=direct CALLS=$d/calls STATE=$d/state FAIL_UNIT=""
: >"$CALLS"
rc=0; out=$(bash ./git-install-agents.sh 2>&1) || rc=$?
# Standalone units: the running marked one restarted, the stopped one still stopped, the unmarked one untouched.
[ -f "$d/state/agent-bus-up.service.restarts" ] || { echo "the running standalone unit was not restarted"; exit 1; }
[ "$(cat "$d/state/agent-bus-down.service")" = inactive ] && [ ! -f "$d/state/agent-bus-down.service.restarts" ] || { echo "the stopped standalone unit was started"; exit 1; }
[ ! -f "$d/state/other.service.restarts" ] || { echo "an unmarked unit was restarted"; exit 1; }
# And exactly the two marked ones, each once, by try-restart.
[ "$(grep -c . "$CALLS")" = 2 ] || { echo "want 2 systemctl calls, got: $(cat "$CALLS")"; exit 1; }
for u in up down; do
  [ "$(grep -cx "try-restart -- agent-bus-$u.service" "$CALLS")" = 1 ] || { echo "agent-bus-$u.service did not get exactly one try-restart: $(cat "$CALLS")"; exit 1; }
done
! grep -q other.service "$CALLS" || { echo "an unmarked unit was touched: $(cat "$CALLS")"; exit 1; }
# User-runner packages: exactly one warning, naming the stale pack package and its refresh.
warnings=$(grep -c '^warning: ' <<<"$out" || true)
[ "$warnings" = 1 ] || { echo "want exactly 1 warning, got $warnings: $out"; exit 1; }
[[ $out == *"warning: alice's user-runner package eventlog differs from this checkout's build; as alice run: $(pwd -P)/agents/eventlog/package.sh ~/.local/share/agent-bus/agent.d/eventlog"* ]] || { echo "the warning does not name the stale package and its refresh: $out"; exit 1; }
[ "$rc" = 1 ] || { echo "a stale package exited $rc, want 1: $out"; exit 1; }
tail -1 "$pkgs/eventlog/agent.json" | grep -qx '# an older build' || { echo "the stale package was changed"; exit 1; }
# Current packages and the unit failure path: a failed try-restart warns by name and the rest still run.
agents/eventlog/package.sh "$pkgs/eventlog" >/dev/null
: >"$CALLS"; rm -f "$d/state/"*.restarts; FAIL_UNIT=agent-bus-down.service
rc=0; out=$(bash ./git-install-agents.sh 2>&1) || rc=$?
[ "$rc" = 1 ] || { echo "a failed try-restart exited $rc, want 1: $out"; exit 1; }
[[ $out == *"warning: standalone agent-bus-down.service did not restart"* ]] || { echo "no warning naming the failed unit: $out"; exit 1; }
[ "$(grep -c '^warning: ' <<<"$out")" = 1 ] || { echo "current packages warned: $out"; exit 1; }
grep -qx 'try-restart -- agent-bus-up.service' "$CALLS" || { echo "the other unit was skipped after a failure: $(cat "$CALLS")"; exit 1; }
# Everything current and healthy: success, no warning.
FAIL_UNIT=""
out=$(bash ./git-install-agents.sh 2>&1) || { echo "a healthy run failed: $out"; exit 1; }
! grep -q '^warning: ' <<<"$out" || { echo "a healthy run warned: $out"; exit 1; }
# A listing over the cap is not compared: one warning naming the person.
out=$(AB_LIST_CAP=10 bash ./git-install-agents.sh 2>&1) && { echo "an over-cap listing exited 0: $out"; exit 1; }
[ "$(grep -c '^warning: ' <<<"$out")" = 1 ] && [[ $out == *"warning: alice's user-runner package listing is over 10 bytes; not compared"* ]] || { echo "an over-cap listing was not refused by name: $out"; exit 1; }
# Without AB_TEST the seams are ignored: the real unit directory, systemctl, getent and
# setpriv from PATH (all mocks here), never the listing as the caller.
mkdir "$d/prod"
printf '#!/bin/bash\nprintf "%%s\\n" "$*" >>"%s/prod-systemctl"\n' "$d" >"$d/prod/systemctl"
printf '#!/bin/bash\necho "alice:x:1234:1235::%s:/bin/bash"\n' "$home" >"$d/prod/getent"
printf '#!/bin/bash\nprintf "%%s\\n" "$@" >>"%s/setpriv-argv"\n' "$d" >"$d/prod/setpriv"
chmod +x "$d/prod/"*
: >"$CALLS"
out=$(env -u AB_TEST PATH="$d/prod:$PATH" bash ./git-install-agents.sh 2>&1) || { echo "the production path failed: $out"; exit 1; }
[ ! -s "$CALLS" ] || { echo "AB_SYSTEMCTL was honoured without AB_TEST: $(cat "$CALLS")"; exit 1; }
[ -f "$d/setpriv-argv" ] || { echo "the listing did not go through setpriv without AB_TEST: $out"; exit 1; }
want="--reuid=1234
--regid=1235
--init-groups
env
-i
HOME=$home
PATH=/usr/bin:/bin
sh
-c"
[ "$(head -9 "$d/setpriv-argv")" = "$want" ] || { echo "setpriv argv is not the person's ids, groups and a clean environment: $(cat "$d/setpriv-argv")"; exit 1; }
[ "$(grep -c '^--reuid=' "$d/setpriv-argv")" = 1 ] || { echo "want one setpriv call: $(cat "$d/setpriv-argv")"; exit 1; }
echo 'git-install try-restarts marked standalone units only and names stale user-runner pack packages: green'
