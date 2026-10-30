package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// BusSetting declares one key of the #bus Configuration: the bus setup.
// See Plans/R1.0-Release/daemon-configuration.md#approved-plan.
type BusSetting struct {
	Key     string
	Help    string
	Default any // nil: no default; realm defaults to the host name at first start
	Max     int // the upper bound of an integer key; 0 for a string
}

// BusSettings are the keys #bus declares, in the order its help lists them.
// The agent-event keys are kept and validated now and applied by RT34.
var BusSettings = []BusSetting{
	{Key: "realm", Help: "The bus's default realm for new automatic names; existing names never change"},
	{Key: "notice_entries", Help: "Agent-event notices kept per agent", Default: 200, Max: 10000},
	{Key: "notice_days", Help: "Days an agent-event notice is kept", Default: 1, Max: 3650},
	{Key: "warning_entries", Help: "Agent-event warnings kept per agent", Default: 200, Max: 10000},
	{Key: "warning_days", Help: "Days an agent-event warning is kept", Default: 7, Max: 3650},
	{Key: "alert_entries", Help: "Agent-event alerts kept per agent", Default: 50, Max: 10000},
	{Key: "alert_days", Help: "Days an agent-event alert is kept", Default: 30, Max: 3650},
}

// validBusConfig checks a whole #bus Configuration: an object of declared
// keys only, a realm in canonical form, integers within their bounds. Nothing
// is trimmed, lower-cased or cut short on the way in.
func validBusConfig(raw json.RawMessage) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var values map[string]any
	if err := dec.Decode(&values); err != nil || values == nil {
		return fmt.Errorf("%w: the bus setup is a JSON object", ErrConfig)
	}
	declared := map[string]BusSetting{}
	for _, s := range BusSettings {
		declared[s.Key] = s
	}
	for key, value := range values {
		s, ok := declared[key]
		if !ok {
			return fmt.Errorf("%w: the bus setup has no key %q", ErrConfig, key)
		}
		if s.Max == 0 {
			realm, ok := value.(string)
			if !ok {
				return fmt.Errorf("%w: realm is a string", ErrConfig)
			}
			if err := protocol.ValidRealm(realm); err != nil {
				return fmt.Errorf("%w: %s", ErrConfig, err)
			}
			continue
		}
		n, ok := value.(json.Number)
		v, err := n.Int64()
		if !ok || err != nil || v < 1 || v > int64(s.Max) {
			return fmt.Errorf("%w: %s is a whole number from 1 to %d", ErrConfig, key, s.Max)
		}
	}
	if _, ok := values["realm"]; !ok {
		return fmt.Errorf("%w: the bus setup needs a realm", ErrConfig)
	}
	return nil
}

func busRealm(cfg json.RawMessage) string {
	var v struct {
		Realm string `json:"realm"`
	}
	if len(cfg) == 0 || json.Unmarshal(cfg, &v) != nil {
		return ""
	}
	return v.Realm
}

// hostRealm is the first-start default: the host name as a realm, or none
// when the host name is not one.
func hostRealm(host string) json.RawMessage {
	realm := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if protocol.ValidRealm(realm) != nil {
		return nil
	}
	cfg, _ := json.Marshal(map[string]string{"realm": realm})
	return cfg
}

// Realm is the effective default realm: the saved bus setup's, or "" when the
// daemon has none. A save applies at once; nothing caches it.
func (b *Bus) Realm() string {
	b.mu.Lock()
	defer b.unlock()
	return busRealm(b.records[DaemonAgent].Config)
}

// configureBus saves the bus setup. Only the daemon Owner may: not a
// Maintainer, an Administrator or a role a message claims. The caller holds
// b.mu; the realm change is audited with its old and new value.
func (b *Bus) configureBus(who string, cfg json.RawMessage) (protocol.Record, error) {
	if who != b.admin {
		return protocol.Record{}, fmt.Errorf("%w: only the daemon Owner writes the bus setup", ErrNotOwner)
	}
	if err := validBusConfig(cfg); err != nil {
		return protocol.Record{}, err
	}
	r, ok := b.records[DaemonAgent]
	if !ok {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrUnknown, DaemonAgent)
	}
	old, next := busRealm(r.Config), busRealm(cfg)
	r.Config = cfg
	r.At = time.Now()
	b.setRecord(DaemonAgent, r)
	if err := b.commit(); err != nil {
		return protocol.Record{}, err
	}
	if old != next && b.journal != nil {
		b.journal.Audit(ports.AuditEntry{At: time.Now(), Actor: who, Operation: "set realm", Target: DaemonAgent,
			Result: "ok: " + orNone(old) + " -> " + next})
	}
	return r.Public(), nil
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
