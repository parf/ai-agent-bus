# Full Darwin archive acceptance

📌 **TL;DR:** MAC.2b supplies the full Darwin archive for the reduced launchd setup. The CEO runs this fresh-fixture check on the owner's Mac; workers have no Mac access. MAC.2 remains pending until the archive install, request/reply, restart and cleanup all pass.

## What this proves

- The delivered archive installs through its own `agent-bus-setup`, without `--exec` or a locally rebuilt binary.
- The [MAC.2 acceptance](macos.md#mac2-daemon-with-a-reduced-setup) runs against that installed daemon, including private storage and registry persistence.
- The fixture is removed with `sysadminctl`, with the group removed only if it remains. Named leftovers fail the run.

## Prepare the owner session

These are **unrun owner steps**, not native acceptance evidence. Use the existing `~/agent-bus-dev/ai-agent-bus` checkout, clean on `main`, at the landed MAC.2c revision, including its archive-revision check. No extra clone, package installation, profile or system setting change is needed. Python 3 and the existing native command-line tools are required; no Go or Bun build runs on the Mac.

1. On the build node, the CEO builds from the exact landed revision: `BUILD_COMMIT="$(git rev-parse HEAD)" GOOS=darwin GOARCH=arm64 src/package.sh tmp/mac2b-dist` for Apple Silicon, or set `GOARCH=amd64` in the same command for Intel. The full 40-character commit is stamped into both Go and TypeScript `build_info`; the harness requires it to match the release-source revision on the Mac. Retain the full archive and its `.sha256` beside it. The client archive is not this test's input.
2. Transfer the native full archive and `.sha256` to `~/agent-bus-dev/mac2b-input/` on the Mac. Transfer missing Git objects through the owner's existing process; do not publish or push a release for this check.
3. In the Mac console, authorize the bounded fixture with `sudo -v`. Run the harness as the installing user, never as root. The harness uses `sudo -n` for each printed privileged invocation. Refusal for existing state is a stop, not permission to delete it.

## Run the archive check

From the Mac checkout, use this shell block. It selects exactly one full archive matching this checkout's version and native CPU, records the revision, preserves the Python exit status and retains the log under the workspace.

```sh
cd "$HOME/agent-bus-dev/ai-agent-bus"
mkdir -p "$HOME/agent-bus-dev/mac2b-evidence"
python3 - <<'PY'
from pathlib import Path
import platform
import subprocess
import sys
import time
repo = Path.cwd()
base = repo.parent
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip()
version = (repo / 'src/internal/version/VERSION').read_text().strip()
cpu = {'arm64': 'aarch64', 'x86_64': 'x86_64'}[platform.machine()]
archive = base / 'mac2b-input' / ('agent-bus-' + version + '-darwin-' + cpu + '.tar.gz')
assert archive.is_file() and Path(str(archive) + '.sha256').is_file()
log = base / 'mac2b-evidence/result.log'
started = time.monotonic()
with log.open('w') as output:
    output.write('revision=' + revision + '\nCPU=' + platform.machine() + '\n')
    output.flush()
    result = subprocess.run(['python3', '-B', 'src/acceptance/mac2-install-check.py',
                             revision, '--archive', str(archive)], stdout=output, stderr=subprocess.STDOUT)
    output.write('rc=%s seconds=%.3f\n' % (result.returncode, time.monotonic() - started))
print(log.read_text())
sys.exit(result.returncode)
PY
```

## Expected checks and cleanup

The [harness](../../src/acceptance/mac2-install-check.py) refuses existing `_agentbusd` user or group, launchd job, storage, sockets, release root, command links or old web link. It verifies the archive hash with Python, every manifest entry (the review-settings example is the existing non-runtime exception), the packed version, executable bits and native version/build answers before setup, including every program’s stamped commit against the release-source revision. GNU `sha256sum` is not required on the Mac.

| Check | Required result |
|---|---|
| Archive install | Reduced setup runs from the extracted full archive without `--exec`; installed daemon bytes match, eleven command links point into the fresh release, and the token wrapper answers its version |
| Role and storage | launchd daemon is `_agentbusd`, never root; hidden non-login account; `0700` storage unreadable by the installing user |
| Request and reply | One pinned-TLS request consumed once; `pong` comes from the registered fixture agent |
| Restart | `launchctl kickstart -k` changes the supervisor PID; the registry record remains |
| Abrupt supervisor exit | Its recorded child exits, a replacement returns and the registry remains |
| Teardown | Job, plist, storage, sockets, fresh release root and exact created links removed; account and group verified absent |
| Workspace | Clean `main` restored; throwaway key/extraction/temp data removed; `du -sh ~/agent-bus-dev` and cache absence reported |

Cleanup runs in `finally` after failed assertions too. It bootouts the fixture job, kills only `_agentbusd` processes, removes only preflight-absent fixture paths, then runs `sudo -n /usr/sbin/sysadminctl -deleteUser _agentbusd`. If the group remains, it runs `sudo -n /usr/sbin/dseditgroup -o delete _agentbusd`; a group already gone is success. Directory lookup failure is not absence. A remaining role raises a named error, returns nonzero and points to the [manual recovery](../../docs/07-setup.md#uninstall-the-macos-test-daemon), including Full Disk Access only for persistent permission denial.

## Return evidence

Send the full log, actual revision/version, CPU, duration, exit status and final `du -sh` to `#ab-lead1` (topic `MAC`, tag `ab-dvp`). Do not include tokens or private keys. Preserve the log until its complete output is filed in `done/`; an excerpt does not establish missing checks. After that, remove only the transferred `mac2b-input` artifacts and `mac2b-evidence` directory and report the final workspace size. A failure leaves MAC.2 pending; do not substitute the older `--exec` run.
