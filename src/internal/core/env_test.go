package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A secret is an env file: basic syntax, nothing application-specific, and a
// refusal that names the line and never repeats it.
// See docs/constitution.md#-private-values.
func TestASecretMustBeAnEnvFile(t *testing.T) {
	for _, ok := range []string{
		"TOKEN=abc",
		"# a comment\n\nexport PGPASSWORD='s3 cret'\nURL=\"https://x/?a=b\" \r\nEMPTY=\n",
		"_UNDER_1=value with spaces and = signs",
	} {
		if err := validEnv(ok); err != nil {
			t.Errorf("%q was refused: %v", ok, err)
		}
	}
	for secret, line := range map[string]string{
		"just a password":          "line 1",
		"GOOD=1\n1BAD=2":           "line 2",
		"A-B=1":                    "line 1",
		"=nokey":                   "line 1",
		"OK=1\n\nQ=\"unterminated": "line 3",
		"Q='":                      "line 1",
	} {
		err := validEnv(secret)
		if !errors.Is(err, ErrEnv) || !strings.Contains(err.Error(), line) {
			t.Errorf("%q: %v, want a refusal naming %s", secret, err, line)
			continue
		}
		for _, part := range []string{"password", "BAD", "nokey", "unterminated"} {
			if strings.Contains(secret, part) && strings.Contains(err.Error(), part) {
				t.Errorf("the refusal repeated the secret: %v", err)
			}
		}
	}
	if err := validEnv("A=1\x00"); !errors.Is(err, ErrEnv) {
		t.Errorf("a NUL byte was accepted: %v", err)
	}
}

// The write is refused whole, and what was stored stays.
func TestAnInvalidSecretIsRefusedAndStoresNothing(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	provision(t, b, []string{"owner@h"}, protocol.Record{Name: "db@h", Kind: protocol.KindService, Owner: "owner@h", Addr: "db:5432", Proto: "postgresql", Allow: []string{"*"}})
	if _, err := b.SetSecret("db@h", "owner@h", "PGPASSWORD=kept"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetSecret("db@h", "owner@h", "not an env file"); !errors.Is(err, ErrEnv) {
		t.Fatalf("an invalid secret was stored: %v", err)
	}
	if got, _ := b.Secret("db@h", "owner@h"); got != "PGPASSWORD=kept" {
		t.Fatalf("the refused write changed the secret to %q", got)
	}
}

// Configuration is read by the record's own principal where it has one, and
// otherwise by whoever its ACL admits; its owner writes it but does not read
// an agent's back.
func TestAConfigurationIsReadByTheRecordsPrincipalOrItsACL(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	provision(t, b, []string{"owner@h", "reader@h", "stranger@h"},
		protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"reader@h"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "db@h", Kind: protocol.KindService, Owner: "owner@h", Addr: "db:5432", Proto: "postgresql", Allow: []string{"reader@h"}},
	)
	for _, name := range []string{"#worker@h", "db@h"} {
		if _, err := b.Configure(name, "owner@h", json.RawMessage(`{ "k" : 1 }`)); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := b.Config("#worker@h", "#worker@h"); err != nil || string(got) != `{"k":1}` {
		t.Fatalf("the agent read %s, %v; want its compact configuration", got, err)
	}
	for _, who := range []string{"owner@h", "reader@h"} {
		if _, err := b.Config("#worker@h", who); !errors.Is(err, ErrPrivate) {
			t.Errorf("%s read the agent's configuration: %v", who, err)
		}
	}
	if got, err := b.Config("db@h", "reader@h"); err != nil || string(got) != `{"k":1}` {
		t.Fatalf("the service's ACL could not read its configuration: %s, %v", got, err)
	}
	if _, err := b.Config("db@h", "stranger@h"); !errors.Is(err, ErrUnknown) {
		t.Errorf("a caller the ACL does not admit read the service's configuration: %v", err)
	}
}

// A stored secret that is not an env file, or a configuration that is not
// compact, is one no write of this version produced: ignored and reported.
func TestRestoreIgnoresANonconformingPrivateValue(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "#badsecret@h", Kind: protocol.KindAgent, Owner: "owner@h", Secret: "not an env file", Full: protocol.OverflowStrict},
		{Name: "#loose@h", Kind: protocol.KindAgent, Owner: "owner@h", Config: []byte(`{ "k": 1 }`), Full: protocol.OverflowStrict},
		{Name: "#fine@h", Kind: protocol.KindAgent, Owner: "owner@h", Secret: "A=1", Config: []byte(`{"k":1}`), Full: protocol.OverflowStrict},
	}}, "owner@h"))
	for _, name := range []string{"#badsecret@h", "#loose@h"} {
		if !rep.has("stored record " + name + " is ignored") {
			t.Errorf("%s was not ignored and reported: %v", name, rep.lines)
		}
	}
	if !b.IsPerson("owner@h") {
		t.Fatal("fixture")
	}
	if _, ok := b.records["#fine@h"]; !ok {
		t.Fatal("a conforming record was ignored")
	}
}
