package core

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Deciding and then minting is not the same as deciding while minting. With
// the registry lock released between the two, an unregistration fits through
// the gap and the credential outlives the name it was issued for. Pinned by
// holding the mint open and showing the removal cannot proceed meanwhile,
// rather than by racing and hoping to land in the window.
func TestIssuingHoldsTheRegistryWhileItMints(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "#svc@h")
	minting, release := make(chan struct{}), make(chan struct{})
	issued := make(chan error, 1)
	go func() {
		_, err := b.IssueFor("#svc@h", "#svc@h", func(string, ports.CredentialPair) (string, error) {
			close(minting)
			<-release
			return "credential", nil
		})
		issued <- err
	}()
	<-minting

	// The removal is known to have been attempted before the wait means
	// anything: a select that times out because a goroutine never ran would
	// pass whatever the lock did.
	removing, removed := make(chan struct{}), make(chan error, 1)
	go func() {
		close(removing)
		removed <- b.Unregister("#svc@h", "#svc@h")
	}()
	<-removing
	select {
	case err := <-removed:
		t.Fatalf("the name was unregistered while its credential was being written: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Correct: the removal is waiting on the registry, as it must.
	}
	close(release)

	if err := <-issued; err != nil {
		t.Fatalf("issuing failed: %v", err)
	}
	if err := <-removed; err != nil {
		t.Fatalf("the removal did not proceed once issuing finished: %v", err)
	}
}

// And the decision itself: nobody is minted for.
func TestIssuingRefusesANameTheDaemonDoesNotKnow(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	called := false
	mint := func(string, ports.CredentialPair) (string, error) { called = true; return "credential", nil }
	if _, err := b.IssueFor("admin@h", "ghost@h", mint); err == nil || called {
		t.Fatalf("minted for a name with nothing behind it: err=%v called=%v", err, called)
	}
	known(t, b, "ghost@h")
	if tok, err := b.IssueFor("ghost@h", "ghost@h", mint); err != nil || tok != "credential" {
		t.Fatalf("a registered name could not be issued one: %q %v", tok, err)
	}
}
