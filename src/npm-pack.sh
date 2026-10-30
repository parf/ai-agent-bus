#!/bin/bash
# Wrap a client-edition archive from package.sh as npm packages: one platform
# package holding the archive's tree, and the thin package whose bins start it
# (docs/07-setup.md#npm-package). Writes two .tgz into <out>; publishing them
# is the owner's, never this script's.
#
#   src/npm-pack.sh <agent-bus-client-VERSION-linux-x86_64.tar.gz> [out]
#
# Names come from NPM_NAME (the package people install) and NPM_PLATFORM_NAME
# (its linux x64 package): not chosen yet, QUESTIONS Q224.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
archive=$(realpath "${1:?usage: npm-pack.sh <client archive> [out]}")
out=$(realpath -m "${2:-$here/../tmp/npm}")
name=$(basename "$archive" .tar.gz)
echo "$name" | grep -Eq '^agent-bus-client-[0-9]+\.[0-9]+\.[0-9]+-linux-x86_64$' ||
  { echo "npm-pack: $name is not a linux x86_64 client archive" >&2; exit 1; }
version=${name#agent-bus-client-}; version=${version%-linux-x86_64}
main=${NPM_NAME:-agent-bus}
plat=${NPM_PLATFORM_NAME:-agent-bus-linux-x64}
mkdir -p "$here/../tmp" "$out"
work=$(mktemp -d "$here/../tmp/npmpack.XXXXXX")
trap 'rm -rf "$work"' EXIT

# Verify the archive before wrapping it: the checksum beside it, then its own manifest.
if [ -f "$archive.sha256" ]; then
  (cd "$(dirname "$archive")" && sha256sum -c "$(basename "$archive").sha256" >/dev/null) ||
    { echo "npm-pack: $archive does not match its .sha256" >&2; exit 1; }
fi
mkdir "$work/unpack"
tar -C "$work/unpack" -xzf "$archive"
tree="$work/unpack/$name"
(cd "$tree" && sha256sum -c MANIFEST.sha256 >/dev/null) ||
  { echo "npm-pack: MANIFEST.sha256 does not match the archive's files" >&2; exit 1; }

# The platform package: the archive tree untouched, under bundle/.
p="$work/platform"
mkdir -p "$p"
mv "$tree" "$p/bundle"
cp "$p/bundle/LICENSE.md" "$p/LICENSE.md"
cat >"$p/package.json" <<J
{
  "name": "$plat",
  "version": "$version",
  "description": "agent-bus client edition for Linux x86_64 (installed through $main)",
  "license": "SEE LICENSE IN LICENSE.md",
  "os": ["linux"],
  "cpu": ["x64"],
  "files": ["bundle", "LICENSE.md"]
}
J

# The thin package: one launcher that finds the platform package and runs a
# program of its bundle; each bin is a one-line file naming its program.
w="$work/main"
mkdir -p "$w/bin"
cp "$p/bundle/LICENSE.md" "$w/LICENSE.md"
cat >"$w/launch.js" <<'J'
"use strict";
// Runs a program of the platform package's bundle, passing arguments, input,
// output, exit status and termination signals through.
const { spawn } = require("node:child_process");
const { join, dirname } = require("node:path");
const PLATFORM = require("./package.json").agentBusPlatformPackage;

module.exports = function launch(program, args) {
  let root;
  try {
    root = dirname(require.resolve(`${PLATFORM}/package.json`));
  } catch {
    console.error(`${program}: the ${PLATFORM} package is not installed; ` +
      `this wrapper supports Linux x86_64 only, and optional dependencies must not be omitted`);
    process.exit(1);
  }
  const bundle = join(root, "bundle");
  // The compiled TypeScript parts are one binary chosen by its first argument.
  const ts = { "agent-bus-mcp": "mcp" };
  const [file, argv] = program in ts
    ? [join(bundle, "agent-bus-ts"), [ts[program], ...args]]
    : [join(bundle, program.startsWith("ab-") ? `launchers/${program}` : program), args];
  const child = spawn(file, argv, { stdio: "inherit" });
  for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"]) process.on(sig, () => child.kill(sig));
  child.on("error", (e) => { console.error(`${program}: ${e.message}`); process.exit(127); });
  child.on("exit", (code, signal) => {
    if (signal) { process.removeAllListeners(signal); process.kill(process.pid, signal); }
    else process.exit(code ?? 1);
  });
};
J
bins=""
for prog in agent-bus agent-bus-runner agent-bus-mcp ab-claude ab-codex ab-opencode ab-opencode1; do
  printf '#!/usr/bin/env node\nrequire("../launch.js")("%s", process.argv.slice(2));\n' "$prog" >"$w/bin/$prog"
  chmod 0755 "$w/bin/$prog"
  bins="$bins\"$prog\": \"bin/$prog\", "
done
cat >"$w/package.json" <<J
{
  "name": "$main",
  "version": "$version",
  "description": "agent-bus client edition: the CLI, the runner, the MCP face and the launchers, to join an existing bus",
  "license": "SEE LICENSE IN LICENSE.md",
  "bin": { ${bins%, } },
  "files": ["bin", "launch.js", "LICENSE.md"],
  "optionalDependencies": { "$plat": "$version" },
  "agentBusPlatformPackage": "$plat",
  "engines": { "node": ">=18" }
}
J
(cd "$p" && npm pack --silent --pack-destination "$out" >/dev/null)
(cd "$w" && npm pack --silent --pack-destination "$out" >/dev/null)
ls "$out/$main-$version.tgz" "$out/$plat-$version.tgz"
