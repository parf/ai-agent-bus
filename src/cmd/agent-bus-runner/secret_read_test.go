package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestEnvReadNeverReturnsValues(t *testing.T) {
	home := t.TempDir()
	r := &runner{agents: filepath.Join(home, "agent.d"), host: filepath.Join(home, "runner")}
	writeFile(t, filepath.Join(r.agents, "debug", "agent.json"), `{"description":"test","command":"x"}`)
	value := "TOKEN=rd105-file-sentinel\nOTHER='multiline\nrd105-line-two'\nTOKEN=last\n"
	for _, name := range []string{"runner/debug/env", "runner/debug/work/env"} {
		writeFile(t, filepath.Join(home, name), value)
		got, err := r.readFile(name)
		if err != nil {
			t.Fatal(err)
		}
		wire, _ := json.Marshal(got)
		var body map[string]any
		json.Unmarshal(wire, &body)
		if strings.Contains(string(wire), "rd105-") || body["content"] != nil {
			t.Fatalf("env read exposed content: %s", wire)
		}
		if body["sha256"] != fmt.Sprintf("%x", sha256.Sum256([]byte(value))) || body["size"] != float64(len(value)) || fmt.Sprint(body["names"]) != "[TOKEN OTHER]" {
			t.Fatalf("wrong metadata: %s", wire)
		}
	}
}

func TestBundledDebugMasksPrivateEnvironment(t *testing.T) {
	cmd := exec.Command("python3", "../../agents/debug/debug.py", "env")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=rd105-home-sentinel", "FILE_SECRET=rd105-file-sentinel", "BUS_SECRET=rd105-bus-sentinel", "AGENT_BUS_FROM=caller@h", "AB_ROLE_OWNER=1"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "rd105-") {
		t.Fatal("debug env exposed a private value")
	}
	for _, want := range []string{"FILE_SECRET=***", "BUS_SECRET=***", "HOME=***", "AGENT_BUS_FROM=caller@h", "AB_ROLE_OWNER=1"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("debug missing %s", want)
		}
	}
}

func TestChildStderrRedactsInjectedValues(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		var journal strings.Builder
		stderr := secretStderr(&journal, []string{"FILE=rd105-file-sentinel", "BUS=rd105-bus-sentinel", "SHORT=abc"})
		if chunked {
			for _, b := range []byte("rd105-file-sentinel\nlast rd105-bus-sentinel") {
				stderr.Write([]byte{b})
			}
		} else {
			cmd := exec.Command("sh", "-c", `printf '%s\nlast %s' "$FILE" "$BUS" >&2; exit 78`)
			cmd.Env = []string{"FILE=rd105-file-sentinel", "BUS=rd105-bus-sentinel"}
			cmd.Stderr = stderr
			if err := cmd.Run(); err == nil {
				t.Fatal("expected configuration exit")
			}
		}
		stderr.flush()
		if journal.String() != "***\nlast ***" || stderr.reason() != "last ***" {
			t.Fatalf("private stderr reached journal or status (chunked=%v)", chunked)
		}
	}
}

// The journal sink is read while the keeper may still finish retirement logs.
type secretJournal struct {
	mu   sync.Mutex
	text strings.Builder
}

func (j *secretJournal) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.text.Write(p)
}
func (j *secretJournal) String() string { j.mu.Lock(); defer j.mu.Unlock(); return j.text.String() }

func TestKeptTokenNeverReachesExitStatusOrJournal(t *testing.T) {
	var journal secretJournal
	r, _, _ := configurationFixtureOutput(t, "itself", `printf 'missing credential %s\n' "$AGENT_BUS_TOKEN" >&2
exit 78
`, &journal)
	st := awaitConfigurationState(t, r, "unconfigured")
	token, err := os.ReadFile(r.tokenFile("#fixture@h"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(st)
	if strings.Contains(string(raw), strings.TrimSpace(string(token))) || strings.Contains(journal.String(), strings.TrimSpace(string(token))) {
		t.Fatal("keeper token escaped into status or journal")
	}
	if st.Reason != "missing credential ***" || st.LastExit == nil || st.LastExit.Reason != "missing credential ***" || !strings.Contains(journal.String(), "missing credential ***") {
		t.Fatal("masked exit78 reason missing from status or journal")
	}
}

func TestStderrUsesSecretSourcesNotConfigurationOrDefaults(t *testing.T) {
	r, b, _ := configurationFixture(t, "", `printf '%s|%s|%s|%s|%s\n' "$MODE" "$AGENT_BUS_CONFIG" "$BUS_SECRET" "$HOST_SECRET" "$INSTANCE_SECRET" >&2
exit 78
`)
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	writeFile(t, filepath.Join(r.agents, "fixture", "env.dist"), "MODE=prod\n")
	writeFile(t, filepath.Join(r.host, "fixture", "env"), "HOST_SECRET=private-host-value\n")
	writeFile(t, filepath.Join(r.host, "fixture", "one", "env"), "INSTANCE_SECRET=private-instance-value\n")
	if _, err := b.SetSecret("#fixture@h", "owner@h", "BUS_SECRET=private-bus-value\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"mode":"prod"}`)); err != nil {
		t.Fatal(err)
	}
	in := s.in
	in.Key = "fixture/one"
	in.Host.Name = "#fixture@h"
	env, private, why := r.environmentWithSecrets(in, s.bus)
	if why != "" {
		t.Fatal(why)
	}
	var journal strings.Builder
	capture := secretStderr(&journal, private)
	capture.Write([]byte("prod|" + `{"mode":"prod"}` + "|private-bus-value|private-host-value|private-instance-value"))
	capture.flush()
	if journal.String() != `prod|{"mode":"prod"}|***|***|***` {
		t.Fatal("instance, host and Secret provenance lost")
	}
	if !strings.Contains(strings.Join(env, "\n"), "INSTANCE_SECRET=private-instance-value") {
		t.Fatal("instance fixture was not injected")
	}
	r.reload()
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "probe"}); err != nil {
		t.Fatal(err)
	}
	answer := configurationReply(t, b)
	if answer.Body != `error: #fixture@h is not configured: prod|{"mode":"prod"}|***|***|` || strings.Contains(answer.Body, "private-host-value") || strings.Contains(answer.Body, "private-bus-value") {
		t.Fatalf("wrong stderr provenance: %s", answer.Body)
	}
}

func TestShortPrivateValuesWarnByKeyWithoutMutilatingDiagnostics(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf configured")
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	writeFile(t, filepath.Join(r.host, "fixture", "env"), "SHORT=on\nEMPTY=\n")
	if _, err := b.SetSecret("#fixture@h", "owner@h", "PIN=1\nLONG=private-long-value\n"); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	r.say = &log // this per-message fixture is idle; no child is writing.
	_, private, why := r.environmentWithSecrets(s.in, s.bus)
	if why != "" {
		t.Fatal(why)
	}
	for _, key := range []string{"SHORT", "PIN"} {
		if strings.Count(log.String(), "secret "+key+" shorter than 4 bytes cannot be redacted reliably") != 1 {
			t.Fatal("missing or duplicated short-secret warning")
		}
	}
	if strings.Contains(log.String(), "secret EMPTY") || strings.Contains(log.String(), "private-long-value") {
		t.Fatal("warning exposed value or warned about empty key")
	}
	var journal strings.Builder
	capture := secretStderr(&journal, private)
	capture.Write([]byte("configuration done: 123 private-long-value"))
	capture.flush()
	if journal.String() != "configuration done: 123 ***" {
		t.Fatal("short secret mutilated ordinary diagnostics")
	}
}

// A value of several lines is hidden by each of its lines, since a script may
// log them one at a time, behind a prefix or escaped; a value holding a
// shorter secret is hidden whole. A real child writes them, and the reason a
// status keeps is the masked last line.
func TestChildStderrMasksEachLineOfAMultiLineValueAndTheLongestOfNestedOnes(t *testing.T) {
	private := []string{"SHORT=abcd", "LONG=abcdEFGHsecret", "KEY=-----BEGIN KEY-----\nMIIEsecretbody\n-----END KEY-----"}
	var journal strings.Builder
	stderr := secretStderr(&journal, private)
	script := `printf 'k1: -----BEGIN KEY-----\nk2: MIIEsecretbody\n' >&2
printf 'escaped %s\n' "$(printf '%s' "$KEY" | sed ':a;N;$!ba;s/\n/\\n/g')" >&2
printf 'long %s\n' "$LONG" >&2
printf 'last %s' "$LONG" >&2; exit 78`
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = []string{"KEY=" + private[2][4:], "LONG=abcdEFGHsecret"}
	cmd.Stderr = stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("expected configuration exit")
	}
	stderr.flush()
	for _, leak := range []string{"MIIEsecretbody", "BEGIN KEY", "EFGHsecret"} {
		if strings.Contains(journal.String(), leak) || strings.Contains(stderr.reason(), leak) {
			t.Fatalf("%q reached the journal or the status reason: %q", leak, journal.String())
		}
	}
	if !strings.Contains(journal.String(), "long ***\n") || stderr.reason() != "last ***" {
		t.Fatalf("the longer value was not hidden whole: %q reason %q", journal.String(), stderr.reason())
	}
}
