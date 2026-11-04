package core

import (
	"fmt"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// DaemonAgent is per daemon, not in the caller's realm.
const DaemonAgent = "#bus"

func daemonAgentRecord(old protocol.Record, owner string) protocol.Record {
	r := protocol.Record{Name: DaemonAgent, Kind: protocol.KindAgent, Owner: owner,
		Descr: "The daemon on the bus: help, status, version and ls", Allow: []string{"*"}, Full: protocol.OverflowStrict,
		ID: old.ID, OwnerID: old.OwnerID, Created: old.Created, At: old.At, Config: old.Config}
	if r.At.IsZero() {
		r.At = time.Now()
	}
	return r
}

// EstablishDaemonAgent runs after owner restoration and before credentials or
// listeners. Only the daemon writes this record; ordinary management cannot.
// Its Configuration is the bus setup: the first start saves the host name as
// the realm, and a saved setup is never replaced by the host's.
func (b *Bus) EstablishDaemonAgent(host string) error {
	b.mu.Lock()
	defer b.unlock()
	if b.admin == "" {
		return fmt.Errorf("daemon owner is not established")
	}
	r := daemonAgentRecord(b.records[DaemonAgent], b.admin)
	saved := false
	if len(r.Config) == 0 {
		r.Config = hostRealm(host)
		saved = len(r.Config) > 0
	}
	b.setRecord(DaemonAgent, r)
	b.ensure(DaemonAgent)
	if err := b.commit(); err != nil {
		return err
	}
	// A first start chooses the realm without the Owner, so it is audited as the daemon's.
	if saved && b.journal != nil {
		b.journal.Audit(ports.AuditEntry{At: time.Now(), Actor: "(daemon start)", Operation: "set realm", Target: DaemonAgent,
			Result: "ok: (none) -> " + busRealm(r.Config)})
	}
	return nil
}
