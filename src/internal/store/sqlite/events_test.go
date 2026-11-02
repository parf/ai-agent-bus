package sqlite

import (
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEventReservationsSurviveReopenAndNeverDecrease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveEventIDs(7, 64); err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveEventIDs(7, 63); err == nil {
		t.Fatal("reservation decreased")
	}
	s.Close()
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, err := s.EventHighWater(7)
	if err != nil || n != 64 {
		t.Fatalf("lost durable reservation: %d %v", n, err)
	}
	n, err = s.EventHighWater(8)
	if err != nil || n != 0 {
		t.Fatalf("new record inherited ID %d %v", n, err)
	}
	for _, bad := range []string{"-1", "064", "9007199254740992", "garbage"} {
		if _, err = s.db.Exec(`UPDATE meta SET value=? WHERE key='event-high:7'`, bad); err != nil {
			t.Fatal(err)
		}
		if _, err = s.EventHighWater(7); err == nil {
			t.Fatalf("damaged high-water %q accepted", bad)
		}
		snap, er := s.Load()
		if er != nil || len(snap.EventIDIssues) != 1 {
			t.Fatalf("registry not isolated: %+v %v", snap.EventIDIssues, er)
		}
	}
}

func TestLoadIsolatesDamagedEventReservationKeys(t *testing.T) {
	for _, key := range []string{"event-high:0", "event-high:01", "event-high:x"} {
		t.Run(key, func(t *testing.T) {
			s, _ := open(t)
			if _, err := s.db.Exec(`INSERT INTO meta(key,value) VALUES(?, '64')`, key); err != nil {
				t.Fatal(err)
			}
			snap, err := s.Load()
			if err != nil || len(snap.EventIDIssues) != 1 {
				t.Fatalf("missing damaged-key diagnostic: %v %v", snap.EventIDIssues, err)
			}
			if key == "event-high:01" {
				if _, err = s.EventHighWater(1); err == nil {
					t.Fatal("noncanonical counter reset to zero")
				}
			}
			if err = s.ReserveEventIDs(2, 64); err != nil {
				t.Fatal(err)
			}
			if n, err := s.EventHighWater(2); err != nil || n != 64 {
				t.Fatalf("other log blocked %d %v", n, err)
			}
		})
	}
}
func TestAFirstEventReservationAloneSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveEventIDs(7, 64); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.EventHighWater(7); err != nil || n != 64 {
		t.Fatalf("lost first reservation %d %v", n, err)
	}
}

type eventReports struct {
	ports.Silent
	lines []string
}

func (j *eventReports) Report(level ports.Severity, line string) {
	if level == ports.Error {
		j.lines = append(j.lines, line)
	}
}
func TestOneDamagedEventLogDoesNotStopTheDaemonRegistry(t *testing.T) {
	s, _ := open(t)
	b := core.New()
	b.Persistence(s)
	b.SetDaemonOwner("admin@h")
	for _, name := range []string{"#bad@h", "#good@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}}); err != nil {
			t.Fatal(err)
		}
	}
	bad, _ := b.Lookup("admin@h", "#bad@h")
	if _, err := s.db.Exec(`INSERT INTO meta(key,value) VALUES(?, 'damaged')`, "event-high:0"+strconv.FormatUint(uint64(bad.ID), 10)); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Load()
	if err != nil {
		t.Fatalf("event damage stopped registry Load: %v", err)
	}
	restarted := core.New()
	j := &eventReports{}
	restarted.Journal(j)
	restarted.Restore(snap)
	restarted.Persistence(s)
	if err = restarted.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatalf("daemon startup refused: %v", err)
	}
	if _, err = restarted.EventRead("admin@h", "#bad@h", ""); eventlog.Code(err) != "storage_invalid" {
		t.Fatalf("damaged log read %v", err)
	}
	if _, err = restarted.EventWrite("#bad@h", "#bad@h", "alert", "must refuse"); eventlog.Code(err) != "storage_invalid" {
		t.Fatalf("damaged log write %v", err)
	}
	if _, err = restarted.EventWrite("#good@h", "#good@h", "notice", "works"); err != nil {
		t.Fatalf("other log stopped: %v", err)
	}
	found := false
	for _, line := range j.lines {
		if strings.Contains(line, "#bad@h") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no error.log diagnostic naming record: %v", j.lines)
	}
}
