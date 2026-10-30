#!/bin/bash
# The npm client wrapper, installed the way a user installs it: package.sh's
# client archive wrapped by npm-pack.sh, both tarballs installed offline into a
# private prefix, then the installed commands used against a disposable daemon.
# Nothing is published; the registry is replaced by the two local tarballs.
set -eu
cd "$(dirname "$0")"
here=$PWD
mkdir -p ../tmp
work=$(mktemp -d "$here/../tmp/npmtest.XXXXXX")
DPID=""
trap '[ -z "$DPID" ] || kill "$DPID" 2>/dev/null; rm -rf "$work"' EXIT
fail() { echo "FAIL $*" >&2; exit 1; }
version=$(tr -d '[:space:]' <internal/version/VERSION)

./package.sh "$work/dist" >/dev/null 2>&1
client=$(ls "$work"/dist/agent-bus-client-*-linux-x86_64.tar.gz)
./npm-pack.sh "$client" "$work/npm" >/dev/null
npm install -g --prefix "$work/pfx" --offline --no-audit --no-fund \
  "$work/npm/agent-bus-linux-x64-$version.tgz" "$work/npm/agent-bus-$version.tgz" >/dev/null 2>&1 || fail "npm install of the two tarballs"
bin=$work/pfx/bin
mods=$work/pfx/lib/node_modules
# The thin package depends on the platform package at its exact version, and
# both carry the licence.
dep=$(node -p 'JSON.stringify(require(process.argv[1]).optionalDependencies)' "$mods/agent-bus/package.json")
[ "$dep" = "{\"agent-bus-linux-x64\":\"$version\"}" ] || fail "the thin package does not pin the platform package at $version: $dep"
for pkg in agent-bus agent-bus-linux-x64; do
  [ -s "$mods/$pkg/LICENSE.md" ] || fail "$pkg ships no LICENSE.md"
done
clean() { env -i PATH="$PATH" HOME="$work" "$@"; }

# Every program answers with the release version and a build stamp.
for p in agent-bus agent-bus-runner; do
  out=$(clean "$bin/$p" --version) || fail "$p --version"
  [ "$(sed -n 1p <<<"$out")" = "$version" ] || fail "$p reports $(sed -n 1p <<<"$out"), want $version"
  grep -q '^build_info: ..*' <<<"$out" && ! grep -q unstamped <<<"$out" || fail "$p has no build stamp"
done
# The wrapper passes the exit status through: bare agent-bus exits 1.
rc=0; clean "$bin/agent-bus" >/dev/null 2>&1 || rc=$?
[ "$rc" = 1 ] || fail "bare agent-bus exited $rc through the wrapper, want 1"
# Every bin the package promises exists and starts its program.
for p in ab-claude ab-codex ab-opencode ab-opencode1 agent-bus-mcp; do
  [ -x "$bin/$p" ] || fail "$p is not linked"
done

# A disposable daemon, built outside the checkout's binaries.
go build -o "$work/dbin/" ./cmd/agent-busd
D=$work/d; mkdir "$D"
"$work/dbin/agent-busd" --addr 127.0.0.1:0 --socket "$D/bus.sock" --owner parf@localhost --db "$D/bus.db" --create --flush-every 0 >"$D/log" 2>&1 &
DPID=$!
for _ in $(seq 1 100); do [ -n "$(curl -s --max-time 1 --unix-socket "$D/bus.sock" http://unix/status 2>/dev/null)" ] && break; sleep 0.1; done
OT=$(clean env AGENT_BUS_ADDR="$D/user-$(id -un).sock" "$bin/agent-bus" token parf@localhost) || fail "owner token"
ab() { clean env AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$1" AGENT_BUS_NAME="$2" "$bin/agent-bus" "${@:3}"; }
for n in '#asker@srv1' '#fixer@srv1'; do ab "$OT" parf@localhost register "$n" --kind agent --allow '*' >/dev/null || fail "register $n"; done
tok() { ab "$OT" parf@localhost token "$1"; }
TA=$(tok '#asker@srv1'); TF=$(tok '#fixer@srv1')

# Request and reply through the installed CLI.
ab "$TF" '#fixer@srv1' consume --wait 10s >"$D/got.json" & CP=$!
sleep 0.3
ab "$TA" '#asker@srv1' send '#fixer@srv1' --topic npm-1 --tag q "ping from npm" >/dev/null
wait $CP
grep -q 'ping from npm' "$D/got.json" || fail "the request did not arrive"
id=$(sed 's/.*"message_id":"\([^"]*\)".*/\1/' "$D/got.json")
ab "$TF" '#fixer@srv1' reply "$id" "pong from npm" >/dev/null || fail "reply"
reply=$(ab "$TA" '#asker@srv1' consume --topic npm-1 --tag q --wait 5s)
grep -q 'pong from npm' <<<"$reply" || fail "the reply did not come back: $reply"

# An MCP tool call through the installed agent-bus-mcp.
mcp=$( { printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ab_ls","arguments":{"kind":"agent"}}}'
  sleep 3; } |
  timeout 15 env -i PATH="$PATH" HOME="$work" AGENT_BUS_ADDR="$D/bus.sock" AGENT_BUS_TOKEN="$TA" AGENT_BUS_NAME='#asker@srv1' "$bin/agent-bus-mcp" 2>/dev/null) || true
grep -q '"id":2' <<<"$mcp" && grep -q '#fixer@srv1' <<<"$mcp" || fail "the MCP ab_ls call did not list #fixer@srv1: $mcp"

# Without the platform package the wrapper says so and fails, not silently.
rm -rf "$work/pfx/lib/node_modules/agent-bus-linux-x64"
rc=0; out=$(clean "$bin/agent-bus" --version 2>&1) || rc=$?
[ "$rc" != 0 ] && grep -q 'package is not installed' <<<"$out" || fail "a missing platform package was not refused: rc=$rc $out"
echo 'npm client wrapper: installed from tarballs, versions, exit status, request/reply, MCP call, missing platform refused'
