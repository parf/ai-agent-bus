package core

import (
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
	"path/filepath"
	"strings"
	"testing"
)

// Each wrapper hides exactly one port of the real SQLite store.
type withoutEventIDStore struct {
	ports.Store
	ports.EventAlertStore
	ports.EventStatePruner
	ports.SubscriptionStore
}
type withoutEventAlertStore struct {
	ports.Store
	ports.EventIDStore
	ports.EventStatePruner
	ports.SubscriptionStore
}
type withoutEventStatePruner struct {
	ports.Store
	ports.EventIDStore
	ports.EventAlertStore
	ports.SubscriptionStore
}
type withoutSubscriptionStore struct {
	ports.Store
	ports.EventIDStore
	ports.EventAlertStore
	ports.EventStatePruner
}

func TestPersistenceRequiresTheEventStoreFamilyBeforeServing(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "bus.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	for _, tc := range []struct {
		name  string
		store ports.Store
	}{
		{"EventIDStore", withoutEventIDStore{st, st, st, st}},
		{"EventAlertStore", withoutEventAlertStore{st, st, st, st}},
		{"EventStatePruner", withoutEventStatePruner{st, st, st, st}},
		{"SubscriptionStore", withoutSubscriptionStore{st, st, st, st}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, previous := range []ports.Store{nil, memory.NewState()} {
				b := New()
				if err := b.Persistence(previous); err != nil {
					t.Fatal(err)
				}
				if err := b.Persistence(tc.store); err == nil || !strings.Contains(err.Error(), "ports."+tc.name) {
					t.Errorf("startup must refuse a store without %s, got %v", tc.name, err)
				}
				if b.store != previous {
					t.Error("refused store replaced the previous binding")
				}
			}
		})
	}
}

func TestExplicitMemoryEventStoreCanBind(t *testing.T) {
	for _, st := range []ports.Store{nil, memory.NewState()} {
		b := eventFixture(t)
		if err := b.Persistence(st); err != nil {
			t.Fatal(err)
		}
		if _, err := b.EventWrite("#a@h", "#a@h", "alert", "in memory deliberately"); err != nil {
			t.Fatal(err)
		}
		if err := b.FlushEvents(); err != nil {
			t.Fatal(err)
		}
	}
}
