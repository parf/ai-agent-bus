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

func open(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "big")
	s, err := Open(dir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
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

func TestFileKeyRoundTripsAndIsPrivate(t *testing.T) {
	s, dir := open(t)
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
	// The key is a file under the record's directory, named by a hash, 0600
	// in a 0700 tree, and the name never reached a path.
	var files []string
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			files = append(files, p)
			if fi.Mode().Perm() != 0o600 {
				t.Errorf("%s is %v", p, fi.Mode().Perm())
			}
		} else if err == nil && fi.IsDir() && fi.Mode().Perm() != 0o700 {
			t.Errorf("%s is %v", p, fi.Mode().Perm())
		}
		return nil
	})
	if len(files) != 1 || !strings.HasPrefix(files[0], filepath.Join(dir, "7")+string(filepath.Separator)) || strings.Contains(files[0], "odd") {
		t.Fatalf("files = %v", files)
	}
}

func TestFileKeysSurviveARestartAndDropExpiredOnes(t *testing.T) {
	s, dir := open(t)
	now := time.Now()
	if _, err := put(s, 1, "keep", "kept bytes", false, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.BigPut(1, "short", strings.NewReader("short"), "", false, now.Add(time.Second), now, lim); err != nil {
		t.Fatal(err)
	}
	if _, err := put(s, 2, "mem", "memory bytes", true, now); err != nil {
		t.Fatal(err)
	}
	again, err := Open(dir, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got := get(t, again, 1, "keep", now); got != "kept bytes" {
		t.Fatalf("after a restart: %q", got)
	}
	if _, _, found, _ := again.BigGet(1, "short", now); found {
		t.Fatal("an expired key came back")
	}
	if _, _, found, _ := again.BigGet(2, "mem", now); found {
		t.Fatal("a memory key survived a restart")
	}
	if _, err := os.Stat(filepath.Join(dir, "1", fileName("short"))); !os.IsNotExist(err) {
		t.Fatalf("the expired key's file was kept: %v", err)
	}
}

func TestACrashLeavesNoKeyAndNoStrayFile(t *testing.T) {
	s, dir := open(t)
	now := time.Now()
	// An interrupted upload leaves a temporary file; a torn file in a record
	// directory has no whole footer.
	os.WriteFile(filepath.Join(dir, tmpDir, "half"), []byte("partial"), 0o600)
	os.MkdirAll(filepath.Join(dir, "3"), 0o700)
	os.WriteFile(filepath.Join(dir, "3", fileName("torn")), []byte("torn bytes without a footer"), 0o600)
	os.WriteFile(filepath.Join(dir, "stray"), []byte("x"), 0o600)
	if _, err := put(s, 3, "whole", "whole bytes", false, now); err != nil {
		t.Fatal(err)
	}
	again, err := Open(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := again.BigList(3, now); len(l) != 1 || l[0].Name != "whole" {
		t.Fatalf("list = %+v", l)
	}
	for _, p := range []string{filepath.Join(dir, tmpDir, "half"), filepath.Join(dir, "3", fileName("torn")), filepath.Join(dir, "stray")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s was kept", p)
		}
	}
}

func TestARefusedPutLeavesNoFileAndTheOldValue(t *testing.T) {
	s, dir := open(t)
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
	s, _ := open(t)
	now := time.Now()
	for _, c := range []struct {
		name   string
		memory bool
	}{{"f", false}, {"m", true}} {
		size := 90
		if c.memory {
			size = 70
		}
		if _, err := put(s, 1, c.name, strings.Repeat("x", size), c.memory, now); c.name == "f" && err != nil {
			t.Fatal(err)
		} else if c.name == "m" && !errors.Is(err, ports.ErrBigFull) {
			t.Fatalf("70 bytes in memory beside 90 in a file passed the record total: %v", err)
		}
	}
	if _, err := put(s, 1, "f", strings.Repeat("y", 90), false, now); err != nil {
		t.Fatalf("a replace counted its old bytes: %v", err)
	}
	// And the other way round: memory first, then a file.
	other, _ := open(t)
	if _, err := put(other, 1, "m", strings.Repeat("x", 90), true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := put(other, 1, "f", strings.Repeat("x", 70), false, now); !errors.Is(err, ports.ErrBigFull) {
		t.Fatalf("70 bytes in a file beside 90 in memory passed the record total: %v", err)
	}
}

func TestAFileUnderTheWrongNameIsRemovedAtStart(t *testing.T) {
	s, dir := open(t)
	now := time.Now()
	put(s, 1, "k", "bytes", false, now)
	good := filepath.Join(dir, "1", fileName("k"))
	raw, _ := os.ReadFile(good)
	wrong := filepath.Join(dir, "1", fileName("another"))
	os.WriteFile(wrong, raw, 0o600) // whole, but not the file this key's name maps to
	if _, err := Open(dir, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wrong); !os.IsNotExist(err) {
		t.Fatal("a whole file under the wrong name was kept")
	}
	if _, err := os.Stat(good); err != nil {
		t.Fatalf("the right file was removed: %v", err)
	}
}

func TestReplaceMovesAKeyBetweenHolders(t *testing.T) {
	s, dir := open(t)
	now := time.Now()
	put(s, 1, "k", "in a file", false, now)
	if _, err := put(s, 1, "k", "in memory", true, now); err != nil {
		t.Fatal(err)
	}
	if got := get(t, s, 1, "k", now); got != "in memory" {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "1", fileName("k"))); !os.IsNotExist(err) {
		t.Fatal("the old file was kept after the key moved to memory")
	}
	if info, _ := put(s, 1, "k", "back in a file", false, now); info.Held != "file" {
		t.Fatalf("held = %s", info.Held)
	}
}

func TestExpiryDropRecordAndSweepRemoveFiles(t *testing.T) {
	s, dir := open(t)
	now := time.Now()
	s.BigPut(1, "short", strings.NewReader("s"), "", false, now.Add(time.Minute), now, lim)
	put(s, 2, "a", "a", false, now)
	s.Sweep(now.Add(time.Minute))
	if _, err := os.Stat(filepath.Join(dir, "1", fileName("short"))); !os.IsNotExist(err) {
		t.Fatal("the sweep kept an expired key's file")
	}
	if err := s.BigDrop(2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2")); !os.IsNotExist(err) {
		t.Fatal("a dropped record's directory was kept")
	}
	if got := s.BigRecords(); len(got) != 0 {
		t.Fatalf("records = %v", got)
	}
}

func TestALinkInPlaceOfAKeyFileIsNotFollowed(t *testing.T) {
	s, dir := open(t)
	now := time.Now()
	put(s, 1, "k", "bytes", false, now)
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte("not a big key"), 0o600)
	path := filepath.Join(dir, "1", fileName("k"))
	os.Remove(path)
	os.Symlink(outside, path)
	if rc, _, _, err := s.BigGet(1, "k", now); err == nil {
		rc.Close()
		t.Fatal("a symbolic link was opened as a key file")
	}
	if _, err := Open(dir, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("the link's target was removed: %v", err)
	}
	again, _ := Open(dir, now)
	if l, _ := again.BigList(1, now); len(l) != 0 {
		t.Fatalf("a linked file was indexed: %+v", l)
	}
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

func TestARecordDirectoryLinkIsNotFollowedByAPutOrADelete(t *testing.T) {
	s, dir := open(t)
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
	if left, _ := os.ReadDir(filepath.Join(dir, tmpDir)); len(left) != 0 {
		t.Fatalf("the refused put left temporary files: %v", left)
	}
	// A key whose record directory is swapped for a link afterwards is neither
	// read nor deleted through it.
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
	if _, err := s.BigDelete(6, "k", now); err != nil {
		t.Fatal(err)
	}
	if got := filesUnder(moved); len(got) != 1 {
		t.Fatalf("a delete removed a file through the link: %v", got)
	}
}

func TestALinkedTempOrRootDirectoryRefusesAPut(t *testing.T) {
	s, dir := open(t)
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
	s, dir := open(t)
	now := time.Now()
	put(s, 3, "k", "bytes", false, now)
	fifo := filepath.Join(dir, "3", fileName("fifo"))
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("no FIFOs here: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := Open(dir, now); done <- err }()
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
