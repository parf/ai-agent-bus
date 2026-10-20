package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/parf/ai-agent-bus/internal/version"
)

var (
	installRoot = "/usr/local/lib/agent-bus"
	installBin  = "/usr/local/bin"
)

var bundleFiles = []string{
	"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-runner", "agent-bus-setup", "agent-bus-token",
	"web/server.ts", "web/agent-bus-web.service",
	"mcp/server.js", "launchers/launcher.js",
	"launchers/ab-claude", "launchers/ab-codex", "launchers/ab-opencode", "launchers/ab-opencode1",
	"internal/version/VERSION", "LICENSE.md", "NOTICE.md", "INSTALL.md",
	"agents/slack/agent.json",
	"agents/slack/env.dist",
	"agents/slack/slack.js",
	"agents/slack/filter-worker.js",
	"agents/telegram/agent.json",
	"agents/telegram/env.dist",
	"agents/telegram/telegram.js",
	"agents/telegram/filter-worker.js",
	"agents/debug/agent.json",
	"agents/debug/env.dist",
	"agents/debug/debug.py",
	"agents/shared/metadata.py",
	"agents/LICENSE.md",
	"agents/NOTICE.md",
	"agents/standalone/install.py",
	"agents/standalone/systemd/agent.json",
	"agents/standalone/systemd/env.dist",
	"agents/standalone/systemd/systemd.py",
	"agents/standalone/systemd/install.py",
	"agents/standalone/server-info/agent.json",
	"agents/standalone/server-info/env.dist",
	"agents/standalone/server-info/server-info.py",
	"agents/standalone/server-info/install.sh",
	"agents/standalone/git/agent.json",
	"agents/standalone/git/env.dist",
	"agents/standalone/git/git_agent.py",
	"agents/standalone/git/install.py",
	"agents/standalone/git/autopull.py",
	"agents/standalone/git/timers.py",
	"agents/standalone/logwatch/agent.json",
	"agents/standalone/logwatch/env.dist",
	"agents/standalone/logwatch/logwatch.py",
	"agents/standalone/logwatch/install.py",
	"agents/standalone/logwatch/cleaning.py",
	"agents/standalone/logwatch/filters.py",
	"agents/standalone/logwatch/watcher.py",
	"clients/LICENSE.md",
	"clients/python/agent_bus/__init__.py",
	"clients/python/agent_bus/_version.py",
	"clients/python/agent_bus/client.py",
	"clients/python/agent_bus/errors.py",
	"clients/python/agent_bus/message.py",
	"clients/python/agent_bus/storage.py",
	"clients/python/agent_bus/transport.py",
}

// rollbackFiles is what an installed release needs to be run again. An older
// release is held to its own manifest, not to today's bundle: before 0.8.50
// it shipped the Go agent-bus-web binary and no web/ tree.
var rollbackFiles = []string{
	"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-setup", "agent-bus-token",
	"internal/version/VERSION",
}

var legacyFiles = map[string]bool{"agent-bus-web": true}

var executableFiles = map[string]bool{
	"agents/slack/slack.js":                        true,
	"agents/telegram/telegram.js":                  true,
	"agents/debug/debug.py":                        true,
	"agents/standalone/install.py":                 true,
	"agents/standalone/systemd/systemd.py":         true,
	"agents/standalone/systemd/install.py":         true,
	"agents/standalone/server-info/server-info.py": true,
	"agents/standalone/server-info/install.sh":     true,
	"agents/standalone/git/git_agent.py":           true,
	"agents/standalone/git/install.py":             true,
	"agents/standalone/logwatch/logwatch.py":       true,
	"agents/standalone/logwatch/install.py":        true,
	"agent-bus":                                    true, "agent-busd": true, "agent-bus-admin": true, "agent-bus-runner": true,
	"agent-bus-setup": true, "agent-bus-token": true,
	"launchers/ab-claude": true, "launchers/ab-codex": true, "launchers/ab-opencode": true, "launchers/ab-opencode1": true,
}

type checkedBundle struct {
	root       string
	releaseID  string
	manifest   []byte
	fileHashes map[string]string
}

func checkBundle(root string) (checkedBundle, error) {
	b, err := checkBundleVersion(root, version.String)
	if err != nil {
		return checkedBundle{}, err
	}
	return b, nil
}

// checkBundleVersion verifies the exact package allow-list. An empty expected
// version is used only for an already-installed release: an upgrade has to
// verify the old release before it can trust it as a rollback target.
func checkBundleVersion(root, expectedVersion string) (checkedBundle, error) {
	var b checkedBundle
	b.root = root
	manifestPath := filepath.Join(root, "MANIFEST.sha256")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return b, fmt.Errorf("package is incomplete: read MANIFEST.sha256: %w", err)
	}
	b.manifest = manifest
	b.fileHashes = make(map[string]string)
	s := bufio.NewScanner(strings.NewReader(string(manifest)))
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) != 2 || len(parts[0]) != sha256.Size*2 {
			return b, fmt.Errorf("package has an invalid manifest line: %q", s.Text())
		}
		name := strings.TrimPrefix(parts[1], "*")
		if filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") {
			return b, fmt.Errorf("package manifest has unsafe path %q", name)
		}
		if _, exists := b.fileHashes[name]; exists {
			return b, fmt.Errorf("package manifest repeats %s", name)
		}
		if _, err := hex.DecodeString(parts[0]); err != nil {
			return b, fmt.Errorf("package manifest has invalid digest for %s", name)
		}
		b.fileHashes[name] = strings.ToLower(parts[0])
	}
	if err := s.Err(); err != nil {
		return b, err
	}
	// The required artifacts, plus the web face's own sources under web/ —
	// a tree whose files change with the face — and nothing else.
	needed := bundleFiles
	allowed := map[string]bool{}
	if expectedVersion == "" {
		needed = rollbackFiles
		for _, name := range bundleFiles {
			allowed[name] = true
		}
		for name := range legacyFiles {
			allowed[name] = true
		}
	}
	required := map[string]bool{}
	for _, name := range needed {
		required[name] = true
		if _, ok := b.fileHashes[name]; !ok {
			return b, fmt.Errorf("package is incomplete: %s is not in MANIFEST.sha256", name)
		}
	}
	for name := range b.fileHashes {
		if !required[name] && !allowed[name] && !strings.HasPrefix(name, "web/") && !strings.HasPrefix(name, "agents/debug/") {
			return b, fmt.Errorf("package manifest names %s, which is not a release artifact", name)
		}
	}
	for _, name := range b.files() {
		want := b.fileHashes[name]
		path := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if err != nil {
			return b, fmt.Errorf("package is incomplete: %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return b, fmt.Errorf("package artifact %s is not a regular file", name)
		}
		if (executableFiles[name] || legacyFiles[name]) && info.Mode().Perm()&0o111 == 0 {
			return b, fmt.Errorf("package artifact %s is not executable", name)
		}
		got, err := fileSHA256(path)
		if err != nil {
			return b, err
		}
		if got != want {
			return b, fmt.Errorf("package artifact %s does not match MANIFEST.sha256", name)
		}
	}
	v, err := os.ReadFile(filepath.Join(root, "internal/version/VERSION"))
	if err != nil {
		return b, err
	}
	bundleVersion := strings.TrimSpace(string(v))
	if expectedVersion != "" && bundleVersion != expectedVersion {
		return b, fmt.Errorf("package version %q does not match setup %q", bundleVersion, expectedVersion)
	}
	digest := sha256.Sum256(manifest)
	b.releaseID = bundleVersion + "-" + hex.EncodeToString(digest[:])
	return b, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func installBundle(b checkedBundle) (string, error) {
	if current, err := currentReleaseID(); err == nil && current != b.releaseID {
		return "", fmt.Errorf("release %s is already installed; use --upgrade to select %s", current, b.releaseID)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if _, err := stageBundle(b); err != nil {
		return "", err
	}
	if err := installCommandLinks(); err != nil {
		return "", err
	}
	return selectBundle(b.releaseID)
}

// stageBundle installs and verifies immutable release content without selecting
// it. Upgrade uses this before stopping the old daemon, so a bad package cannot
// turn a running node into downtime.
func stageBundle(b checkedBundle) (string, error) {
	releases := filepath.Join(installRoot, "releases")
	final := filepath.Join(releases, b.releaseID)
	if err := preflightCommandLinks(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(releases, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(final); os.IsNotExist(err) {
		stage, err := os.MkdirTemp(releases, ".stage-")
		if err != nil {
			return "", err
		}
		keep := false
		defer func() {
			if !keep {
				_ = os.RemoveAll(stage)
			}
		}()
		for _, name := range b.files() {
			if err := copyBundleFile(b.root, stage, name); err != nil {
				return "", err
			}
		}
		if err := os.WriteFile(filepath.Join(stage, "MANIFEST.sha256"), b.manifest, 0o644); err != nil {
			return "", err
		}
		if _, err := checkBundle(stage); err != nil {
			return "", fmt.Errorf("verify staged release: %w", err)
		}
		if err := syncTree(stage); err != nil {
			return "", err
		}
		// MkdirTemp deliberately starts private. The immutable release itself is
		// public executable content; daemon and runner accounts must traverse it.
		if err := os.Chmod(stage, 0o755); err != nil {
			return "", err
		}
		if err := os.Rename(stage, final); err != nil {
			return "", err
		}
		if err := syncDir(releases); err != nil {
			return "", err
		}
		keep = true
	} else if err != nil {
		return "", err
	} else if _, err := checkBundle(final); err != nil {
		return "", fmt.Errorf("installed release %s is damaged: %w", b.releaseID, err)
	}

	return final, nil
}

func selectBundle(releaseID string) (string, error) {
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		return "", err
	}
	tmp := filepath.Join(installRoot, fmt.Sprintf(".current-%d", os.Getpid()))
	_ = os.Remove(tmp)
	if err := os.Symlink(filepath.Join("releases", releaseID), tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, filepath.Join(installRoot, "current")); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := syncDir(installRoot); err != nil {
		return "", err
	}
	pruneCommandLinks()
	return filepath.Join(installRoot, "current"), nil
}

// pruneCommandLinks takes away each command link the selected release has
// nothing behind: a rollback past the runner's arrival would otherwise leave
// agent-bus-runner a link to nothing, a command that fails strangely.
func pruneCommandLinks() {
	for name, target := range commandLinks() {
		path := filepath.Join(installBin, name)
		if _, err := os.Stat(target); os.IsNotExist(err) && isSymlink(path) {
			_ = os.Remove(path)
		}
	}
}

// priorReleaseForRollback is the release a failed reinstall returns to. No
// release yet is a first install; anything else unreadable is refused, or a
// rollback would start the new program against the restored old home.
func priorReleaseForRollback() (string, error) {
	id, err := currentReleaseID()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("the current release cannot be read, so a failed reinstall could not be rolled back: %w", err)
	}
	return id, nil
}

func currentReleaseID() (string, error) {
	current := filepath.Join(installRoot, "current")
	target, err := os.Readlink(current)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(target)
	wantDir := filepath.Join("releases", filepath.Base(clean))
	if filepath.IsAbs(target) || clean != wantDir {
		return "", fmt.Errorf("%s has unsafe target %q", current, target)
	}
	return filepath.Base(clean), nil
}

func copyBundleFile(srcRoot, dstRoot, name string) error {
	src := filepath.Join(srcRoot, filepath.FromSlash(name))
	dst := filepath.Join(dstRoot, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	mode := os.FileMode(0o644)
	if executableFiles[name] {
		mode = 0o755
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
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

func syncTree(root string) error {
	var dirs []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		err = f.Sync()
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := syncDir(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}

func commandLinks() map[string]string {
	links := make(map[string]string)
	for _, name := range []string{"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-runner", "agent-bus-setup", "agent-bus-token"} {
		links[name] = filepath.Join(installRoot, "current", name)
	}
	for _, name := range []string{"ab-claude", "ab-codex", "ab-opencode", "ab-opencode1"} {
		links[name] = filepath.Join(installRoot, "current", "launchers", name)
	}
	return links
}

func preflightCommandLinks() error {
	for name := range commandLinks() {
		path := filepath.Join(installBin, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refusing to replace unrelated command %s; move it and run setup again", path)
		}
	}
	return nil
}

func installCommandLinks() error {
	if err := os.MkdirAll(installBin, 0o755); err != nil {
		return err
	}
	// The Go dashboard's link, from before 0.8.50, would dangle.
	if old := filepath.Join(installBin, "agent-bus-web"); isSymlink(old) {
		_ = os.Remove(old)
	}
	for name, target := range commandLinks() {
		path := filepath.Join(installBin, name)
		tmp := filepath.Join(installBin, fmt.Sprintf(".agent-bus-%s-%d", name, os.Getpid()))
		_ = os.Remove(tmp)
		if err := os.Symlink(target, tmp); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// files is every artifact the manifest names, sorted: the required ones and
// the web face's tree.
func (b checkedBundle) files() []string {
	out := make([]string, 0, len(b.fileHashes))
	for name := range b.fileHashes {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
