package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

const releaseBundledDirectory = "/usr/local/lib/agent-bus/current/agents"

type agentSource struct {
	path       string
	bundled    bool
	standalone bool
}

// Resolve the installed launcher link before detecting a git checkout. A
// release uses current/agents, not the directory containing /usr/local/bin's link.
func defaultBundledDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		return releaseBundledDirectory
	}
	return bundledDirectory(executable)
}

func bundledDirectory(executable string) string {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return releaseBundledDirectory
	}
	source := filepath.Dir(resolved)
	if filepath.Base(source) == "src" {
		if _, err := os.Stat(filepath.Join(source, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(source, "..", ".git")); err == nil {
				return filepath.Join(source, "agents")
			}
		}
	}
	return releaseBundledDirectory
}

func (r *runner) sources() []agentSource {
	sources := []agentSource{{path: r.agents}}
	if r.bundled != "" {
		sources = append(sources, agentSource{path: r.bundled, bundled: true}, agentSource{path: filepath.Join(r.bundled, "standalone"), bundled: true, standalone: true})
	}
	return sources
}

func (r *runner) codeDir(in installed) string {
	if in.Path != "" {
		return in.Path
	}
	root := in.Source
	if root == "" {
		root = r.agents
	} // manually constructed snapshots in callers/tests
	return filepath.Join(root, in.Dir)
}

// packagePath answers where the package named name under source lives: the
// directory itself, or the directory a symbolic link there points to, such as
// a git checkout linked in by hand. Any other entry is not a package (false);
// a link to anything but a directory, or to the runner's own directory host
// (settings, secrets) or into source itself, is refused by name, never skipped.
func packagePath(source, host, name string) (string, bool, error) {
	p := filepath.Join(source, name)
	fi, err := os.Lstat(p)
	if err != nil {
		return "", false, err
	}
	if fi.IsDir() {
		return p, true, nil
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return "", false, nil
	}
	// Every path compared below is absolute, so a relative --host or agents directory cannot
	// bypass the guard against an absolute target.
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", true, fmt.Errorf("%s is a symbolic link that cannot be resolved: %v", p, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", true, fmt.Errorf("%s is a symbolic link that leads nowhere: %v", p, err)
	}
	if st, err := os.Stat(real); err != nil || !st.IsDir() {
		return "", true, fmt.Errorf("%s is a symbolic link to %s, which is not a directory", p, real)
	}
	// A package link points at a checkout elsewhere: never at what the host keeps private
	// (env secrets, overrides) and never back into the directory of packages.
	for _, guarded := range []struct{ dir, why string }{{host, "the runner's own directory, which holds secrets and settings"}, {source, "the package directory itself"}} {
		if guarded.dir == "" {
			continue
		}
		resolved, err := canonicalDir(guarded.dir)
		if err != nil {
			return "", true, fmt.Errorf("%s is a symbolic link, and %s cannot be resolved to check it against %s: %v", p, guarded.dir, guarded.why, err)
		}
		if within(real, resolved) {
			return "", true, fmt.Errorf("%s is a symbolic link to %s, which is inside %s; a package link points at a checkout elsewhere", p, real, guarded.why)
		}
	}
	return real, true, nil
}

// canonicalDir is dir made absolute with its links resolved.
func canonicalDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if errors.Is(err, os.ErrNotExist) {
		// A root that does not exist yet has no links to resolve; no real path can lie inside it.
		return abs, nil
	}
	return resolved, err
}

// within is whether path is dir or something under it; both are already resolved.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// File controls use the same first-wins choice as loading. Host files stay
// under runner/<name>; only the read-only author's root changes.
func (r *runner) packageRoot(name string) (string, error) {
	for _, source := range r.sources() {
		dir, ok, err := packagePath(source.path, r.host, name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		if source.bundled {
			if _, err := os.Stat(filepath.Join(dir, "agent.json")); os.IsNotExist(err) {
				continue
			}
		}
		if source.standalone {
			var manifest agentFile
			path := filepath.Join(dir, "agent.json")
			data, err := agentmeta.ReadFile(path)
			if err == nil && strictData("agent.json", data, &manifest) == nil && source.metadataOnly(data, manifest) {
				continue
			}
		}
		return source.path, nil
	}
	return "", fmt.Errorf("no agent %s is installed in agent.d or the bundled directories", name)
}

// Checkout agents import the shared engines and clients outside their own
// package. Only git-node bundles get these public code roots; host packages
// and release bundles must remain self-contained. Never bind the checkout
// itself: it can hold host files beside the code.
func (r *runner) codeReads(in installed, dir string) []string {
	read := []string{dir}
	if !in.Bundled || !gitAgentDirectory(r.bundled) {
		return read
	}
	src := filepath.Dir(r.bundled)
	for _, path := range []string{
		filepath.Join(r.bundled, "shared"),
		filepath.Join(src, "clients", "python"),
		filepath.Join(src, "clients", "typescript"),
		filepath.Join(src, "internal", "version"),
	} {
		if _, err := os.Stat(path); err == nil {
			read = append(read, path)
		}
	}
	return read
}

func gitAgentDirectory(root string) bool {
	src := filepath.Dir(root)
	if filepath.Base(root) != "agents" || filepath.Base(src) != "src" {
		return false
	}
	if _, err := os.Stat(filepath.Join(src, "go.mod")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(src, "..", ".git"))
	return err == nil
}

// Standalone services publish method metadata for their own units, without
// a runner command. A present but empty/bad command remains a refused package.
func (source agentSource) metadataOnly(data []byte, manifest agentFile) bool {
	if !source.standalone || strings.TrimSpace(manifest.Description) == "" {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return false
	}
	for field := range fields {
		if strings.EqualFold(field, "command") { // Match encoding/json's field lookup.
			return false
		}
	}
	return agentmeta.Manifest(manifest).Check() == nil
}
