package bigfile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

var lim = ports.BigLimits{Value: 100, Record: 150, Total: 250, Names: 3}

func open(t *testing.T) (*Store, string, *MemoryIndex) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "big")
	idx := NewMemoryIndex()
	s, err := Open(dir, idx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return s, dir, idx
}

func reopen(t *testing.T, dir string, idx ports.BigIndex, now time.Time) *Store {
	t.Helper()
	s, err := Open(dir, idx, now)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func put(s *Store, record uint32, name, data string, memory bool, now time.Time) (ports.BigInfo, error) {
	i, _, err := s.BigPut(record, name, strings.NewReader(data), "", memory, now.Add(time.Hour), now, lim)
	return i, err
}

func get(t *testing.T, s *Store, record uint32, name string, now time.Time) string {
	t.Helper()
	rc, _, found, err := s.BigGet(record, name, now)
	if err != nil || !found {
		t.Fatalf("get %s: %v %v", name, found, err)
	}
	defer rc.Close()
	raw, _ := io.ReadAll(rc)
	return string(raw)
}

func absent(t *testing.T, s *Store, record uint32, name string, now time.Time) {
	t.Helper()
	if rc, _, found, _ := s.BigGet(record, name, now); found {
		rc.Close()
		t.Fatalf("%s was found", name)
	}
}

func rowOf(t *testing.T, idx ports.BigIndex, record uint32, name string) ports.BigRow {
	t.Helper()
	r, found, err := idx.BigRowGet(record, name)
	if err != nil || !found {
		t.Fatalf("no row for %s: %v", name, err)
	}
	return r
}

func filesUnder(root string) []string {
	var out []string
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// crash runs f and swallows a panic raised at the named crash point, leaving
// the state a crash there would.
func crash(t *testing.T, stage string, f func()) {
	t.Helper()
	was := crashPoint
	crashPoint = func(s string) {
		if s == stage {
			panic("crash at " + stage)
		}
	}
	defer func() {
		crashPoint = was
		if r := recover(); r == nil {
			t.Fatalf("the operation never reached %q", stage)
		}
	}()
	f()
}

func TestFileKeyRoundTripsAndIsPrivate(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	data := "hello\x00world"
	info, err := put(s, 7, "a name/with ../ odd bytes", data, false, now)
	sum := sha256.Sum256([]byte(data))
	if err != nil || info.Held != "file" || info.Size != int64(len(data)) || info.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("put = %+v %v", info, err)
	}
	if got := get(t, s, 7, "a name/with ../ odd bytes", now); got != data {
		t.Fatalf("got %q", got)
	}
	row := rowOf(t, idx, 7, "a name/with ../ odd bytes")
	if row.Size != int64(len(data)) || row.Held != "file" || row.SHA256 != info.SHA256 {
		t.Fatalf("row = %+v", row)
	}
	// The key is one file under the record's directory, named by a hash and a
	// random suffix, 0600 in a 0700 tree; the name never reached a path.
	files := filesUnder(dir)
	if len(files) != 1 || files[0] != filepath.Join(dir, "7", row.File) || strings.Contains(files[0], "odd") {
		t.Fatalf("files = %v, row file %s", files, row.File)
	}
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() && fi.Mode().Perm() != 0o600 {
			t.Errorf("%s is %v", p, fi.Mode().Perm())
		} else if err == nil && fi.IsDir() && fi.Mode().Perm() != 0o700 {
			t.Errorf("%s is %v", p, fi.Mode().Perm())
		}
		return nil
	})
}

func TestFileKeysSurviveARestartAndDropExpiredOnes(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "keep", "kept bytes", false, now)
	s.BigPut(1, "short", strings.NewReader("short"), "", false, now.Add(time.Second), now, lim)
	put(s, 2, "mem", "memory bytes", true, now)
	short := rowOf(t, idx, 1, "short")
	again := reopen(t, dir, idx, now.Add(2*time.Second))
	if got := get(t, again, 1, "keep", now); got != "kept bytes" {
		t.Fatalf("after a restart: %q", got)
	}
	absent(t, again, 1, "short", now)
	absent(t, again, 2, "mem", now)
	if _, found, _ := idx.BigRowGet(1, "short"); found {
		t.Fatal("the expired row was kept at start")
	}
	if _, err := os.Stat(filepath.Join(dir, "1", short.File)); !os.IsNotExist(err) {
		t.Fatalf("the expired key's file was kept: %v", err)
	}
}

// Start reconciles in both directions, the rows being the truth.
func TestStartRemovesARowWhoseFileIsGone(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "k", "bytes", false, now)
	os.Remove(filepath.Join(dir, "1", rowOf(t, idx, 1, "k").File))
	again := reopen(t, dir, idx, now)
	if _, found, _ := idx.BigRowGet(1, "k"); found {
		t.Fatal("a row without its file was kept")
	}
	absent(t, again, 1, "k", now)
	if l, _ := again.BigList(1, now); len(l) != 0 {
		t.Fatalf("list = %+v", l)
	}
}

func TestStartRemovesAFileNoRowNames(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 3, "whole", "whole bytes", false, now)
	os.MkdirAll(filepath.Join(dir, "3"), 0o700)
	stray := filepath.Join(dir, "3", "no-row-names-this")
	os.WriteFile(stray, []byte("x"), 0o600)
	os.MkdirAll(filepath.Join(dir, "9"), 0o700)
	os.WriteFile(filepath.Join(dir, "9", "orphan-record"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(dir, "stray"), []byte("x"), 0o600)
	again := reopen(t, dir, idx, now)
	if got := get(t, again, 3, "whole", now); got != "whole bytes" {
		t.Fatalf("the row's own file was disturbed: %q", got)
	}
	for _, p := range []string{stray, filepath.Join(dir, "9", "orphan-record"), filepath.Join(dir, "stray")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s was kept", p)
		}
	}
}

func TestStartRemovesTemporaryLeftovers(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "k", "bytes", false, now)
	os.WriteFile(filepath.Join(dir, tmpDir, "half"), []byte("partial"), 0o600)
	again := reopen(t, dir, idx, now)
	if _, err := os.Stat(filepath.Join(dir, tmpDir, "half")); !os.IsNotExist(err) {
		t.Fatal("a temporary file survived a start")
	}
	if got := get(t, again, 1, "k", now); got != "bytes" {
		t.Fatalf("got %q", got)
	}
}

// A row whose file is whole but is not the one the row describes goes: the
// footer is checked against the row, never trusted over it.
func TestStartRemovesARowWhoseFileDoesNotMatchIt(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "k", "right bytes", false, now)
	put(s, 1, "other", "different bytes!", false, now)
	k, other := rowOf(t, idx, 1, "k"), rowOf(t, idx, 1, "other")
	raw, _ := os.ReadFile(filepath.Join(dir, "1", other.File))
	os.WriteFile(filepath.Join(dir, "1", k.File), raw, 0o600) // whole, but another key's
	again := reopen(t, dir, idx, now)
	if _, found, _ := idx.BigRowGet(1, "k"); found {
		t.Fatal("a row whose file is another key's was kept")
	}
	absent(t, again, 1, "k", now)
	if got := get(t, again, 1, "other", now); got != "different bytes!" {
		t.Fatalf("the other key was disturbed: %q", got)
	}
}

// A restore brings back rows from a backup of the database while the files
// are not there: the rows are dropped at start.
func TestARestoredIndexDropsRowsWhoseFilesAreNotPresent(t *testing.T) {
	s, _, idx := open(t)
	now := time.Now()
	put(s, 1, "a", "aaa", false, now)
	put(s, 2, "b", "bbb", false, now)
	restoredDir := filepath.Join(t.TempDir(), "big")
	again := reopen(t, restoredDir, idx, now)
	if rows, _ := idx.BigRows(); len(rows) != 0 {
		t.Fatalf("orphan rows survived the restore: %+v", rows)
	}
	absent(t, again, 1, "a", now)
	if got := again.BigRecords(); len(got) != 0 {
		t.Fatalf("records = %v", got)
	}
}

// A crash after the file is renamed and before its row commits: the file has
// no row. A new key does not exist; a replaced key keeps its old value.
func TestACrashBetweenRenameAndCommitLeavesTheOldStateAndStartRemovesTheFile(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "old", "old value", false, now)
	crash(t, "renamed", func() { put(s, 1, "old", "new value", false, now) })
	crash(t, "renamed", func() { put(s, 1, "fresh", "never committed", false, now) })
	if len(filesUnder(filepath.Join(dir, "1"))) != 3 {
		t.Fatalf("setup: files = %v", filesUnder(dir))
	}
	again := reopen(t, dir, idx, now)
	if got := get(t, again, 1, "old", now); got != "old value" {
		t.Fatalf("a replace that never committed changed the value: %q", got)
	}
	absent(t, again, 1, "fresh", now)
	if n := len(filesUnder(filepath.Join(dir, "1"))); n != 1 {
		t.Fatalf("files after start = %v", filesUnder(dir))
	}
}

// A crash after a replace commits and before the old file is unlinked.
func TestACrashBetweenCommitAndTheOldFilesUnlinkServesTheNewValueAndStartRemovesTheOldFile(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "k", "old value", false, now)
	crash(t, "committed", func() { put(s, 1, "k", "new value", false, now) })
	if len(filesUnder(filepath.Join(dir, "1"))) != 2 {
		t.Fatalf("setup: files = %v", filesUnder(dir))
	}
	again := reopen(t, dir, idx, now)
	if got := get(t, again, 1, "k", now); got != "new value" {
		t.Fatalf("got %q", got)
	}
	if n := len(filesUnder(filepath.Join(dir, "1"))); n != 1 {
		t.Fatalf("the old file survived: %v", filesUnder(dir))
	}
}

// A crash after a delete removed the row and before it unlinked the file.
func TestACrashBetweenRowDeleteAndUnlinkLeavesNoKeyAndStartRemovesTheFile(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "k", "bytes", false, now)
	crash(t, "row-deleted", func() { s.BigDelete(1, "k", now) })
	if len(filesUnder(dir)) != 1 {
		t.Fatalf("setup: files = %v", filesUnder(dir))
	}
	if _, found, _ := idx.BigRowGet(1, "k"); found {
		t.Fatal("the row survived its delete")
	}
	again := reopen(t, dir, idx, now)
	absent(t, again, 1, "k", now)
	if n := len(filesUnder(dir)); n != 0 {
		t.Fatalf("the file survived: %v", filesUnder(dir))
	}
}

// The commit order: the row commits only after the file is in place.
func TestAFailedRenameCommitsNoRow(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a read-only directory does not stop root")
	}
	s, dir, idx := open(t)
	now := time.Now()
	os.MkdirAll(filepath.Join(dir, "1"), 0o700)
	os.Chmod(filepath.Join(dir, "1"), 0o500)
	defer os.Chmod(filepath.Join(dir, "1"), 0o700)
	if _, err := put(s, 1, "k", "bytes", false, now); err == nil {
		t.Fatal("a put into an unwritable record directory succeeded")
	}
	if rows, _ := idx.BigRows(); len(rows) != 0 {
		t.Fatalf("a row was committed for a file that was never in place: %+v", rows)
	}
}

type failingIndex struct {
	*MemoryIndex
	failDelete, failPut bool
}

func (f failingIndex) BigRowDelete(record uint32, name string) (ports.BigRow, bool, error) {
	if f.failDelete {
		return ports.BigRow{}, false, errors.New("the database refused")
	}
	return f.MemoryIndex.BigRowDelete(record, name)
}

func (f failingIndex) BigRowPut(r ports.BigRow) (ports.BigRow, bool, error) {
	if f.failPut {
		return ports.BigRow{}, false, errors.New("the database refused")
	}
	return f.MemoryIndex.BigRowPut(r)
}

// The delete order: the row goes first, so a refused row delete leaves the key
// whole instead of a row pointing at nothing.
func TestARefusedRowDeleteLeavesTheKeyReadable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "big")
	idx := failingIndex{MemoryIndex: NewMemoryIndex()}
	s, err := Open(dir, idx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	put(s, 1, "k", "bytes", false, now)
	idx.failDelete = true
	s.idx = idx
	if _, err := s.BigDelete(1, "k", now); err == nil {
		t.Fatal("a refused row delete was reported as done")
	}
	if got := get(t, s, 1, "k", now); got != "bytes" {
		t.Fatalf("the key is damaged after a refused delete: %q", got)
	}
}

// A refused commit leaves no file either.
func TestARefusedCommitLeavesNoFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "big")
	idx := failingIndex{MemoryIndex: NewMemoryIndex(), failPut: true}
	s, err := Open(dir, idx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := put(s, 1, "k", "bytes", false, time.Now()); err == nil {
		t.Fatal("a refused commit was reported as done")
	}
	if n := len(filesUnder(dir)); n != 0 {
		t.Fatalf("files left: %v", filesUnder(dir))
	}
}

// A memory bigfile has no row but still lists, reads and expires beside the
// rows.
func TestMemoryKeysListGetAndExpireBesideRows(t *testing.T) {
	s, _, idx := open(t)
	now := time.Now()
	put(s, 1, "a-file", "in a file", false, now)
	s.BigPut(1, "b-mem", strings.NewReader("in memory"), "", true, now.Add(time.Minute), now, lim)
	if _, found, _ := idx.BigRowGet(1, "b-mem"); found {
		t.Fatal("a memory key got a row")
	}
	l, _ := s.BigList(1, now)
	if len(l) != 2 || l[0].Name != "a-file" || l[0].Held != "file" || l[1].Name != "b-mem" || l[1].Held != "memory" {
		t.Fatalf("list = %+v", l)
	}
	if got := get(t, s, 1, "b-mem", now); got != "in memory" {
		t.Fatalf("got %q", got)
	}
	later := now.Add(time.Minute)
	absent(t, s, 1, "b-mem", later)
	if l, _ := s.BigList(1, later); len(l) != 1 || l[0].Name != "a-file" {
		t.Fatalf("list after the memory key expired = %+v", l)
	}
	if got := get(t, s, 1, "a-file", later); got != "in a file" {
		t.Fatalf("the file key = %q", got)
	}
}

func TestARefusedPutLeavesNoFileAndTheOldValue(t *testing.T) {
	s, dir, _ := open(t)
	now := time.Now()
	if _, err := put(s, 1, "k", "old", false, now); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		data string
		sum  string
		want error
	}{
		{"over the cap", strings.Repeat("x", 101), "", ports.ErrBigTooLarge},
		{"empty", "", "", ports.ErrBigEmpty},
		{"wrong hash", "new", strings.Repeat("0", 64), ports.ErrBigHash},
	}
	for _, c := range cases {
		if _, _, err := s.BigPut(1, "k", strings.NewReader(c.data), c.sum, false, now.Add(time.Hour), now, lim); !errors.Is(err, c.want) {
			t.Errorf("%s = %v", c.name, err)
		}
	}
	if got := get(t, s, 1, "k", now); got != "old" {
		t.Fatalf("a refused put changed the value: %q", got)
	}
	if left, _ := os.ReadDir(filepath.Join(dir, tmpDir)); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

func TestLimitsCountFilesAndMemoryTogether(t *testing.T) {
	s, _, _ := open(t)
	now := time.Now()
	if _, err := put(s, 1, "f", strings.Repeat("x", 90), false, now); err != nil {
		t.Fatal(err)
	}
	if _, err := put(s, 1, "m", strings.Repeat("x", 70), true, now); !errors.Is(err, ports.ErrBigFull) {
		t.Fatalf("70 bytes in memory beside 90 in a file passed the record total: %v", err)
	}
	if _, err := put(s, 1, "f", strings.Repeat("y", 90), false, now); err != nil {
		t.Fatalf("a replace counted its old bytes: %v", err)
	}
	other, _, _ := open(t)
	if _, err := put(other, 1, "m", strings.Repeat("x", 90), true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := put(other, 1, "f", strings.Repeat("x", 70), false, now); !errors.Is(err, ports.ErrBigFull) {
		t.Fatalf("70 bytes in a file beside 90 in memory passed the record total: %v", err)
	}
}

func TestReplaceMovesAKeyBetweenHolders(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "k", "in a file", false, now)
	if _, err := put(s, 1, "k", "in memory", true, now); err != nil {
		t.Fatal(err)
	}
	if got := get(t, s, 1, "k", now); got != "in memory" {
		t.Fatalf("got %q", got)
	}
	if _, found, _ := idx.BigRowGet(1, "k"); found {
		t.Fatal("the row stayed after the key moved to memory")
	}
	if n := len(filesUnder(dir)); n != 0 {
		t.Fatalf("the old file was kept after the key moved to memory: %v", filesUnder(dir))
	}
	if info, _ := put(s, 1, "k", "back in a file", false, now); info.Held != "file" {
		t.Fatalf("held = %s", info.Held)
	}
	if l, _ := s.BigList(1, now); len(l) != 1 || l[0].Held != "file" {
		t.Fatalf("list = %+v", l)
	}
}

func TestExpiryDropRecordAndSweepRemoveRowsAndFiles(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	s.BigPut(1, "short", strings.NewReader("s"), "", false, now.Add(time.Minute), now, lim)
	put(s, 2, "a", "a", false, now)
	short := rowOf(t, idx, 1, "short")
	s.Sweep(now.Add(time.Minute))
	if _, err := os.Stat(filepath.Join(dir, "1", short.File)); !os.IsNotExist(err) {
		t.Fatal("the sweep kept an expired key's file")
	}
	if _, found, _ := idx.BigRowGet(1, "short"); found {
		t.Fatal("the sweep kept an expired key's row")
	}
	if err := s.BigDrop(2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2")); !os.IsNotExist(err) {
		t.Fatal("a dropped record's directory was kept")
	}
	if rows, _ := idx.BigRows(); len(rows) != 0 {
		t.Fatalf("rows left: %+v", rows)
	}
	if got := s.BigRecords(); len(got) != 0 {
		t.Fatalf("records = %v", got)
	}
}

func TestALinkInPlaceOfAKeyFileIsNotFollowed(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 1, "k", "bytes", false, now)
	row := rowOf(t, idx, 1, "k")
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte("not a big key"), 0o600)
	path := filepath.Join(dir, "1", row.File)
	os.Remove(path)
	os.Symlink(outside, path)
	if rc, _, _, err := s.BigGet(1, "k", now); err == nil {
		rc.Close()
		t.Fatal("a symbolic link was opened as a key file")
	}
	reopen(t, dir, idx, now)
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("the link's target was removed: %v", err)
	}
	if _, found, _ := idx.BigRowGet(1, "k"); found {
		t.Fatal("a linked file kept its row")
	}
}

func TestARecordDirectoryLinkIsNotFollowedByAPutOrADelete(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "5")); err != nil {
		t.Fatal(err)
	}
	if _, err := put(s, 5, "escaped", "payload", false, now); err == nil {
		t.Fatal("a put went through a linked record directory")
	}
	if got := filesUnder(outside); len(got) != 0 {
		t.Fatalf("a put wrote outside the root: %v", got)
	}
	if rows, _ := idx.BigRows(); len(rows) != 0 {
		t.Fatalf("a refused put left a row: %+v", rows)
	}
	if left, _ := os.ReadDir(filepath.Join(dir, tmpDir)); len(left) != 0 {
		t.Fatalf("the refused put left temporary files: %v", left)
	}
	if _, err := put(s, 6, "k", "inside", false, now); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "moved")
	os.Rename(filepath.Join(dir, "6"), moved)
	os.Symlink(moved, filepath.Join(dir, "6"))
	if rc, _, _, err := s.BigGet(6, "k", now); err == nil {
		rc.Close()
		t.Fatal("a get went through a linked record directory")
	}
	s.BigDelete(6, "k", now)
	if got := filesUnder(moved); len(got) != 1 {
		t.Fatalf("a delete removed a file through the link: %v", got)
	}
}

func TestALinkedTempOrRootDirectoryRefusesAPut(t *testing.T) {
	s, dir, _ := open(t)
	outside := t.TempDir()
	os.RemoveAll(filepath.Join(dir, tmpDir))
	os.Symlink(outside, filepath.Join(dir, tmpDir))
	if _, err := put(s, 1, "k", "payload", false, time.Now()); err == nil {
		t.Fatal("a put went through a linked temporary directory")
	}
	if got := filesUnder(outside); len(got) != 0 {
		t.Fatalf("a put wrote outside the root: %v", got)
	}
}

func TestStartDoesNotBlockOnAFifoOrOpenAnyNonRegularEntry(t *testing.T) {
	s, dir, idx := open(t)
	now := time.Now()
	put(s, 3, "k", "bytes", false, now)
	fifo := filepath.Join(dir, "3", "a-fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("no FIFOs here: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := Open(dir, idx, now); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("start blocked on a FIFO in a record directory")
	}
	if _, err := os.Lstat(fifo); !os.IsNotExist(err) {
		t.Fatal("the FIFO was kept")
	}
}
