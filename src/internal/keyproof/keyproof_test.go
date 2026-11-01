package keyproof

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ns(v string) json.RawMessage { b, _ := json.Marshal(v); return b }

const goodNonce = "0123456789abcdef0123456789abcdef"

func TestCheckRefusesWhatIsNotTheBusProof(t *testing.T) {
	cases := []struct {
		name string
		c    Challenge
		want string // "" = accepted
	}{
		{"the bus's own answer", Challenge{"alice@r", goodNonce, ns("agent-bus")}, ""},
		{"an answer without a namespace", Challenge{"alice@r", goodNonce, nil}, ""},
		{"an empty namespace", Challenge{"alice@r", goodNonce, json.RawMessage(`""`)}, "namespace"},
		{"a null namespace", Challenge{"alice@r", goodNonce, json.RawMessage(`null`)}, "namespace"},
		{"a numeric namespace", Challenge{"alice@r", goodNonce, json.RawMessage(`7`)}, "namespace"},
		{"another namespace", Challenge{"alice@r", goodNonce, ns("git")}, "namespace"},
		{"another name", Challenge{"bob@r", goodNonce, ns("agent-bus")}, "name"},
		{"a short nonce", Challenge{"alice@r", "abc", ns("agent-bus")}, "nonce"},
		{"a nonce with capitals", Challenge{"alice@r", strings.ToUpper(goodNonce), ns("agent-bus")}, "nonce"},
		{"a nonce that is not hex", Challenge{"alice@r", strings.Repeat("z", 32), ns("agent-bus")}, "nonce"},
		{"a long free-text message", Challenge{"alice@r", "tree 0123\nauthor x\n" + goodNonce, ns("agent-bus")}, "nonce"},
	}
	for _, tc := range cases {
		err := tc.c.Check("alice@r")
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: refused: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one naming %q", tc.name, err, tc.want)
			continue
		}
		for _, secret := range []string{"git", "bob@r", "author x"} {
			if strings.Contains(string(tc.c.Namespace)+tc.c.Name+tc.c.Nonce, secret) && strings.Contains(err.Error(), secret) {
				t.Errorf("%s: the refusal echoes %q: %v", tc.name, secret, err)
			}
		}
	}
}

// stub puts an ssh-keygen on PATH that records how it was called.
func stub(t *testing.T) (log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\ncat >> '" + log + ".stdin'\necho SIGNATURE\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh-keygen"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return log
}

func TestSignAlwaysUsesTheBusNamespace(t *testing.T) {
	log := stub(t)
	key := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(key, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	sig, err := Sign(key, goodNonce)
	if err != nil || !strings.Contains(sig, "SIGNATURE") {
		t.Fatalf("Sign = %q, %v", sig, err)
	}
	argv, _ := os.ReadFile(log)
	if !strings.Contains(string(argv), "-n agent-bus ") {
		t.Fatalf("ssh-keygen was called with %q, want -n agent-bus", argv)
	}
	stdin, _ := os.ReadFile(log + ".stdin")
	if string(stdin) != goodNonce {
		t.Fatalf("the message signed was %q, want the nonce", stdin)
	}
}
