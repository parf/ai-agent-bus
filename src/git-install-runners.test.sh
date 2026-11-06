#!/bin/bash
# git-install-runners.sh against a fake systemctl that records every call. No host unit is touched.
set -eu
cd "$(dirname "$0")"
mkdir -p ../tmp
d=$(mktemp -d "$(cd .. && pwd)/tmp/git-install-runners.XXXXXX")
trap 'rm -rf "$d"' EXIT
mkdir "$d/bin"
cat >"$d/bin/systemctl" <<'FAKE'
#!/bin/bash
printf '%s\n' "$*" >>"$CALLS"
case $1 in
  list-units)
    # The fake honours --state=active the way systemctl does.
    echo "agent-bus-runner@alice.service loaded active running agent-bus runner for alice"
    echo "agent-bus-runner@bob.service loaded active running agent-bus runner for bob"
    case " $* " in *" --state=active "*) ;; *) echo "agent-bus-runner@carol.service loaded inactive dead agent-bus runner for carol" ;; esac ;;
  restart) [ "$2" != "$FAIL_UNIT" ] ;;
esac
FAKE
chmod +x "$d/bin/systemctl"
export PATH="$d/bin:$PATH" CALLS="$d/calls" FAIL_UNIT=""
: >"$CALLS"
out=$(bash ./git-install-runners.sh 2>&1) || { echo "restart failed on a healthy fake: $out"; exit 1; }
restarts=$(grep -c '^restart ' "$CALLS" || true)
[ "$restarts" = 2 ] || { echo "want 2 restarts, got $restarts: $(cat "$CALLS")"; exit 1; }
for u in alice bob; do
  [ "$(grep -cx "restart agent-bus-runner@$u.service" "$CALLS")" = 1 ] || { echo "$u was not restarted exactly once: $(cat "$CALLS")"; exit 1; }
done
! grep -q carol "$CALLS" || { echo "an inactive instance was restarted: $(cat "$CALLS")"; exit 1; }
grep -qx 'list-units agent-bus-runner@\*.service --state=active --plain --no-legend' "$CALLS" || { echo "the listing is not the active runner instances only: $(cat "$CALLS")"; exit 1; }
[[ $out == *"agent-bus-runner@alice.service restarted"* && $out == *"agent-bus-runner@bob.service restarted"* ]] || { echo "the run did not say what it restarted: $out"; exit 1; }
# One instance failing is a warning and a failed exit, and the others are still restarted.
: >"$CALLS"; FAIL_UNIT="agent-bus-runner@alice.service"
rc=0; out=$(bash ./git-install-runners.sh 2>&1) || rc=$?
[ "$rc" = 1 ] || { echo "a failed restart exited $rc, want 1: $out"; exit 1; }
[[ $out == *"warning: agent-bus-runner@alice.service did not restart"* ]] || { echo "no warning naming the failed instance: $out"; exit 1; }
grep -qx 'restart agent-bus-runner@bob.service' "$CALLS" || { echo "the instance after the failed one was skipped: $(cat "$CALLS")"; exit 1; }
# Nothing active: nothing restarted, success.
cat >"$d/bin/systemctl" <<'EMPTY'
#!/bin/bash
printf '%s\n' "$*" >>"$CALLS"
EMPTY
: >"$CALLS"; FAIL_UNIT=""
bash ./git-install-runners.sh >/dev/null || { echo "no active instance must succeed"; exit 1; }
! grep -q '^restart ' "$CALLS" || { echo "restarted with none active: $(cat "$CALLS")"; exit 1; }
echo 'git-install restarts each active root-account runner once and only those: green'
