#!/bin/bash
# A signed throwaway release for trying install.sh --client on a Mac, made on
# a Linux node. It takes the two macOS client archives package.sh built,
# adds stand-ins for the four Linux archives release-sign.sh insists on,
# signs everything with a key made here and thrown away (only its public half
# is kept, inside the copy of install.sh), and writes a copy of install.sh
# that trusts that key and fetches from a local HTTP server.
#
#   src/acceptance/mac-throwaway-release.sh <dir with the darwin archives> <out dir> [port]
#
# On the Mac: cd <out>/serve && python3 -m http.server <port> --bind 127.0.0.1,
# then, in another shell: sh <out>/install-throwaway.sh --client --version <v>
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
src=$(cd "$here/.." && pwd)
in=$(realpath "${1:?usage: mac-throwaway-release.sh <dir with the darwin archives> <out dir> [port]}")
out=$(realpath -m "${2:?out dir}")
port=${3:-8787}
[ ! -e "$out" ] || { echo "$out exists" >&2; exit 1; }
mapfile -t archives < <(ls "$in"/agent-bus-client-*-darwin-*.tar.gz)
[ ${#archives[@]} = 2 ] || { echo "want the two macOS client archives in $in, found ${#archives[@]}" >&2; exit 1; }
version=$(basename "${archives[0]}" | sed -E 's/^agent-bus-client-([0-9.]+)-darwin-.*/\1/')
rel=$out/serve/download/v$version
mkdir -p "$rel" "$out/key"
cp "${archives[@]}" "$rel/"
for p in agent-bus/x86_64 agent-bus/aarch64 agent-bus-client/x86_64 agent-bus-client/aarch64; do
  printf 'stand-in %s\n' "$p" >"$rel/${p%%/*}-$version-linux-${p##*/}.tar.gz"
done
ssh-keygen -q -t ed25519 -N '' -C throwaway -f "$out/key/release"
"$src/release-sign.sh" "$rel" "$out/key/release" "$version" --no-ci-check >/dev/null
pub=$(cut -d' ' -f1,2 "$out/key/release.pub")
rm -f "$out/key/release"  # the private half is gone; nothing here can sign again
python3 - "$src/../install.sh" "$out/install-throwaway.sh" "$port" "$pub" <<'P'
import sys
src, dst, port, pub = sys.argv[1:5]
s = open(src).read()
for old, new in {
    'RELEASES="https://github.com/parf/ai-agent-bus/releases"': 'RELEASES="http://127.0.0.1:%s"' % port,
    'RELEASE_KEYS=""': 'RELEASE_KEYS="agent-bus-release %s"' % pub,
    " --proto '=https' --proto-redir '=https' --tlsv1.2": "",
}.items():
    assert s.count(old) == 1, old
    s = s.replace(old, new)
open(dst, 'w').write(s)
P
chmod +x "$out/install-throwaway.sh"
echo "release $version in $out/serve (port $port); install script $out/install-throwaway.sh"
ls "$rel"
