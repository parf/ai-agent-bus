package auth

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A store whose Touch can be held open, so a flush can be caught mid-write.
type slowTouch struct {
	*memory.Tokens
	entered chan struct{}
	release chan struct{}
	fail    error
}

func (s *slowTouch) Touch(used map[string]time.Time) error {
	if s.entered != nil {
		s.entered <- struct{}{}
		<-s.release
	}
	if s.fail != nil {
		return s.fail
	}
	return s.Tokens.Touch(used)
}

func storedUsed(t *testing.T, st ports.TokenStore, name string) time.Time {
	t.Helper()
	rows, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rows {
		if c.Name == name {
			return c.Used
		}
	}
	return time.Time{}
}

// B6: the last-use batch is written outside the credential lock, so a slow
// disk never holds up an authenticated call or a credential change.
func TestASlowLastUseFlushBlocksNoCredentialCall(t *testing.T) {
	st := &slowTouch{Tokens: memory.NewTokens()}
	tokens, err := testTokens(st, "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	alice, err := tokens.Issue("alice@h")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := tokens.Credential(alice); !ok {
		t.Fatal("alice's token is not accepted")
	}
	st.entered, st.release = make(chan struct{}), make(chan struct{})
	var once sync.Once
	open := func() { once.Do(func() { close(st.release) }) }
	defer open()
	flushed := make(chan error, 1)
	go func() { flushed <- tokens.FlushUsed() }()
	select {
	case <-st.entered:
	case <-time.After(time.Second):
		t.Fatal("the flush never reached the store")
	}
	calls := map[string]func(){
		"Credential": func() { tokens.Credential(alice) },
		"Issue":      func() { tokens.Issue("bob@h") },
		"Rotate":     func() { tokens.Rotate("owner@h") },
		"Forget":     func() { tokens.Forget("bob@h") },
	}
	for _, name := range []string{"Credential", "Issue", "Rotate", "Forget"} {
		done := make(chan struct{})
		go func() { calls[name](); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			open()
			t.Fatalf("%s waited on the last-use write", name)
		}
	}
	open()
	if err := <-flushed; err != nil {
		t.Fatal(err)
	}
	if storedUsed(t, st, "alice@h").IsZero() {
		t.Fatal("the flush did not store alice's last use")
	}
	if storedUsed(t, st, "bob@h") != (time.Time{}) {
		t.Fatal("a credential forgotten during the flush came back")
	}
	// The Credential call made during the flush is a newer use; the next flush
	// writes it, and with nothing moved after that, the one after writes nothing.
	st.entered = nil
	if err := tokens.FlushUsed(); err != nil {
		t.Fatal(err)
	}
	var wrote map[string]time.Time
	probe := &recordTouch{slowTouch: st, wrote: &wrote}
	tokens.store = probe
	if err := tokens.FlushUsed(); err != nil || len(wrote) != 0 {
		t.Fatalf("an unmoved time was written again: %v %v", wrote, err)
	}
}

type recordTouch struct {
	*slowTouch
	wrote *map[string]time.Time
}

func (r *recordTouch) Touch(used map[string]time.Time) error {
	*r.wrote = used
	return r.slowTouch.Touch(used)
}

// A refused write advances nothing, so the next flush writes the same time.
func TestARefusedLastUseFlushIsRetried(t *testing.T) {
	st := &slowTouch{Tokens: memory.NewTokens()}
	tokens, err := testTokens(st, "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	alice, _ := tokens.Issue("alice@h")
	tokens.Credential(alice)
	st.fail = errors.New("disk full")
	if err := tokens.FlushUsed(); err == nil {
		t.Fatal("a refused write reported success")
	}
	st.fail = nil
	if err := tokens.FlushUsed(); err != nil {
		t.Fatal(err)
	}
	if storedUsed(t, st, "alice@h").IsZero() {
		t.Fatal("the time a refused flush held back was never written")
	}
}
