package main

import (
	"os/exec"
	"time"
)

// These are applied limits, not measured usage; absent with the sandbox off.
type statusLimits struct {
	MemoryBytes int64 `json:"memory_bytes"`
	Processes   int   `json:"processes"`
	DiskBytes   int64 `json:"disk_bytes"`
}

// One observation only: replaced, not appended, and cleared by a fresh load.
type exitStatus struct {
	Code   int       `json:"code"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
}

func (r *runner) recordExit(s *served, k *keeper, cmd *exec.Cmd) {
	// A local Start failure is not a process exit.
	if cmd == nil || cmd.ProcessState == nil {
		return
	}
	exit := &exitStatus{Code: cmd.ProcessState.ExitCode(), At: time.Now().UTC()}
	if stderr, ok := cmd.Stderr.(*stderrLine); ok {
		exit.Reason = stderr.reason()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// A retired process must not overwrite its replacement's observation.
	if r.served[s.name] != s || (k != nil && s.kept != k) {
		return
	}
	for i := range r.installed {
		if r.installed[i].Key == s.in.Key {
			r.installed[i].LastExit = exit
			return
		}
	}
}
