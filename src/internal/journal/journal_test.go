package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
)

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFilesAndModes(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	for _, name := range []string{"audit.log", "error.log"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o640 {
			t.Fatalf("%s mode %v", name, st.Mode().Perm())
		}
	}
	// The debug log is not there until somebody asks for it.
	if _, err := os.Stat(filepath.Join(dir, "debug.log")); !os.IsNotExist(err) {
		t.Fatal("a debug log exists that nobody asked for")
	}
}

func TestAuditEntry(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	j.Audit(ports.AuditEntry{Actor: "alice", Operation: "manage", Target: "#svc@h", Result: "ok", ClientIP: "127.0.0.1"})
	j.Audit(ports.AuditEntry{Actor: "bob", Operation: "user-state", Target: "carol", Result: "refused 403"})
	got := read(t, dir, "audit.log")
	for _, want := range []string{`actor="alice" op="manage" target="#svc@h" result="ok" ip="127.0.0.1"`, `actor="bob" op="user-state" target="carol" result="refused 403"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("audit log lacks %s:\n%s", want, got)
		}
	}
	// No IP is invented for a call that had none.
	if strings.Count(got, "ip=") != 1 {
		t.Fatalf("an ip appeared where none was given:\n%s", got)
	}
}

// A value with a newline in it cannot forge a second entry.
func TestAValueCannotForgeAnEntry(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	j.Audit(ports.AuditEntry{Actor: "mallory", Operation: "manage", Target: "x\n2026 actor=\"root\"", Result: "ok"})
	if lines := strings.Count(read(t, dir, "audit.log"), "\n"); lines != 1 {
		t.Fatalf("%d lines", lines)
	}
}

func TestReportWritesTheErrorLog(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	j.Report(ports.Alert, "token pair names alice and #svc@h, whose owner is bob")
	got := read(t, dir, "error.log")
	if !strings.Contains(got, `alert "token pair names alice and #svc@h, whose owner is bob"`) {
		t.Fatalf("error log:\n%s", got)
	}
}

// sent records the severity each syslog line was sent at.
type sent struct{ lines []string }

func (s *sent) Warning(m string) error { s.lines = append(s.lines, "warning "+m); return nil }
func (s *sent) Err(m string) error     { s.lines = append(s.lines, "err "+m); return nil }
func (s *sent) Alert(m string) error   { s.lines = append(s.lines, "alert "+m); return nil }
func (s *sent) Close() error           { return nil }

// Every error-log line reaches syslog once, at the severity it was reported
// at, not one level for all.
func TestReportReachesSyslogAtItsSeverity(t *testing.T) {
	j, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	sys := &sent{}
	j.sys = sys
	j.Report(ports.Warning, "w")
	j.Report(ports.Error, "e")
	j.Report(ports.Alert, "a")
	if got, want := strings.Join(sys.lines, "|"), "warning w|err e|alert a"; got != want {
		t.Fatalf("syslog got %q, want %q", got, want)
	}
}

func TestDebugIsOnDemand(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	j.Request(ports.RequestLine{Method: "GET", Path: "/status", Status: 200})
	if _, err := os.Stat(filepath.Join(dir, "debug.log")); !os.IsNotExist(err) {
		t.Fatal("a request was logged with the debug log off")
	}
	if err := j.SetDebug(true); err != nil {
		t.Fatal(err)
	}
	j.Request(ports.RequestLine{Caller: "alice", Method: "POST", Path: "/send", Status: 403})
	if err := j.SetDebug(false); err != nil {
		t.Fatal(err)
	}
	j.Request(ports.RequestLine{Caller: "after", Method: "GET", Path: "/status", Status: 200})
	got := read(t, dir, "debug.log")
	if !strings.Contains(got, `caller="alice" POST "/send" status=403`) || strings.Contains(got, "after") {
		t.Fatalf("debug log:\n%s", got)
	}
}

// A debug log asked for at start that cannot be opened is said in the error
// log, not only in the refusal (K.33).
func TestADebugLogThatWillNotOpenIsReported(t *testing.T) {
	dir := t.TempDir()
	// A directory where the file would be cannot be opened as one.
	if err := os.Mkdir(filepath.Join(dir, "debug.log"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir, true); err == nil {
		t.Fatal("a debug log that cannot open was opened")
	}
	if got := read(t, dir, "error.log"); !strings.Contains(got, `error "debug.log could not be opened: `) {
		t.Fatalf("error log:\n%s", got)
	}
}
