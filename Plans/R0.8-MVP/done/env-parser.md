# Shared ENV parser

Completed 2026-10-17 in 0.8.154, commit `a0c64ef` (K.39).
Current contract: [env-file syntax](../../../docs/03-records-service.md#env-file-syntax).

## Requirement

One parser for the [env-file syntax](../../../docs/03-records-service.md#env-file-syntax), the official dotenv format with nothing evaluated (Q171–Q173), used by the daemon's check on a secret, by the runner for `env.dist`, `env` and the bus secret, and by the web face's validation through the daemon; a TypeScript parser (the Slack agent's) passes the same table of cases; the dev node's stored secrets and runner env files audited using that parser in the stopped-home backup window BEFORE deploying or restarting the strict build; any that do not fit fixed by hand — no legacy form

## Acceptance

dotenv's own test file, its `.env` and the values its tests expect, is the shared table, run through the Go parser, the daemon's save and the runner's delivery, and the Slack agent's parser; a line dotenv cannot read is refused, by line number and never echoed; `"a\nb"` becomes a newline; `${X}` and `$(…)` arrive literally. Mutants: decode `\n` in single quotes, expand `${X}`, or give the save path its own parser, and a check must fail

## Checks

| Check | Evidence |
|---|---|
| Common grammar | Upstream dotenv `.env` and expected values from `test-parse.js`, plus boundary cases in `src/internal/envfile/testdata/common.json`; Go `TestENVConformanceCases`, core `TestSecretValidationUsesENVConformance` and `TestSecretWritesAndENVConformanceAgree`, runner `TestRunnerDeliveryUsesENVConformance`, Slack shared-table tests |
| Setup and systemd | `TestRunnerTokenFileIsTheSharedParsersMachineWrittenSubset`: setup reads with the shared parser and keeps its machine-written hex-token file canonical |
| Named failures | Compiling Go mutants: decode escapes in single quotes, expand `${X}`, stop decoding double-quoted newlines, echo private input, use the old independent save parser, remove NUL guard — each fails its assertion. TS mutants: single-quote escapes, expansion, skip invalid lines, remove NUL guard — each fails its assertion |
| Reference comparison | 216 generated forms compared against dotenv's regex; 204 accepted by the bus, zero value mismatches; unreadable lines are deliberately refused |
| Full gate | `src/smoke.sh --slow`: 1017 passed, 0 failed, frozen 0.8.154 tree; local log `tmp/env-final-slow-2.log` |
| Audit witness | A disposable database with two nonempty secrets and one env file reports the two invalid inputs by identity and line, without values; disabling the parser-error guard compiles and fails the audit assertion |

## Development node

Before the strict daemon first started, stopped it, took a root-only complete
home backup, and ran the audit against the shared parser: **83 raw records,
0 nonempty secrets, 1 runner env file, 0 invalid**. No private values printed
and no stored values needed repair. Backup:
`/var/lib/agent-bus/daemon.backup-0.8.153-20261017-k39-151713.tar` (0600).

The first restart exposed a stale `addr.conf` systemd override carrying the
removed shared runner account's `-user` mapping. Backed up that override at
`/var/lib/agent-bus/addr.conf.before-k39` (0600), removed only the obsolete
mapping and reloaded systemd. No registry or token changes were required.

Stamped build reports 0.8.154 and `a0c64ef`. Daemon, web face and
`agent-bus-runner@parf` are active/running with no restart loop; web returns
HTTP 200, runner status answers at 0.8.154, and `#debug@parf.us` acknowledges
and answers `hello ENV parser`.
