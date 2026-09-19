// Where a running script service leaves a note of itself, and the two verbs
// that read it. `stop` and `logs` have to find a process and its output, and
// the daemon knows neither: a script service is a process on somebody's
// machine, and the registry holds a description of it, not a handle to it.
//
// The note lives in the **owner's own state directory**, which is what makes
// "only whoever started it may stop it" true without a check — nobody else
// can see the file. See docs/08-runner-role.md#script-agents.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type running struct {
	Name    string    `json:"name"`
	PID     int       `json:"pid"`
	Script  string    `json:"script"`
	Sandbox string    `json:"sandbox"`
	Log     string    `json:"log"`
	Started time.Time `json:"started"`
}

// runDir holds a service's note, its work directory and its log. It is the
// XDG state directory and not the cache one: a cache is something a machine
// may throw away, and a running service's pid is not.
func runDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "agent-bus")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "agent-bus")
}

// segment turns a name into one path segment. A name carries `@` and may
// carry one `/` (docs/01-identity-and-roles.md#names), and the slash is the one that
// would otherwise make it two.
func segment(name string) string { return strings.ReplaceAll(name, "/", "%") }

func notePath(name string) string {
	return filepath.Join(runDir(), "services", segment(name)+".json")
}
func workPath(name string) string { return filepath.Join(runDir(), "work", segment(name)) }
func logPath(name string) string {
	return filepath.Join(runDir(), "logs", segment(name)+".log")
}

func (r running) note() error {
	if err := os.MkdirAll(filepath.Dir(notePath(r.Name)), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(notePath(r.Name), append(b, '\n'), 0o600)
}

// alive is the note plus the one thing a note cannot say: that the process
// is still there. A service killed outright leaves its note behind, so the
// stale one is cleared here rather than reported as a running service.
func alive(name string) (running, error) {
	b, err := os.ReadFile(notePath(name))
	if err != nil {
		return running{}, fmt.Errorf("%s is not running from this account", name)
	}
	var r running
	if err := json.Unmarshal(b, &r); err != nil {
		return running{}, fmt.Errorf("the note for %s is not readable: %w", name, err)
	}
	if syscall.Kill(r.PID, 0) != nil {
		os.Remove(notePath(name))
		return running{}, fmt.Errorf("%s left a note but no process; it is gone", name)
	}
	return r, nil
}

// stopVerb ends one service and waits for it. Graceful is the whole
// contract (serve, in start.go), so this waits for it to finish rather than
// reporting a stop that has not happened.
// Siblings are untouched: a name names one process here.
func stopVerb(args []string) error {
	pos, _ := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("stop wants one agent name")
	}
	r, err := alive(pos[0])
	if err != nil {
		return err
	}
	if err := syscall.Kill(r.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("could not stop %s (pid %d): %w", r.Name, r.PID, err)
	}
	for range 300 {
		if syscall.Kill(r.PID, 0) != nil {
			fmt.Printf("%s stopped\n", r.Name)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s (pid %d) is still finishing what it started", r.Name, r.PID)
}

// logsVerb prints what a service wrote. The log outlives the service on
// purpose: what a run said is most wanted once the run has ended.
func logsVerb(args []string) error {
	pos, flags := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("logs wants one agent name")
	}
	last := 50
	if v := flags["lines"]; v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("--lines wants a positive number, not %q", v)
		}
		last = n
	}
	path := logPath(pos[0])
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("nothing logged for %s here", pos[0])
	}
	if lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n"); len(b) > 0 {
		if len(lines) > last {
			lines = lines[len(lines)-last:]
		}
		fmt.Println(strings.Join(lines, "\n"))
	}
	if !has(flags, "follow") {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(int64(len(b)), io.SeekStart); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if err == io.EOF {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
	}
}
