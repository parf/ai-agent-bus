package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// dirEnv carries the checked, resolved directory of an agent that asked for
// one (agent.json needs.dir) to the agent: inside the sandbox only this path
// exists, so the agent must use it rather than its Configuration's spelling.
const dirEnv = "AGENT_BUS_DIR"

// bindDir sets dirEnv for an agent with needs.dir, from its Configuration key
// "dir", after resolveBindDir's checks; a refusal names why and the agent is
// not served. Every other agent has dirEnv removed, so no environment file can
// forge a bind.
func (r *runner) bindDir(a agentFile, env []string) ([]string, string) {
	out := make([]string, 0, len(env)+1)
	config := ""
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, dirEnv+"="):
		case strings.HasPrefix(kv, "AGENT_BUS_CONFIG="):
			config = strings.TrimPrefix(kv, "AGENT_BUS_CONFIG=")
			out = append(out, kv)
		default:
			out = append(out, kv)
		}
	}
	if a.Needs == nil || !a.Needs.Dir {
		return out, ""
	}
	var settings map[string]json.RawMessage
	var raw string
	if config != "" && json.Unmarshal([]byte(config), &settings) == nil {
		_ = json.Unmarshal(settings["dir"], &raw)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, "needs.dir: the Configuration key dir names no directory"
	}
	home, _ := os.UserHomeDir()
	real, err := resolveBindDir(raw, []string{r.host, r.agents, r.bundled}, home)
	if err != nil {
		return nil, "needs.dir: " + err.Error()
	}
	return append(out, dirEnv+"="+real), ""
}

// dirReads is the one read-only bind an agent with needs.dir gets.
func dirReads(a agentFile, env []string) []string {
	if a.Needs == nil || !a.Needs.Dir {
		return nil
	}
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, dirEnv+"="); ok && v != "" {
			return []string{v}
		}
	}
	return nil
}

// resolveBindDir turns the configured value into the real path to bind, or says
// why not. The checks run on the resolved path, so a symlink cannot launder a
// forbidden directory: `/`, the home root (the account's home and the directory
// holding homes), and the runner's own state (the runner directory, the shared
// agent directory and the bundled agents), whether the candidate is one of
// them, inside one, or contains one.
func resolveBindDir(raw string, state []string, home string) (string, error) {
	if strings.ContainsRune(raw, 0) || !filepath.IsAbs(raw) {
		return "", fmt.Errorf("%q is not an absolute path", raw)
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(raw))
	if err != nil {
		return "", fmt.Errorf("%s cannot be resolved: %v", raw, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("%s cannot be read: %v", real, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%s is not a directory", real)
	}
	roots := []string{"/", "/home", "/root"}
	if home != "" {
		roots = append(roots, resolved(home))
		if filepath.Base(filepath.Dir(home)) == "home" {
			roots = append(roots, resolved(filepath.Dir(home)))
		}
	}
	for _, root := range roots {
		if real == root {
			return "", fmt.Errorf("%s is %s, which cannot be watched", real, rootName(root))
		}
	}
	for _, s := range state {
		if s == "" {
			continue
		}
		s = resolved(s)
		if within(real, s) || within(s, real) {
			return "", fmt.Errorf("%s overlaps the runner's own state %s", real, s)
		}
	}
	return real, nil
}

func rootName(p string) string {
	if p == "/" {
		return "the root directory"
	}
	return "a home root"
}

func resolved(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return filepath.Clean(p)
}

// within reports whether p is dir or below it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
