#!/bin/sh
# Sign one release for install.sh: run by the owner, on the machine that holds
# the release key, after downloading the draft release's archives.
#
#   src/release-sign.sh <dir> <private-key> <version> [<ci-sums>]
#
# <dir> must hold exactly the four archives of <version> — the full node and
# the client edition, x86_64 and aarch64 — and nothing else ending in .tar.gz.
# <ci-sums> is the hash list the release workflow printed in its summary;
# given, every archive must match it, so a draft swapped after CI is refused.
# Writes <dir>/SHA256SUMS and SHA256SUMS.sig, an ssh-keygen signature in the
# agent-bus-release namespace; upload both to the release. The version is
# signed through the archive names. The key never leaves that machine and is
# never given to CI (docs/07-setup.md#simple-install).
set -eu
usage="usage: release-sign.sh <dir> <private-key> <version> [<ci-sums>]"
dir=${1:?$usage}
key=${2:?$usage}
version=${3:?$usage}
sums=${4:-}
case $version in *[!0-9.]*|.*|*.|*..*) echo "release-sign: $version is not MAJOR.MINOR.PATCH" >&2; exit 1 ;; esac
[ -z "$sums" ] || sums=$(cd "$(dirname "$sums")" && pwd)/$(basename "$sums")
cd "$dir"
want=""
for edition in agent-bus agent-bus-client; do
    for arch in x86_64 aarch64; do want="$want $edition-$version-linux-$arch.tar.gz"; done
done
for f in *.tar.gz; do
    case " $want " in *" $f "*) ;; *) echo "release-sign: $f is not one of this release's archives; nothing signed" >&2; exit 1 ;; esac
done
for f in $want; do
    [ -f "$f" ] || { echo "release-sign: $f is missing; nothing signed" >&2; exit 1; }
done
trap 'rm -f SHA256SUMS.new' EXIT
# shellcheck disable=SC2086 # the four fixed names
sha256sum $want > SHA256SUMS.new
if [ -n "$sums" ]; then
    while read -r hash name; do
        grep -qxF "$hash  $name" "$sums" || { echo "release-sign: $name does not match the CI hash list; nothing signed" >&2; exit 1; }
    done < SHA256SUMS.new
fi
cat SHA256SUMS.new
mv SHA256SUMS.new SHA256SUMS
rm -f SHA256SUMS.sig
ssh-keygen -Y sign -q -f "$key" -n agent-bus-release SHA256SUMS
echo "signed $version: $dir/SHA256SUMS and SHA256SUMS.sig"
