#!/bin/sh
# Historical filename: generate SHA256SUMS only, never a signature or key.
# release-sign.sh <dir> <version> [<ci-sums>]
# Require a complete Linux release and either both macOS client archives or neither.
set -eu
usage="usage: release-sign.sh <dir> <version> [<ci-sums>]"
[ $# -ge 2 ] && [ $# -le 3 ] || { echo "$usage" >&2; exit 2; }
dir=$1 version=$2 sums=${3:-}
echo "$version" | grep -Eqx '[0-9]+\.[0-9]+\.[0-9]+' || { echo "release-sums: $version is not MAJOR.MINOR.PATCH" >&2; exit 1; }
if [ -n "$sums" ]; then
    [ -f "$sums" ] || { echo "release-sums: no CI hash list at $sums" >&2; exit 1; }
    sums=$(cd "$(dirname "$sums")" && pwd)/$(basename "$sums")
fi
cd "$dir"
want=""
for edition in agent-bus agent-bus-client; do
    for arch in x86_64 aarch64; do want="$want $edition-$version-linux-$arch.tar.gz"; done
done
for arch in x86_64 aarch64; do
    if [ -e "agent-bus-client-$version-darwin-$arch.tar.gz" ]; then
        for a in x86_64 aarch64; do want="$want agent-bus-client-$version-darwin-$a.tar.gz"; done
        break
    fi
done
for f in *.tar.gz; do
    case " $want " in *" $f "*) ;; *) echo "release-sums: $f is not one of this release's archives; no checksums written" >&2; exit 1 ;; esac
done
for f in $want; do
    [ -f "$f" ] && [ ! -L "$f" ] || { echo "release-sums: $f is missing or not a regular file; no checksums written" >&2; exit 1; }
done
trap 'rm -f SHA256SUMS.new' EXIT
# shellcheck disable=SC2086 # the fixed names
sha256sum $want > SHA256SUMS.new
if [ -n "$sums" ]; then
    while read -r hash name; do
        grep -qxF "$hash  $name" "$sums" || { echo "release-sums: $name does not match the CI hash list; no checksums written" >&2; exit 1; }
    done < SHA256SUMS.new
fi
cat SHA256SUMS.new
mv SHA256SUMS.new SHA256SUMS
echo "checksummed $version: $dir/SHA256SUMS"
