# J78 reload-safe runner settings

📌 **TL;DR:** Reload and inbox polling no longer race on the shared-reader
setting. Delayed JSONL retirement and shutdown also capture their settings
before leaving the lock; consumes never hold the settings lock.

## Change and sweep

The implementation commit `d9914199` lists every replaced field and reader.

| Writer | Fields replaced |
|---|---|
| Existing one-message or kept entry in `serve` | `in`, `env`, `secretEnv`, `box`, `net`, `jail`, `cmd` |
| Existing JSONL entry in `serve` | `in`, `jail` |

Both writes hold `r.mu` then the entry's `settingsMu`. The polling reader
copies `Host.Share` under `settingsMu` and releases it before consume.
It cannot take `r.mu`: stop holds that mutex while joining `readDone`.

| Reader or goroutine | Settings protection |
|---|---|
| Inbox read | New per-poll entry snapshot |
| JSONL idle-retirement delayed kill | Deadline captured together with its jail through `launchSettings`, before the pool lock and delayed goroutine |
| Stop retirement callback | Deadline captured before scheduling retirement |
| Shutdown goroutines | Pool, deadline and jail captured under `r.mu`, before scheduling |
| `runOne`; keeper command, deadline and socket watcher; JSONL start and kill | Existing `launchSettings` snapshot under `r.mu` |
| Synchronous `keep` and `startStream` setup | Adoption's `r.mu`, before reload can update the entry |
| Status, observed status, readiness, exit/configuration callbacks, serve/preflight and jail comparison | Existing `r.mu` |
| Signal and socket-watch helpers | Immutable actual per-process jail values |

Name, work, client and code directory are fixed at adoption. This change
adds no lock around blocking network calls or child waits.

## Proof

The E1b gate log `tmp/gate-logs/20261101-154113-ab-w1b_E1b.log:780–845`
reports the original writer at `serve.go:404` and inbox reader at `:750`,
inside `TestSharedAgentWinsAndRemovalFallsBackToBundle`. On main code, the
new `TestReloadWhileInboxPollsSnapshotsSharedSettings` reproduces that same
race in 0.309 s. Its reload token, owner and configuration checks use a real
disposable API; only consume is an immediate fixture response, keeping polls
active while 128 real reloads update the same served entry.

Four compiling mutants ran only their owning test under a separately
acquired and confirmed slow lock:

| Mutant | Red test | Failure |
|---|---|---|
| Read `Host.Share` without the poll snapshot | `TestReloadWhileInboxPollsSnapshotsSharedSettings` | `WARNING: DATA RACE`, reload writer vs inbox reader (0.284 s) |
| Hold entry lock across consume | `TestReloadDoesNotHoldSettingsLockAcrossBlockedConsume` | `reload waited for the blocking consume's settings lock` (1.026 s); cancellation releases it, no hang |
| Read deadline inside delayed idle-kill goroutine | `TestJsonlIdleRetirementSnapshotsDeadlineDuringSettingsReplacement` | `WARNING: DATA RACE`, deadline replacement vs delayed read (0.374 s) |
| Restore shutdown's post-unlock deadline/jail reads | `TestStopKeptSnapshotsSettingsBeforeStartingShutdownGoroutines` | `WARNING: DATA RACE` for both fields (0.048 s) |

All sources restored. Four focused race checks pass (1.635 s); the lock was
released. Every new wait is bounded and fixture readers/children are joined.

The full runner race suite passes (`go test -race ./cmd/agent-bus-runner
-count=1 -timeout 90s`, 33.226 s), including the original shared/bundled
regression. Vet, Darwin arm64 cross-build, link and diff checks pass. This is
a shipped behaviour fix; the gate applies its bump. No host deployment ran.
