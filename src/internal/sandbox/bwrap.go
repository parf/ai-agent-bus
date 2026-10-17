package sandbox

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Bwrap confines a child with bubblewrap in namespaces of its own, the managed
// runner's backend (Plans/R1.0-Release/runner-implementation.md#sandboxing):
// the system's programs read-only, the script's own directory read-only, its
// work directory the one place it may write, a private /tmp, no /home, no
// /etc, no other process in sight, and no network unless it asked for one. It
// needs no user manager and no capability: an unprivileged user namespace.
type Bwrap struct{}

func (Bwrap) Name() string { return "bwrap" }

// netEtc is what a child that asked for the network needs from /etc to use
// it — name resolution and certificates — and nothing else of /etc.
var netEtc = []string{"/etc/resolv.conf", "/etc/hosts", "/etc/nsswitch.conf", "/etc/ssl", "/etc/pki", "/etc/ca-certificates"}

func (Bwrap) Wrap(j ports.Job) []string {
	out := []string{"bwrap",
		"--unshare-all", "--die-with-parent", "--new-session", "--cap-drop", "ALL",
		"--ro-bind", "/usr", "/usr",
		// A merged-/usr layout's top-level names, so #! lines and the
		// dynamic loader find what they expect.
		"--symlink", "usr/bin", "/bin", "--symlink", "usr/sbin", "/sbin",
		"--symlink", "usr/lib", "/lib", "--symlink", "usr/lib64", "/lib64",
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp",
	}
	for _, p := range j.Read {
		out = append(out, "--ro-bind", p, p)
	}
	out = append(out, "--bind", j.Work, j.Work, "--chdir", j.Work)
	if j.Net {
		out = append(out, "--share-net")
		for _, p := range netEtc {
			if _, err := os.Stat(p); err == nil {
				out = append(out, "--ro-bind", p, p)
			}
		}
	}
	return append(append(out, "--"), j.Argv...)
}

// ProbeBwrap answers whether this host can confine with bubblewrap, by
// running the real thing once: a missing program, or user namespaces turned
// off, is a host that refuses to start a sandboxed agent rather than one that
// runs it unconfined.
func ProbeBwrap() error {
	if _, err := exec.LookPath("bwrap"); err != nil {
		return fmt.Errorf("bubblewrap is not installed: %w", err)
	}
	argv := Bwrap{}.Wrap(ports.Job{Argv: []string{"/usr/bin/true"}, Work: os.TempDir()})
	if out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil {
		return fmt.Errorf("bubblewrap cannot make a sandbox here: %v: %s", err, out)
	}
	return nil
}
