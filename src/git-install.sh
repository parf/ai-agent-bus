#!/bin/bash
# git-install: point the installed programs at this checkout, so a change is a
# build and a restart rather than a reinstall. Development only.
#
# It touches nothing but the symlinks in /usr/local/bin. Daemon state under
# /var/lib/agent-bus is not ours and never comes from git.
#
# The checkout must live outside /home. The daemon runs as its own account
# behind ProtectHome=yes and cannot exec a binary in somebody's home directory
# at all — and a home is 0700, so it could not traverse one either. Keeping the
# checkout in /usr/local/src is what makes this need no loosened isolation.
set -euo pipefail
cd "$(dirname "$0")"
src="$(pwd -P)"

progs=(agent-bus agent-busd agent-bus-admin agent-bus-setup agent-bus-token)
launchers=(ab-claude ab-codex ab-opencode)
bindir=/usr/local/bin
unit=agent-busd.service
want=/usr/local/src

build=yes revert=no reinstall=no
for a in "$@"; do case $a in
    --revert)   revert=yes ;;
    --reinstall) reinstall=yes ;;
    --no-build) build=no ;;
    -h|--help)  sed -n '2,12p' "$0"
                echo
                echo "  $0 [--no-build]   symlink this checkout into $bindir"
                echo "  $0 --revert       copy the real binaries back"
                echo "  $0 --reinstall    link, then set the daemon's state aside and install fresh (the 0.7 cutover)"
                exit 0 ;;
    *) echo "unknown option: $a" >&2; exit 2 ;;
esac; done

if [ "$(id -u)" != 0 ]; then
    echo "this needs root once, to write $bindir. Run:"
    echo
    echo "    sudo $0 $*"
    exit 1
fi

if [ "$revert" = yes ]; then
    for p in "${progs[@]}"; do
        [ -L "$bindir/$p" ] || continue
        target=$(readlink -f "$bindir/$p")
        # Through a temp name: install(1) over a live symlink follows it and
        # would overwrite the build output it points at.
        install -m 0755 "$target" "$bindir/.$p.new"
        mv -f "$bindir/.$p.new" "$bindir/$p"
        echo "restored $p"
    done
    systemctl restart "$unit"
    echo "$unit restarted from real binaries"
    exit 0
fi

case "$src" in
    "$want"/*) ;;
    *) cat >&2 <<EOF
this checkout is at $src, and the daemon cannot run from there.

It runs as its own account with ProtectHome=yes, so a home directory is not
merely unreadable to it — it is not present in its filesystem at all. Move the
checkout under $want and work through a symlink:

    sudo mv $(dirname "$src") $want/
    ln -s $want/$(basename "$(dirname "$src")") ~/src/ab

then run this again from there.
EOF
       exit 1 ;;
esac

if [ "$build" = yes ]; then
    # As the checkout's owner: root would leave root-owned objects behind in a
    # tree its owner has to keep building in.
    sudo -u "$(stat -c %U "$src")" ./build.sh
fi
for p in "${progs[@]}"; do
    [ -x "$src/$p" ] || { echo "not built: $src/$p" >&2; exit 1; }
done

# A checkout copied from a home directory carries user_home_t, which init_t
# may not execute — the failure is 203/EXEC with the permission bits correct
# and nothing but the audit log to say why. Relabelling for this path is what
# a build here produces anyway, so it is a no-op on a clean checkout.
if command -v restorecon >/dev/null && [ "$(getenforce 2>/dev/null)" != Disabled ]; then
    restorecon -RF "$src"
fi

for p in "${progs[@]}"; do
    ln -sfn "$src/$p" "$bindir/$p"
done
# The launchers are TypeScript, which build.sh does not compile into the
# checkout, so they only pick up an edit when run from here. Each ab-* prefers
# a launcher.js beside itself, which is how a copied install goes on running a
# stale bundle however often the source changes.
for p in "${launchers[@]}"; do
    ln -sfn "$src/launchers/$p" "$bindir/$p"
done
echo "linked ${#progs[@]} programs and ${#launchers[@]} launchers to $src"

# A copy earlier in the owner's PATH wins silently, and the symptom is an edit
# that never appears. Report it rather than reaching into somebody's home.
# A login shell, because the owner's PATH is the one that decides; and resolved
# paths, because /usr/local/sbin is a symlink to bin on some systems and would
# otherwise report itself.
owner=$(stat -c %U "$src")
for p in "${launchers[@]}"; do
    other=$(sudo -iu "$owner" sh -lc "command -v $p" 2>/dev/null || true)
    if [ -n "$other" ] && [ "$(readlink -f "$other")" != "$(readlink -f "$bindir/$p")" ]; then
        echo "warning: $other shadows $bindir/$p; point it here with" >&2
        echo "    ln -sfn $src/launchers/$p $other" >&2
    fi
done

if [ "$reinstall" = yes ]; then
    # The cutover, against the linked daemon: stop, set the old state and the
    # unit's drop-ins aside, initialize a fresh database, write the unit and
    # bootstrap the installer (Plans/R0.8/0.7-cutover.md#procedure).
    "$src/agent-bus-setup" --reinstall --exec "$bindir/agent-busd"
else
    systemctl restart "$unit"
fi
# active is not serving: the listener is up a moment after the unit is, and a
# script that returns between the two teaches you to add your own sleep.
for _ in $(seq 30); do
    systemctl is-active --quiet "$unit" && "$bindir/agent-bus" status >/dev/null 2>&1 && break
    sleep 0.1
done
if ! systemctl is-active --quiet "$unit"; then
    echo "$unit did not come back:" >&2
    journalctl -u "$unit" -n 10 --no-pager -o cat >&2
    exit 1
fi
echo "$unit restarted from the checkout"
echo
echo "from here on:"
echo "    $src/build.sh && sudo systemctl restart $unit"
