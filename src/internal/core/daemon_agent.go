package core

import (
	"fmt"
	"reflect"
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
	r.SyncTags()
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
	if len(r.Config) == 0 {
		r.Config = hostRealm(host)
	}
	b.setRecord(DaemonAgent, r)
	b.ensure(DaemonAgent)
	return b.commit()
}

// Restore never trusts stored ACL, routing, private data or ownership for the
// reserved identity. Queue contents and record IDs retain normal durability.
func (b *Bus) restoreDaemonAgent(r protocol.Record) protocol.Record {
	next := daemonAgentRecord(r, b.admin)
	if len(next.Config) > 0 && validBusConfig(next.Config) != nil {
		next.Config = nil
		b.report(ports.Warning, "stored bus setup is invalid; ignored")
	}
	if user, ok := b.users[b.admin]; ok {
		next.OwnerID = user.ID
	}
	if !reflect.DeepEqual(r, next) {
		b.report(ports.Warning, "stored #bus metadata conflicts with the daemon-owned identity; ignored")
	}
	return next
}
