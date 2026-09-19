package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	dirfile "github.com/parf/ai-agent-bus/internal/directory/file"
	"github.com/parf/ai-agent-bus/internal/keyproof"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/signature/sshkeygen"
)

// G.1.1 is an application boundary, not an OS ban on exec: the daemon may
// launch its fixed verifier but must keep user service data inert.
func TestUserServiceInputsRemainData(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	live := httptest.NewServer(s.Handler())
	defer live.Close()
	call := func(method, path, who string, in, out any) {
		t.Helper()
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		r, err := http.NewRequest(method, live.URL+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set(HeaderToken, token(who))
		response, err := live.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatalf("%s: %d", path, response.StatusCode)
		}
		if out != nil {
			if err := json.NewDecoder(response.Body).Decode(out); err != nil {
				t.Fatal(err)
			}
		}
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	program := filepath.Join(dir, "user-program")
	// Quote paths even on a checkout whose test-temp parent has spaces.
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nprintf 'executed' > " + quote(marker) + "\n"
	if err := os.WriteFile(program, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	// The positive control rules out a harmless-looking probe that cannot run
	// or cannot create its marker. Only the test harness executes it.
	if out, err := exec.Command(program).CombinedOutput(); err != nil {
		t.Fatalf("probe cannot run: %v: %s", err, out)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "executed" {
		t.Fatalf("probe produced %q: %v", data, err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	inert := func(phase string) {
		t.Helper()
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("%s executed user-supplied content (marker stat: %v)", phase, err)
		}
	}
	var rec protocol.Record
	call("POST", "/register", "owner@h", protocol.Record{Kind: protocol.KindAgent, Name: "probe@h", Descr: program, Addr: program, Proto: "exec"}, &rec)
	inert("registration")
	if rec.Descr != program || rec.Addr != program || rec.Proto != "exec" {
		t.Fatalf("registration discarded the probe: %+v", rec)
	}
	config := map[string]string{"command": program, "script": script}
	call("POST", "/configure", "owner@h", map[string]any{"name": "probe@h", "config": config}, nil)
	inert("configuration")
	var stored map[string]string
	call("GET", "/config?name=probe@h", "probe@h", nil, &stored)
	if stored["command"] != program || stored["script"] != script {
		t.Fatalf("configuration did not retain the probe: %v", stored)
	}
	for _, body := range []string{program, script, "$(" + quote(program) + ")"} {
		var sent, got protocol.Envelope
		call("POST", "/send", "owner@h", protocol.Envelope{To: "probe@h", Body: body}, &sent)
		inert("send")
		call("GET", "/consume?wait=0s", "probe@h", nil, &got)
		inert("consume")
		if got.ID != sent.ID || got.Body != body {
			t.Fatalf("message was not preserved: %+v", got)
		}
	}
}

// A real SSH key and the shipped verifier keep the allowed subprocess path
// working. A wrong key must fail before the matching key may enrol.
func TestSignedEnrolmentUsesTheShippedVerifier(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "key")
	wrong := filepath.Join(dir, "wrong")
	for _, path := range []string{key, wrong} {
		if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", path).CombinedOutput(); err != nil {
			t.Fatalf("generate key: %v: %s", err, out)
		}
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(dir, "directory")
	if err := os.WriteFile(directory, []byte("newcomer "+string(pub)), 0600); err != nil {
		t.Fatal(err)
	}
	b := core.New()
	b.Directories(map[string]ports.Directory{"vouched": dirfile.New(directory)}, sshkeygen.New())
	s, _ := serverFor(t, b, "owner@h")
	live := httptest.NewServer(s.Handler())
	defer live.Close()
	post := func(in any, code int, out any) {
		t.Helper()
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		response, err := live.Client().Post(live.URL+"/enrol", "application/json", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != code {
			t.Fatalf("enrol: %d, want %d", response.StatusCode, code)
		}
		if out != nil {
			if err := json.NewDecoder(response.Body).Decode(out); err != nil {
				t.Fatal(err)
			}
		}
	}
	var challenge struct{ Nonce, Namespace string }
	post(map[string]string{"name": "newcomer@vouched"}, 200, &challenge)
	if challenge.Nonce == "" || challenge.Namespace != protocol.SigNamespace {
		t.Fatalf("bad challenge: %+v", challenge)
	}
	sign := func(path string) string {
		t.Helper()
		sig, err := keyproof.Sign(path, challenge.Namespace, challenge.Nonce)
		if err != nil {
			t.Fatal(err)
		}
		return sig
	}
	post(map[string]string{"nonce": challenge.Nonce, "signature": sign(wrong)}, 403, nil)
	if _, ok := b.Lookup("owner@h", "newcomer@vouched"); ok {
		t.Fatal("wrong key created a record")
	}
	var enrolled struct {
		protocol.Record
		Token string
	}
	post(map[string]string{"nonce": challenge.Nonce, "signature": sign(key)}, 200, &enrolled)
	if enrolled.Name != "newcomer@vouched" || enrolled.Owner != enrolled.Name || enrolled.Token == "" {
		t.Fatalf("bad enrolment: %+v", enrolled.Record)
	}
	r, err := http.NewRequest("GET", live.URL+"/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set(HeaderToken, enrolled.Token)
	response, err := live.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var status struct{ You string }
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || status.You != enrolled.Name {
		t.Fatalf("enrolled token answered %d as %s", response.StatusCode, status.You)
	}
}
