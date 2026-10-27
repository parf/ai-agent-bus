package auth

import (
	"errors"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"testing"
	"time"
)

func TestImportedCredentialsPublishOnlyAfterCommitAndSerializeRotation(t *testing.T) {
	tokens, e := Load(memory.NewTokens(), "owner@h")
	if e != nil {
		t.Fatal(e)
	}
	old, e := tokens.Issue("#fixture@h")
	if e != nil {
		t.Fatal(e)
	}
	c := ports.Credential{Name: "#fixture@h", Current: "fixture-imported-private-token", CredentialPair: ports.CredentialPair{UserID: 2, AgentID: 3}}
	if e = tokens.CommitImport(map[string]ports.Credential{c.Name: c}, nil, func() error { return errors.New("failed") }); e == nil {
		t.Fatal("commit refusal swallowed")
	}
	if _, _, ok := tokens.Credential(c.Current); ok {
		t.Fatal("uncommitted token published")
	}
	if _, _, ok := tokens.Credential(old); !ok {
		t.Fatal("failed import revoked existing token")
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- tokens.CommitImport(map[string]ports.Credential{c.Name: c}, nil, func() error { close(entered); <-release; return nil })
	}()
	<-entered
	rotation := make(chan error, 1)
	go func() { _, e := tokens.Rotate(c.Name); rotation <- e }()
	select {
	case <-rotation:
		t.Fatal("rotation passed an in-progress import commit")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if e = <-rotation; e != nil {
		t.Fatal(e)
	}
	// A normal rotation keeps imported current as previous; it cannot resurrect
	// the token that import replaced.
	if _, _, ok := tokens.Credential(c.Current); !ok {
		t.Fatal("rotation lost imported credential")
	}
	if _, _, ok := tokens.Credential(old); ok {
		t.Fatal("rotation resurrected replaced credential")
	}
}
