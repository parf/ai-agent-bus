# Web authority isolation

📌 **TL;DR:** G.1.3 confines the supervised web child; installed probes use disposable state, not production canaries.

## Scope

The [web authority boundary](../../../docs/11-processes.md#web-authority-boundary)
is implemented in the supervisor using bubblewrap. The existing service account
and generated systemd policy remain; the web receives individual inputs rather
than access to that account's filesystem. `src/build.sh` produces a static web
binary with the same stamp as the other programs.

This adds a host dependency and requires unprivileged user namespaces. Current
upstream [bubblewrap documentation](https://github.com/containers/bubblewrap#user-namespaces)
no longer offers the historical setuid fallback. Missing support refuses the
web launch; it never selects the old unconfined path.

## Checks

`src/acceptance/web-isolation.py` creates a disposable **real systemd unit**
from `agent-bus-setup --print-unit`. It replaces installation paths and ports,
enables the web child, and injects inherited canary environment values. The
account, capability, NoNewPrivileges, filesystem and private-temp policy comes
from the generated unit. No PID1 stub is involved.

| Check | Evidence boundary |
|---|---|
| Credentials, snapshot and SSH authorization unreadable and unmodifiable | Static adversarial replacement web binary, launched by the actual supervisor; real read and append attempts against disposable files |
| Negative controls have real targets | Same account outside the sandbox can read/write the files and reach the mapped socket as the daemon owner |
| Mapped account socket inaccessible | Connection refused in the sandbox; the shared socket remains usable |
| Host process filesystem inaccessible | Host bus `/proc/<pid>/root` read denied; separate host-process visibility assertion pins private proc itself |
| No inherited credential or arbitrary environment | Probe checks every environment key and rejects inherited canary values; package check covers the wrapper before bubblewrap starts |
| Visitor authority preserved | An ordinary visitor's token works through the shared socket and reports that visitor; anonymous and invalid-token requests are refused |
| Actual renderer works | Same unit launches the real web binary; sign-in page and visitor session succeed, then HTTPS succeeds with individually bound TLS files |
| Fail closed | Missing requested TLS key cannot serve HTTP; missing bubblewrap cannot start an unconfined web child and names the missing prerequisite |

Recorded exploratory runs are under `tmp/web-isolation/`. The first complete
installed acceptance passed, followed by the refined process-visibility check.
Final installed acceptance: 23 checks, zero failures in
`tmp/web-isolation/installed-final.log`; exit 0. Frozen `--slow`: **589 passed, 0 failed**, vet and race green
(`tmp/web-isolation/slow-final.log:931`, checks at lines 10–11). All source
hashes in `source-frozen.json` remained unchanged through the run. Tracked
`src/smoke.sh` and its frozen copy shared SHA-256
`eea052ffb0de77dfa6c7d27f0d12dfd6d741d0b912cd872c233108f43b4ee440`.

## Mutations

Ten installed mutations build isolated Go overlays and run fresh disposable
units (`mutations-final.log`); all fail the named check. One additional overlay
runs the wrapper-environment package test (`wrapper-env/test.log`). The complete
smoke suite is separate, not repeated per mutation.

| Change | Named failure |
|---|---|
| Expose token read-only / read-write, separately | Credential read / modification |
| Expose snapshot read-only / read-write, separately | Snapshot read / modification |
| Expose SSH authorization read-only / read-write, separately | SSH authorization read / modification |
| Bind mapped account socket into the namespace | Mapped socket access |
| Bind host proc instead of private proc | Host-process visibility |
| Add an inherited-canary value to the explicit environment | Environment restriction |
| Remove the shared socket bind | Ordinary visitor API access |
| Restore wrapper environment inheritance (package test) | `TestWebWrapperInheritsNoEnvironment` |

The first host-proc mutation **survived** the host-root-file assertion:
another layer still denied dereferencing that process's root. The original log
is retained (`mutations-first.log`); the added visibility assertion catches
exposure of host processes without claiming that exposure alone defeats every
credential-read protection. File write mutations target the unit's explicit
`ReadWritePaths`, so the outer read-only filesystem cannot mask them.

## Limits and live inspection

The live 0.5.44 unit was inspected read-only before implementation deployment.
Permission checks inside its web mount namespace, as `agent-busd`, found the
daemon token, snapshot, SSH authorization and mapped daemon-account socket
readable/writable (`tmp/web-isolation/live-before.txt`). No file content was
printed and no production write probe or socket authentication was attempted.

Acceptance uses the installed host's kernel and existing service accounts,
with isolated state and runtime directories. It is not fresh-host packaging
acceptance, does not prove every distribution permits user namespaces, and
does not close the broader installed capability gate or G.1.2 resource limits.
Host networking remains available; this is not an outbound network sandbox.
TLS files are explicit operator inputs. A standalone web invocation does not
receive supervisor confinement.
