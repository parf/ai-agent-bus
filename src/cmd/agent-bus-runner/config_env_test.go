package main

import (
	"github.com/parf/ai-agent-bus/internal/protocol"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunnerPerMessageGetsCurrentConfiguration(t *testing.T) {
	t.Setenv("AGENT_BUS_CONFIG", "parent-wrong")
	r, b, _ := configurationFixture(t, "", `printf '%s' "${AGENT_BUS_CONFIG-unset}"`)
	for _, tc := range []struct{ raw, want string }{{"", "unset"}, {"{ }", "{}"}, {"{\n\"channel\":\"one\"}", `{"channel":"one"}`}, {`{"channel":"two"}`, `{"channel":"two"}`}} {
		if tc.raw != "" {
			if _, err := b.Configure("#fixture@h", "owner@h", []byte(tc.raw)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "probe"}); err != nil {
			t.Fatal(err)
		}
		got := configurationReply(t, b).Body
		if got != tc.want {
			t.Fatalf("runner script config %q want %q", got, tc.want)
		}
	}
	_ = r
}
func TestKeptConfigurationChangesOnlyOnNextStartAndReload(t *testing.T) {
	r, b, _ := configurationFixture(t, "itself", `printf '%s\n' "${AGENT_BUS_CONFIG-unset}" >> "$AGENT_BUS_WORK/config-starts"; while :; do sleep .02; done`)
	var path string
	// Work roots are resolved by the runner, not assumed by this check.
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	if s == nil {
		t.Fatal("fixture not served")
	}
	path = filepath.Join(s.work, "config-starts")
	read := func(want string) {
		t.Helper()
		end := time.Now().Add(3 * time.Second)
		for {
			raw, _ := os.ReadFile(path)
			if string(raw) == want {
				return
			}
			if time.Now().After(end) {
				t.Fatalf("starts %q want %q", raw, want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	read("unset\n")
	if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"channel":"new"}`)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "unset\n" {
		t.Fatalf("running process changed: %q", raw)
	}
	r.reload()
	read("unset\n{\"channel\":\"new\"}\n")
	if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"channel":"restart"}`)); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	k := r.served["#fixture@h"].kept
	r.mu.Unlock()
	cmd, _, err := k.command()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(cmd.Env, "\n"), `AGENT_BUS_CONFIG={"channel":"restart"}`) {
		t.Fatalf("restart env missing new config: %v", cmd.Env)
	}
}
