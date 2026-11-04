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
func TestImapGenerationLockInsideTheRunnerSandbox(t *testing.T) {
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
	script := `import os, pathlib, sys
sys.path.insert(0, sys.argv[1])
import outbox
assert not pathlib.Path('/etc/passwd').exists(), 'sandbox mount policy was bypassed'
work = pathlib.Path(sys.argv[2])
probe = work / 'link-source'
probe.write_bytes(b'probe')
os.link(probe, work / 'link-target')
assert probe.stat().st_ino == (work / 'link-target').stat().st_ino
with outbox.Outbox(work):
    try:
        outbox.Outbox(work, lock_wait=0)
    except outbox.PreviousGeneration:
        pass
    else:
        raise AssertionError('concurrent generation admitted')
with outbox.Outbox(work):
    pass
obsolete = work / 'mail-outbox' / 'pending.eml'
obsolete.write_bytes(b'pending fixture')
try:
    outbox.Outbox(work)
except outbox.OutboxError as error:
    assert 'unsupported generation directory contents' in str(error)
else:
    raise AssertionError('pending body adopted')
assert obsolete.read_bytes() == b'pending fixture', 'refusal deleted pending body'
print('current generation lock confined; pending bodies refused')
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
	if err != nil || !strings.Contains(string(out), "current generation lock confined; pending bodies refused") {
		t.Fatalf("sandboxed outbox: %v\n%s", err, out)
	}
}
