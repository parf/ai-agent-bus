#!/bin/sh
# Run inside the built image: archive owners and permissive modes must not survive extraction.
set -eu
tree=$(readlink -f /usr/local/lib/agent-bus/current)
[ -d "$tree" ] || { echo 'FAIL release tree is absent'; exit 1; }
bad=$(find "$tree" \( ! -uid 0 -o ! -gid 0 -o \( ! -type l -perm /022 \) \) -print)
if [ -n "$bad" ]; then
    echo 'FAIL release tree keeps an archive owner or group/other write permission'
    exit 1
fi
echo 'PASS release tree is root-owned without group/other write permission'
