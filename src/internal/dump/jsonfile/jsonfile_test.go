package jsonfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestSnapshotReplacementIsWholePrivateAndLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	d := New(filepath.Join(dir, "dump.json"))
	for _, state := range []string{"active", "banned"} {
		if err := d.Save(ports.Snapshot{Users: []protocol.User{{Name: "alice@h", State: state}}}); err != nil {
			t.Fatal(err)
		}
		v, exists, err := d.Load()
		if err != nil || !exists || len(v.Users) != 1 || v.Users[0].State != state {
			t.Fatalf("snapshot replacement lost %s: %#v, %v", state, v, err)
		}
	}
	info, err := os.Stat(d.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot permissions are %o", info.Mode().Perm())
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "dump.json" {
		t.Fatalf("temporary snapshot files remain: %v", files)
	}
	// A failed replacement leaves the previous complete snapshot readable.
	err = d.Save(ports.Snapshot{Records: []protocol.Record{{Config: json.RawMessage(`invalid json`)}}})
	if err == nil {
		t.Fatal("invalid snapshot was acknowledged")
	}
	v, _, err := d.Load()
	if err != nil || len(v.Users) != 1 || v.Users[0].State != "banned" {
		t.Fatal("failed replacement erased the saved ban")
	}
}

func TestSnapshotIOFailureIsReturned(t *testing.T) {
	p := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(p, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := New(filepath.Join(p, "dump.json")).Save(ports.Snapshot{}); err == nil {
		t.Fatal("write failure was reported as success")
	}
}
