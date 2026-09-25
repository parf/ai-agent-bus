# Installed acceptance

## Inspection

Historical observation on `parf.us`, 2026-09-13, from a read-only inspection of
the active `agent-busd.service`. This is the development host, not a clean
installation. The installed binary reported `0.5.6`; the checkout reported
`0.5.7`. These observations did not certify installation of that checkout.
The missing account, socket and capability evidence is now complete on a
[packaged disposable host](done/installed-shared-host.md#checks); the table is
retained as the earlier observation, not current status.

| Check | Observed result | Remaining evidence |
|---|---|---|
| Account and state | Supervisor, bus and web ran as `agent-busd`; account home and unit working directory agreed. State directory was owned by that account with mode `0700` | Completed by the packaged-host checks |
| Local socket | Runtime directory mode `0711`; mapped `parf` socket owned by `parf`, mode `0600`; a status call authenticated as `parf@parf` | Completed with two actual accounts, cross-account refusals and mutations |
| Capabilities | Supervisor permitted, effective and ambient sets contained only `CAP_CHOWN`; bus and web had zero permitted, effective and ambient capabilities. All three had `NoNewPrivs=1` | Completed with removal and child-inheritance mutations |

The children's inheritable and bounding sets still contained `CAP_CHOWN`.
The observation is about their permitted, effective and ambient sets, not a
claim that every capability set was empty.

## Resource confinement

The 2026-09-13 inspection found the supervisor, bus and web in one unlimited
cgroup. That historical gap is closed by [G.1.2's generated-unit pressure and
mutation evidence](done/web-resources.md#checks): web and its wrapper now have
their own delegated limits while the bus remains outside and keeps answering.

The destructive pressure exercise uses disposable state, sockets and
credentials. The production unit receives read-only postflight checks after
deployment; no resource-exhaustion experiment runs against live state.
The [0.5.49 postflight](done/web-resources.md#live-postflight) verified the
generated delegation, separate web group and exact limits on the active unit.

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
