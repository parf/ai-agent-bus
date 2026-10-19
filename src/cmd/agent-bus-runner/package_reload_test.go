package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReloadRestartsRepackagedKeeperOnlyWhenFilesChange(t *testing.T) {
	body := func(version string) []byte {
		return []byte("printf '" + version + "\\n' >> \"$AGENT_BUS_WORK/package-starts\"; while :; do sleep .02; done")
	}
	r, _, script := configurationFixture(t, "itself", string(body("old")))
	r.mu.Lock()
	s := r.served["#fixture@h"]
	original := s.kept
	path := filepath.Join(s.work, "package-starts")
	r.mu.Unlock()
	await := func(want string) {
		t.Helper()
		end := time.Now().Add(3 * time.Second)
		for {
			raw, _ := os.ReadFile(path)
			if string(raw) == want {
				return
			}
			if time.Now().After(end) {
				t.Fatalf("package starts %q, want %q", raw, want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	await("old\n")
	r.reload()
	r.mu.Lock()
	same := s.kept == original
	r.mu.Unlock()
	if !same {
		t.Fatal("unchanged package restarted its keeper")
	}
	// The package publisher replaces the directory while retaining agent.json
	// and the command line. Only the code's bytes change, with the same size.
	dir := filepath.Dir(script)
	replacement := dir + ".new"
	if err := os.Mkdir(replacement, 0700); err != nil {
		t.Fatal(err)
	}
	meta, err := os.ReadFile(filepath.Join(dir, "agent.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, "agent.json"), meta, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, filepath.Base(script)), body("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, dir); err != nil {
		t.Fatal(err)
	}
	r.reload()
	await("old\nnew\n")
	r.mu.Lock()
	current := s.kept
	r.mu.Unlock()
	r.reload()
	r.mu.Lock()
	same = s.kept == current
	r.mu.Unlock()
	if !same {
		t.Fatal("identical repackaged files restarted again")
	}
}
