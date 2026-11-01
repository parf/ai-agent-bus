package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type reports struct {
	ports.Silent
	mu   sync.Mutex
	rows []string
}

func (r *reports) Report(s ports.Severity, m string) {
	r.mu.Lock()
	r.rows = append(r.rows, s.String()+": "+m)
	r.mu.Unlock()
}
func (r *reports) all() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.rows, "\n")
}

func bigBus(t *testing.T) (*core.Bus, *reports) {
	t.Helper()
	b := core.New()
	rep := &reports{}
	b.Journal(rep)
	b.Persistence(memory.NewState())
	b.SetDaemonOwner("admin@h")
	if _, err := b.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	return b, rep
}

// A plain file where the directory belongs must not stop the daemon: it
// starts, reports an error, and refuses every big-key call naming the cause.
func TestAPlantedFileAtTheBigKeyPathRefusesCallsAndKeepsTheBusUp(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "big-keys"), []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, rep := bigBus(t)
	bindBigKeys(b, home)
	if got := rep.all(); !strings.Contains(got, "big-key directory could not be prepared; big-key calls are refused") || !strings.HasPrefix(got, "error") {
		t.Fatalf("reports = %q, want an error-level report naming the refusal", got)
	}
	calls := map[string]func() error{
		"put": func() error {
			_, _, err := b.BigPut("alice@h", "#svc@h", "k", strings.NewReader("x"), 0, false, "")
			return err
		},
		"get":    func() error { _, _, err := b.BigGet("alice@h", "#svc@h", "k"); return err },
		"list":   func() error { _, _, err := b.BigList("alice@h", "#svc@h"); return err },
		"delete": func() error { _, err := b.BigDelete("alice@h", "#svc@h", "k"); return err },
	}
	for name, call := range calls {
		err := call()
		if !errors.Is(err, core.ErrBigNoStore) || !strings.Contains(err.Error(), "not a directory") && !strings.Contains(err.Error(), "could not be prepared") {
			t.Errorf("%s = %v, want the unavailable refusal naming the directory problem", name, err)
		}
	}
	// The bus itself still works.
	if _, ok := b.Lookup("alice@h", "#svc@h"); !ok {
		t.Fatal("the bus does not serve its records")
	}
}
