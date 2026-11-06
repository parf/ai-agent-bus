package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

// Embedding the registry and ID ports models a wrapper that accidentally hides
// the durable-alert methods of its underlying store.
type registryOnlyEventStore struct {
	ports.Store
	ports.EventIDStore
}

func TestPersistenceRequiresTheAlertStoreBeforeServing(t *testing.T) {
	b := eventFixture(t)
	underlying, err := sqlite.Open(filepath.Join(t.TempDir(), "bus.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { underlying.Close() })
	st := registryOnlyEventStore{Store: underlying, EventIDStore: underlying}
	if err := b.Persistence(st); err == nil || !strings.Contains(err.Error(), "EventAlertStore") {
		t.Errorf("startup must refuse a store without EventAlertStore, got %v", err)
	}
	// Even a caller that ignores the startup error cannot write alerts through
	// the old silent memory-only fallback.
	if _, err := b.EventWrite("#a@h", "#a@h", "alert", "must not disappear"); err == nil || !strings.Contains(err.Error(), "EventAlertStore") {
		t.Fatalf("invalid binding silently accepted an alert: %v", err)
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
