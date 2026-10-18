package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The host's controls, M.5 (Plans/R1.0-Release/TODO.md#runner-waves): each is
// `<verb> <agent>`, the agent by its name or its agents.json key, and answers
// as status does. start, stop and restart last until the next reload or the
// runner's restart; enable and disable write runner/agents.json. Every one
// rereads the files as reload does, so what it answers is what is on disk.

// control carries out one control call, or says why it cannot.
func (r *runner) control(verb, agent string) error {
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	key, err := r.keyOf(agent)
	if err != nil {
		return err
	}
	switch verb {
	case "stop":
		r.setStopped(key, true)
	case "start":
		r.setStopped(key, false)
	case "restart":
		// Taken down, then served afresh: a kept process drains as a stop
		// does, and a message already being run finishes.
		r.setStopped(key, true)
		r.apply()
		r.setStopped(key, false)
	case "enable", "disable":
		if err := setEnabled(r.host, key, verb == "enable"); err != nil {
			return err
		}
	}
	r.apply()
	return nil
}

func (r *runner) setStopped(key string, on bool) {
	if r.stopped == nil {
		r.stopped = map[string]bool{}
	}
	if on {
		r.stopped[key] = true
	} else {
		delete(r.stopped, key)
	}
}

// keyOf is the installed key that agent names, by its key or its bus name.
func (r *runner) keyOf(agent string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loadErr != "" {
		return "", errors.New("nothing is loaded: " + r.loadErr)
	}
	for _, in := range r.installed {
		if in.Key == agent || r.agentName(in) == agent {
			return in.Key, nil
		}
	}
	return "", fmt.Errorf("no agent %s is installed here", agent)
}

// setEnabled writes `enabled` on key's agents.json entry, keeping every other
// entry and field as it was.
func setEnabled(runnerDir, key string, on bool) error {
	path := filepath.Join(runnerDir, "agents.json")
	entries := map[string]map[string]json.RawMessage{}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal(b, &entries); err != nil {
			return fmt.Errorf("agents.json: %w", err)
		}
	}
	if entries[key] == nil {
		entries[key] = map[string]json.RawMessage{}
	}
	entries[key]["enabled"] = json.RawMessage(fmt.Sprint(on))
	return writeJSON(path, entries)
}

// writeJSON replaces path with v as indented JSON, whole or not at all.
func writeJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return replaceFile(path, buf.Bytes())
}

// replaceFile writes b to path through a temporary file in its directory, so
// a reader sees the old file or the new one, never half of one. The runner's
// files hold host decisions and secrets: they are its user's alone.
func replaceFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
