// Package sandbox holds the adapters behind the sandbox port: one that
// confines a script service with systemd-run and one that confines nothing.
// Choosing between them is Pick's, which is the only place that asks the
// host what it can actually do. See docs/08-runner-role.md#sandboxing.
package sandbox

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// What a child may spend unless its record says otherwise. Bounds, not
// budgets: they are here so that one runaway script cannot take the host
// with it.
const (
	maxTasks  = 64
	maxMemory = "512M"
)

// Off is the other setting — a setting, not an absence. The runner says
// which one it got, so running unsandboxed is something you were told.
type Off struct{}

func (Off) Name() string              { return "off" }
func (Off) Wrap(j ports.Job) []string { return j.Argv }

// SystemdRun puts the child in a transient **user** scope, so it needs no
// privilege of its own and the child keeps the runner's uid: the
// confinement is the unit's properties, not a change of identity.
type SystemdRun struct{}

func (SystemdRun) Name() string { return "systemd-run" }

func (SystemdRun) Wrap(j ports.Job) []string {
	// --pipe keeps the runner's stdin, stdout and stderr, which is the whole
	// contract of a script service; --collect leaves no unit behind when one
	// fails, or a service that fails often fills the manager with them.
	out := []string{"systemd-run", "--user", "--pipe", "--collect", "--quiet",
		"--working-directory=" + j.Work,
		"-p", "NoNewPrivileges=yes",
		"-p", "ProtectSystem=strict",
		"-p", "ProtectHome=read-only",
		"-p", "PrivateTmp=yes",
		// Bound in, not merely listed as writable: with a private /tmp a
		// work directory under /tmp is not there at all to be written to.
		"-p", "BindPaths=" + j.Work,
		"-p", "TasksMax=" + strconv.Itoa(maxTasks),
		"-p", "MemoryMax=" + maxMemory,
	}
	if !j.Net {
		out = append(out, "-p", "PrivateNetwork=yes")
	}
	for _, p := range j.Read {
		// The same problem from the other side: the script is a file the
		// private /tmp can hide just as easily.
		out = append(out, "-p", "BindReadOnlyPaths="+p)
	}
	for _, kv := range j.Env {
		// A transient unit starts from the user manager's environment, so
		// nothing the runner exported reaches the script unless it is said
		// here — the envelope included.
		out = append(out, "--setenv="+kv)
	}
	return append(out, j.Argv...)
}

// Pick answers what this host can actually do. `off` is honoured as asked,
// `on` is an error where nothing can provide it, and an unset want takes
// whatever is there.
func Pick(want string) (ports.Sandbox, error) {
	switch want {
	case "off":
		return Off{}, nil
	case "", "on":
	default:
		return nil, fmt.Errorf("--sandbox is on or off, not %q", want)
	}
	if err := probe(); err != nil {
		if want == "on" {
			return nil, fmt.Errorf("this host cannot sandbox: %w", err)
		}
		return Off{}, nil
	}
	return SystemdRun{}, nil
}

// probe runs the real thing once rather than guessing from a PATH entry and
// an environment variable: a user manager that is not running looks exactly
// like one that is, until something is asked of it.
func probe() error {
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return err
	}
	return exec.Command("systemd-run", "--user", "--pipe", "--collect", "--quiet", "/bin/true").Run()
}
