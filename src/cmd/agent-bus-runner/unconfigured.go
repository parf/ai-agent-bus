package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// stderrLine preserves the journal output and retains only the last line,
// bounded by the same limit as a script answer. A final newline ends a line;
// it does not add an empty one. An unterminated final line is a reason too.
type stderrLine struct {
	out   io.Writer
	line  []byte
	ended bool
}

func (s *stderrLine) Write(p []byte) (int, error) {
	n, err := s.out.Write(p)
	if n == 0 {
		return n, err
	}
	p = p[:n]
	ended := p[len(p)-1] == '\n'
	end := len(p)
	if ended {
		end--
	}
	if i := bytes.LastIndexByte(p[:end], '\n'); i >= 0 {
		s.line = s.line[:0]
		p = p[i+1:]
	} else if s.ended {
		s.line = s.line[:0]
	}
	if ended {
		p = p[:len(p)-1]
	}
	room := maxOutput - len(s.line)
	s.line = append(s.line, p[:min(room, len(p))]...)
	s.ended = ended
	return n, err
}

func (s *stderrLine) reason() string { return strings.TrimSuffix(string(s.line), "\r") }

func configurationExit(cmd *exec.Cmd, err error) (string, bool) {
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 78 {
		return "", false
	}
	// Both execution paths install this writer before Start. Other writers
	// keep ordinary exit handling rather than panicking here.
	stderr, ok := cmd.Stderr.(*stderrLine)
	if !ok {
		return "", false
	}
	return stderr.reason(), true
}

// notConfigured retires this particular serving, never its replacement. A
// fresh load at reload clears the transient state and may adopt it again.
func (r *runner) notConfigured(s *served, k *keeper, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.served[s.name] != s || (k != nil && s.kept != k) {
		return
	}
	if s.kept != nil {
		s.kept.cancel()
	}
	r.stop(s.name, s)
	for i := range r.installed {
		if r.installed[i].Key == s.in.Key {
			r.installed[i].Unconfigured = true
			r.installed[i].Refused = reason
		}
	}
	fmt.Fprintf(r.say, "%s: %s is not configured: %s\n", r.name, s.name, reason)
}
