package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const releaseBundledDirectory = "/usr/local/lib/agent-bus/current/agents"

type agentSource struct {
	path    string
	bundled bool
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
		sources = append(sources, agentSource{path: r.bundled, bundled: true}, agentSource{path: filepath.Join(r.bundled, "standalone"), bundled: true})
	}
	return sources
}

func (r *runner) codeDir(in installed) string {
	root := in.Source
	if root == "" {
		root = r.agents
	} // manually constructed snapshots in callers/tests
	return filepath.Join(root, in.Dir)
}

// File controls use the same first-wins choice as loading. Host files stay
// under runner/<name>; only the read-only author's root changes.
func (r *runner) packageRoot(name string) (string, error) {
	for _, source := range r.sources() {
		dir := filepath.Join(source.path, name)
		fi, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			continue
		}
		if source.bundled {
			if _, err := os.Stat(filepath.Join(dir, "agent.json")); os.IsNotExist(err) {
				continue
			}
		}
		return source.path, nil
	}
	return "", fmt.Errorf("no agent %s is installed in agent.d or the bundled directories", name)
}
