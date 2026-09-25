# Installed shared-host boundary

📌 **TL;DR:** A packaged fresh host gives each mapped OS account only its own
socket, while `CAP_CHOWN` stays with the supervisor and state stays with the
service account.

## Scope

The existing package-only acceptance now continues past installation on the
same disposable real-systemd host. It creates two actual local accounts through
the installed administration path, applies their durable account mappings with
a full supervisor restart, and uses only installed commands afterward.

This closes the installed account/state, per-user-socket and capability gates.
It does not accept runtime launchers, interactive endpoint isolation, browser
journeys or production-host mutation.

## Checks

The final host is Arch Linux with systemd 260.1, unified cgroup v2 and no
runtime network. The archive, fixture and evidence directory are its only host
mounts; the checkout and `/rd` are absent.

| Boundary | Measured result |
|---|---|
| Account and state | Supervisor, bus and web run as `agent-busd`; daemon state is owned by that account with mode `0700`; changing the generated unit to root prevents the service-account setup journey from completing |
| Positive identity | Actual accounts `alice` and `bob` discover their installed sockets without environment configuration and `/status` reports `alice@fresh` and `bob@fresh` respectively |
| Socket isolation | Runtime directory is `0711`; each account socket is owned by that account with mode `0600`; both cross-account attempts fail; the shared socket without a token also fails |
| Capability placement | Supervisor permitted, effective and ambient sets contain only `CAP_CHOWN`; bus and web have zero permitted, effective and ambient capabilities; all three have `NoNewPrivs=1` |
| Claim limit | The bus inheritable and bounding sets still contain `CAP_CHOWN`; they are recorded, not claimed empty. The web drops those sets inside its separate confinement |

The fresh-host journey passes thirteen named groups, including the existing
package-integrity, dashboard, service-call and idempotent-reinstall controls.
Evidence is under `tmp/installed-shared-host-20260918a/baseline-final/`.

## Mutations

Five isolated source overlays each build a complete package and run on a new
real-systemd host:

| Change | Named failure |
|---|---|
| Run the generated unit as root | Service-account setup cannot establish the expected installed unit |
| Remove the supervisor ambient `CAP_CHOWN` grant | Supervisor capability readback differs before account mapping is credited |
| Make ambient clearing a no-op | Bus permitted-capability readback differs before web readiness can mask it |
| Widen account sockets from `0600` to `0666` | Both intended users succeed first, then Alice reaches Bob's credential-bearing socket |
| Chown account sockets to the daemon account | The intended account cannot use its own mapped socket |

Final result: **5/5 caught**. Replay and per-mutant logs are under
`tmp/installed-shared-host-mutations-a/`; the driver is
`tmp/installed-shared-host-mutations-a.py`.

Early mutation attempts receive no credit. Two copied-source attempts omitted
the ignored MCP build dependencies and failed during packaging. A later child-
capability run reached dashboard failure before the direct bus-capability
assertion; the final fixture checks supervisor and bus immediately after API
readiness. One partial rerun then used the old expected diagnostic after that
ordering correction. The final run starts clean and attributes all five
failures to their named controls.

## Repository verification

The acceptance change adds no product behavior, schema or version bump. The
frozen slow smoke passes **609/0**, including vet and race. All **220** tracked
`src/` hashes remain unchanged through the run. The tracked smoke script and
the accepted fresh-host fixture retain SHA-256
`5af2a0bd6b940be7c842244215d7205f5c24037c7d6049d261fc02986ad8d1ac`
and
`13f713770af77534840c33a08431d5339d44dc48811e3912d1a40f69180aad7b`
respectively. The documentation checker reads 165 Markdown files and 2,873
local links with zero errors. Its first invocation used a copied script at the
wrong directory depth and found no repository; the corrected invocation is the
one credited.
