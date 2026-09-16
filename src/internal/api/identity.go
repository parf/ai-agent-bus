package api

import (
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

// Keep the public projection separate from private status. There is no caller
// or ACL in these node totals, and no record or person list in the answer.
func (s *Server) identity(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	load, _ := os.ReadFile("/proc/loadavg")
	w.Header().Set("Cache-Control", "no-store")
	ok(w, protocol.NodeIdentity{
		Version: version.String, Build: version.Build, Owner: s.owner,
		Up: s.bus.Uptime(), Host: host, Load: parseLoad(load),
		Messages: s.bus.NodeMessages(time.Now()),
	})
}

// Missing or unsupported host measurements stay unknown, never a fabricated 0.
func parseLoad(data []byte) *[3]float64 {
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return nil
	}
	var load [3]float64
	for i := range load {
		n, err := strconv.ParseFloat(fields[i], 64)
		if err != nil || n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil
		}
		load[i] = n
	}
	return &load
}
