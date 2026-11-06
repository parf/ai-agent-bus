package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F4: the line a stop writes to the agent's log names the cause of that stop. It used to say "on request"
// for every one, including an agent that exited as not configured and one whose reload was refused.
func recordEvents(t *testing.T, r *runner) *recorded {
	t.Helper()
	rec := &recorded{}
	r.events = newEventSink(r)
	r.events.send = func(_ context.Context, e agentEvent) error { rec.add(e); return nil }
	r.events.start()
	t.Cleanup(func() { closeSink(t, r.events) })
	return rec
}

// noSecretBus is the retirement fixture's bus where the agent has no secret record.
type noSecretBus struct{ retirementBus }

func (b noSecretBus) RoundTrip(q *http.Request) (*http.Response, error) {
	if q.URL.Path == "/secret" {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	}
	return b.retirementBus.RoundTrip(q)
}

func stopNotices(rec *recorded) []string {
	var out []string
	for _, e := range rec.list() {
		if e.level == levelNotice {
			out = append(out, e.text)
		}
	}
	return out
}

func wantStopNotice(t *testing.T, rec *recorded, want string) {
	t.Helper()
	awaitCount(t, rec, levelNotice, 1)
	if got := stopNotices(rec); len(got) != 1 || got[0] != want {
		t.Fatalf("stop notices %q, want exactly %q", got, want)
	}
}

func TestAnAgentThatExitedNotConfiguredIsStoppedForThatCause(t *testing.T) {
	r, s, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	r.installed = []installed{s.in}
	r.notConfigured(s, nil, "missing token")
	wantStopNotice(t, rec, "runner stopped serving this agent: it exited as not configured")
}

func TestAReloadThatDisablesAnAgentSaysSo(t *testing.T) {
	r, s, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	no := false
	s.in.Host.Enabled = &no
	r.installed = []installed{s.in}
	awaitRetirement(t, serveReload(r))
	wantStopNotice(t, rec, "runner stopped serving this agent: it is disabled")
}

func TestAStopCallIsTheOneStopThatSaysOnRequest(t *testing.T) {
	r, s, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	s.in.Stopped = true
	r.installed = []installed{s.in}
	awaitRetirement(t, serveReload(r))
	wantStopNotice(t, rec, "runner stopped serving this agent: it was stopped on request")
}

func TestAReloadThatRefusesAnAgentSaysSo(t *testing.T) {
	r, s, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	other := s.in
	other.Key, other.Dir = "other", "other"
	r.installed = []installed{s.in, other} // two entries that come to one bus name are both refused
	awaitRetirement(t, serveReload(r))
	wantStopNotice(t, rec, "runner stopped serving this agent: it was refused at reload")
}

// A served agent whose jail the reload cannot make (here its work directory is over its disk limit)
// is refused, not "no longer installed": the entry is still there and still enabled.
func TestAReloadThatCannotMakeTheJailSaysItRefusedTheAgent(t *testing.T) {
	r, s, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	r.probe.Do(func() {}) // the sandbox is "on" and available, so the jail is made
	r.bus = &client{http: &http.Client{Transport: noSecretBus{retirementBus{name: safeRetiringName, called: make(chan struct{}, 32)}}}, base: "http://fixture"}
	s.bus = r.bus
	s.in.Agent.Serves = "runner"
	s.in.Host.Sandbox = "on"
	s.in.Host.Limits = &limits{Disk: "1K"}
	r.installed = []installed{s.in}
	work := filepath.Join(r.host, "work", s.in.Key)
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "big"), make([]byte, 8192), 0o600); err != nil {
		t.Fatal(err)
	}
	awaitRetirement(t, serveReload(r))
	r.mu.Lock()
	refused := r.installed[0].Refused
	r.mu.Unlock()
	if !strings.Contains(refused, "over its disk limit") {
		t.Fatalf("the refusal came from elsewhere than the jail: %q", refused)
	}
	wantStopNotice(t, rec, "runner stopped serving this agent: it was refused at reload")
}

// A nested or parent key installed in its place overlaps the served agent's; the key itself is not
// another agent's name.
func TestAReloadThatInstallsAnOverlappingKeySaysItOverlaps(t *testing.T) {
	r, s, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	nested := s.in
	nested.Key, nested.Dir = s.in.Key+"/main", s.in.Key+"/main"
	nested.Host.Name = "#other@h"
	r.installed = []installed{nested}
	done := serveReload(r)
	release() // the stopped agent's keeper drains, so the reload can go on to the replacement
	awaitRetirement(t, done)
	awaitCount(t, rec, levelNotice, 1)
	if got := stopNotices(rec); len(got) == 0 || got[0] != "runner stopped serving this agent: its key overlaps another agent's" {
		t.Fatalf("stop notices %q", got)
	}
}

func TestAReloadThatNoLongerFindsAnAgentSaysSo(t *testing.T) {
	r, _, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	r.installed = nil
	awaitRetirement(t, serveReload(r))
	wantStopNotice(t, rec, "runner stopped serving this agent: it is no longer installed")
}

func TestAnAgentOverItsDiskLimitIsStoppedForThatCause(t *testing.T) {
	r, s, _, release, _ := retiringFixture(t, safeRetiringName)
	defer release()
	rec := recordEvents(t, r)
	r.installed = []installed{s.in}
	if err := os.WriteFile(filepath.Join(s.work, "big"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	r.checkDisk(s, &jail{disk: 1024})
	wantStopNotice(t, rec, "runner stopped serving this agent: its work directory is over its disk limit")
}
