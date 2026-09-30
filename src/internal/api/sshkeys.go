// A User's ssh key, added by whoever may edit that User: one authorized_keys
// line forced to the credential program, as `agent-bus-admin user add` writes
// it (docs/09-setup.md#ssh-admin). The daemon account owns the file, so the
// daemon may write it; it does only when started with -ssh-keys.
package api

import (
	"errors"
	"net/http"

	"github.com/parf/ai-agent-bus/internal/authkeys"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

var (
	errNoSSHKeys   = errors.New("this node takes ssh keys on the host: agent-bus-admin user add")
	errOperatorKey = errors.New("that user's key reaches the admin program, and is changed on the host")
)

// SSHKeys lets the API add keys to the file at path, forced to program.
func (s *Server) SSHKeys(path, program string) { s.sshKeys, s.sshToken = path, program }

func (s *Server) userKey(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	}
	if !s.readStrict(w, r, &in) {
		return
	}
	n, err := protocol.ParseName(in.Name)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	key, err := authkeys.Ed25519(in.Key)
	if err != nil {
		s.refuse(w, http.StatusBadRequest, "malformed", err.Error())
		return
	}
	if err := s.bus.MayAddUserKey(caller.String(), n.String()); err != nil {
		s.reply(w, nil, err)
		return
	}
	if s.sshKeys == "" {
		s.refuse(w, http.StatusNotImplemented, "malformed", errNoSSHKeys.Error())
		return
	}
	err = authkeys.Update(s.sshKeys, func(lines []string) ([]string, error) {
		for _, l := range lines {
			if who, program := authkeys.Whose(l); who == n.String() && program == "agent-bus-admin" {
				return nil, errOperatorKey
			}
		}
		return append(authkeys.Without(lines, n.String()), authkeys.Line(s.sshToken, n.String(), key)), nil
	})
	if errors.Is(err, errOperatorKey) {
		s.refuse(w, http.StatusConflict, "busy", err.Error())
		return
	}
	s.reply(w, map[string]string{"name": n.String(), "key": key}, err)
}
