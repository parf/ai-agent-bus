# Installed acceptance

## Inspection

Historical observation on `parf.us`, 2026-09-13, from a read-only inspection of
the active `agent-busd.service`. This is the development host, not a clean
installation. The installed binary reported `0.5.6`; the checkout reported
`0.5.7`. These observations do not certify installation of the checkout.
The [active gates](TODO.md#installed-stage-gate) retain acceptance ownership.

| Check | Observed result | Remaining evidence |
|---|---|---|
| Account and state | Supervisor, bus and web ran as `agent-busd`; account home and unit working directory agreed. State directory was owned by that account with mode `0700` | Wrong-account mutation; fresh-host exercise |
| Local socket | Runtime directory mode `0711`; mapped `parf` socket owned by `parf`, mode `0600`; a status call authenticated as `parf@parf` | Two actual user accounts, positive calls and cross-account refusals; ownership/mode mutations |
| Capabilities | Supervisor permitted, effective and ambient sets contained only `CAP_CHOWN`; bus and web had zero permitted, effective and ambient capabilities. All three had `NoNewPrivs=1` | Installed mutations that remove the supervisor capability or give it to a child |

The children's inheritable and bounding sets still contained `CAP_CHOWN`.
The observation is about their permitted, effective and ambient sets, not a
claim that every capability set was empty.

## Resource confinement

The supervisor, bus and web shared the unit's cgroup. Its memory limit was
`max` and CPU quota was `max 100000`; the web process also had unlimited CPU
time and address space limits. No separate web cgroup was present. Inspection
of the current setup unit generator and child launcher found no web-specific
resource limit.

[G.1.2](TODO.md#remaining-work) therefore needs implementation before its
load and mutation acceptance can run. Applying a shared unit limit alone
would not establish confinement of the web child while the bus continues.
No resource-exhaustion experiment was run against the active installation.

## Reproducing the inspection

Obtain the current supervisor PID with
`systemctl show agent-busd.service -p MainPID --value`; discover its direct
children with `ps -o pid,ppid,user,group,comm --ppid <pid>`.

| Evidence | Read-only source |
|---|---|
| Unit account, paths and capability configuration | `systemctl show agent-busd.service` |
| Actual identities, capabilities and privilege flag | `/proc/<pid>/status` for the supervisor and both children |
| Account home and path permissions | `getent passwd agent-busd`; `stat` the state directory, runtime directory and mapped socket |
| Cgroup membership and limits | `/proc/<pid>/cgroup`; the corresponding cgroup's `memory.max` and `cpu.max`; web `/proc/<pid>/limits` |
| Authenticated positive control | `curl --max-time 5 --unix-socket /run/agent-bus/user-parf.sock http://unix/status` as `parf` |
| Installed build | `agent-busd --version` |

Repeat these reads after installation changes; process IDs and the observations
above are snapshots, not configuration defaults.

## Checkout verification

On the same date, `src/smoke.sh --slow` passed 529 checks with zero failures,
including Go vet/race tests and the launcher harness. The run used isolated
scratch under `tmp/dvp-check/` and port base `18761`; its log is
`tmp/dvp-check/smoke.log`. This validates the development checkout, not the
remaining live-runtime or installed stage gates.
