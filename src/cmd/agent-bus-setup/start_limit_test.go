package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J59: a daemon the unit refuses to start because it was started too often
// is a failed setup that says why and for how long, not a bare exit status.
func TestSetupSaysWhenTheDaemonIsInItsStartLimit(t *testing.T) {
	dir := t.TempDir()
	fake := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  show) printf 'Result=%s\\nStartLimitIntervalUSec=%s\\n' \"$SHOW_RESULT\" \"$SHOW_INTERVAL\" ;;\n" +
		"  *) echo 'Job for agent-busd.service failed because start of the service was attempted too often.' >&2; exit 1 ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	choice := &tlsChoice{}
	for _, tc := range []struct{ result, interval, want string }{
		{"start-limit-hit", "10s", "start-limit-hit, wait 10s and retry"},
		{"start-limit-hit", "1min 30s", "start-limit-hit, wait 90s and retry"},
		{"start-limit-hit", "", "start-limit-hit, wait for its start-limit window and retry"},
	} {
		t.Setenv("SHOW_RESULT", tc.result)
		t.Setenv("SHOW_INTERVAL", tc.interval)
		err := choice.startDaemon(false, false, "127.0.0.1:1", "v", "b")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("interval %q: %v, want it to say %q", tc.interval, err, tc.want)
		}
	}
	// Any other failure is not called a start limit.
	t.Setenv("SHOW_RESULT", "exit-code")
	t.Setenv("SHOW_INTERVAL", "10s")
	if err := choice.startDaemon(false, false, "127.0.0.1:1", "v", "b"); err == nil || strings.Contains(err.Error(), "start-limit") {
		t.Fatalf("an ordinary failure was called a start limit, or hidden: %v", err)
	}
}
