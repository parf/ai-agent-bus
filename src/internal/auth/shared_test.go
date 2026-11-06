package auth

import (
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// B7: two stored names on one token refuse the load, naming both names and
// never the token; Load used to give the token to whichever row came last.
func TestLoadRefusesTwoNamesOnOneToken(t *testing.T) {
	pair := ports.CredentialPair{UserID: 1}
	const shared = "tok-shared-secret"
	for _, c := range []struct {
		what string
		a, b ports.Credential
	}{
		{"current and current", ports.Credential{Name: "alice@h", Current: shared, CredentialPair: pair},
			ports.Credential{Name: "bob@h", Current: shared, CredentialPair: pair}},
		{"previous and current", ports.Credential{Name: "alice@h", Current: "tok-a", Previous: shared, CredentialPair: pair},
			ports.Credential{Name: "bob@h", Current: shared, CredentialPair: pair}},
		{"current and previous", ports.Credential{Name: "alice@h", Current: shared, CredentialPair: pair},
			ports.Credential{Name: "bob@h", Current: "tok-b", Previous: shared, CredentialPair: pair}},
		{"previous and previous", ports.Credential{Name: "alice@h", Current: "tok-a", Previous: shared, CredentialPair: pair},
			ports.Credential{Name: "bob@h", Current: "tok-b", Previous: shared, CredentialPair: pair}},
	} {
		st := memory.NewTokens()
		st.Put(c.a)
		st.Put(c.b)
		_, err := Load(st)
		if err == nil {
			t.Fatalf("%s: two names on one token loaded", c.what)
		}
		msg := err.Error()
		if !strings.Contains(msg, "alice@h") || !strings.Contains(msg, "bob@h") || !strings.Contains(msg, "share one credential") {
			t.Fatalf("%s: the refusal does not name both names: %q", c.what, msg)
		}
		if strings.Contains(msg, shared) {
			t.Fatalf("%s: the refusal shows the token: %q", c.what, msg)
		}
	}
	// Control: the same rows with distinct tokens load, and each token backs its own name.
	st := memory.NewTokens()
	st.Put(ports.Credential{Name: "alice@h", Current: "tok-a", Previous: "tok-a0", CredentialPair: pair})
	st.Put(ports.Credential{Name: "bob@h", Current: "tok-b", Previous: "tok-b0", CredentialPair: pair})
	tokens, err := Load(st)
	if err != nil {
		t.Fatal(err)
	}
	for tok, want := range map[string]string{"tok-a": "alice@h", "tok-a0": "alice@h", "tok-b": "bob@h", "tok-b0": "bob@h"} {
		if who, ok := tokens.Principal(tok); !ok || who != want {
			t.Fatalf("%s backs %q (%v), want %s", tok, who, ok, want)
		}
	}
}
