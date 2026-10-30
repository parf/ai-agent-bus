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
)

// Deploy adds no privilege: the host must have provisioned this directory.
func (r *runner) deployRoot() error {
	if err := r.noLinks(r.agents); err != nil {
		return err
	}
	fi, err := os.Stat(r.agents)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0222 == 0 {
		return fmt.Errorf("install requires a host-provisioned writable --agents directory; this agents directory is read-only or unavailable")
	}
	return nil
}

func deployName(name string) error {
	if !segment.MatchString(name) || name == "work" || name == "tokens" {
		return fmt.Errorf("deploy needs one package name, excluding the runner's work and tokens directories")
	}
	return nil
}

func (r *runner) installPackage(ctx context.Context, body string) error {
	in, err := parseInstall(body)
	if err != nil {
		return err
	}
	if err := deployName(in.agent); err != nil {
		return err
	}
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.deployRoot(); err != nil {
		return err
	}
	if err := r.noLinks(r.agents, in.agent); err != nil {
		return err
	}
	if err := r.noLinks(r.host, in.agent); err != nil {
		return err
	}
	live := filepath.Join(r.agents, in.agent)
	_, existingErr := os.Lstat(live)
	existing := existingErr == nil
	if existingErr != nil && !errors.Is(existingErr, os.ErrNotExist) {
		return existingErr
	}
	// A bundled source itself is an available source, not an already deployed
	// tree; opt-in does not require --replace. An installed writable tree does.
	if existing && !in.replace {
		return fmt.Errorf("%s already exists; install --replace is required", in.agent)
	}
	entries, err := readAgents(r.host)
	if err != nil {
		return err
	}
	if err := checkAgents(mustJSON(entries)); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(r.agents, ".install-")
	if err != nil {
		return fmt.Errorf("install requires a writable --agents directory: %w", err)
	}
	clean := true
	defer func() {
		if clean {
			os.RemoveAll(temp)
		}
	}()
	staged := filepath.Join(temp, in.agent)
	if in.remote != "" {
		if err := os.Mkdir(staged, 0700); err != nil {
			return err
		}
		if err := r.fetchDeploy(ctx, in, staged); err != nil {
			return err
		}
	} else {
		source, err := r.bundledInstallSource(in.agent)
		if err != nil {
			return err
		}
		script := filepath.Join(source, "package.sh")
		scriptInfo, scriptErr := os.Lstat(script)
		if scriptErr == nil || gitAgentDirectory(r.bundled) {
			if scriptErr != nil || !scriptInfo.Mode().IsRegular() {
				return fmt.Errorf("checkout bundled package has no regular package.sh")
			}
			buildCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			cmd := exec.CommandContext(buildCtx, "/bin/sh", script, staged)
			// Only the host's trusted bundled packager is executed. Never a script
			// from a fetched repository. Its output is not a secret-return channel.
			err = cmd.Run()
			cancel()
			if err != nil {
				return fmt.Errorf("bundled package build failed; live package unchanged")
			}
		} else if err := copyDeployTree(source, staged); err != nil {
			return err
		}
	}
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
	for key, entry := range entries {
		if key == in.agent || strings.HasPrefix(key, in.agent+"/") {
			found = true
			if in.remote != "" {
				entry["origin"] = mustJSON(origin{Remote: in.remote, Commit: in.commit})
			} else {
				delete(entry, "origin")
			}
		}
	}
	if !found {
		entries[in.agent] = map[string]json.RawMessage{"enabled": json.RawMessage("true")}
		if in.remote != "" {
			entries[in.agent]["origin"] = mustJSON(origin{Remote: in.remote, Commit: in.commit})
		}
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
