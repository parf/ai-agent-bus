package main

import (
	"encoding/json"
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
