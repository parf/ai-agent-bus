package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

const upgradeJournalName = "upgrade.json"

type upgradeJournal struct {
	OldRelease string `json:"old_release"`
	NewRelease string `json:"new_release"`
	Backup     string `json:"backup,omitempty"`
	UnitSHA256 string `json:"unit_sha256"`
}

func upgradeBundle(b checkedBundle) error {
	if _, err := os.Stat(upgradeJournalPath()); err == nil {
		return fmt.Errorf("an interrupted upgrade is recorded; run agent-bus-setup --recover first")
	} else if !os.IsNotExist(err) {
		return err
	}
	oldID, old, unit, addr, err := upgradePreflight(b)
	if err != nil {
		return err
	}
	if oldID == b.releaseID {
		return fmt.Errorf("release %s is already current; use setup without --upgrade to repair it", oldID)
	}
	if _, err := stageBundle(b); err != nil {
		return fmt.Errorf("stage release: %w", err)
	}
	if err := installCommandLinks(); err != nil {
		return err
	}
	if err := enoughBackupSpace(svcHome, stateRoot); err != nil {
		return err
	}
	j := upgradeJournal{OldRelease: oldID, NewRelease: b.releaseID, UnitSHA256: bytesSHA256(unit)}
	if err := writeUpgradeJournal(j); err != nil {
		return err
	}
	if err := run("systemctl", "stop", "agent-busd"); err != nil {
		if startErr := startDaemon(); startErr == nil {
			_ = os.Remove(upgradeJournalPath())
			return fmt.Errorf("stop old release: %w; old release is serving", err)
		} else {
			return fmt.Errorf("stop old release: %w; restart also failed: %v; recovery marker retained", err, startErr)
		}
	}
	backup := filepath.Join(stateRoot, "backups", fmt.Sprintf("%s-%d-%s", time.Now().UTC().Format("20060102T150405Z"), os.Getpid(), oldID))
	if err := copyTreeAtomic(svcHome, backup); err != nil {
		if startErr := startDaemon(); startErr == nil {
			_ = os.Remove(upgradeJournalPath())
			return fmt.Errorf("back up stopped daemon state: %w; old release is serving", err)
		} else {
			return fmt.Errorf("back up stopped daemon state: %w; restart also failed: %v; recovery marker retained", err, startErr)
		}
	}
	j.Backup = backup
	if err := writeUpgradeJournal(j); err != nil {
		if startErr := startDaemon(); startErr == nil {
			_ = os.Remove(upgradeJournalPath())
		}
		return err
	}
	if _, err := selectBundle(b.releaseID); err != nil {
		return rollbackUpgrade(j, addr, fmt.Errorf("select new release: %w", err))
	}
	if err := startDaemon(); err != nil {
		return rollbackUpgrade(j, addr, fmt.Errorf("start new release: %w", err))
	}
	if err := waitIdentity(addr, b.releaseIDVersion(), 10*time.Second); err != nil {
		return rollbackUpgrade(j, addr, fmt.Errorf("verify new release: %w", err))
	}
	if err := waitRunningRelease(b.releaseID, 10*time.Second); err != nil {
		return rollbackUpgrade(j, addr, fmt.Errorf("verify running release: %w", err))
	}
	unitAfter, err := os.ReadFile(unitPath)
	if err != nil || bytesSHA256(unitAfter) != j.UnitSHA256 {
		if err == nil {
			err = fmt.Errorf("unit changed during upgrade")
		}
		return rollbackUpgrade(j, addr, fmt.Errorf("verify operator configuration: %w", err))
	}
	if err := os.Remove(upgradeJournalPath()); err != nil {
		return fmt.Errorf("upgrade succeeded but the recovery marker remains: %w", err)
	}
	if err := syncDir(installRoot); err != nil {
		return err
	}
	fmt.Printf("upgraded agent-bus from %s to %s; state backup: %s\n", old.releaseIDVersion(), b.releaseIDVersion(), backup)
	return nil
}

func recoverUpgrade() error {
	b, err := os.ReadFile(upgradeJournalPath())
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no interrupted upgrade is recorded")
		}
		return err
	}
	var j upgradeJournal
	if err := json.Unmarshal(b, &j); err != nil || j.OldRelease == "" || j.NewRelease == "" {
		if err != nil {
			return fmt.Errorf("invalid upgrade recovery marker: %w", err)
		}
		return fmt.Errorf("invalid upgrade recovery marker: release fields are missing")
	}
	unit, err := os.ReadFile(unitPath)
	if err != nil {
		return err
	}
	if bytesSHA256(unit) != j.UnitSHA256 {
		return fmt.Errorf("refusing recovery: %s changed after the upgrade began", unitPath)
	}
	addr, err := unitAddress(unit)
	if err != nil {
		return err
	}
	if err := performRollback(j, addr); err != nil {
		return err
	}
	fmt.Printf("recovered interrupted upgrade; restored agent-bus %s with its consistent state\n", releaseVersion(j.OldRelease))
	return nil
}

func upgradePreflight(next checkedBundle) (string, checkedBundle, []byte, string, error) {
	oldID, err := currentReleaseID()
	if err != nil {
		return "", checkedBundle{}, nil, "", fmt.Errorf("read current release: %w", err)
	}
	oldRoot := filepath.Join(installRoot, "releases", oldID)
	old, err := checkBundleVersion(oldRoot, "")
	if err != nil {
		return "", checkedBundle{}, nil, "", fmt.Errorf("current release is not a valid rollback target: %w", err)
	}
	if old.releaseID != oldID {
		return "", checkedBundle{}, nil, "", fmt.Errorf("current release name %s does not match its manifest %s", oldID, old.releaseID)
	}
	if err := preflightCommandLinks(); err != nil {
		return "", checkedBundle{}, nil, "", err
	}
	unit, err := os.ReadFile(unitPath)
	if err != nil {
		return "", checkedBundle{}, nil, "", fmt.Errorf("read installed unit: %w", err)
	}
	wantExec := "ExecStart=" + filepath.Join(installRoot, "current", "agent-busd") + " "
	if !strings.Contains(string(unit), wantExec) {
		return "", checkedBundle{}, nil, "", fmt.Errorf("refusing upgrade: %s does not start the managed current release", unitPath)
	}
	addr, err := unitAddress(unit)
	if err != nil {
		return "", checkedBundle{}, nil, "", err
	}
	if err := run("systemctl", "is-active", "--quiet", "agent-busd"); err != nil {
		return "", checkedBundle{}, nil, "", fmt.Errorf("refusing upgrade: agent-busd is not active: %w", err)
	}
	return oldID, old, unit, addr, nil
}

func rollbackUpgrade(j upgradeJournal, addr string, cause error) error {
	if err := performRollback(j, addr); err != nil {
		return fmt.Errorf("%v; rollback failed: %w; run agent-bus-setup --recover", cause, err)
	}
	return fmt.Errorf("%v; rolled back to %s with its consistent state", cause, releaseVersion(j.OldRelease))
}

func performRollback(j upgradeJournal, addr string) error {
	_ = run("systemctl", "stop", "agent-busd")
	if _, err := selectBundle(j.OldRelease); err != nil {
		return fmt.Errorf("could not select %s: %w", j.OldRelease, err)
	}
	if j.Backup != "" {
		if err := restoreTreeAtomic(j.Backup, svcHome); err != nil {
			return fmt.Errorf("could not restore state: %w", err)
		}
	}
	if err := startDaemon(); err != nil {
		return fmt.Errorf("could not start %s: %w", j.OldRelease, err)
	}
	if err := waitIdentity(addr, releaseVersion(j.OldRelease), 10*time.Second); err != nil {
		return fmt.Errorf("rollback start was not healthy: %w", err)
	}
	if err := waitRunningRelease(j.OldRelease, 10*time.Second); err != nil {
		return fmt.Errorf("rollback processes are inconsistent: %w", err)
	}
	if err := os.Remove(upgradeJournalPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rollback succeeded but its recovery marker remains: %w", err)
	}
	_ = syncDir(installRoot)
	return nil
}

func startDaemon() error {
	// A start-broken candidate may exhaust systemd's burst limit before the
	// health deadline. Rollback is the recovery path and must not inherit the
	// failed candidate's limiter state.
	if err := run("systemctl", "reset-failed", "agent-busd"); err != nil {
		return err
	}
	return run("systemctl", "start", "agent-busd")
}

func (b checkedBundle) releaseIDVersion() string { return releaseVersion(b.releaseID) }

func releaseVersion(releaseID string) string {
	if i := strings.IndexByte(releaseID, '-'); i >= 0 {
		return releaseID[:i]
	}
	return releaseID
}

func unitAddress(unit []byte) (string, error) {
	for _, line := range strings.Split(string(unit), "\n") {
		if !strings.HasPrefix(line, "ExecStart=") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "ExecStart="))
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "-addr" {
				return fields[i+1], nil
			}
		}
	}
	return "", fmt.Errorf("refusing upgrade: %s has no explicit daemon -addr", unitPath)
}

func waitIdentity(addr, wantVersion string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	var last error
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/identity")
		if err == nil {
			var got protocol.NodeIdentity
			err = json.NewDecoder(resp.Body).Decode(&got)
			_ = resp.Body.Close()
			if err == nil && resp.StatusCode == http.StatusOK && got.Version == wantVersion {
				return nil
			}
			if err == nil {
				err = fmt.Errorf("identity reports version %q, want %q", got.Version, wantVersion)
			}
		}
		last = err
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("identity at %s was not healthy: %w", addr, last)
}

func waitRunningRelease(releaseID string, timeout time.Duration) error {
	wantRoot := filepath.Join(installRoot, "releases", releaseID)
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		mainPID, err := commandOutput("systemctl", "show", "agent-busd", "-p", "MainPID", "--value")
		if err == nil && strings.TrimSpace(mainPID) != "0" {
			cgroup, cgErr := commandOutput("systemctl", "show", "agent-busd", "-p", "ControlGroup", "--value")
			if cgErr == nil {
				pids, readErr := cgroupPIDs(filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(strings.TrimSpace(cgroup), "/")))
				if readErr == nil {
					daemons, webs, webBinds := 0, 0, 0
					wrong := ""
					for _, pid := range pids {
						exe, linkErr := os.Readlink(filepath.Join("/proc", pid, "exe"))
						if linkErr != nil {
							continue
						}
						switch filepath.Base(exe) {
						case "agent-busd":
							daemons++
							if exe != filepath.Join(wantRoot, "agent-busd") {
								wrong = exe
							}
						case "agent-bus-web":
							webs++
							if exe != filepath.Join(wantRoot, "agent-bus-web") && exe != "/agent-bus-web" {
								wrong = exe
							}
						}
						cmdline, _ := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
						args := strings.ReplaceAll(string(cmdline), "\x00", " ")
						if strings.Contains(args, filepath.Join(wantRoot, "agent-bus-web")) && strings.Contains(args, " /agent-bus-web") {
							webBinds++
						}
					}
					if wrong == "" && daemons >= 2 && webs >= 1 && webBinds >= 1 {
						return nil
					}
					last = fmt.Errorf("release processes: daemons=%d web=%d web-bind=%d wrong=%q", daemons, webs, webBinds, wrong)
				} else {
					last = readErr
				}
			} else {
				last = cgErr
			}
		} else {
			last = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("unit did not run only %s components: %w", releaseVersion(releaseID), last)
}

func cgroupPIDs(root string) ([]string, error) {
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() != "cgroup.procs" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, pid := range strings.Fields(string(b)) {
			seen[pid] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(seen))
	for pid := range seen {
		out = append(out, pid)
	}
	return out, nil
}

func commandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func upgradeJournalPath() string { return filepath.Join(installRoot, upgradeJournalName) }

func writeUpgradeJournal(j upgradeJournal) error {
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		return err
	}
	tmp := upgradeJournalPath() + fmt.Sprintf(".tmp-%d", os.Getpid())
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, upgradeJournalPath()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDir(installRoot)
}

func bytesSHA256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func enoughBackupSpace(source, destinationRoot string) error {
	need, err := treeBytes(source)
	if err != nil {
		return err
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(destinationRoot, &fs); err != nil {
		return err
	}
	available := uint64(fs.Bavail) * uint64(fs.Bsize)
	// Keep room for the backup and a second copy used by atomic recovery.
	if available < uint64(need)*2 {
		return fmt.Errorf("not enough free space for upgrade recovery: need %d bytes, have %d", need*2, available)
	}
	return nil
}

func treeBytes(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func copyTreeAtomic(source, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("backup already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	stage := destination + fmt.Sprintf(".stage-%d", os.Getpid())
	_ = os.RemoveAll(stage)
	if err := copyTree(source, stage); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	if err := os.Rename(stage, destination); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	return syncDir(filepath.Dir(destination))
}

func restoreTreeAtomic(backup, destination string) error {
	stage := destination + fmt.Sprintf(".restore-%d", os.Getpid())
	failed := destination + fmt.Sprintf(".failed-%d", os.Getpid())
	_ = os.RemoveAll(stage)
	_ = os.RemoveAll(failed)
	if err := copyTree(backup, stage); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	if err := os.Rename(destination, failed); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	if err := os.Rename(stage, destination); err != nil {
		_ = os.Rename(failed, destination)
		_ = os.RemoveAll(stage)
		return err
	}
	if err := syncDir(filepath.Dir(destination)); err != nil {
		return err
	}
	return os.RemoveAll(failed)
}

func copyTree(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(destination, rel)
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("read ownership for %s", path)
		}
		switch {
		case info.IsDir():
			if err := os.Mkdir(dst, info.Mode().Perm()); err != nil && !os.IsExist(err) {
				return err
			}
			if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
				return err
			}
			if err := os.Chown(dst, int(stat.Uid), int(stat.Gid)); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyRegular(path, dst, info.Mode().Perm()); err != nil {
				return err
			}
			if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
				return err
			}
			if err := os.Chown(dst, int(stat.Uid), int(stat.Gid)); err != nil {
				return err
			}
			if err := os.Chtimes(dst, info.ModTime(), info.ModTime()); err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dst); err != nil {
				return err
			}
			if err := os.Lchown(dst, int(stat.Uid), int(stat.Gid)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("refusing special file in daemon state: %s", path)
		}
		return nil
	})
}

func copyRegular(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	if copyErr == nil {
		copyErr = out.Sync()
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func syncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
