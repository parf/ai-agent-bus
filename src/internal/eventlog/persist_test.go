package eventlog_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"

	_ "modernc.org/sqlite"
)

// The daemon restarts: a new engine on the reopened store.
func restart(t *testing.T, s *sqlite.Store, path string, now *time.Time, old *eventlog.Engine) (*sqlite.Store, *eventlog.Engine) {
	t.Helper()
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err := sqlite.Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	return s, eventlog.New(s, s, func() time.Time { return *now })
}

// Alerts survive a daemon restart within their TTL and size; notices and
// warnings do not; ids never repeat; an expired alert is gone.
func TestAlertsSurviveARestartWithinTTLAndSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := sqlite.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1793318400, 0)
	e := eventlog.New(s, s, func() time.Time { return now })
	p := eventlog.Defaults()
	const rec = 7
	// Sixty distinct alerts, a second apart: the ring keeps its newest fifty.
	for i := 1; i <= 60; i++ {
		if _, err := e.Write(rec, "alert", fmt.Sprintf("alert %d", i), p); err != nil {
			t.Fatalf("alert %d: %v", i, err)
		}
		now = now.Add(time.Second)
	}
	e.Write(rec, "notice", "just a notice", p)
	e.Write(rec, "warning", "just a warning", p)
	s, e = restart(t, s, path, &now, e)
	defer s.Close()
	got, err := e.Read(rec, "x", "", p)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got.Rings["alert"]); n != 50 || got.Rings["alert"][0].Text != "alert 60" || got.Rings["alert"][49].Text != "alert 11" {
		t.Fatalf("after a restart: %d alerts, first %q last %q; want the newest fifty", n, got.Rings["alert"][0].Text, got.Rings["alert"][len(got.Rings["alert"])-1].Text)
	}
	if len(got.Rings["notice"]) != 0 || len(got.Rings["warning"]) != 0 {
		t.Fatalf("volatile levels survived a restart: %+v", got.Rings)
	}
	// A new write gets an id beyond every one issued before the restart.
	w, err := e.Write(rec, "alert", "after the restart", p)
	if err != nil || w.ID <= 60 {
		t.Fatalf("id %d (%v) after a restart, want above 60", w.ID, err)
	}
	// Past the alert TTL (30 days) a restart finds them expired.
	now = now.Add(31 * 24 * time.Hour)
	s, e = restart(t, s, path, &now, e)
	got, err = e.Read(rec, "x", "alert", p)
	if err != nil || len(got.Rings["alert"]) != 0 {
		t.Fatalf("expired alerts after a restart: %+v %v", got.Rings["alert"], err)
	}
	s, e = restart(t, s, path, &now, e)
	defer func() { s.Close() }()
	got, err = e.Read(rec, "x", "alert", p)
	if err != nil || len(got.Rings["alert"]) != 0 {
		t.Fatalf("the expiry was not kept across a second restart: %+v %v", got.Rings["alert"], err)
	}
}

// A log cleaned whole is not found after a restart; one that is active but
// empty (after a pull or an expiry) still is.
func TestCleanedAndActiveEmptyLogsDifferAcrossARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := sqlite.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1793318400, 0)
	e := eventlog.New(s, s, func() time.Time { return now })
	p := eventlog.Defaults()
	e.Write(1, "alert", "pulled away", p)
	e.Write(2, "alert", "cleaned away", p)
	if _, err := e.Pull(1, "a", "", 10, p); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Clean(2, "b", "", 0, p); err != nil {
		t.Fatal(err)
	}
	s, e = restart(t, s, path, &now, e)
	defer s.Close()
	if r, err := e.Read(1, "a", "", p); err != nil || len(r.Rings["alert"]) != 0 {
		t.Fatalf("an active empty log after a restart: %+v %v, want an empty answer", r, err)
	}
	if _, err := e.Read(2, "b", "", p); eventlog.Code(err) != "not_found" {
		t.Fatalf("a cleaned log after a restart: %v, want not_found", err)
	}
}

// One record's damaged alert rows fail that log with a storage error; every
// other log goes on.
func TestADamagedAlertRowFailsThatLogAfterARestartAndNoOther(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := sqlite.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1793318400, 0)
	e := eventlog.New(s, s, func() time.Time { return now })
	p := eventlog.Defaults()
	e.Write(1, "alert", "fine", p)
	e.Write(2, "alert", "to be damaged", p)
	s.Close()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE event_alerts SET count = 0 WHERE record_id = 2`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	s, err = sqlite.Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e = eventlog.New(s, s, func() time.Time { return now })
	if _, err := e.Read(2, "b", "", p); eventlog.Code(err) != "storage_invalid" {
		t.Fatalf("a damaged log: %v, want storage_invalid", err)
	}
	if r, err := e.Read(1, "a", "", p); err != nil || len(r.Rings["alert"]) != 1 {
		t.Fatalf("an undamaged log was affected: %+v %v", r, err)
	}
}
