package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// dirEnv carries the checked, resolved directory of an agent that asked for
// one (agent.json needs.dir) to the agent: inside the sandbox only this path
// exists, so the agent must use it rather than its Configuration's spelling.
const dirEnv = "AGENT_BUS_DIR"

// bindDir sets dirEnv for an agent with needs.dir, from its Configuration key
// "dir": the value must resolve inside one of the node owner's roots (the
// agent's `roots` in runner/agents.json, empty by default so nothing is
// watched until the owner opts a root in) and pass resolveBindDir's checks. A
// refusal names why and the agent is not served. It returns the one read-only
// bind the launch adds. Every other agent has dirEnv removed, so no environment
// file can forge a bind.
//
// It runs at adoption, so a refusal shows in status, and again at every launch
// on the Configuration just read, so a changed `dir` or a swapped path is never
// bound from an earlier check.
func (r *runner) bindDir(a agentFile, roots []string, env []string) ([]string, []string, string) {
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
		return out, nil, ""
	}
	var settings map[string]json.RawMessage
	var raw string
	if config != "" && json.Unmarshal([]byte(config), &settings) == nil {
		_ = json.Unmarshal(settings["dir"], &raw)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil, "needs.dir: the Configuration key dir names no directory"
	}
	home, _ := os.UserHomeDir()
	real, err := resolveBindDir(raw, roots, []string{r.host, r.agents, r.bundled}, home, os.Getenv("XDG_RUNTIME_DIR"))
	if err != nil {
		return nil, nil, "needs.dir: " + err.Error()
	}
	return append(out, dirEnv+"="+real), []string{real}, ""
}

// bindSockets bounds the walk that looks for a unix socket under a bind root.
const bindSockets = 100000

// pseudo are the directories that expose the host rather than files: process
// environments, kernel and device state, and the runtime directory with the
// account sockets. None may be a bind root, lie inside one, or hold one.
//
// The bus's own state and configuration directories are listed too: they hold the
// tokens and environment files of every agent, which the runner account can read.
var pseudo = []string{"/proc", "/sys", "/dev", "/run", "/var/run", "/var/lib/agent-bus", "/etc/agent-bus"}

// resolveBindDir turns the configured value into the real path to bind, or says
// why not. The path must lie inside one of the node owner's roots, after
// symlinks are followed, so a link cannot reach outside one; with no roots
// nothing is allowed. The checks run on the resolved path, so a symlink cannot
// launder a forbidden directory: `/`, the home root (the account's home and the
// directory holding homes), the pseudo-filesystems and runtime directories
// (/proc, /sys, /dev, /run, /var/run and $XDG_RUNTIME_DIR), and the runner's own
// state (the runner directory, the shared agent directory and the bundled
// agents), whether the candidate is one of them, inside one, or contains one.
// A tree holding a unix socket is refused too: a bind would expose it.
func resolveBindDir(raw string, allowed, state []string, home, runtime string) (string, error) {
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
	avoid := append([]string{}, pseudo...)
	if runtime != "" {
		avoid = append(avoid, runtime)
	}
	for _, p := range avoid {
		p = resolved(p)
		if within(real, p) || within(p, real) {
			return "", fmt.Errorf("%s overlaps %s, which exposes the host", real, p)
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
	if len(allowed) == 0 {
		return "", errors.New("this node allows no watch root; its owner lists roots in runner/agents.json (\"roots\") for this agent")
	}
	inside := false
	for _, root := range allowed {
		if !filepath.IsAbs(root) {
			return "", fmt.Errorf("roots entry %q is not an absolute path", root)
		}
		if within(real, resolved(root)) {
			inside = true
		}
	}
	if !inside {
		return "", fmt.Errorf("%s is outside this node's watch roots", real)
	}
	if sock := findSocket(real); sock != "" {
		return "", fmt.Errorf("%s holds a unix socket, %s, which a bind would expose", real, sock)
	}
	return real, nil
}

// findSocket is the first unix socket under root, or "". It never follows a
// link and gives up after bindSockets entries; a tree larger than that is
// checked only that far.
func findSocket(root string) string {
	found, seen := "", 0
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if seen++; seen > bindSockets {
			return fs.SkipAll
		}
		if d.Type()&fs.ModeSocket != 0 {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	return found
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
