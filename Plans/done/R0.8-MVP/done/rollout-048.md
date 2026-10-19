# Combined rollout to 0.5.48

📌 **TL;DR:** The live daemon moved from 0.5.44 to 0.5.48; confined web and administrative persistence are active, existing runtime sessions kept running.

## Measured

| Item | Result |
|---|---|
| Source | `aba9e9e`, pushed; full slow smoke 589/0, vet/race green |
| Build | Daemon and web: `parf@parf.us 2026-09-17 02:31:27` |
| Restart | One authorized `agent-busd.service` restart; service active afterwards |
| Public identity and sign-in | Version 0.5.48 and matching build rendered |
| Web confinement | Private PID namespace, zero effective capabilities, shared API socket mounted; daemon state, mapped owner socket and host home absent |
| Environment | Exactly `AGENT_BUS_ADDR=/bus.sock` and `PWD=/` |
| Authority | Before/after groups and record name/owner/ACL/Maintainers/Disabled/kind unchanged |
| Anonymous gates | `/status`, `/ls`, `/users`, `/recent` still 401 |
| Peer reconnect | OpenCode returned `OAB-048-RECONNECTED` from the same session without manual re-registration |

Artifacts: `tmp/rollout-48/`, including `verification-final.json` (13 checks,
0 failures). No live crash, pressure test, secret read or canary write was used.
The [disposable durability](administrative-durability.md#checks) and
[web isolation](web-isolation.md#checks) exercises provide the destructive evidence.

## Correction retained

The first new postflight assertion omitted bubblewrap's `PWD` and failed;
`verification.json` retains it. The installed web probe already allowed only
`PWD=/`. The corrected read-only probe pins that exact value, not merely the
key. A peer message sent before inspecting the failed result was corrected
immediately; no initial green result is claimed.

## Boundaries

The 0.5.45 binaries are installed, but setup was not rerun: the live daemon
account's shell configuration was not changed or credited as SSH acceptance.
Existing interactive runtime sidecars were not relaunched. The new Codex
capability boundary applies to subsequent launcher starts, not retroactively
to those existing processes. H.9.5's interactive co-exercise and H.9.6's recovery
gate remain open despite this successful bus-channel reconnect.

OpenCode reviewed .46, .47 and .48 source/evidence and returned no findings;
its same-session receipt is the reconnect observation above.
