package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	scriptEnv "github.com/parf/ai-agent-bus/internal/script"
)

// Deploy adds no privilege: the host must have provisioned this directory.
func (r *runner) deployRoot() error {
	if err := r.noLinks(r.agents); err != nil {
		return err
	}
	fi, err := os.Stat(r.agents)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("install requires a host-provisioned writable --agents directory: %s", r.agents)
	}
	if fi.Mode().Perm()&0222 == 0 {
		return fmt.Errorf("the runner's agents directory is read-only: %s", r.agents)
	}
	return nil
}

func deployName(name string) error {
	if !segment.MatchString(name) || name == "work" || name == "tokens" {
		return fmt.Errorf("deploy needs one package name, excluding the runner's work and tokens directories")
	}
	return nil
}

// installTarget is what an install may start from: the checks on the host's
// directories, whether the package is already installed, and the inventory.
// It is asked twice: before the slow fetch or build, so a plain mistake is
// refused at once, and again under reloadMu, which is the answer that counts.
func (r *runner) installTarget(in installRequest) (existing bool, entries map[string]map[string]json.RawMessage, err error) {
	if err := r.deployRoot(); err != nil {
		return false, nil, err
	}
	if err := r.noLinks(r.agents, in.agent); err != nil {
		return false, nil, err
	}
	if err := r.noLinks(r.host, in.agent); err != nil {
		return false, nil, err
	}
	live := filepath.Join(r.agents, in.agent)
	_, existingErr := os.Lstat(live)
	existing = existingErr == nil
	if existingErr != nil && !errors.Is(existingErr, os.ErrNotExist) {
		return false, nil, existingErr
	}
	// A bundled source itself is an available source, not an already deployed
	// tree; opt-in does not require --replace. An installed writable tree does.
	if existing && !in.replace {
		return false, nil, fmt.Errorf("%s already exists; install --replace is required", in.agent)
	}
	entries, err = readAgents(r.host)
	if err != nil {
		return false, nil, err
	}
	if err := checkAgents(mustJSON(entries)); err != nil {
		return false, nil, err
	}
	return existing, entries, nil
}

// stageInstall fetches or builds the package into a private staging directory
// under agents/ (the loader skips dot directories). It takes no lock: a git
// fetch and a package build can each last two minutes, and every control call
// of the runner, stop and disable included, waits for reloadMu. On failure it
// removes the staging directory.
func (r *runner) stageInstall(ctx context.Context, in installRequest) (temp, staged string, err error) {
	dir, err := os.MkdirTemp(r.agents, ".install-")
	if err != nil {
		return "", "", fmt.Errorf("create install staging directory: %w", err)
	}
	// dir, not the result temp: a failing return has already blanked temp.
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()
	temp = dir
	staged = filepath.Join(temp, in.agent)
	if in.remote != "" {
		if err := os.Mkdir(staged, 0700); err != nil {
			return "", "", err
		}
		fetch := r.fetchDeploy
		if r.fetch != nil {
			fetch = r.fetch
		}
		if err := fetch(ctx, in, staged); err != nil {
			return "", "", err
		}
		return temp, staged, nil
	}
	source, err := r.bundledInstallSource(in.agent)
	if err != nil {
		return "", "", err
	}
	script := filepath.Join(source, "package.sh")
	scriptInfo, scriptErr := os.Lstat(script)
	if scriptErr == nil || gitAgentDirectory(r.bundled) {
		if scriptErr != nil || !scriptInfo.Mode().IsRegular() {
			return "", "", fmt.Errorf("checkout bundled package has no regular package.sh")
		}
		buildCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		cmd := exec.CommandContext(buildCtx, "/bin/sh", script, staged)
		cmd.Env = scriptEnv.ChildEnv(os.Environ())
		// Only the host's trusted bundled packager is executed. Never a script
		// from a fetched repository. Its output is not a secret-return channel.
		err = cmd.Run()
		cancel()
		if err != nil {
			return "", "", fmt.Errorf("bundled package build failed; live package unchanged")
		}
	} else if err := copyDeployTree(source, staged); err != nil {
		return "", "", err
	}
	return temp, staged, nil
}

func (r *runner) installPackage(ctx context.Context, body string) error {
	in, err := parseInstall(body)
	if err != nil {
		return err
	}
	if err := deployName(in.agent); err != nil {
		return err
	}
	// The slow part first, without the lock; the lock only for what changes the host.
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, _, err := r.installTarget(in); err != nil {
		return err
	}
	temp, staged, err := r.stageInstall(ctx, in)
	if err != nil {
		return err
	}
	clean := true
	defer func() {
		if clean {
			os.RemoveAll(temp)
		}
	}()
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	existing, entries, err := r.installTarget(in)
	if err != nil {
		return err
	}
	live := filepath.Join(r.agents, in.agent)
	if err := validateDeployTree(staged); err != nil {
		return err
	}
	list, err := loadSources([]agentSource{{path: temp}}, r.host)
	if err != nil {
		return err
	}
	for _, a := range list {
		if a.Dir == in.agent && a.Refused != "" {
			return fmt.Errorf("staged package refused: %s", a.Refused)
		}
	}
	found := false
	for key := range entries {
		if key == in.agent || strings.HasPrefix(key, in.agent+"/") {
			found = true
		}
	}
	if !found {
		entries[in.agent] = map[string]json.RawMessage{"enabled": json.RawMessage("true")}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	backup := filepath.Join(temp, "previous")
	if existing {
		if err := os.Rename(live, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(staged, live); err != nil {
		if existing {
			if e := os.Rename(backup, live); e != nil {
				clean = false
				return fmt.Errorf("publish failed: %v; restore failed, previous tree retained at %s: %w", err, backup, e)
			}
		}
		return err
	}
	if err := writeJSON(filepath.Join(r.host, "agents.json"), entries); err != nil {
		// The old tree stays until inventory publication succeeds.
		if e := os.Rename(live, staged); e != nil {
			clean = false
			return fmt.Errorf("inventory failed: %v; new tree could not be withdrawn, previous tree retained at %s: %w", err, backup, e)
		}
		if existing {
			if e := os.Rename(backup, live); e != nil {
				clean = false
				return fmt.Errorf("inventory failed: %v; restore failed, previous tree retained at %s: %w", err, backup, e)
			}
		}
		return err
	}
	r.apply()
	return nil
}

func validateDeployTree(root string) error {
	var manifest agentFile
	if err := strict(filepath.Join(root, "agent.json"), &manifest); err != nil {
		return fmt.Errorf("deploy requires a regular agent.json at the repository root, a self-contained package; subdirectory layouts are refused")
	}
	if _, err := manifest.check(); err != nil {
		return err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("package has an unresolved symbolic link")
			}
			target, err = filepath.Abs(target)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(absolute, target)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return fmt.Errorf("package symbolic link leaves its self-contained root")
			}
		} else if !d.IsDir() && !d.Type().IsRegular() {
			return fmt.Errorf("package contains a special file")
		}
		return nil
	})
}

func copyDeployTree(source, dest string) error {
	if err := validateDeployTree(source); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(out, info.Mode().Perm()|0700)
		}
		if d.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if filepath.IsAbs(target) {
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil {
					return err
				}
				target, err = filepath.Rel(filepath.Dir(path), resolved)
				if err != nil {
					return err
				}
			}
			return os.Symlink(target, out)
		}
		from, err := os.Open(path)
		if err != nil {
			return err
		}

		to, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			from.Close()
			return err
		}
		_, err = io.Copy(to, from)
		from.Close()
		closeErr := to.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}

func (r *runner) removePackage(name string) error {
	if err := deployName(name); err != nil {
		return err
	}
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	// Bundles remain available even when their writable shadow is removed.
	// Refusing avoids bringing them back with a changed identity/configuration.
	if _, err := r.bundledInstallSource(name); err == nil {
		return fmt.Errorf("%s is a bundled agent; disable it instead", name)
	}
	if err := r.deployRoot(); err != nil {
		return err
	}
	if err := r.noLinks(r.agents, name); err != nil {
		return err
	}
	if err := r.noLinks(r.host, name); err != nil {
		return err
	}
	live := filepath.Join(r.agents, name)
	if fi, err := os.Lstat(live); err != nil || !fi.IsDir() {
		return fmt.Errorf("no writable package %s is installed", name)
	}
	entries, err := readAgents(r.host)
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp(r.agents, ".remove-")
	if err != nil {
		return err
	}
	clean := true
	defer func() {
		if clean {
			os.RemoveAll(temp)
		}
	}()
	if err := os.Rename(live, filepath.Join(temp, "package")); err != nil {
		return err
	}
	restore := func(cause error) error {
		if e := os.Rename(filepath.Join(temp, "package"), live); e != nil {
			clean = false
			return fmt.Errorf("%v; restore failed, package retained at %s: %w", cause, temp, e)
		}
		return cause
	}
	// Private host files can be on a different filesystem: rename them within
	// their own parent, retaining them until the inventory write succeeds.
	host := filepath.Join(r.host, name)
	trash := ""
	if _, err := os.Lstat(host); err == nil {
		trash, err = os.MkdirTemp(r.host, ".remove-")
		if err != nil {
			return restore(err)
		}
		defer func() {
			if clean {
				os.RemoveAll(trash)
			}
		}()
		if err := os.Rename(host, filepath.Join(trash, "settings")); err != nil {
			return restore(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return restore(err)
	}
	for key := range entries {
		if key == name || strings.HasPrefix(key, name+"/") {
			delete(entries, key)
		}
	}
	if err := writeJSON(filepath.Join(r.host, "agents.json"), entries); err != nil {
		if trash != "" {
			if e := os.Rename(filepath.Join(trash, "settings"), host); e != nil {
				clean = false
				return restore(fmt.Errorf("%v; settings retained at %s: %w", err, trash, e))
			}
		}
		return restore(err)
	}
	for key := range r.stopped {
		if key == name || strings.HasPrefix(key, name+"/") {
			delete(r.stopped, key)
		}
	}
	r.apply()
	return nil
}
