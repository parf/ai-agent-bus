package main

import (
	"context"
	"os"
	"path/filepath"
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
