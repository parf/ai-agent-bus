#!/bin/bash
# After a deploy, put the agents that run outside the runner on the new code:
#  1. every standalone agent unit (first line "# agent-bus standalone: ", written by
#     src/agents/standalone/install.py) gets try-restart, so a running one restarts
#     and a stopped or disabled one stays as it is; an unmarked unit is never touched;
#  2. each person's user-runner pack packages (a package with support/agents/VERSION
#     in ~/.local/share/agent-bus/agent.d) that differ from this checkout's build are
#     named, with the command that refreshes them. Warn only: nothing is changed and
#     no person's code runs. The person's own agents (no marker) are ignored.
# Root never opens a path under a home: the person lists their packages' hashes.
# Exits 1, after trying everything, if a restart failed or a package is stale.
#
# Test seams, honoured only with AB_TEST=1, which a deploy never sets: AB_UNIT_DIR,
# AB_SYSTEMCTL, AB_PASSWD (a passwd-format file instead of `getent passwd`),
# AB_AS_PERSON=direct (list as the caller with the person's HOME, not through setpriv),
# AB_LIST_CAP (bytes). Root with AB_TEST=1 against the real unit directory is refused.
set -u
src=$(cd "$(dirname "$0")" && pwd -P)
unit_dir=/etc/systemd/system systemctl=systemctl passwd='' as_person='' cap=$((4 * 1024 * 1024))
if [ "${AB_TEST:-}" = 1 ]; then
    unit_dir=${AB_UNIT_DIR:-$unit_dir} systemctl=${AB_SYSTEMCTL:-$systemctl} passwd=${AB_PASSWD:-}
    as_person=${AB_AS_PERSON:-} cap=${AB_LIST_CAP:-$cap}
    if [ "$(id -u)" = 0 ] && [ "$unit_dir" = /etc/systemd/system ]; then
        echo "refused: AB_TEST=1 is for tests; root against /etc/systemd/system never takes it" >&2
        exit 2
    fi
fi
marker='# agent-bus standalone: '
userpkgs=.local/share/agent-bus/agent.d
failed=0

shopt -s nullglob
for file in "$unit_dir"/*.service; do
    [ -f "$file" ] && [ ! -L "$file" ] || continue
    IFS= read -r first <"$file" || [ -n "$first" ] || continue
    [[ $first == "$marker"* ]] || continue
    unit=${file##*/}
    if "$systemctl" try-restart -- "$unit"; then
        echo "$unit restarted if it was running"
    else
        echo "warning: standalone $unit did not restart; run: systemctl try-restart $unit" >&2
        failed=1
    fi
done

# One line per file of each pack package: "<package> <sha256> <relative path>".
# Runs as the person, in their home; prints nothing when there is no agent.d.
# `*/` never matches a dot name, so ".", ".." and hidden directories are never packages.
# shellcheck disable=SC2016
list='cd "$HOME/'"$userpkgs"'" 2>/dev/null || exit 0
for p in */; do
    p=${p%/}
    [ -f "$p/support/agents/VERSION" ] && [ ! -L "$p" ] || continue
    (cd "$p" && find . -type f ! -path "*/__pycache__/*" -print0 | sort -z | xargs -0r sha256sum) | sed "s|^|$p |"
done'

checkout_agent() { # name -> the checkout agent directory that packages it
    local d
    for d in "$src/agents/$1" "$src/agents/standalone/$1"; do
        [ -x "$d/package.sh" ] && { echo "$d"; return 0; }
    done
    return 1
}

work=$(mktemp -d "$src/../tmp/git-install-agents.XXXXXX" 2>/dev/null || mktemp -d)
trap 'rm -rf "$work"' EXIT
accounts=$(if [ -n "$passwd" ]; then cat "$passwd"; else getent passwd; fi)
while IFS=: read -r person _ uid gid _ home shell; do
    [ "$uid" -ge 1000 ] 2>/dev/null && [ "$uid" -lt 60000 ] && [ -d "$home" ] || continue
    case $shell in */nologin|*/false) continue ;; esac
    # Bounded: a huge agent.d or a hung home must not stall the deploy.
    if [ "$as_person" = direct ]; then
        HOME=$home timeout 60 sh -c "$list" </dev/null | head -c $((cap + 1)) >"$work/list"
    else
        timeout 60 setpriv --reuid="$uid" --regid="$gid" --init-groups env -i HOME="$home" PATH=/usr/bin:/bin sh -c "$list" </dev/null |
            head -c $((cap + 1)) >"$work/list"
    fi
    rc=${PIPESTATUS[0]}
    if [ "$(stat -c %s "$work/list")" -gt "$cap" ]; then
        echo "warning: $person's user-runner package listing is over $cap bytes; not compared" >&2; failed=1; continue
    elif [ "$rc" = 124 ]; then
        echo "warning: listing $person's user-runner packages timed out; not compared" >&2; failed=1; continue
    elif [ "$rc" != 0 ]; then
        echo "warning: could not list $person's user-runner packages; not compared" >&2; failed=1; continue
    fi
    have=$(cat "$work/list")
    [ -n "$have" ] || continue
    while IFS= read -r name; do
        [[ $name =~ ^[a-z0-9][a-z0-9._-]*$ ]] || continue
        dir=$(checkout_agent "$name") || continue
        built=$work/$name
        if [ ! -d "$built" ]; then
            "$dir/package.sh" "$built" </dev/null >/dev/null 2>&1 || { echo "warning: could not build $name to compare $person's copy" >&2; failed=1; continue; }
        fi
        want=$(cd "$built" && find . -type f ! -path '*/__pycache__/*' -print0 | sort -z | xargs -0r sha256sum)
        mine=$(awk -v n="$name" '$1 == n { sub(/^[^ ]+ /, ""); print }' <<<"$have")
        # Stale when a file the checkout ships is missing or differs; extra files are the person's.
        if [ -n "$(comm -23 <(sort <<<"$want") <(sort <<<"$mine"))" ]; then
            echo "warning: $person's user-runner package $name differs from this checkout's build; as $person run: $dir/package.sh ~/$userpkgs/$name" >&2
            failed=1
        fi
    done < <(cut -d' ' -f1 <<<"$have" | sort -u)
done <<<"$accounts"
exit "$failed"
