# Mac run: CLI and MCP face

📌 **TL;DR:** What the Mac session does today to prove the macOS CLI and the MCP face: run two prebuilt darwin binaries, or build them natively from the `ab-w1b/MAC1` branch, then against a bus over HTTPS do a status, a request and reply, and one MCP tool call. Nothing is installed and nothing is written outside one scratch directory. The runner and the installable archive are not part of this run ([macOS plan](macos.md#tasks)).

## Before you start

| Need | Detail |
|---|---|
| From the owner | `AGENT_BUS_ADDR` (`https://<host>:<port>`), `AGENT_BUS_TLS_FINGERPRINT` (`sha256:…`) and one token for one agent name, for example `#ab-mac@<realm>`, kept in the shell only, never in a file you commit or a message |
| Binaries | path A (prebuilt, fastest) or path B (build here, also proves the native build); do A first, B if time allows |
| Scratch | `mkdir -p ~/ab-mac-run && cd ~/ab-mac-run`; everything below happens in it |

Report anything unexpected as it is, with the output; do not fix and retry silently.

## Path A: prebuilt binaries

The owner copies the directory `/usr/local/src/ai-agent-bus/tmp/ab-w1b-mac1/tmp/mac-out/` from the development node (built from branch `ab-w1b/MAC1`, cross-built without cgo, so the process title is not set). Use the `arm64` pair on Apple Silicon, `amd64` and `x64` on Intel.

| Step | Command | Expected |
|---|---|---|
| A1 verify | `shasum -a 256 -c SHA256SUMS` | `OK` for each file you copied; a missing file is an error you may ignore for the other CPU |
| A2 allow | `chmod +x agent-bus-darwin-* agent-bus-ts-darwin-*; xattr -d com.apple.quarantine * 2>/dev/null; true` | no output; the binaries are unsigned, so a copy that carried the quarantine flag would be refused otherwise |
| A3 name them | `cp agent-bus-darwin-arm64 agent-bus; cp agent-bus-ts-darwin-arm64 agent-bus-ts` | files `agent-bus` and `agent-bus-ts` |
| A4 version | `./agent-bus --version; ./agent-bus-ts 2>&1 \| head -2` | the first line is the release version, the second `build_info: mac-test …`; the second command prints `agent-bus-ts: run as one of …`, which proves the binary runs; it must not say `cannot execute` or `killed` |

## Path B: build natively

Needs `git`, Go (the version in `src/go.mod`), the Xcode command line tools (a C compiler, for the process title) and `bun`. Get the branch: the owner pushes it, or hands you `git bundle` output of `ab-w1b/MAC1`; then `git clone … && git checkout ab-w1b/MAC1`.

| Step | Command (from the repository root) | Expected |
|---|---|---|
| B1 CLI | `cd src && go build -ldflags "-X 'github.com/parf/ai-agent-bus/internal/version.Build=mac-native $(date +%Y-%m-%d)'" -o ~/ab-mac-run/agent-bus ./cmd/agent-bus && cd ..` | no output, under two minutes the first time |
| B2 MCP face | `(cd src/mcp && bun install --frozen-lockfile) && (cd src/web && bun install --frozen-lockfile) && cd src && AGENT_BUS_BUILD_VERSION=$(cat internal/version/VERSION) AGENT_BUS_BUILD_INFO=mac-native bun build --compile --target=bun-darwin-$(uname -m \| sed s/x86_64/x64/) ts-main.ts --outfile ~/ab-mac-run/agent-bus-ts --env 'AGENT_BUS_BUILD_*' && cd ..` | a `compile` line naming the target; about a minute |
| B3 version | `cd ~/ab-mac-run && ./agent-bus --version` | the first line is `cat src/internal/version/VERSION`, the second `build_info: mac-native …` |

Do not run `src/build.sh` or `package.sh`: they build the daemon and runner, which do not build on macOS yet.

## Against the bus (both paths)

```
export AGENT_BUS_ADDR=…  AGENT_BUS_TLS_FINGERPRINT=sha256:…  AGENT_BUS_TOKEN=…
export AGENT_BUS_NAME='#ab-mac@<realm>'
```

| Step | Command | Expected |
|---|---|---|
| 1 status | `./agent-bus status` | a JSON line with `"you":"#ab-mac@<realm>"`; a refusal naming the fingerprint means the fingerprint is wrong, nothing to fix here |
| 2 request | `./agent-bus send "$AGENT_BUS_NAME" --topic mac-1 --tag q 'ping from the Mac'` | prints a message id |
| 3 receive | `./agent-bus consume --topic mac-1 --tag q --wait 10s` | one JSON message whose body is `ping from the Mac`; note its `message_id` |
| 4 reply | `./agent-bus reply <message_id> 'pong from the Mac'`, then `./agent-bus consume --topic mac-1 --tag q --wait 10s` | the second message carries `pong from the Mac` |
| 5 MCP | the block below | a line with `"id":2` that lists `#ab-mac@<realm>` |

```
{ printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"mac","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ab_ls","arguments":{"kind":"agent"}}}'
  sleep 3; } | ./agent-bus-ts mcp
```

The `sleep` keeps standard input open until the answer is written. If the owner's session already holds `#ab-mac@<realm>` as a reader, the MCP face says it is read by another session; use a different name and token for step 5.

## Evidence back

One bus message to `#ab-lead1` (topic `mac-run`, tag `ab-dvp`) with: `uname -m`, `sw_vers -productVersion`, the branch commit (`git rev-parse --short HEAD`, or the stamp in `--version` for path A), each step's number with its exit status, and the full output of A4 or B3, steps 1, 3, 4 and 5. Never include the token. A step that failed is reported with its output and the next steps still run where they can.
