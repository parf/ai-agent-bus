package auth

import (
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"testing"
)

func testTokens(st ports.TokenStore, owner string) (*Tokens, error) {
	t, err := Load(st)
	if err != nil {
		return nil, err
	}
	t.PairOf(func(name string) (ports.CredentialPair, error) {
		p := ports.CredentialPair{UserID: 1}
		if protocol.IsAgentName(name) {
			p.AgentID = 2
		}
		return p, nil
	})
	_, err = t.Issue(owner)
	return t, err
}

func TestStoredCredentialIDsAreRequiredWithoutConversion(t *testing.T) {
	for _, c := range []ports.Credential{
		{Name: "owner@h", Current: "kept"},
		{Name: "#agent@h", Current: "kept", CredentialPair: ports.CredentialPair{UserID: 1}},
		{Name: "owner@h", Current: "kept", CredentialPair: ports.CredentialPair{UserID: 1, AgentID: 2}},
	} {
		st := memory.NewTokens()
		st.Put(c)
		if _, err := Load(st); err == nil {
			t.Fatalf("invalid pair accepted for %s", c.Name)
		}
		rows, _ := st.Load()
		if len(rows) != 1 || rows[0] != c {
			t.Fatal("load converted stored credential")
		}
	}
}

func TestCredentialsMintOnlyAfterRealIDsAreAvailable(t *testing.T) {
	st := memory.NewTokens()
	tokens, err := Load(st)
	if err != nil {
		t.Fatal(err)
	}
	if rows, _ := st.Load(); len(rows) != 0 {
		t.Fatal("load minted an unbound owner token")
	}
	if _, err := tokens.Issue("owner@h"); err == nil {
		t.Fatal("minted before registry binding")
	}
	tokens.PairOf(func(string) (ports.CredentialPair, error) { return ports.CredentialPair{UserID: 3}, nil })
	if _, err := tokens.Issue("owner@h"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(st); err != nil {
		t.Fatalf("canonical credential did not reload: %v", err)
	}
}

func TestBoundOrphanCredentialLoadsWithoutRebinding(t *testing.T) {
	st := memory.NewTokens()
	c := ports.Credential{Name: "#former@h", Current: "retained", CredentialPair: ports.CredentialPair{UserID: 72, AgentID: 91}}
	if err := st.Put(c); err != nil {
		t.Fatal(err)
	}
	tokens, err := Load(st)
	if err != nil {
		t.Fatal(err)
	}
	if got := tokens.Pairs()[c.Name]; got != c.CredentialPair {
		t.Fatalf("orphan was rebound: %+v", got)
	}
}
