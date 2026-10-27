package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestKeptStdoutRedactsOwnToken(t *testing.T) {
	var log secretJournal
	r, _, _ := configurationFixtureOutput(t, "itself", `printf 'stderr reason\n' >&2
printf 'own token: %s\n' "$AGENT_BUS_TOKEN"
exit 78
`, &log)
	st := awaitConfigurationState(t, r, "unconfigured")
	if st.Reason != "stderr reason" || st.LastExit == nil || st.LastExit.Reason != "stderr reason" {
		t.Fatal("stdout replaced the stderr diagnostic")
	}
	raw, err := os.ReadFile(r.tokenFile("#fixture@h"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(log.String(), strings.TrimSpace(string(raw))) || !strings.Contains(log.String(), "own token: ***") {
		t.Fatal("keeper stdout leaked its own credential")
	}
}

func TestPackageDigestIgnoresGitMetadata(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "agent.json"), `{"description":"fixture","command":"true"}`)
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")
	before, err := packageDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".git", "FETCH_HEAD"), "new upstream object\n")
	after, err := packageDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("fetch-only git metadata changed package digest")
	}
	writeFile(t, filepath.Join(dir, "agent.json"), `{"description":"changed","command":"true"}`)
	changed, err := packageDigest(dir)
	if err != nil || changed == after {
		t.Fatal("package code change failed to change digest")
	}
}

func TestInventoryKeysCannotEscapeOrNestWorkPaths(t *testing.T) {
	for _, key := range []string{"mail/../../../outside", "../outside", "mail/a/b", "mail/..", "mail/.", "/mail"} {
		t.Run(key, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{key: map[string]any{}})
			if checkAgents(raw) == nil {
				t.Error("write accepted unsafe inventory key")
			}
			home := t.TempDir()
			agents := filepath.Join(home, "agent.d")
			host := filepath.Join(home, "runner")
			writeFile(t, filepath.Join(agents, "mail", "agent.json"), `{"description":"fixture","command":"true"}`)
			writeFile(t, filepath.Join(host, "agents.json"), string(raw))
			if _, err := load(agents, host); err == nil {
				t.Error("load accepted unsafe inventory key")
			}
		})
	}
}

func TestCheckAgentsRejectsTrailingJSONClosers(t *testing.T) {
	for _, raw := range []string{"{} }", "{}]", "{} {}"} {
		if checkAgents([]byte(raw)) == nil {
			t.Errorf("write accepted trailing JSON: %s", raw)
		}
	}
	if err := checkAgents([]byte(`{"mail/work":{"enabled":false}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestReloadRequiresRunnerManagement(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf ready")
	if err := r.control("stop", "fixture"); err != nil {
		t.Fatal(err)
	}
	r.handle(protocol.Envelope{From: "owner@h", Body: "reload"}) // no delivered management role
	answer := configurationReply(t, b)
	if !strings.HasPrefix(answer.Body, "error:") {
		t.Fatal("non-manager reload was accepted")
	}
	if r.status().Agents[0].State != "stopped" {
		t.Fatal("non-manager reload cleared manager stop")
	}
	r.handle(protocol.Envelope{From: "owner@h", Body: "reload", Roles: []string{"maintainer"}})
	if strings.HasPrefix(configurationReply(t, b).Body, "error:") {
		t.Fatal("maintainer reload refused")
	}
}

func TestRunnerHelpHonoursManagementAndAnswersAllAsks(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf ready")
	for _, roles := range [][]string{nil, {"owner"}} {
		r.handle(protocol.Envelope{From: "owner@h", Ask: "help", Roles: roles})
		help := configurationReply(t, b).Body
		for _, method := range []string{"- reload", "- read <file>", "- start <agent>"} {
			if strings.Contains(help, method) != (len(roles) > 0) {
				t.Errorf("help exposed or omitted management command %s", method)
			}
		}
	}
	for _, ask := range []string{"help-config", "help-secret"} {
		r.handle(protocol.Envelope{From: "owner@h", Ask: ask})
		answer := configurationReply(t, b).Body
		if strings.HasPrefix(answer, "error:") {
			t.Errorf("runner refused %s", ask)
		}
	}
	r.handle(protocol.Envelope{From: "owner@h", Ask: "help-json"})
	if body := configurationReply(t, b).Body; body != "unsupported: help-json" {
		t.Errorf("unclear help-json refusal: %s", body)
	}
}

func TestConfigurationReadFailureAnswersInsteadOfDroppingWork(t *testing.T) {
	r, s, replies, dir := deadlineFixture(t, "touch ran\nprintf ran", "1s")
	s.bus.http.Transport = secretTransport(func(q *http.Request) (*http.Response, error) {
		if q.URL.Path == "/config" {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("away")), Header: make(http.Header)}, nil
		}
		return replies.RoundTrip(q)
	})
	r.runOne(s, protocol.Envelope{ID: "work", From: "caller@h"})
	replies.mu.Lock()
	defer replies.mu.Unlock()
	count := 0
	for _, e := range replies.replies {
		if e.Receipt == "" {
			count++
			if e.Body != "error: configuration unavailable" {
				t.Errorf("wrong config failure answer: %s", e.Body)
			}
		}
	}
	if count != 1 {
		t.Fatal("configuration read failure lost the request without exactly one answer")
	}
	if _, err := os.Stat(filepath.Join(dir, "ran")); !os.IsNotExist(err) {
		t.Fatal("script ran with unavailable configuration")
	}
}

func TestConfigurationWarningsAppearOnlyAtLoad(t *testing.T) {
	var log secretJournal
	r, b, _ := configurationFixtureOutput(t, "", "printf ready", &log)
	if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"new_setting":"value"}`)); err != nil {
		t.Fatal(err)
	}
	r.reload()
	for i := 0; i < 2; i++ {
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "probe"}); err != nil {
			t.Fatal(err)
		}
		configurationReply(t, b)
	}
	if n := strings.Count(log.String(), `Configuration key "new_setting" is not described`); n != 1 {
		t.Fatalf("configuration warning repeated per message: %d", n)
	}
}
