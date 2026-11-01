package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// A test cannot be root, so users/ is "root's" by being the test user's.
func personFilesFixture(t *testing.T) (root string, uid, gid int) {
	t.Helper()
	uid, gid = os.Getuid(), os.Getgid()
	oldUID, oldGID := personFilesRootUID, personFilesRootGID
	personFilesRootUID, personFilesRootGID = uid, gid
	t.Cleanup(func() { personFilesRootUID, personFilesRootGID = oldUID, oldGID })
	return t.TempDir(), uid, gid
}

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode()
}

func TestPersonFilesAreCreatedOnceAndLeftAlone(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	notes, err := ensurePersonFiles(root, "parf", uid, gid)
	if err != nil || len(notes) != 0 {
		t.Fatalf("create: %v %v", notes, err)
	}
	dir := personFilesDir(root, "parf")
	for path, want := range map[string]os.FileMode{filepath.Join(root, "users"): 0o755, filepath.Dir(dir): 0o700, dir: 0o700} {
		if got := modeOf(t, path); !got.IsDir() || got.Perm() != want {
			t.Fatalf("%s is %v, want a directory %#o", path, got, want)
		}
		if st := mustStat(t, path); int(st.Uid) != uid || int(st.Gid) != gid {
			t.Fatalf("%s belongs to %d:%d", path, st.Uid, st.Gid)
		}
	}
	if !personFilesReady(root, "parf", uid, gid) {
		t.Fatal("a made folder is not ready")
	}
	keep := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(keep, []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	if notes, err = ensurePersonFiles(root, "parf", uid, gid); err != nil || len(notes) != 0 {
		t.Fatalf("rerun: %v %v", notes, err)
	}
	if b, _ := os.ReadFile(keep); string(b) != "contents" {
		t.Fatalf("contents touched: %q", b)
	}
}

func mustStat(t *testing.T, path string) *syscall.Stat_t {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Sys().(*syscall.Stat_t)
}

func TestPersonFilesAcceptsAFolderMadeByHand(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	hand := filepath.Join(root, "users", "parf")
	if err := os.MkdirAll(hand, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "users"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hand, "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if notes, err := ensurePersonFiles(root, "parf", uid, gid); err != nil || len(notes) != 0 {
		t.Fatalf("%v %v", notes, err)
	}
	if b, _ := os.ReadFile(filepath.Join(hand, "notes.txt")); string(b) != "mine" {
		t.Fatalf("contents touched: %q", b)
	}
	if modeOf(t, personFilesDir(root, "parf")).Perm() != 0o700 {
		t.Fatal("files not made")
	}
}

func TestPersonFilesTightensALooseModeAndSaysSo(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	dir := personFilesDir(root, "parf")
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if personFilesReady(root, "parf", uid, gid) {
		t.Fatal("a loose folder counts as ready")
	}
	notes, err := ensurePersonFiles(root, "parf", uid, gid)
	if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "from 0755 to 0700") {
		t.Fatalf("tighten: %v %v", notes, err)
	}
	if modeOf(t, dir).Perm() != 0o700 {
		t.Fatal("mode not tightened")
	}
}

func TestPersonFilesRefusesALinkAtEachLevelAndTouchesNothingThroughIt(t *testing.T) {
	for _, level := range []string{"users", "person", "files"} {
		t.Run(level, func(t *testing.T) {
			root, uid, gid := personFilesFixture(t)
			outside := t.TempDir()
			if err := os.Chmod(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			switch level {
			case "users":
				mustSymlink(t, outside, filepath.Join(root, "users"))
			case "person":
				mustMkdir(t, filepath.Join(root, "users"), 0o755)
				mustSymlink(t, outside, filepath.Join(root, "users", "parf"))
			case "files":
				mustMkdir(t, filepath.Join(root, "users", "parf"), 0o700)
				mustSymlink(t, outside, filepath.Join(root, "users", "parf", "files"))
			}
			if _, err := ensurePersonFiles(root, "parf", uid, gid); err == nil || !strings.Contains(err.Error(), "link or not a directory") {
				t.Fatalf("a link at %s: %v", level, err)
			}
			if modeOf(t, outside).Perm() != 0o755 {
				t.Fatalf("the mode of what the link led to changed to %v", modeOf(t, outside))
			}
			if entries, _ := os.ReadDir(outside); len(entries) != 0 {
				t.Fatalf("something was made through the link: %v", entries)
			}
		})
	}
}

func TestPersonFilesRefusesAFileInThePlaceOfAFolder(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	mustMkdir(t, filepath.Join(root, "users", "parf"), 0o700)
	if err := os.WriteFile(filepath.Join(root, "users", "parf", "files"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("a file where the folder goes: %v", err)
	}
}

func TestPersonFilesRefusesAnotherOwnersFolderWithoutChangingIt(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	dir := personFilesDir(root, "parf")
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The folder belongs to this uid, so asking for another person's uid is a wrong owner.
	if _, err := ensurePersonFiles(root, "parf", uid+1, gid); err == nil || !strings.Contains(err.Error(), "belongs to uid") {
		t.Fatalf("a wrong owner: %v", err)
	}
	if modeOf(t, dir).Perm() != 0o755 {
		t.Fatalf("a refused folder was changed: %v", modeOf(t, dir))
	}
}

func TestPersonFilesRefusesARootThatIsALinkAndAnUnsafeName(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	link := filepath.Join(t.TempDir(), "state")
	mustSymlink(t, root, link)
	if _, err := ensurePersonFiles(link, "parf", uid, gid); err == nil {
		t.Fatal("a state directory that is a link was used")
	}
	for _, name := range []string{"", "..", "a/b", "A", "x y"} {
		if _, err := ensurePersonFiles(root, name, uid, gid); err == nil {
			t.Fatalf("name %q accepted", name)
		}
	}
	if _, err := ensurePersonFiles(root, "parf", 0, 0); err == nil {
		t.Fatal("uid 0 accepted")
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
