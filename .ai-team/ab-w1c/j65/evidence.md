# J65 gate landing audit

Candidate: `.ai-team/gate.sh.new`; live `.ai-team/gate.sh` remains byte-identical to `.ai-team/ab-w1c/j65/gate.before.sh`.

Only script change: after successful ff-only merge, append `LANDED range=<old-full-main>..<new-full-main> branch=<branch> topic=<T-n-or-envelope-topic>` to that attempt's existing log. Red/refused paths never reach it. Both endpoints are the exact pre-merge main and actual rebased/amended tip, so a multi-commit landing is recoverable.

Run `python3 .ai-team/ab-w1c/j65/test_gate_audit.py`: four checks pass (0.309 s) using real Git in throwaway repositories, stub true/false smoke, GATE_NOLOCK=1 (no real smoke, no live merge or bus call). A two-commit landing has exactly the expected audit row; envelope topic is retained when no T-n is provided; red and missing-branch refusals have no landing rows and preserve main. All test repositories are cleaned automatically.

The exact landing check fails on the original script (one AssertionError, 0.083 s) and on an isolated candidate mutant omitting the audit printf (one AssertionError, 0.090 s); no load/build error. Restored candidate four checks pass. Mutants held smoke-gate slow, released immediately. Logs: before.log, mutant.log, restored.log in this directory.

## CEO switch-over

1. Wait for the live gate to finish and be idle; no active gate process may read the old file during replacement.
2. After the one-reader diff/test review, atomically install the already executable candidate: `mv -f .ai-team/gate.sh.new .ai-team/gate.sh`. Only CEO performs this node-local switch; worker has not installed it.
3. The next genuine authorized landing must contain exactly one `LANDED range=... branch=... topic=...` row in its gate log. Compare both full commit IDs with that landing's old/new main and branch/topic; retain that log as live proof. Do not fabricate a landing just for the check.
4. Tracked docs-only branch `ab-w1c/J65` separately adds the audit statement to the gate-role row; no bump.

SHA256 candidate: c6c705355e5550b5534eacd2d5a8256e43333fa711060dd508fb6ef3d6c1f29a
SHA256 live/before: 9030876770616bc62044ea2b35eb35e0dca1838a719c90f37b5da22f3b24953e

## Live switch-over proof

CEO installed the candidate while idle. The first real landing on the swapped gate is the docs commit `d3f306919f37e00b7048b05c24b4ab555107862d`, whose parent is `27bb1c7212d4fdb95f5be5553fc957b3571a9179`. `tmp/gate-logs/20261030-220415-ab-w1c_J65.log:1047` contains exactly one landing audit row:

```text
LANDED range=27bb1c7212d4fdb95f5be5553fc957b3571a9179..d3f306919f37e00b7048b05c24b4ab555107862d branch=ab-w1c/J65 topic=J65
```

Both full IDs match Git; branch and envelope topic match the J65 request. Verified by W1C after landing. Temporary test scripts, snapshots and logs were removed after recording this proof; this evidence file is retained.
