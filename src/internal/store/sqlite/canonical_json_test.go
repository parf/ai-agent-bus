package sqlite

import (
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestStoredRecordJSONRefusesUnsupportedMembers(t *testing.T) {
	for name, body := range map[string]string{
		"personal":          `{"name":"a@h","kind":"queue","personal":false}`,
		"reading":           `{"name":"a@h","kind":"queue","reading":false}`,
		"unknown":           `{"name":"a@h","kind":"queue","retired":true}`,
		"case":              `{"Name":"a@h","kind":"queue"}`,
		"duplicate":         `{"name":"a@h","name":"a@h","kind":"queue"}`,
		"scalar maintainer": `{"name":"a@h","kind":"queue","maintainers":"owner@h"}`,
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := open(t)
			if _, err := s.db.Exec(`INSERT INTO records(name,id,kind,body,owner_id) VALUES ('a@h',1,'queue',?,1)`, body); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Load(); err == nil {
				t.Fatal("unsupported stored JSON accepted")
			}
			var after string
			if err := s.db.QueryRow(`SELECT body FROM records WHERE name='a@h'`).Scan(&after); err != nil || after != body {
				t.Fatal("refusal changed stored row")
			}
		})
	}
}

func TestStoredCanonicalJSONKeepsOpaqueConfiguration(t *testing.T) {
	s, _ := open(t)
	r := record("a@h", 1)
	r.Config = []byte(`{"personal":false,"Name":"application data","reading":true}`)
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": r}}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Load()
	if err != nil || len(snap.Records) != 1 || !strings.Contains(string(snap.Records[0].Config), `"personal"`) {
		t.Fatalf("opaque Configuration changed: %+v %v", snap, err)
	}
}
