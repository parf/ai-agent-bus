package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Each person set up gets a folder of their own for files they hand to the
// bus: <state>/users/<person>/files, the place to point AGENT_BUS_BIGFILE_DIR.
// Nothing sets that variable for them (docs/03-records-kv.md#bigfiles).
//
// users/ is root's (0755). users/<person> and its files folder belong to the
// person, 0700. The person can replace anything under their own folder, so
// root never follows a path there and never changes the owner of something it
// did not just make itself:
//
//   - A new directory is made under a random name in users/, which only root
//     can write, given its owner and mode through its open descriptor, and only
//     then renamed into place without replacing anything. Nothing the person
//     does can reach it before it is theirs.
//   - A directory that already exists is opened from its parent with no link
//     followed, its owner is checked on the open descriptor, and only its mode
//     is set (fchmod). Its owner is never changed. Another owner, a link or a
//     non-directory is refused; a mode other than 0700 is set to 0700 and said;
//     a group other than the person's is left and said.
//
// The contents are never touched.

const (
	personFilesMode = 0o700
	stagePrefix     = ".new-"
	staleAfter      = time.Minute
)

// users/ belongs to root; a test, which cannot be root, names its own owner.
var personFilesRootUID, personFilesRootGID = 0, 0

// beforePublish is a test hook: it runs after a new directory is made and
// given its owner, just before it is renamed into place, which is where a
// person could race by making the name themselves.
var beforePublish = func(stage int, tmp string, parent int, name string) {}

// fchown is the one place an owner is changed, a variable so a test can see it is never used on a directory that was there.
var fchown = unix.Fchown

// afterStage is a test hook that sees the directory a new one was made in.
var afterStage = func(stage int, name string) {}

// personFilesDir is where a person's files folder is.
func personFilesDir(root, person string) string {
	return filepath.Join(root, "users", person, "files")
}

// fullMode is the permission bits and the setuid, setgid and sticky bits.
func fullMode(m os.FileMode) os.FileMode {
	return m & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
}

// personFilesReady says whether the three directories already are what
// ensurePersonFiles would leave: the right owner and the exact mode, special
// bits included. Any doubt (a link, a wrong owner or mode) is false, so the
// action runs and refuses or corrects by name.
func personFilesReady(root, person string, uid int) bool {
	for _, d := range []struct {
		path string
		uid  int
		mode os.FileMode
	}{
		{filepath.Join(root, "users"), personFilesRootUID, 0o755},
		{filepath.Join(root, "users", person), uid, personFilesMode},
		{personFilesDir(root, person), uid, personFilesMode},
	} {
		fi, err := os.Lstat(d.path)
		if err != nil || !fi.IsDir() || fullMode(fi.Mode()) != d.mode {
			return false
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != d.uid {
			return false
		}
	}
	return true
}

// ensurePersonFiles makes users/<person>/files under root, for the account
// with uid and gid, and returns what it said about a folder that existed.
func ensurePersonFiles(root, person string, uid, gid int) ([]string, error) {
	if !personName.MatchString(person) {
		return nil, fmt.Errorf("a person's files folder needs a plain name, not %q", person)
	}
	if uid <= 0 {
		return nil, fmt.Errorf("refusing uid %d for %s's files folder", uid, person)
	}
	rootFd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("%s is not a directory I can open without following a link: %w", root, err)
	}
	defer unix.Close(rootFd)
	sweepStale(rootFd)
	var notes []string
	usersFd, note, err := ensureDirAt(rootFd, rootFd, filepath.Join(root, "users"), "users", personFilesRootUID, personFilesRootGID, 0o755)
	if err != nil {
		return nil, err
	}
	defer unix.Close(usersFd)
	notes = append(notes, note...)
	sweepStale(usersFd)
	// users/<person> is made inside users/, which only root writes, so it can be made in place.
	personFd, note, err := ensureDirAt(usersFd, usersFd, filepath.Join(root, "users", person), person, uid, gid, personFilesMode)
	if err != nil {
		return notes, err
	}
	defer unix.Close(personFd)
	notes = append(notes, note...)
	// files is inside the person's own folder: made in users/ and renamed in.
	filesFd, note, err := ensureDirAt(personFd, usersFd, personFilesDir(root, person), "files", uid, gid, personFilesMode)
	if err != nil {
		return notes, err
	}
	unix.Close(filesFd)
	return append(notes, note...), nil
}

// ensureDirAt opens name under parent, making it if absent. stage is a
// directory only root can write, on the same filesystem, where a new one is
// made before it is renamed into place.
func ensureDirAt(parent, stage int, path, name string, uid, gid int, mode uint32) (int, []string, error) {
	var st unix.Stat_t
	err := unix.Fstatat(parent, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	switch {
	case errors.Is(err, unix.ENOENT):
		fd, published, err := makeAndPublish(parent, stage, path, name, uid, gid, mode)
		if err != nil {
			return -1, nil, err
		}
		if published {
			return fd, nil, nil
		}
		// The name appeared while the new one was made: it is somebody's, so it is checked, not replaced.
	case err != nil:
		return -1, nil, fmt.Errorf("look at %s: %w", path, err)
	}
	return openExisting(parent, path, name, uid, gid, mode)
}

// makeAndPublish makes the directory under a random name in stage, gives it
// its owner and mode through its descriptor, and renames it to name under
// parent without replacing anything. It reports false when name already exists.
func makeAndPublish(parent, stage int, path, name string, uid, gid int, mode uint32) (int, bool, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return -1, false, err
	}
	tmp := stagePrefix + hex.EncodeToString(random[:])
	if err := unix.Mkdirat(stage, tmp, 0o700); err != nil {
		return -1, false, fmt.Errorf("make %s: %w", path, err)
	}
	afterStage(stage, tmp)
	fd, err := unix.Openat(stage, tmp, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		unix.Unlinkat(stage, tmp, unix.AT_REMOVEDIR)
		return -1, false, fmt.Errorf("open the new %s: %w", path, err)
	}
	drop := func() { unix.Close(fd); unix.Unlinkat(stage, tmp, unix.AT_REMOVEDIR) }
	if err := fchown(fd, uid, gid); err != nil {
		drop()
		return -1, false, fmt.Errorf("give %s to uid %d: %w", path, uid, err)
	}
	if err := unix.Fchmod(fd, mode); err != nil {
		drop()
		return -1, false, fmt.Errorf("set the mode of %s: %w", path, err)
	}
	beforePublish(stage, tmp, parent, name)
	switch err := renameNoReplace(stage, tmp, parent, name); {
	case err == nil:
		return fd, true, nil
	case errors.Is(err, unix.EEXIST) || errors.Is(err, unix.ENOTEMPTY):
		drop()
		return -1, false, nil
	default:
		drop()
		return -1, false, fmt.Errorf("put %s in place: %w", path, err)
	}
}

// openExisting opens a directory that was there, checks its owner on the open
// descriptor, and sets only its mode. It never changes an owner.
func openExisting(parent int, path, name string, uid, gid int, mode uint32) (int, []string, error) {
	fd, err := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, nil, fmt.Errorf("%s is a link or not a directory; move it away first: %w", path, err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		unix.Close(fd)
		return -1, nil, fmt.Errorf("look at %s: %w", path, err)
	}
	if int(st.Uid) != uid {
		unix.Close(fd)
		return -1, nil, fmt.Errorf("%s belongs to uid %d, not uid %d; refusing to use another owner's folder", path, st.Uid, uid)
	}
	var notes []string
	if int(st.Gid) != gid {
		notes = append(notes, fmt.Sprintf("%s has group %d, not %d; left as it is", path, st.Gid, gid))
	}
	if uint32(st.Mode)&0o7777 != mode {
		if err := unix.Fchmod(fd, mode); err != nil {
			unix.Close(fd)
			return -1, nil, fmt.Errorf("set the mode of %s: %w", path, err)
		}
		notes = append(notes, fmt.Sprintf("set the mode of %s from %#o to %#o", path, uint32(st.Mode)&0o7777, mode))
	}
	return fd, notes, nil
}

// sweepStale removes the leftovers of a run that stopped between making a
// directory and renaming it into place: empty directories in users/ with the
// staging prefix, owned by this users/'s owner, older than a minute.
func sweepStale(usersFd int) {
	dup, err := unix.Dup(usersFd)
	if err != nil {
		return
	}
	dir := os.NewFile(uintptr(dup), "users")
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		if !strings.HasPrefix(name, stagePrefix) {
			continue
		}
		var st unix.Stat_t
		if unix.Fstatat(usersFd, name, &st, unix.AT_SYMLINK_NOFOLLOW) != nil || uint32(st.Mode)&unix.S_IFMT != unix.S_IFDIR ||
			int(st.Uid) != personFilesRootUID || time.Since(modTime(&st)) < staleAfter {
			continue
		}
		unix.Unlinkat(usersFd, name, unix.AT_REMOVEDIR) // only an empty one goes
	}
}

func modTime(st *unix.Stat_t) time.Time { return time.Unix(st.Mtim.Sec, st.Mtim.Nsec) }
