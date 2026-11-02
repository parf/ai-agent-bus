package subscription

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"os"
	"testing"
)

func TestSubscriptionNameVectorsMatchProtocol(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/client-conformance/subscription-names.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		Accept []struct{ Name, Canonical string }
		Reject []string
	}
	if err = json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors.Accept) == 0 || len(vectors.Reject) == 0 {
		t.Fatal("name vectors empty")
	}
	for _, v := range vectors.Accept {
		t.Run("accept/"+v.Name, func(t *testing.T) {
			n, err := protocol.ParseName(v.Name)
			if err != nil || n.String() != v.Canonical {
				t.Fatalf("protocol expected %q: %+v %v", v.Canonical, n, err)
			}
			e, err := New(nil, nil, Config{Source: v.Name})
			if err != nil || e.cfg.Source != v.Canonical {
				t.Fatalf("source name %q: %v", v.Name, err)
			}
			for _, parse := range []func(string) (string, error){targetText, identity} {
				got, err := parse(v.Name)
				if err != nil || got != v.Canonical {
					t.Fatalf("subscription name %q: got %q (%v), want %q", v.Name, got, err, v.Canonical)
				}
			}
		})
	}
	for _, name := range vectors.Reject {
		t.Run("reject/"+name, func(t *testing.T) {
			if _, err := protocol.ParseName(name); err == nil {
				t.Fatalf("protocol accepted rejected name %q", name)
			}
			if _, err := targetText(name); err != ErrInvalidTarget {
				t.Fatalf("invalid target %q: %v", name, err)
			}
			if _, err := identity(name); err != ErrNotAuthor {
				t.Fatalf("invalid caller %q: %v", name, err)
			}
			if _, err := New(nil, nil, Config{Source: name}); err != ErrInvalidTarget {
				t.Fatalf("invalid source %q: %v", name, err)
			}
		})
	}
}
