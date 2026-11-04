package sqlite

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

func openBig(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bus.db")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func row(record uint32, name string, expires time.Time) ports.BigRow {
	return ports.BigRow{Record: record, Name: name, File: "f-" + name, Size: 10, SHA256: strings.Repeat("a", 64), Held: "file", Expires: expires}
}

// The index survives a restart, replaces by name and returns what it replaced.
func TestBigfileRowsSurviveARestartAndReplaceByName(t *testing.T) {
	s, path := openBig(t)
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Nanosecond)
	if _, replaced, err := s.BigRowPut(row(3, "a", exp)); err != nil || replaced {
		t.Fatalf("first put = %v %v", replaced, err)
	}
	changed := row(3, "a", exp)
	changed.File, changed.Size = "f-new", 99
	old, replaced, err := s.BigRowPut(changed)
	if err != nil || !replaced || old.File != "f-a" || old.Size != 10 {
		t.Fatalf("a replace = %+v %v %v", old, replaced, err)
	}
	s.BigRowPut(row(3, "b", exp))
	s.BigRowPut(row(4, "z", exp))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	got, found, err := again.BigRowGet(3, "a")
	if err != nil || !found || got.File != "f-new" || got.Size != 99 || !got.Expires.Equal(exp) || got.Held != "file" {
		t.Fatalf("after a restart: %+v %v %v", got, found, err)
	}
	if rows, _ := again.BigRowsOf(3); len(rows) != 2 || rows[0].Name != "a" || rows[1].Name != "b" {
		t.Fatalf("record 3's rows = %+v", rows)
	}
	if all, _ := again.BigRows(); len(all) != 3 {
		t.Fatalf("all rows = %+v", all)
	}
}

func TestBigfileRowsDeleteDropAndExpire(t *testing.T) {
	s, _ := openBig(t)
	defer s.Close()
	now := time.Now()
	s.BigRowPut(row(1, "live", now.Add(time.Hour)))
	s.BigRowPut(row(1, "short", now.Add(time.Minute)))
	s.BigRowPut(row(2, "other", now.Add(time.Hour)))
	if gone, err := s.BigRowsDropExpired(now.Add(time.Minute)); err != nil || len(gone) != 1 || gone[0].Name != "short" {
		t.Fatalf("expired = %+v %v", gone, err)
	}
	if r, found, _ := s.BigRowDelete(1, "live"); !found || r.File != "f-live" {
		t.Fatalf("delete = %+v %v", r, found)
	}
	if _, found, _ := s.BigRowDelete(1, "live"); found {
		t.Fatal("a second delete found it")
	}
	s.BigRowPut(row(2, "second", now.Add(time.Hour)))
	if dropped, err := s.BigRowsDropRecord(2); err != nil || len(dropped) != 2 {
		t.Fatalf("drop = %+v %v", dropped, err)
	}
	if all, _ := s.BigRows(); len(all) != 0 {
		t.Fatalf("rows left: %+v", all)
	}
}

// Retired database layouts are refused without conversion.
func TestSchemaEightIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range tables {
		if strings.HasPrefix(stmt, "CREATE TABLE bigfiles") || strings.HasPrefix(stmt, "CREATE TABLE event_") || strings.HasPrefix(stmt, "CREATE INDEX event_") {
			continue
		}
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO kv (record_id, name, value) VALUES (3, 'kept', 'value')`,
		`PRAGMA user_version = 8`,
	} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	old.Close()
	if s, err := Open(path, false); err == nil {
		s.Close()
		t.Fatal("retired schema was accepted")
	}
}
