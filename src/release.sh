#!/usr/bin/env bash
# Deploy one commit to this node: the only way the live daemon and launchers
# change. Nothing is served from the working tree, so uncommitted edits —
# yours or another worker's — never reach the live node.
#
#   sudo ./release.sh             build HEAD, switch to it, restart, verify
#   sudo ./release.sh <commit>    the same for any commit
#   sudo ./release.sh --rollback  switch back to the previous release
#
# Layout: /usr/local/lib/agent-bus/releases/<version>-<sha>/ holds one built
# program directory (build.sh <out>); /usr/local/lib/agent-bus/current points
# at the live one and is switched with one rename; /usr/local/bin links go
# through current. The last five releases are kept for --rollback.
set -euo pipefail
cd "$(dirname "$0")"
src="$(pwd -P)"
root=/usr/local/lib/agent-bus
bindir=/usr/local/bin
unit=agent-busd.service
progs=(agent-bus agent-busd agent-bus-admin agent-bus-setup agent-bus-token)
launchers=(ab-claude ab-codex ab-opencode)
keep=5

[ "$(id -u)" = 0 ] || { echo "run as root: sudo $0 $*" >&2; exit 1; }
owner=$(stat -c %U "$src")
as_owner() { sudo -u "$owner" -- "$@"; }

switch() { # release-dir
    # Relative, as a packaged install writes it: agent-bus-setup refuses an
    # absolute current link.
    ln -sfn "releases/$(basename "$1")" "$root/current.new"
    mv -T "$root/current.new" "$root/current"
    for p in "${progs[@]}"; do ln -sfn "$root/current/$p" "$bindir/$p"; done
    # The Go dashboard is gone since 0.8.50; its link would dangle.
    [ -L "$bindir/agent-bus-web" ] && rm -f "$bindir/agent-bus-web"
    for p in "${launchers[@]}"; do ln -sfn "$root/current/launchers/$p" "$bindir/$p"; done
    if command -v restorecon >/dev/null && [ "$(getenforce 2>/dev/null)" != Disabled ]; then
        restorecon -RF "$root" >/dev/null
    fi
    systemctl restart "$unit"
    want=$(cat "$1/internal/version/VERSION")
    for _ in $(seq 1 50); do
        systemctl is-active --quiet "$unit" && "$bindir/agent-busd" -version 2>/dev/null | grep -qx "$want" && break
        sleep 0.2
    done
    systemctl is-active --quiet "$unit" || { echo "$unit did not start; roll back with: sudo $0 --rollback" >&2; exit 1; }
    echo "live: $(basename "$1") ($("$bindir/agent-busd" -version | tail -1))"
    # The web face is its own unit: restarted so it serves the same release.
    if [ -f /etc/systemd/system/agent-bus-web.service ]; then
        systemctl restart agent-bus-web.service
        addr=$(systemctl show agent-bus-web.service -p Environment --value | tr ' ' '\n' | sed -n 's/^AGENT_BUS_WEB_ADDR=//p')
        for _ in $(seq 1 50); do curl -sf -o /dev/null "http://${addr:-127.0.0.1:6780}/healthz" && break; sleep 0.2; done
        curl -sf -o /dev/null "http://${addr:-127.0.0.1:6780}/healthz" && echo "web: agent-bus-web answers on http://${addr:-127.0.0.1:6780}/" \
            || echo "web: agent-bus-web did not answer; journalctl -u agent-bus-web" >&2
    fi
}

if [ "${1:-}" = --rollback ]; then
    now=$(readlink -f "$root/current")
    prev=$(ls -1dt "$root"/releases/*/ | sed 's:/$::' | grep -vx "$now" | head -1)
    [ -n "$prev" ] || { echo "no earlier release to roll back to" >&2; exit 1; }
    switch "$prev"
    exit 0
fi

commit=$(as_owner git -C "$src" rev-parse --verify "${1:-HEAD}^{commit}")
sha=${commit:0:7}
version=$(as_owner git -C "$src" show "$commit:src/internal/version/VERSION" | tr -d '[:space:]')
dest="$root/releases/$version-$sha"
if [ ! -x "$dest/agent-busd" ]; then
    work=$(as_owner mktemp -d "$src/../tmp/release.XXXXXX")
    trap 'rm -rf "$work"' EXIT
    as_owner sh -c "git -C '$src/..' archive '$commit' src LICENSE.md | tar -x -C '$work'"
    # The MCP face's dependencies are not in Git; hard links cost nothing.
    as_owner cp -al "$src/mcp/node_modules" "$work/src/mcp/node_modules"
    as_owner env BUILD_COMMIT="$sha" "$work/src/build.sh" "$work/out" >/dev/null
    mkdir -p "$root/releases"
    cp -a "$work/out" "$dest.new"
    chown -R root:root "$dest.new"
    chmod -R go-w "$dest.new"
    mv -T "$dest.new" "$dest"
fi
switch "$dest"
# Keep the newest few, and always the live one.
live=$(readlink -f "$root/current")
ls -1dt "$root"/releases/*/ | sed 's:/$::' | tail -n +$((keep + 1)) | while read -r old; do
    [ "$old" = "$live" ] || rm -rf "$old"
done
