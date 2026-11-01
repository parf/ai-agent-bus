//go:build linux

package main

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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
	if !personFilesReady(root, "parf", uid) {
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
	if personFilesReady(root, "parf", uid) {
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

func usersEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "users"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func hookPublish(t *testing.T, f func(stage int, tmp string, parent int, name string)) {
	t.Helper()
	beforePublish = f
	t.Cleanup(func() { beforePublish = func(int, string, int, string) {} })
}

// A new directory is made where the person cannot write, never in their own folder.
func TestPersonFilesMakesNewDirectoriesInUsersNotInThePersonsFolder(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	madeIn := map[string]string{}
	afterStage = func(stage int, name string) {
		where, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(stage))
		if err != nil {
			t.Fatal(err)
		}
		madeIn[name] = where
	}
	t.Cleanup(func() { afterStage = func(int, string) {} })
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	if len(madeIn) != 3 {
		t.Fatalf("expected three staged directories, got %v", madeIn)
	}
	for tmp, where := range madeIn {
		if where == filepath.Join(root, "users", "parf") || where == personFilesDir(root, "parf") {
			t.Fatalf("%s was made inside the person's own folder %s", tmp, where)
		}
	}
	if left := usersEntries(t, root); len(left) != 1 || left[0] != "parf" {
		t.Fatalf("a staging directory was left behind: %v", left)
	}
}

// The name is made by the person (or anyone) while the new directory waits to be renamed: the new one never
// replaces it, and what is there is checked like any other directory that was there.
func TestPersonFilesNeverReplacesANameThatAppearsBeforeThePublish(t *testing.T) {
	for _, kind := range []string{"empty directory", "directory with a file", "link"} {
		t.Run(kind, func(t *testing.T) {
			root, uid, gid := personFilesFixture(t)
			outside := t.TempDir()
			if err := os.Chmod(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			var planted uint64
			hookPublish(t, func(stage int, tmp string, parent int, name string) {
				if name != "files" {
					return
				}
				target := filepath.Join(root, "users", "parf", "files")
				defer func() {
					if st, err := os.Lstat(target); err == nil {
						planted = st.Sys().(*syscall.Stat_t).Ino
					}
				}()
				switch kind {
				case "link":
					mustSymlink(t, outside, target)
				default:
					if err := os.Mkdir(target, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(target, 0o755); err != nil {
						t.Fatal(err)
					}
					if kind == "directory with a file" {
						if err := os.WriteFile(filepath.Join(target, "theirs.txt"), []byte("x"), 0o600); err != nil {
							t.Fatal(err)
						}
					}
				}
			})
			_, err := ensurePersonFiles(root, "parf", uid, gid)
			if kind == "link" {
				if err == nil || !strings.Contains(err.Error(), "link or not a directory") {
					t.Fatalf("a link made at the name: %v", err)
				}
			} else if err != nil {
				t.Fatalf("a directory of the right owner at the name is checked and kept: %v", err)
			}
			target := filepath.Join(root, "users", "parf", "files")
			if kind == "link" && modeOf(t, outside).Perm() != 0o755 {
				t.Fatal("the mode of what the link led to changed")
			}
			if kind != "link" && mustStat(t, target).Ino != planted {
				t.Fatal("the new directory replaced the one that was at the name")
			}
			if kind == "directory with a file" {
				if b, _ := os.ReadFile(filepath.Join(target, "theirs.txt")); string(b) != "x" {
					t.Fatalf("what was at the name was replaced: %q", b)
				}
			}
			if kind != "link" && modeOf(t, target).Perm() != 0o700 {
				t.Fatalf("the directory that was there was not tightened: %v", modeOf(t, target))
			}
			if left := usersEntries(t, root); len(left) != 1 || left[0] != "parf" {
				t.Fatalf("a staging directory was left behind: %v", left)
			}
		})
	}
}

func TestPersonFilesSweepsOnlyStaleEmptyStagingDirectories(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	users := filepath.Join(root, "users")
	mustMkdir(t, users, 0o755)
	old := time.Now().Add(-time.Hour)
	for _, name := range []string{".new-stale", ".new-full", "keep"} {
		mustMkdir(t, filepath.Join(users, name), 0o700)
		if err := os.Chtimes(filepath.Join(users, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(users, ".new-full", "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(users, ".new-fresh"), 0o700)
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(usersEntries(t, root), " ")
	if got != ".new-fresh .new-full keep parf" {
		t.Fatalf("after the sweep users/ holds %q; only the stale empty staging directory goes", got)
	}
}

func TestPersonFilesReadyAndTightenCountTheSpecialModeBits(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	dir := personFilesDir(root, "parf")
	if err := os.Chmod(dir, os.ModeSetgid|0o700); err != nil {
		t.Fatal(err)
	}
	if personFilesReady(root, "parf", uid) {
		t.Fatal("a setgid folder counts as ready")
	}
	notes, err := ensurePersonFiles(root, "parf", uid, gid)
	if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "from 02700 to 0700") {
		t.Fatalf("tighten a special bit: %v %v", notes, err)
	}
	if fullMode(modeOf(t, dir)) != 0o700 {
		t.Fatalf("mode %v", modeOf(t, dir))
	}
}

func TestPersonFilesLeavesAnExistingFoldersGroupAndSaysSo(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	notes, err := ensurePersonFiles(root, "parf", uid, gid+4242)
	if err != nil || len(notes) != 2 || !strings.Contains(notes[0], "left as it is") {
		t.Fatalf("a folder of another group: %v %v", notes, err)
	}
	if got := int(mustStat(t, personFilesDir(root, "parf")).Gid); got != gid {
		t.Fatalf("an existing folder's group was changed to %d", got)
	}
}

func TestPersonFilesNeverChownsADirectoryThatWasThere(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fchown = func(fd, u, g int) error { calls++; return unix.Fchown(fd, u, g) }
	t.Cleanup(func() { fchown = unix.Fchown })
	if err := os.Chmod(personFilesDir(root, "parf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePersonFiles(root, "parf", uid, gid+4242); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("an existing directory was chowned %d times", calls)
	}
}

func TestPersonFilesSweepsStateRootStagingLeftBeforeUsersWasCreated(t *testing.T) {
	root, uid, gid := personFilesFixture(t)
	old := time.Now().Add(-time.Hour)
	for _, name := range []string{".new-stale", ".new-full", "keep"} {
		path := filepath.Join(root, name)
		mustMkdir(t, path, 0700)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".new-full", "kept"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(root, ".new-full"), old, old); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(root, ".new-fresh"), 0700)
	outside := t.TempDir()
	mustSymlink(t, outside, filepath.Join(root, ".new-link"))
	if err := os.WriteFile(filepath.Join(root, ".new-file"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePersonFiles(root, "parf", uid, gid); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if got := strings.Join(names, " "); got != ".new-file .new-fresh .new-full .new-link keep users" {
		t.Fatalf("state root after recovery = %q; only stale empty staging must go", got)
	}
	if body, err := os.ReadFile(filepath.Join(root, ".new-full", "kept")); err != nil || string(body) != "keep" {
		t.Fatalf("nonempty state-root staging changed: %q %v", body, err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("staging symlink target changed: %v", err)
	}
}
