#!/bin/bash
# go_failure_text keeps a failing test's lines and adds nothing, and for a run that failed
# before any test ran it prints go's own error, which the filter used to drop (J25).
set -eu
cd "$(dirname "$0")"
source ./smoke-gofail.sh
fail() { echo "FAIL $*" >&2; exit 1; }
setup=$'# github.com/parf/ai-agent-bus/cmd/x\ncmd/x/main.go:12:2: undefined: nothingHere\nok  \tgithub.com/parf/ai-agent-bus/internal/core\t0.01s\nFAIL\t./... [setup failed]\nFAIL'
got=$(printf '%s\n' "$setup" | go_failure_text)
grep -q 'go: cmd/x/main.go:12:2: undefined: nothingHere' <<<"$got" || fail "the build error of a setup failure is not printed: $got"
grep -q 'go: # github.com/parf/ai-agent-bus/cmd/x' <<<"$got" || fail "the failing package header is not printed: $got"
grep -q 'internal/core' <<<"$got" && fail "a passing package is printed: $got"
test=$'ok  \tgithub.com/parf/ai-agent-bus/internal/core\t0.01s\n--- FAIL: TestSomething (0.00s)\n    thing_test.go:41: got 1, want 2\nFAIL\nFAIL\tgithub.com/parf/ai-agent-bus/internal/x\t0.02s\nFAIL'
got=$(printf '%s\n' "$test" | go_failure_text)
grep -q -- '--- FAIL: TestSomething' <<<"$got" || fail "a failing test is not printed: $got"
grep -q 'thing_test.go:41: got 1, want 2' <<<"$got" || fail "its file:line is not printed: $got"
grep -q '    go: ' <<<"$got" && fail "a run that names its failing test also dumps go's lines: $got"
race=$'==================\nWARNING: DATA RACE\nWrite at 0x00c0001 by goroutine 8:\n==================\nFAIL\tgithub.com/parf/ai-agent-bus/internal/y\t1.0s'
got=$(printf '%s\n' "$race" | go_failure_text)
grep -q 'WARNING: DATA RACE' <<<"$got" || fail "a race report is not printed: $got"
echo 'go failure text: setup failures say why, test failures stay terse, races print whole'
