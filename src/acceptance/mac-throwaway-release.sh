#!/bin/bash
# An unsigned throwaway release for trying install.sh --client on a Mac,
# made on Linux from package.sh's two macOS client archives and Linux stand-ins.
# It generates SHA256SUMS and a copy of install.sh fetching from loopback HTTP.
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
mkdir -p "$rel"
cp "${archives[@]}" "$rel/"
for p in agent-bus/x86_64 agent-bus/aarch64 agent-bus-client/x86_64 agent-bus-client/aarch64; do
  printf 'stand-in %s\n' "$p" >"$rel/${p%%/*}-$version-linux-${p##*/}.tar.gz"
done
"$src/release-sign.sh" "$rel" "$version" >/dev/null
python3 - "$src/../install.sh" "$out/install-throwaway.sh" "$port" <<'P'
import sys
src, dst, port = sys.argv[1:4]
s = open(src).read()
for old, new in {
    'RELEASES="https://github.com/parf/ai-agent-bus/releases"': 'RELEASES="http://127.0.0.1:%s"' % port,
    " --proto '=https' --proto-redir '=https' --tlsv1.2": "",
}.items():
    assert s.count(old) == 1, old
    s = s.replace(old, new)
open(dst, 'w').write(s)
P
chmod +x "$out/install-throwaway.sh"
echo "release $version in $out/serve (port $port); install script $out/install-throwaway.sh"
ls "$rel"
