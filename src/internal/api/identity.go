package api

import (
	"net/http"
	"os"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

// Calls binds the process-owned request counter before any listener serves.
// An unbound counter is unavailable, never a fabricated zero.
func (s *Server) Calls(snapshot func(time.Time) protocol.CallStats) { s.calls = snapshot }

func (s *Server) identity(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	node := protocol.NodeIdentity{Version: version.String, Build: version.Build,
		Owner: s.owner, Up: s.bus.Uptime(), Host: host}
	if s.calls != nil {
		stats := s.calls(time.Now())
		node.Calls = &stats
	}
	w.Header().Set("Cache-Control", "no-store")
	ok(w, node)
}
