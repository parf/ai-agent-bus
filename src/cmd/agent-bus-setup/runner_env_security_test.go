package main

import (
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

type tokenReplyTransport struct{}

func (tokenReplyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"token":"abcdef"}`))}, nil
}

func TestRunnerTokenWriteDoesNotFollowAPlantedTemporarySymlink(t *testing.T) {
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	p := personRunner{private: dir, account: account.Username, name: "#runner/test@h"}
	if err := os.Symlink(victim, p.envFile()+".new"); err != nil {
		t.Fatal(err)
	}
	c := &ownerClient{http: http.Client{Transport: tokenReplyTransport{}}}
	if err := (runnerOps{bus: c}).writeEnv(p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(victim)
	if err != nil || string(b) != "untouched" {
		t.Fatal("token write followed the planted symlink")
	}
	b, err = os.ReadFile(p.envFile())
	if err != nil || string(b) != "AGENT_BUS_TOKEN=abcdef\n" {
		t.Fatal("the real token file was not written")
	}
	st, err := os.Lstat(p.envFile())
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 {
		t.Fatal("token file is not a private regular file")
	}
}
