package api

import (
	"net/http"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func (s *Server) removeIdentity(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct{ Name string }
	if s.read(w, r, &in) {
		s.reply(w, nil, s.bus.RemoveOwnerless(caller.String(), in.Name, s.tokens.Forget))
	}
}
