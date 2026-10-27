package core

import (
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestOldRecordCopyMigratesEmptyTagsOnce(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "old.db")
	source, _, store := openTransferSQLite(t, sourcePath, true)
	if _, err := source.Register(protocol.Record{Name: "#legacy@source", Kind: protocol.KindAgent, Owner: "owner@source", Descr: "keep this", Tags: []string{"ops"}}); err != nil {
		t.Fatal(err)
	}
	snap := source.Snapshot()
	var old protocol.Record
	for _, r := range snap.Records {
		if r.Name == "#legacy@source" {
			old = r
		}
	}
	old.Tags = []string{"ops", "no-tags"}
	if err := store.Commit(ports.Change{Records: map[string]*protocol.Record{old.Name: &old}}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	bytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	copiedPath := filepath.Join(root, "copy.db")
	if err = os.WriteFile(copiedPath, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	b, _, copied := openTransferSQLite(t, copiedPath, false)
	n, err := b.MigrateLegacyEmptyTags()
	if err != nil || n != 1 {
		t.Fatalf("migration count=%d err=%v", n, err)
	}
	saved, err := copied.Load()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range saved.Records {
		if r.Name == old.Name {
			found = true
			if !slices.Equal(r.Tags, []string{"ops"}) || r.ID != old.ID || r.Owner != old.Owner || r.Descr != old.Descr || !r.At.Equal(old.At) {
				t.Fatalf("record changed beyond tags: %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("record lost")
	}
	copied.Close()
	again, _, persisted := openTransferSQLite(t, copiedPath, false)
	defer persisted.Close()
	if n, e := again.MigrateLegacyEmptyTags(); e != nil || n != 0 {
		t.Fatalf("second startup count=%d err=%v", n, e)
	}
}
