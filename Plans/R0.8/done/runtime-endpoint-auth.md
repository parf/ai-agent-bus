# Runtime endpoint authentication

📌 **TL;DR:** 0.5.48 protects the Codex App Server; installed endpoint checks pass, full H.9.5 interactive acceptance remains open.

## Change

A disposable Codex 0.154.0 App Server accepted initialization from the actual
`agent-bus-runner` account over its unprotected loopback WebSocket. A random port
was discoverability, not an account boundary. OpenCode already required a server
password; launcher rename control already required a bearer capability.

The Codex launcher now enables native capability-token authentication before
WebSocket upgrade. Its private run directory holds a mode-0600 token file; the
pusher sends Authorization and the TUI uses `--remote-auth-token-env`. No token
appears in launcher-supplied argv. An unsupported runtime fails startup rather
than reverting to an open listener. [Current contract](../../../docs/08-runner-role.md#runtime-isolation-and-recovery).

## Checks

| Evidence | Bound |
|---|---|
| `src/acceptance/runtime-isolation.ts` | Installed Codex 0.154.0 and OpenCode 1.18.30; fresh homes, disposable sessions, second actual OS account; no model calls |
| `tmp/runtime-isolation/installed-final.log` | 20 checks, 0 failures: intended clients read/rename; second account cannot; remove authentication and the same second-account probes read/rename successfully |
| Secret exposure | Codex token file and both runtime environments unreadable to second account; tokens absent from runtime argv; wrong Codex capability refused |
| Launcher control | Second-account requests refused without changing the title; intended capability permits rename |
| Launcher/rename smoke | Separate runtime fixtures with real daemon, MCP and pusher; Codex TUI fixture performs a WebSocket handshake rather than merely reporting an environment variable |
| Full frozen smoke | `tmp/runtime-isolation/slow-final.log`: 589 passed, 0 failed; vet and race green; source manifest unchanged |

The tracked smoke script and its frozen `src/runtime-isolation-smoke.local.sh`
copy both hash to `d873f2bd046422220fb9f6fc0d96734ebd0f291548d6cdabab62a424b9054bd3`
(SHA-256).

The installed checks call the same `codexAuth` function as the launcher, and
use the production Codex/OpenCode adapters for intended-client requests. They
exercise read and **rename**, not an unauthorized model turn. The launcher
fixture checks complete request/reply delivery separately.

## Mutations

`tmp/runtime-isolation/mutate.py` copies source into isolated trees. Native
checks use fresh runtime homes for each mutant; no mutation touches a live
session or the working source.

| Change | Named failure |
|---|---|
| Remove native Codex authentication arguments | Protected Codex allows second-account read/rename |
| Omit the pusher's Authorization header | Capability-required adapter test fails before reading the server |
| Remove launcher-control authentication | Second account renames without a capability |
| Omit the TUI's token environment option | TUI fixture cannot authenticate; its named authentication check fails |

Final `mutations-final.log`: 4/4 caught by the named checks. Native checks,
targeted adapter tests and launcher integration runs are separate scopes, not
a full suite per mutant. The first TUI-mutant run also used stale versioned
artifacts; its output remains in `mutations-first.log`. The final rerun uses
0.5.48 artifacts and pins the TUI authentication assertion directly.

## Harness corrections

The first run rejected an unsupported Bun stderr setting before runtime checks.
The second found a probe defect: resume/list did not establish read access to the
fresh thread. The corrected probe reads the known disposable thread directly
and renames it; the unprotected control proves both operations succeed before
crediting a refusal. Earlier outputs are retained under `tmp/runtime-isolation`.

## Still open

H.9.5 retains the **installed interactive co-exercise** of TUI, MCP tools and
pusher for every shipped runtime. Endpoint checks and fixture delivery do not
substitute for that gate. H.9.6 restart/sidecar recovery is separate. No live
launcher was restarted or attached to by these probes, and no live daemon was
changed. This protects accounts from one another, not from root or other
processes already running as the owning account.

A separate disposable native Codex TUI probe authenticated and opened a real
thread (`tmp/runtime-isolation/tui-probe.log`); no model call or MCP/pusher
co-exercise is credited to that probe.

OpenCode reviewed the source over the agent bus and returned no findings.
The remaining H.9.5 co-exercise boundary was explicitly retained.
