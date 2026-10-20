# Review of 0.9.24 through 0.9.47

Historical review, 2026-10-19. Fixed range: `f46817bf..b432a094`
(`b432a094c190f5d2e34900c0930dd7e0b38be8c5`, version 0.9.47).
Source inspection and isolated local fixtures only; no live services, records,
credentials or repositories were changed. Older git caps, systemd scope and
cgroup behavior were included in the requested boundary sweep.

**Result: two reproduced P2 findings and two reproduced P3 findings; no P1.**
This report contains no production fixes. Locations refer to the fixed endpoint.
Fable's parallel security review is separate; this is not a claim to incorporate
findings it had not yet delivered.

## Findings

### P2 — A self-serving credential escapes the runner's stderr redaction

Locations: [itself.go](../../src/cmd/agent-bus-runner/itself.go):192–195,
[status_observations.go](../../src/cmd/agent-bus-runner/status_observations.go):28–29,
[unconfigured.go](../../src/cmd/agent-bus-runner/unconfigured.go):130–133.

The keeper appends its own `AGENT_BUS_TOKEN` to the child's environment, but
passes only the earlier `env` slice to `secretStderr`. That slice does not
contain the appended credential. If a self-serving agent includes its token in
an error diagnostic, the runner copies it to its journal and retains it in
`last_exit.reason`. Exit 78 additionally puts it in the unconfigured reason.
Someone admitted to the runner's status API can thereby obtain an agent
credential without access to its protected token file. This is runner-owned
status/journal handling, rather than an agent deliberately answering with a
secret. RD105's existing sentinel tests cover injected env/Secret values but
miss this separately injected credential.

**Reproduced:** a Go overlay added one test to the existing real-daemon
`configurationFixture`. Its self-serving shell printed `$AGENT_BUS_TOKEN` to
stderr and exited 78. `awaitConfigurationState(..., "unconfigured")` returned
a reason byte-for-byte equal to the temporary fixture credential read from
`r.tokenFile("#fixture@h")`. The assertion passed in 0.01 s, including a second run under `go test -race`
with no race report. No credential was
printed in test output. Source tracing shows the same unfiltered stderr writer
feeds the journal and the exit observation; the status disclosure was directly
observed, not merely inferred.

### P2 — Logwatch's line-count bounds leave memory and startup work unbounded

Locations: [watcher.py](../../src/agents/standalone/logwatch/watcher.py):52–59,
129–135, 189–191; [cleaning.py](../../src/agents/standalone/logwatch/cleaning.py):20–34.

A producer writing a watched file without newlines makes `partial` grow on every
poll. The 64 KiB read size limits each read, not the accumulated buffer. Once a
newline arrives, the whole string is cleaned and retained as one ring entry,
regardless of `ring_size`. At startup, looking backwards for ten newlines can
read an entire arbitrarily large file, repeatedly copying its accumulated
bytes. Journal iteration likewise has no per-line byte limit.

Thus a malformed log stream or one containing a very large request/payload can
exhaust the long-lived log reader's memory or stall startup. A small configured
ring does not prevent it. This finding concerns absence of any byte bound;
no actual host OOM was induced.

**Reproduced:** production `Monitor` and `Ring(1)`, one private watched file,
no journal. After the initial newline, append 1 MiB, 2 MiB and 4 MiB of `x`
without newlines, ticking once per 64 KiB. Observed `partial` sizes were
1,048,576; 3,145,728; and 7,340,032 bytes while ring capacity remained one.
Append a newline and tick: the sole retained entry's text was exactly
7,340,032 bytes. All file handles were closed and the fixture file removed.
The continuing growth and oversized retained entry were directly observed;
the eventual OOM and quadratic startup cost are consequences, not claimed
crash reproductions.

### P3 — Packaged help loses native Configuration scope and access-cap filtering

Locations: [run.go](../../src/cmd/agent-bus/run.go):531–540,
[help.go](../../src/internal/agentmeta/help.go):58–60, 110–113;
[git_agent.py](../../src/agents/standalone/git/git_agent.py):279–283;
[systemd.py](../../src/agents/standalone/systemd/systemd.py):147–151.

The shared starter now supplies `--package-dir`, so the normal help asks are
answered before the native handler runs. The generic renderer filters delivered
roles but deliberately never reads Configuration. Consequently an owner of a
git instance whose only repository is capped at `read` sees `push`, `commit`
and destructive methods in machine help and the web Methods card. Native body
`help` correctly omits those methods and explains the cap. A dedicated systemd
instance similarly loses the configured unit names from normal help, although
its body help displays them. The documented first command, `agent-bus help`,
therefore gives a different availability/scope account from `call ... help`.

**Reproduced:** load the shipped git manifest with production `agentmeta.Load`
and render `help-json` with delivered `owner`: `methods.push` is present.
With the same owner role and Configuration containing only an `ab` repository
capped at `read`, production `git_agent.answer("help")` reports `ab (read)` and
omits `push`. Production `systemd.describe()` with `units:"a.service,b.timer"`
shows both names; the generic renderer has no Configuration input from which
to show them. The existing CLI metadata interception test also passes, proving
that asks are answered without invoking the script. **This is misleading help,
not an execution authorization bypass:** actual operations still check caps
and unit membership.

### P3 — Restart parsing consumes runtime arguments after the separator

Location: [restart.ts](../../src/launchers/restart.ts):4–11.

`restartOption` filters every argument, including those after `--`. A caller
using the separator to pass a literal `--restart=10` to the runtime loses that
argument and unexpectedly enables launcher restart. A literal `--restart=0`
causes a launcher validation error instead of reaching the runtime. This also
differs from the adjacent `descriptionOption`, which stops parsing at `--`.
It affects all three launchers because they share this parser.

**Reproduced:** production parser under Bun:
`restartOption(["--", "--restart=10"])` returned
`{args:["--"],delay:10000}`; `restartOption(["--", "--restart=0"])`
threw the positive-timeout error. No runtime was launched.

## Coverage and verification

| Area | Inspection and independent checks |
|---|---|
| RD105 and the two secret stores | Traced metadata-only env reads, writes/errors, blank web editor, debug masking, streaming stderr filtering, status retention and keeper injection. Full runner Go unit suite passed; credential probe above found the missing injected value |
| RD104, git B.10 and systemd B.9 | Traced delivered owner/maintainer roles, no body authority, cap/tier intersection before git, scope before systemctl, strict paths/options, timer Configuration rereads and dirty-tree refusal. Python git 17/17 and systemd 22/22 passed |
| RT9 loader and all help forms | Inspected strict nested loading, role unions, shared arguments, text/JSON forms, secret declarations, ordering, defaults and hidden-count filtering. Agentmeta suite and bundled runner grammar tests passed; native Slack/Telegram metadata tests passed. Scope mismatch above remains |
| RT9.4 web/cache | Traced visitor-session/name/record-at cache key, 60 s lifetime, 256-entry bound, shared two-second send/answer budget, fallback, escaping and role-hidden details. Web metadata tests 9/9, 65 assertions |
| May-send | Inspected the exact shared `sendTarget` path, caller identity and no queue/counter mutation. Complete core and API Go suites passed |
| TG/Slack/shared router and onReply | Traced no-re matching, sender/topic/tag/expiry checks, state serialization, own-property lookups, sender admission, retry/dedup order and bounded retries. Telegram router, both native metadata sets, push and restart tests together: 26 passed, 309 assertions. No new reproduced routing defect |
| Logwatch | Inspected source/configuration boundaries, destination management, regex worker limits, queue/ring behavior and journal ingestion. Python suite 9/9. Byte-bound defect above reproduced separately |
| Debug/doctor/selftest and TLS | Inspected credential masking, bus-only doctor, tag normalization/discovery, bounded probes, thresholds/template grouping, DER/token pin comparison, openssl absence, and deploy verdict parsing. Debug real-bus and deploy tests passed as part of the runner suite |
| Restart and rename retry | Inspected foreground-only restart, clean exit, sidecar fatal path, preserved bindings/session, ownership recheck, Ctrl-C cleanup, and bounded rename-only handoff retry. Push tests 10/10; parser test passed but omitted the separator case above. No real SIGSEGV fixtures were run |
| Cgroup/package/mode reload | Inspected killed-generation recreation and two-second empty wait, both launch paths, digest-triggered keeper replacement, mode retirement/join before replacement, and package exports/layout/license/checksum checks. Cgroup and complete runner unit suites passed |
| RD103 and deadline follow-ups | Independently ran CLI role delivery, package asks, setup deadline, fast/slow receipts and stalled-ack checks: seven selected top-level tests passed. Runner's corresponding tests passed in its complete suite |
| Mutation-first, docs and simplicity | Compared named assertions with implementation, including hidden counts, masks, caps and restart conditions. Existing mutation evidence is historical evidence, not an independently repeated run of every mutant. Compared current help/agent guides with code; no additional justified simplification finding |

Executed Go suites: `go test ./internal/agentmeta ./internal/core ./internal/api
./internal/cgroup ./cmd/agent-bus-runner`, plus the selected CLI tests and two
isolated overlay probes. Python used `unittest discover` for logwatch, systemd
and git. Bun used the six named files above, with web tests run from `src/web`
so its JSX configuration applied.

Two initial harness invocations were corrected: the fresh worktree lacked the
existing TypeScript dependencies, and launching the web test from the repository
root selected the wrong JSX runtime. Reusing the main checkout's dependency
directory and running from the web directory made the unchanged suites pass.
Neither setup failure is counted as a product finding.

No new full smoke gate, privileged cgroup kernel fixture, fresh-install container,
live remote-platform delivery, or exhaustive mutation rerun was performed for
this report. Those limits matter: unit/source coverage is not a new release gate.

## Reproducing the secret probe

Append this test to an isolated copy of the existing runner
`unconfigured_test.go` and supply it with Go's `-overlay` option; its existing
imports and fixture provide everything else:

```go
func TestReviewSelfServingTokenInStatus(t *testing.T) {
    r, _, _ := configurationFixture(t, "itself",
        "printf '%s' \"$AGENT_BUS_TOKEN\" >&2; exit 78\n")
    st := awaitConfigurationState(t, r, "unconfigured")
    token, err := os.ReadFile(r.tokenFile("#fixture@h"))
    if err != nil { t.Fatal(err) }
    if st.Reason != strings.TrimSpace(string(token)) {
        t.Fatal("credential disclosure not reproduced")
    }
}
```

This assertion deliberately confirms the faulty observation; it is review
evidence, not a regression test accepting the disclosure as intended behavior.
