package ports

import "time"

// The journal port: the daemon's three logs, and nothing about where they are
// written (docs/constitution.md#logs). No entry carries a token, a secret, a
// configuration body or a message body; the types below have no field that
// could hold one.

// Severity says how much a reported condition needs attention.
type Severity int

const (
	Warning Severity = iota
	Error
	Alert
)

func (s Severity) String() string {
	switch s {
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "alert"
	}
}

// AuditEntry is one administrative action or entity edit: who, what, to which
// name, with what outcome, and from where when the call came over TCP.
type AuditEntry struct {
	At        time.Time
	Actor     string
	Operation string
	Target    string
	Result    string
	ClientIP  string
}

// RequestLine is one request, for the debug log.
type RequestLine struct {
	At       time.Time
	Caller   string
	Method   string
	Path     string
	Status   int
	Duration time.Duration
	ClientIP string
}

// Journal writes the three logs.
type Journal interface {
	// Audit writes one entry to the audit log.
	Audit(AuditEntry)
	// Report writes a warning, error or alert to the error log and syslog.
	Report(Severity, string)
	// Request writes one line to the debug log when it is on, and nothing
	// otherwise.
	Request(RequestLine)
	// SetDebug turns the debug log on or off; DebugOn says which it is.
	SetDebug(on bool) error
	DebugOn() bool
}

// Silent is a journal that writes nothing: an embedded or test bus.
type Silent struct{}

func (Silent) Audit(AuditEntry)        {}
func (Silent) Report(Severity, string) {}
func (Silent) Request(RequestLine)     {}
func (Silent) SetDebug(bool) error     { return nil }
func (Silent) DebugOn() bool           { return false }
