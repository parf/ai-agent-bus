#!/bin/bash
# Restart every active root-account runner (agent-bus-runner@<person>.service)
# once, after a deploy replaced the binary or the unit template they run: a
# running runner keeps the old code and limits until it is restarted. Inactive
# instances are left as they are; a user-account runner is the user manager's
# and is refreshed by `agent-bus-setup --runner refresh --user-runner`.
# Exits 1, after trying every instance, if any restart failed.
set -u
failed=0
while read -r unit _; do
    [ -n "$unit" ] || continue
    if systemctl restart "$unit"; then
        echo "$unit restarted"
    else
        echo "warning: $unit did not restart; run: systemctl restart $unit" >&2
        failed=1
    fi
done < <(systemctl list-units 'agent-bus-runner@*.service' --state=active --plain --no-legend)
exit "$failed"
