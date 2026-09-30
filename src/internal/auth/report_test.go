package auth

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type heard struct {
	mu    sync.Mutex
	lines []string
}

func (h *heard) Report(sev ports.Severity, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lines = append(h.lines, fmt.Sprint(sev)+" "+msg)
}

// A credential write the store refuses reaches the error log and syslog, not
// only the caller (K.33).
func TestAStoreThatRefusesACredentialWriteIsReported(t *testing.T) {
	store := memory.NewTokens()
	tokens, err := Load(store, "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	h := &heard{}
	tokens.Journal(h)
	store.Err = errors.New("disk full")
	if _, err := tokens.Issue("alice@h"); err == nil {
		t.Fatal("an issue the store refused succeeded")
	}
	if err := tokens.Forget("owner@h"); err == nil {
		t.Fatal("a removal the store refused succeeded")
	}
	got := strings.Join(h.lines, "\n")
	for _, want := range []string{"the token store refused the issue of alice@h's credential: disk full", "the token store refused the removal of owner@h's credential"} {
		if !strings.Contains(got, want) {
			t.Errorf("not reported: %q in\n%s", want, got)
		}
	}
	// No report carries a credential: a token is 48 hex digits.
	if regexp.MustCompile(`[0-9a-f]{48}`).MatchString(got) {
		t.Errorf("a report carries a credential:\n%s", got)
	}
}
