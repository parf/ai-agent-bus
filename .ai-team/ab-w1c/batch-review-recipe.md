# Reviewer reproduction notes

The original temporary drivers were removed after cleanup. These commands reconstruct the mutations; run in your own detached tree only, restoring each file from your saved bytes after the owning check. Retained original output: .ai-team/ab-w1c/batch-mutants.log and q1-proof.log. This recipe itself has not been rerun.

## Four mutations

- G3: in src/cmd/agent-bus-setup/user_runner.go, replace the unique `people, err := userRecords(ops.bus)\n\tif err != nil {\n\t\treturn st, err\n\t}` with `people, _ := userRecords(ops.bus)`. Run from src: `go test ./cmd/agent-bus-setup -run '^TestUserUninstallRefusesARegistryListingFailureBeforePlanningRemoval$' -count=1`.
- F4: replace `cmd.Env = scriptEnv.ChildEnv(os.Environ())` with `cmd.Env = os.Environ()` in src/cmd/agent-bus-runner/install.go (retains the helper import used elsewhere if any; otherwise retain it with a blank reference `_ = scriptEnv.ChildEnv`). Run from src: `go test ./cmd/agent-bus-runner -run '^TestBundledPackageBuildNeverReceivesTheRunnerToken$' -count=1`.
- L2: replace `except (Invalid, UnicodeError): cfg, secret = None, b''` with `except (BusError, Invalid, UnicodeError): cfg, secret = None, b''` in src/agents/webhook/webhook.py. Run from src/agents/webhook: `python3 -m unittest test_webhook.Startup.test_transient_configuration_or_secret_failure_exits_for_runner_restart`.
- State-root sweep: remove ONLY `sweepStale(rootFd)` from src/cmd/agent-bus-setup/person_files.go (keep the users sweep). Run from src: `go test ./cmd/agent-bus-setup -run '^TestPersonFilesSweepsStateRootStagingLeftBeforeUsersWasCreated$' -count=1`.

## Q1 actual Containerfile fixture

Use a private mktemp directory under repo tmp/, with cleanup. Create its release/ directory. The following Python creates the synthetic archive (pass release directory as argv1):

```python
import hashlib, io, pathlib, sys, tarfile
out = pathlib.Path(sys.argv[1]); out.mkdir(parents=True, exist_ok=True)
files = {'internal/version/VERSION': b'0.0.0\n', 'probe': b'ownership probe\n'}
for n in ('agent-bus','agent-busd','agent-bus-admin','agent-bus-runner','agent-bus-token','agent-bus-ts'):
    files[n] = b'#!/bin/sh\nexit 0\n'
files['MANIFEST.sha256'] = ''.join(hashlib.sha256(v).hexdigest()+'  '+k+'\n' for k,v in files.items()).encode()
archive = out/'agent-bus-0.0.0-linux-amd64.tar.gz'
with tarfile.open(archive, 'w:gz') as t:
    for n,v in files.items():
        e=tarfile.TarInfo('agent-bus-0.0.0/'+n); e.size=len(v); e.uid=1234; e.gid=2345; e.mode=0o777
        t.addfile(e, io.BytesIO(v))
(archive.parent/(archive.name+'.sha256')).write_text(hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name+'\n')
```

Build the ACTUAL src/container/Containerfile with its existing context:
`podman build --build-context release=<private release dir> -t <own unique image tag> src/container`
Then run the committed runtime assertion:
`podman run --rm -i --entrypoint /bin/sh <own unique image tag> -s < src/container/check-release-tree.sh`
Fixed builds/check green. Pre-fix and mutant both remove precisely `--no-same-owner --no-same-permissions ` from Containerfile: builds must succeed, runtime check must exit1 with `FAIL release tree keeps an archive owner or group/other write permission`. Restore fixed bytes afterward. Remove own containers and image tags in finally/trap; do not purge shared cache. This is extraction proof, not daemon-role acceptance.
