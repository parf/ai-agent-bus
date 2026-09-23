// Package journal writes the daemon's three logs under one directory —
// debug.log, audit.log and error.log — and copies every error-log line to
// syslog (docs/constitution.md#logs). Files are created 0640, so a directory
// that is set-group-ID to adm, as setup makes /var/log/agent-bus, lets that
// group read them; rotation is logrotate's, by copy and truncate, so nothing
// here reopens a file.
package journal

import (
	"fmt"
	"log/syslog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Journal is the three files and the syslog connection.
type Journal struct {
	dir   string
	mu    sync.Mutex
	audit *os.File
	error *os.File
	debug *os.File
	sys   *syslog.Writer
	// sysWarned records that syslog could not be reached, said once.
	sysWarned bool
}

// Open opens the audit and error logs in dir, creating it if absent. The
// debug log is opened only when asked for.
func Open(dir string, debug bool) (*Journal, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	j := &Journal{dir: dir}
	var err error
	if j.audit, err = j.open("audit.log"); err != nil {
		return nil, err
	}
	if j.error, err = j.open("error.log"); err != nil {
		j.audit.Close()
		return nil, err
	}
	// Syslog is the second copy, not the first: a host without one still
	// has the error log, and the error log says the copy was not made.
	j.sys, _ = syslog.New(syslog.LOG_DAEMON|syslog.LOG_WARNING, "agent-busd")
	if debug {
		if err := j.SetDebug(true); err != nil {
			j.Close()
			return nil, err
		}
	}
	return j, nil
}

func (j *Journal) open(name string) (*os.File, error) {
	return os.OpenFile(filepath.Join(j.dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
}

// Close closes every file and the syslog connection.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, f := range []*os.File{j.audit, j.error, j.debug} {
		if f != nil {
			f.Close()
		}
	}
	if j.sys != nil {
		j.sys.Close()
	}
	return nil
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// field keeps one value on one line and readable by a shell: it is quoted,
// and a newline or a quote inside it cannot start a forged entry.
func field(s string) string { return strconv.Quote(s) }

// Audit writes one audit entry.
func (j *Journal) Audit(e ports.AuditEntry) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s actor=%s op=%s target=%s result=%s", stamp(e.At),
		field(e.Actor), field(e.Operation), field(e.Target), field(e.Result))
	if e.ClientIP != "" {
		fmt.Fprintf(&b, " ip=%s", field(e.ClientIP))
	}
	b.WriteByte('\n')
	j.mu.Lock()
	defer j.mu.Unlock()
	j.audit.WriteString(b.String())
}

// Report writes a line to the error log and the same line to syslog.
func (j *Journal) Report(sev ports.Severity, msg string) {
	line := fmt.Sprintf("%s %s %s\n", stamp(time.Now()), sev, field(msg))
	j.mu.Lock()
	defer j.mu.Unlock()
	j.error.WriteString(line)
	if j.sys == nil {
		if !j.sysWarned {
			j.sysWarned = true
			j.error.WriteString(fmt.Sprintf("%s warning %s\n", stamp(time.Now()),
				field("syslog is unreachable; error-log lines are not copied to it")))
		}
		return
	}
	var err error
	switch sev {
	case ports.Warning:
		err = j.sys.Warning(msg)
	case ports.Error:
		err = j.sys.Err(msg)
	default:
		err = j.sys.Alert(msg)
	}
	if err != nil && !j.sysWarned {
		j.sysWarned = true
		j.error.WriteString(fmt.Sprintf("%s warning %s\n", stamp(time.Now()),
			field("syslog refused a line: "+err.Error())))
	}
}

// Request writes one debug line while the debug log is on.
func (j *Journal) Request(r ports.RequestLine) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.debug == nil {
		return
	}
	if r.At.IsZero() {
		r.At = time.Now()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s caller=%s %s %s status=%d duration=%s", stamp(r.At), field(r.Caller),
		r.Method, field(r.Path), r.Status, r.Duration.Round(time.Microsecond))
	if r.ClientIP != "" {
		fmt.Fprintf(&b, " ip=%s", field(r.ClientIP))
	}
	b.WriteByte('\n')
	j.debug.WriteString(b.String())
}

// SetDebug opens or closes the debug log.
func (j *Journal) SetDebug(on bool) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if on == (j.debug != nil) {
		return nil
	}
	if !on {
		err := j.debug.Close()
		j.debug = nil
		return err
	}
	f, err := j.open("debug.log")
	if err != nil {
		return err
	}
	j.debug = f
	return nil
}

// DebugOn says whether request lines are being written.
func (j *Journal) DebugOn() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.debug != nil
}
