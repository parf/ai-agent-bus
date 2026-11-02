package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Source only the real shared assertions, never start a fixture or smoke run.
// The early match leaves more than a pipe buffer unread; quiet grep must not
// make a successful assertion depend on the producer's scheduling.
func TestGuideAssertionsHandleAnEarlyMatchInLargeOutput(t *testing.T) {
	path, err := filepath.Abs("../../acceptance/agent-guide/lib.sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	script := `set -uo pipefail
source "$1"
printf -v tail '%1048576s' ''
out=$'needle\n'"$tail"
has present "$out" needle >/dev/null
[ "$pass:$fail" = 1:0 ] || { echo "has early match: $pass passed/$fail failed"; exit 1; }
lacks present "$out" needle >/dev/null
[ "$pass:$fail" = 1:1 ] || { echo "lacks early match: $pass passed/$fail failed"; exit 1; }
has absent "$out" absent >/dev/null
[ "$pass:$fail" = 1:2 ] || { echo "has absent: $pass passed/$fail failed"; exit 1; }
lacks absent "$out" absent >/dev/null
[ "$pass:$fail" = 2:2 ] || { echo "lacks absent: $pass passed/$fail failed"; exit 1; }
`
	out, err := exec.CommandContext(ctx, "bash", "-c", script, "guide-assertions", path).CombinedOutput()
	if err != nil {
		t.Fatalf("large-output assertions: %v: %s", err, out)
	}
}
