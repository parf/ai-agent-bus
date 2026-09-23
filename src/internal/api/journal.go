package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The API's part of the daemon's logs (docs/constitution.md#logs): an audit
// entry for every administrative action and entity edit, and — only while the
// daemon Owner has it on — a debug line for every request. Neither reads a
// body beyond the name it is about, so no token, secret, configuration or
// message reaches either.

// Journal binds the daemon's logs. Unbound, the server writes none.
func (s *Server) Journal(j ports.Journal) { s.journal = j }

func (s *Server) logs() ports.Journal {
	if s.journal == nil {
		return ports.Silent{}
	}
	return s.journal
}

// statusWriter remembers the status a handler answered with.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}

// callerBox is where a guard leaves the principal it established, so the
// request line can name who called without reading the credential again.
type callerBox struct{ name string }

type callerKey struct{}

func noteCaller(r *http.Request, who protocol.Name) {
	if b, ok := r.Context().Value(callerKey{}).(*callerBox); ok {
		b.name = who.String()
	}
}

// clientIP is the address of a TCP caller. A unix-socket call has none, and
// none is invented for it.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || net.ParseIP(host) == nil {
		return ""
	}
	return host
}

// logged writes the debug line of a request while the debug log is on.
func (s *Server) logged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		box := &callerBox{}
		r = r.WithContext(context.WithValue(r.Context(), callerKey{}, box))
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		j := s.logs()
		if !j.DebugOn() {
			return
		}
		j.Request(ports.RequestLine{
			At: start, Caller: box.name, Method: r.Method, Path: r.URL.Path,
			Status: sw.status(), Duration: time.Since(start), ClientIP: clientIP(r),
		})
	})
}

// targetFields are the request fields that name what an operation is about,
// most specific first.
var targetFields = []string{"name", "channel", "account", "owner", "principal"}

// audited writes one audit entry for an administrative action, whatever its
// outcome. Only the naming fields of the body are read, and only after the
// handler has had the body it was sent.
func (s *Server) audited(op string, next func(http.ResponseWriter, *http.Request, protocol.Name)) func(http.ResponseWriter, *http.Request, protocol.Name) {
	return func(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		sw := &statusWriter{ResponseWriter: w}
		next(sw, r, caller)
		// Enrolment is the one edit made before there is a caller: the
		// signature is the credential, so the actor is said to be that.
		actor := caller.String()
		if caller == (protocol.Name{}) {
			actor = "(enrolment signature)"
		}
		s.logs().Audit(ports.AuditEntry{
			Actor: actor, Operation: op, Target: target(body, caller),
			Result: result(sw.status()), ClientIP: clientIP(r),
		})
	}
}

func target(body []byte, caller protocol.Name) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) == nil {
		for _, f := range targetFields {
			var v string
			if raw, ok := fields[f]; ok && json.Unmarshal(raw, &v) == nil && v != "" {
				return v
			}
		}
	}
	return caller.String()
}

func result(code int) string {
	switch {
	case code < 400:
		return "ok"
	case code < 500:
		return fmt.Sprintf("refused %d", code)
	default:
		return fmt.Sprintf("failed %d", code)
	}
}

// debugLog lets the daemon Owner switch the debug log at run time; the flag
// at start is the other way. Anybody else is refused and nothing changes.
func (s *Server) debugLog(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	if caller.String() != s.bus.DaemonOwner() {
		s.refuse(w, http.StatusForbidden, "acl", "only the daemon owner switches the debug log")
		return
	}
	if r.Method == http.MethodPost {
		var in struct {
			On *bool `json:"on"`
		}
		if !s.read(w, r, &in) {
			return
		}
		if in.On == nil {
			s.refuse(w, http.StatusBadRequest, "malformed", `want {"on": true} or {"on": false}`)
			return
		}
		if err := s.logs().SetDebug(*in.On); err != nil {
			oops(w, err)
			return
		}
	}
	ok(w, struct {
		On bool `json:"on"`
	}{s.logs().DebugOn()})
}
