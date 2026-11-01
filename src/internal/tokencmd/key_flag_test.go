package tokencmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeyTakesItsFileOnlyAsKeyEquals(t *testing.T) {
	file := filepath.Join(t.TempDir(), "id")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	def := func(a []string) (string, bool, string, error) { return parse(a, true) }
	if n, r, k, err := def([]string{"alice@test", "--key=" + file, "--rotate"}); err != nil || n != "alice@test" || !r || k != file {
		t.Errorf("--key=<file>: %q %v %q %v", n, r, k, err)
	}
	if n, r, k, err := def([]string{"--key", "--rotate", "alice@test"}); err != nil || n != "alice@test" || !r || k == "--rotate" || k == "" {
		t.Errorf("--key --rotate <name>: %q %v %q %v", n, r, k, err)
	}
	if n, _, k, err := def([]string{"--key", "alice@test"}); err != nil || n != "alice@test" || k == "alice@test" || k == "" {
		t.Errorf("--key <name>: %q %q %v", n, k, err)
	}
	if _, _, _, err := def([]string{"--key", file}); err == nil || !strings.Contains(err.Error(), "--key=<file>") {
		t.Errorf("--key <existing file> = %v, want a refusal that says --key=<file>", err)
	}
	// A request that arrived over SSH never reaches a file check.
	if _, _, _, err := parse([]string{"--key", file}, false); err != nil {
		t.Errorf("an SSH request was answered by what exists here: %v", err)
	}
}

func TestAnEmptyKeyIsRefusedNotTreatedAsNoKey(t *testing.T) {
	for _, argv := range []bool{true, false} {
		if _, _, _, err := parse([]string{"alice@test", "--key="}, argv); err == nil || !strings.Contains(err.Error(), "--key=") {
			t.Errorf("argv=%v: --key= = %v, want a refusal", argv, err)
		}
	}
}
