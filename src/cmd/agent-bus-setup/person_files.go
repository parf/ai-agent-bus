package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// Each person set up gets a folder of their own for files they hand to the
// bus: <state>/users/<person>/files, the place to point AGENT_BUS_BIGFILE_DIR.
// Nothing sets that variable for them (docs/03-records-kv.md#bigfiles).
//
// users/ is root's (0755). users/<person> and its files folder belong to the
// person, 0700. The person can replace anything under their own folder, so
// root never follows a path there: every step is a directory opened from its
// parent with no link followed, and the owner and mode are read and set on the
// open directory, never on a name. A link, a non-directory or another
// person's folder is refused; a folder that is the person's with another mode
// (a looser one is the case that matters) is set to 0700 and said so. The contents are never touched.

const personFilesMode = 0o700

// users/ belongs to root; a test, which cannot be root, names its own owner.
var personFilesRootUID, personFilesRootGID = 0, 0

// personFilesDir is where a person's files folder is.
func personFilesDir(root, person string) string {
	return filepath.Join(root, "users", person, "files")
}

// personFilesReady says whether the three directories already are what
// ensurePersonFiles would leave; any doubt (a link, a wrong owner or mode) is
// false, so the action runs and refuses by name.
func personFilesReady(root, person string, uid, gid int) bool {
	for _, d := range []struct {
		path     string
		uid, gid int
		mode     os.FileMode
	}{
		{filepath.Join(root, "users"), personFilesRootUID, personFilesRootGID, 0o755},
		{filepath.Join(root, "users", person), uid, gid, personFilesMode},
		{personFilesDir(root, person), uid, gid, personFilesMode},
	} {
		fi, err := os.Lstat(d.path)
		if err != nil || !fi.IsDir() || fi.Mode().Perm() != d.mode {
			return false
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != d.uid || int(st.Gid) != d.gid {
			return false
		}
	}
	return true
}

// ensurePersonFiles makes users/<person>/files under root, for the account
// with uid and gid, and returns what it changed on a folder that existed.
func ensurePersonFiles(root, person string, uid, gid int) ([]string, error) {
	if !personName.MatchString(person) {
		return nil, fmt.Errorf("a person's files folder needs a plain name, not %q", person)
	}
	if uid <= 0 {
		return nil, fmt.Errorf("refusing uid %d for %s's files folder", uid, person)
	}
	parent, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("%s is not a directory I can open without following a link: %w", root, err)
	}
	var notes []string
	path := root
	for _, d := range []struct {
		name     string
		uid, gid int
		mode     uint32
	}{{"users", personFilesRootUID, personFilesRootGID, 0o755}, {person, uid, gid, personFilesMode}, {"files", uid, gid, personFilesMode}} {
		path = filepath.Join(path, d.name)
		fd, note, err := ensureDirAt(parent, path, d.name, d.uid, d.gid, d.mode)
		unix.Close(parent)
		if err != nil {
			return notes, err
		}
		notes = append(notes, note...)
		parent = fd
	}
	unix.Close(parent)
	return notes, nil
}

// ensureDirAt opens name under the directory parent, making it if absent. It
// returns the open directory. The owner must already be uid, except that the
// group and the mode are put right; a new one is made with that owner and mode.
func ensureDirAt(parent int, path, name string, uid, gid int, mode uint32) (int, []string, error) {
	var st unix.Stat_t
	created := false
	err := unix.Fstatat(parent, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	switch {
	case errors.Is(err, unix.ENOENT):
		if err := unix.Mkdirat(parent, name, 0o700); err != nil {
			return -1, nil, fmt.Errorf("make %s: %w", path, err)
		}
		created = true
	case err != nil:
		return -1, nil, fmt.Errorf("look at %s: %w", path, err)
	}
	fd, err := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, nil, fmt.Errorf("%s is a link or not a directory; move it away first: %w", path, err)
	}
	if err := unix.Fstat(fd, &st); err != nil {
		unix.Close(fd)
		return -1, nil, fmt.Errorf("look at %s: %w", path, err)
	}
	var notes []string
	if created {
		if err := unix.Fchown(fd, uid, gid); err != nil {
			unix.Close(fd)
			return -1, nil, fmt.Errorf("give %s to uid %d: %w", path, uid, err)
		}
	} else if int(st.Uid) != uid {
		unix.Close(fd)
		return -1, nil, fmt.Errorf("%s belongs to uid %d, not uid %d; refusing to change another owner's folder", path, st.Uid, uid)
	} else if int(st.Gid) != gid {
		if err := unix.Fchown(fd, uid, gid); err != nil {
			unix.Close(fd)
			return -1, nil, fmt.Errorf("set the group of %s: %w", path, err)
		}
		notes = append(notes, fmt.Sprintf("set the group of %s to %d", path, gid))
	}
	if created || st.Mode&0o7777 != mode {
		if err := unix.Fchmod(fd, mode); err != nil {
			unix.Close(fd)
			return -1, nil, fmt.Errorf("set the mode of %s: %w", path, err)
		}
		if !created {
			notes = append(notes, fmt.Sprintf("set the mode of %s from %#o to %#o", path, st.Mode&0o7777, mode))
		}
	}
	return fd, notes, nil
}
