package auth

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A store that can be told to refuse, so both halves of every write are
// reachable. The last set it was handed is kept even on a refusal, because
// what a failed write *tried* to say is the thing worth checking.
type brittle struct {
	*memory.Tokens
	fail    error
	offered []ports.Credential
}

func (b *brittle) Save(creds []ports.Credential) error {
	b.offered = append([]ports.Credential(nil), creds...)
	if b.fail != nil {
		return b.fail
	}
	return b.Tokens.Save(creds)
}

func has(creds []ports.Credential, name string) bool {
	for _, c := range creds {
		if c.Name == name {
			return true
		}
	}
	return false
}

// Forget writes before it takes effect. Dropping the maps first and then
// failing to write left a credential that had stopped working and came back at
// the next restart — a revocation that un-revokes itself, silently.
// See docs/02-access.md#token-lifetime.
func TestForgetKeepsEverythingWhenTheWriteFails(t *testing.T) {
	store := &brittle{Tokens: memory.NewTokens()}
	tok, err := Load(store, "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	doomed, err := tok.Issue("goes@h")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tok.Rotate("goes@h"); err != nil {
		t.Fatal(err)
	}
	current, _ := tok.Issue("goes@h")
	session, err := tok.StartSession("goes@h")
	if err != nil {
		t.Fatal(err)
	}

	store.fail = errors.New("disk is full")
	if err := tok.Forget("goes@h"); err == nil {
		t.Fatal("a write that failed was reported as a removal")
	}
	// Nothing moved: both generations still authenticate and so does the
	// session. A caller told "no" must be able to believe it.
	if who, ok := tok.Principal(current); !ok || who != "goes@h" {
		t.Error("the current token stopped working although the removal failed")
	}
	if who, ok := tok.Principal(doomed); !ok || who != "goes@h" {
		t.Error("the previous token stopped working although the removal failed")
	}
	if who, ok := tok.Principal(session); !ok || who != "goes@h" {
		t.Error("the browser session ended although the removal failed")
	}
	// And the store still has it, so a restart would not resurrect a
	// half-removal either.
	kept, _ := store.Load()
	if !has(kept, "goes@h") {
		t.Error("the store lost the credential although the write failed")
	}

	// The set it offered is the one it meant to write: without the name.
	if has(store.offered, "goes@h") {
		t.Error("the write it attempted still contained the name being removed")
	}
	if !has(store.offered, "owner@h") {
		t.Error("the write it attempted dropped an unrelated credential")
	}
}

// And when the write lands, both generations and every browser session for
// that name go — a session is a credential without being a token, so one that
// outlived its token would be "no registration, no access" not holding, for up
// to IdleLife. See docs/01-identity.md#unregistering.
func TestForgetTakesBothGenerationsAndTheSessions(t *testing.T) {
	store := &brittle{Tokens: memory.NewTokens()}
	tok, err := Load(store, "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	previous, _ := tok.Issue("goes@h")
	current, _ := tok.Rotate("goes@h")
	mine, _ := tok.StartSession("goes@h")
	also, _ := tok.StartSession("goes@h")
	// Somebody else's, as the control: a removal that ended every session
	// would sign the whole node out.
	theirs, _ := tok.Issue("stays@h")
	elsewhere, _ := tok.StartSession("stays@h")

	if err := tok.Forget("goes@h"); err != nil {
		t.Fatal(err)
	}
	for what, cred := range map[string]string{"current token": current, "previous token": previous, "session": mine, "second session": also} {
		if _, ok := tok.Principal(cred); ok {
			t.Errorf("the %s still authenticates after the credential was removed", what)
		}
	}
	if who, ok := tok.Principal(theirs); !ok || who != "stays@h" {
		t.Error("an unrelated credential was removed too")
	}
	if who, ok := tok.Principal(elsewhere); !ok || who != "stays@h" {
		t.Error("an unrelated browser session was ended too")
	}
	if kept, _ := store.Load(); has(kept, "goes@h") || !has(kept, "stays@h") {
		t.Errorf("the store holds %v", kept)
	}
}

// A name with no token can still have a session standing for it: the record
// went, something forgot the credential, and the browser is still signed in.
// Absent is success, and success has to mean nothing answers to the name.
func TestForgetEndsSessionsWithNoTokenToForget(t *testing.T) {
	store := &brittle{Tokens: memory.NewTokens()}
	tok, err := Load(store, "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	// No Issue: a session is started for a name that holds no token at all,
	// so Forget takes the branch where there is nothing to forget. Reaching
	// it by forgetting twice does not work — the first call has already taken
	// the sessions, and the check passes without the branch doing anything.
	session, _ := tok.StartSession("ghost@h")
	if who, ok := tok.Principal(session); !ok || who != "ghost@h" {
		t.Fatal("the session does not stand for the name; the check below proves nothing")
	}
	if _, held := tok.tok["ghost@h"]; held {
		t.Fatal("the name holds a token, so this is not the absent case")
	}
	if err := tok.Forget("ghost@h"); err != nil {
		t.Errorf("forgetting an absent credential is an error: %v", err)
	}
	if _, ok := tok.Principal(session); ok {
		t.Error("a session outlived the credential it came from")
	}
	// An unrelated session is still there, so this is not "ends everything".
	other, _ := tok.StartSession("owner@h")
	if err := tok.Forget("ghost@h"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tok.Principal(other); !ok {
		t.Error("forgetting an absent name ended somebody else's session")
	}
}

// Minting is the same ordering, for the opposite reason: a credential that was
// not written down is one a restart forgets, so it must not be handed out.
// This used to mutate and roll back; the rollback is gone, so the check that
// it leaves nothing behind matters more, not less.
func TestMintHandsOutNothingItCouldNotWrite(t *testing.T) {
	store := &brittle{Tokens: memory.NewTokens()}
	tok, err := Load(store, "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	first, err := tok.Issue("rotor@h")
	if err != nil {
		t.Fatal(err)
	}

	store.fail = errors.New("disk is full")
	got, err := tok.Rotate("rotor@h")
	if err == nil {
		t.Fatal("a rotation that was not written was handed out")
	}
	if got != "" {
		t.Error("a token came back with the error")
	}
	// The one that was there still works, and the one it tried to mint does
	// not — including through the store, which never saw it.
	if who, ok := tok.Principal(first); !ok || who != "rotor@h" {
		t.Error("a failed rotation took the credential that was already working")
	}
	if has(store.offered, "rotor@h") {
		for _, c := range store.offered {
			if c.Name == "rotor@h" && c.Current == first {
				t.Error("the write it attempted was the old credential, so it wrote nothing new")
			}
		}
	}

	// And it recovers: the next rotation, with the store working, succeeds and
	// demotes the right one.
	store.fail = nil
	second, err := tok.Rotate("rotor@h")
	if err != nil {
		t.Fatal(err)
	}
	if who, ok := tok.Principal(second); !ok || who != "rotor@h" {
		t.Error("the rotation after the failure did not work")
	}
	if who, ok := tok.Principal(first); !ok || who != "rotor@h" {
		t.Error("the demoted credential stopped working, stranding queued traffic")
	}
}
