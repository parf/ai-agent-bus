package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/bigfile"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
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
// starts, reports an error, and refuses every bigfile call naming the cause.
func TestAPlantedFileAtTheBigKeyPathRefusesCallsAndKeepsTheBusUp(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "bigfile"), []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, rep := bigBus(t)
	bindBigKeys(b, home, bigfile.NewMemoryIndex())
	if got := rep.all(); !strings.Contains(got, "bigfile directory could not be prepared; bigfile calls are refused") || !strings.HasPrefix(got, "error") {
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

func putOne(t *testing.T, b *core.Bus, data string) error {
	t.Helper()
	_, _, err := b.BigPut("alice@h", "#svc@h", "k", strings.NewReader(data), 0, false, "")
	return err
}

func getOne(t *testing.T, b *core.Bus) string {
	t.Helper()
	rc, _, err := b.BigGet("alice@h", "#svc@h", "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer rc.Close()
	raw, _ := io.ReadAll(rc)
	return string(raw)
}

// A directory under the old name is renamed once, and its bigfiles are served.
func TestTheOldBigKeysDirectoryIsRenamedAndItsFilesServed(t *testing.T) {
	home := t.TempDir()
	first, _ := bigBus(t)
	idx := bigfile.NewMemoryIndex()
	s, err := bigfile.Open(filepath.Join(home, oldBigDir), idx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	first.SetBigStore(s)
	if err := putOne(t, first, "kept through the rename"); err != nil {
		t.Fatal(err)
	}
	// The same record IDs, as after a daemon restart: a fresh bus, the same names.
	second, rep := bigBus(t)
	bindBigKeys(second, home, idx)
	if strings.Contains(rep.all(), "could not be prepared") {
		t.Fatalf("the rename was refused: %s", rep.all())
	}
	if _, err := os.Lstat(filepath.Join(home, oldBigDir)); !os.IsNotExist(err) {
		t.Fatalf("the old directory is still there: %v", err)
	}
	if got := getOne(t, second); got != "kept through the rename" {
		t.Fatalf("after the rename: %q", got)
	}
}

// Both directories at once are never guessed at: calls are refused with the
// cause, and neither directory is touched.
func TestBothBigDirectoriesRefuseCallsAndTouchNeither(t *testing.T) {
	home := t.TempDir()
	for _, d := range []string{oldBigDir, bigDir} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, d, "marker"), []byte(d), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b, rep := bigBus(t)
	bindBigKeys(b, home, bigfile.NewMemoryIndex())
	err := putOne(t, b, "x")
	if !errors.Is(err, core.ErrBigNoStore) || !strings.Contains(err.Error(), "both") || !strings.Contains(err.Error(), oldBigDir) || !strings.Contains(err.Error(), bigDir) {
		t.Fatalf("a put with both directories = %v, want the unavailable refusal naming both", err)
	}
	if !strings.HasPrefix(rep.all(), "error") {
		t.Fatalf("reports = %q, want an error", rep.all())
	}
	for _, d := range []string{oldBigDir, bigDir} {
		if raw, err := os.ReadFile(filepath.Join(home, d, "marker")); err != nil || string(raw) != d {
			t.Fatalf("%s was touched: %q %v", d, raw, err)
		}
	}
}

// Neither directory is the ordinary first start: the new one is created.
func TestNeitherBigDirectoryStartsTheNewOne(t *testing.T) {
	home := t.TempDir()
	b, _ := bigBus(t)
	bindBigKeys(b, home, bigfile.NewMemoryIndex())
	if err := putOne(t, b, "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, bigDir)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(home, oldBigDir)); !os.IsNotExist(err) {
		t.Fatalf("the old name appeared: %v", err)
	}
}

// The index lives in the daemon's database: a bigfile survives a restart that
// rebuilds nothing from the files, and a restored database whose files are not
// present loses those rows at start.
func TestBigfilesSurviveARestartThroughTheDatabaseAndARestoreDropsOrphanRows(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "agent-bus.db")
	db, err := sqlite.Open(dbPath, true)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := bigBus(t)
	bindBigKeys(first, home, db)
	if err := putOne(t, first, "kept in the database"); err != nil {
		t.Fatal(err)
	}
	rows, err := db.BigRows()
	if err != nil || len(rows) != 1 || rows[0].Name != "k" || rows[0].Size != int64(len("kept in the database")) {
		t.Fatalf("rows = %+v %v", rows, err)
	}
	db.Close()

	db, err = sqlite.Open(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := bigBus(t)
	bindBigKeys(second, home, db)
	if got := getOne(t, second); got != "kept in the database" {
		t.Fatalf("after a restart: %q", got)
	}
	db.Close()

	// The database comes back from a backup while the files do not.
	if err := os.RemoveAll(filepath.Join(home, bigDir)); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	third, _ := bigBus(t)
	bindBigKeys(third, home, db)
	if rows, _ := db.BigRows(); len(rows) != 0 {
		t.Fatalf("orphan rows survived the restore: %+v", rows)
	}
	if _, _, err := third.BigGet("alice@h", "#svc@h", "k"); !errors.Is(err, core.ErrBigAbsent) {
		t.Fatalf("a bigfile whose file is gone = %v", err)
	}
}
