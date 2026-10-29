package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// The actual runner bwrap+seccomp backend, not a directory-permission mock.
// No service, host account, provider, cgroup-limit rejection or journal write.
func TestImapOutboxPublishesInsideTheRunnerSandbox(t *testing.T) {
	code, err := filepath.Abs(filepath.Join("..", "..", "agents", "imap"))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	filter, err := Filter()
	if err != nil {
		t.Fatal(err)
	}
	filterPath := filepath.Join(work, "seccomp.bin")
	if err := os.WriteFile(filterPath, filter, 0600); err != nil {
		t.Fatal(err)
	}
	fd, err := os.Open(filterPath)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	script := `import errno, os, pathlib, sys
sys.path.insert(0, sys.argv[1])
import outbox
assert not pathlib.Path('/etc/passwd').exists(), 'sandbox mount policy was bypassed'
work = pathlib.Path(sys.argv[2])
probe = work / 'link-source'
probe.write_bytes(b'probe')
os.link(probe, work / 'link-target')
assert probe.stat().st_ino == (work / 'link-target').stat().st_ino
for mode in ('normal', 'fallback'):
    directory = work / mode
    directory.mkdir()
    original = outbox.os.link
    def refused(*args, **kwargs):
        raise OSError(errno.EPERM, 'fixture link refusal')
    if mode == 'fallback': outbox.os.link = refused
    try:
        with outbox.Outbox(directory) as box:
            name = outbox.filename('fixture', 7, 100)
            assert box.put(name, b'raw fixture mail')
            assert box.get(name) == b'raw fixture mail'
            assert box.sweep({name}) == 0
            assert box.sweep(set()) == 1
    finally:
        outbox.os.link = original
print('normal and fallback publication confined')
`
	job := ports.Job{Argv: []string{"/usr/bin/python3", "-B", "-c", script, code, work},
		Work: work, Read: []string{code}, Seccomp: true}
	argv := (Bwrap{}).Wrap(job)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.ExtraFiles = []*os.File{fd}
	cmd.Env = []string{"PATH=/usr/bin", "PYTHONDONTWRITEBYTECODE=1"}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "normal and fallback publication confined") {
		t.Fatalf("sandboxed outbox: %v\n%s", err, out)
	}
}
