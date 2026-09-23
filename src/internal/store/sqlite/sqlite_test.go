package sqlite

import (
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bus.db")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

// A database that is not there is not made unless asked for: a daemon that
// silently started an empty node would look healthy with everything gone.
func TestMissingDatabaseIsNotCreatedUnasked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	if _, err := Open(path, false); !errors.Is(err, ErrMissing) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused open left a database behind")
	}
}

func TestDatabaseIsPrivate(t *testing.T) {
	_, path := open(t)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
}

func TestCommitThenLoad(t *testing.T) {
	s, path := open(t)
	owner := "admin@h"
	user := protocol.User{ID: 7, Name: "admin@h", Status: "active", Email: "a@example.com"}
	rec := protocol.Record{ID: 9, Name: "svc@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"@ops"}}
	group := protocol.Record{ID: 10, Name: "@ops", Kind: protocol.KindGroup, Owner: "admin@h", Allow: []string{"admin@h"}}
	nextRec, nextUser := uint32(11), uint32(8)
	if err := s.Commit(ports.Change{
		NextRecordID: &nextRec, NextUserID: &nextUser,
		Owner:    &owner,
		Accounts: map[string]string{"parf": "admin@h"},
		Users:    map[string]*protocol.User{user.Name: &user},
		Records:  map[string]*protocol.Record{rec.Name: &rec, group.Name: &group},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveQueues([]ports.Queue{{Name: "svc@h", In: 3, Out: 1, Dropped: 1, Messages: []protocol.Envelope{{ID: "m1", Body: "one"}, {ID: "m2", Body: "two"}}}}, nil, true); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s2, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	snap, err := s2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !snap.OwnerEstablished || snap.Owner != owner || !snap.AccountsEstablished || len(snap.Accounts) != 1 || !snap.Clean {
		t.Fatalf("meta %+v", snap)
	}
	if len(snap.Users) != 1 || snap.Users[0].Email != "a@example.com" || snap.Users[0].ID != 7 {
		t.Fatalf("users %+v", snap.Users)
	}
	// A Group is a record: its membership is its allow list, stored as one.
	byName := map[string]protocol.Record{}
	for _, r := range snap.Records {
		byName[r.Name] = r
	}
	if r := byName["svc@h"]; len(snap.Records) != 2 || r.Allow[0] != "@ops" || r.ID != 9 {
		t.Fatalf("records %+v", snap.Records)
	}
	if g := byName["@ops"]; g.Kind != protocol.KindGroup || len(g.Allow) != 1 || g.Allow[0] != "admin@h" || g.ID != 10 {
		t.Fatalf("group %+v", g)
	}
	if snap.NextRecordID != 11 || snap.NextUserID != 8 {
		t.Fatalf("high-water marks %d %d", snap.NextRecordID, snap.NextUserID)
	}
	if len(snap.Queues) != 1 || snap.Queues[0].In != 3 || len(snap.Queues[0].Messages) != 2 || snap.Queues[0].Messages[1].ID != "m2" {
		t.Fatalf("queues %+v", snap.Queues)
	}
}

// Removing a record in one change removes its queue in the same change.
func TestDropQueueGoesWithTheRecord(t *testing.T) {
	s, _ := open(t)
	rec := protocol.Record{ID: 1, Name: "svc@h", Kind: protocol.KindAgent, Owner: "admin@h"}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{rec.Name: &rec}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveQueues([]ports.Queue{{Name: "svc@h", In: 1, Messages: []protocol.Envelope{{ID: "m"}}}}, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{rec.Name: nil}, DropQueues: []string{"svc@h"}}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Records) != 0 || len(snap.Queues) != 0 {
		t.Fatalf("left %+v", snap)
	}
}

// A commit that fails part-way leaves nothing of itself behind.
func TestFailedCommitWritesNothing(t *testing.T) {
	s, _ := open(t)
	good := protocol.Record{ID: 1, Name: "a@h", Kind: protocol.KindAgent, Owner: "admin@h"}
	// The queue drop runs after the record statement, so the refusal fails
	// the transaction with the record already written inside it.
	if _, err := s.db.Exec(`CREATE TRIGGER refuse BEFORE DELETE ON messages BEGIN SELECT RAISE(ABORT, 'refused'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO queues (name, in_count, out_count, dropped, expired) VALUES ('q@h', 1, 0, 0, 0); INSERT INTO messages (queue, seq, body) VALUES ('q@h', 0, '{}')`); err != nil {
		t.Fatal(err)
	}
	err := s.Commit(ports.Change{
		Records:    map[string]*protocol.Record{good.Name: &good},
		DropQueues: []string{"q@h"},
	})
	if err == nil {
		t.Fatal("the commit should have failed")
	}
	snap, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Records) != 0 {
		t.Fatal("a failed commit kept part of itself")
	}
}

func TestCredentials(t *testing.T) {
	s, _ := open(t)
	tok := s.Tokens()
	pair := ports.CredentialPair{UserID: 3, AgentID: 7}
	if err := tok.Put(ports.Credential{Name: "#a@h", Current: "c", Previous: "p", CredentialPair: pair}); err != nil {
		t.Fatal(err)
	}
	if err := tok.Put(ports.Credential{Name: "b@h", Current: "d", CredentialPair: ports.CredentialPair{UserID: 4}}); err != nil {
		t.Fatal(err)
	}
	used := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	if err := tok.Touch(map[string]time.Time{"#a@h": used}); err != nil {
		t.Fatal(err)
	}
	got, err := tok.Load()
	if err != nil || len(got) != 2 || got[0].Name != "#a@h" || got[0].Previous != "p" || got[0].CredentialPair != pair || !got[0].Used.Equal(used) {
		t.Fatalf("%+v %v", got, err)
	}
	// One row at a time: dropping one leaves the other.
	if err := tok.Drop("#a@h"); err != nil {
		t.Fatal(err)
	}
	if got, _ := tok.Load(); len(got) != 1 || got[0].Name != "b@h" {
		t.Fatalf("after a drop: %+v", got)
	}
}

// A transfer rebinds, and a removal deletes, credentials in the change's own
// transaction; a change that fails writes neither.
func TestCommitCarriesCredentials(t *testing.T) {
	s, _ := open(t)
	tok := s.Tokens()
	for _, n := range []string{"#moved@h", "#gone@h"} {
		if err := tok.Put(ports.Credential{Name: n, Current: n, CredentialPair: ports.CredentialPair{UserID: 1, AgentID: 2}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Commit(ports.Change{Credentials: map[string]*ports.CredentialPair{
		"#moved@h": {UserID: 9, AgentID: 2}, "#gone@h": nil,
	}}); err != nil {
		t.Fatal(err)
	}
	got, _ := tok.Load()
	if len(got) != 1 || got[0].Name != "#moved@h" || got[0].UserID != 9 {
		t.Fatalf("after the commit: %+v", got)
	}
	// The queue drop runs after the credential statement, so a refusal there
	// fails the transaction with the rebind already applied inside it.
	if _, err := s.db.Exec(`CREATE TRIGGER refuse BEFORE DELETE ON messages BEGIN SELECT RAISE(ABORT, 'refused'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO messages (queue, seq, body) VALUES ('q@h', 0, '{}')`); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ports.Change{
		Credentials: map[string]*ports.CredentialPair{"#moved@h": {UserID: 5, AgentID: 2}},
		DropQueues:  []string{"q@h"},
	}); err == nil {
		t.Fatal("the commit should have failed")
	}
	if got, _ := tok.Load(); got[0].UserID != 9 {
		t.Fatalf("a failed commit rebound a credential: %+v", got)
	}
}

func TestIncompatibleSchemaIsRefused(t *testing.T) {
	s, path := open(t)
	if _, err := s.db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := Open(path, false); err == nil || !strings.Contains(err.Error(), "schema 99") {
		t.Fatalf("got %v", err)
	}
}

func TestDamagedDatabaseIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	if err := os.WriteFile(path, []byte("this is not a database, it is a file somebody wrote"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, false); err == nil {
		t.Fatal("a damaged database opened")
	}
}

// A second daemon on the same database cannot serve. Checked across
// processes, because the lock is the operating system's.
func TestSecondOpenerIsRefused(t *testing.T) {
	if os.Getenv("SQLITE_HOLDER") != "" {
		s, err := Open(os.Getenv("SQLITE_HOLDER"), false)
		if err != nil {
			os.Exit(3)
		}
		os.Stdout.WriteString("held\n")
		var b [1]byte
		os.Stdin.Read(b[:])
		s.Close()
		os.Exit(0)
	}
	s, path := open(t)
	s.Close()
	holder := exec.Command(os.Args[0], "-test.run", "^TestSecondOpenerIsRefused$")
	holder.Env = append(os.Environ(), "SQLITE_HOLDER="+path)
	stdin, _ := holder.StdinPipe()
	stdout, _ := holder.StdoutPipe()
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); holder.Wait() }()
	var buf [5]byte
	if _, err := stdout.Read(buf[:]); err != nil || string(buf[:4]) != "held" {
		t.Fatalf("holder did not take the database: %q %v", buf, err)
	}
	if second, err := Open(path, false); err == nil {
		second.Close()
		t.Fatal("a second opener got the database while the first held it")
	} else if !strings.Contains(err.Error(), "in use") {
		t.Fatalf("refused for the wrong reason: %v", err)
	}
	stdin.Close()
	holder.Wait()
	// Released on close: the next start gets it.
	third, err := Open(path, false)
	if err != nil {
		t.Fatalf("a released database stayed locked: %v", err)
	}
	third.Close()
}

// schema4 is the layout 0.8.11 and earlier wrote, verbatim.
var schema4 = []string{
	`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE users (name TEXT PRIMARY KEY, id INTEGER NOT NULL UNIQUE, body TEXT NOT NULL)`,
	`CREATE TABLE records (name TEXT PRIMARY KEY, id INTEGER NOT NULL UNIQUE, kind TEXT NOT NULL, body TEXT NOT NULL)`,
	`CREATE TABLE accounts (account TEXT PRIMARY KEY, principal TEXT NOT NULL)`,
	`CREATE TABLE queues (name TEXT PRIMARY KEY, in_count INTEGER NOT NULL, out_count INTEGER NOT NULL, dropped INTEGER NOT NULL, expired INTEGER NOT NULL)`,
	`CREATE TABLE messages (queue TEXT NOT NULL, seq INTEGER NOT NULL, body TEXT NOT NULL, PRIMARY KEY (queue, seq))`,
	`CREATE TABLE credentials (name TEXT PRIMARY KEY, current TEXT NOT NULL, previous TEXT NOT NULL, issued TEXT NOT NULL, used TEXT NOT NULL DEFAULT '', user_id INTEGER NOT NULL DEFAULT 0, agent_id INTEGER NOT NULL DEFAULT 0)`,
	`INSERT INTO meta (key, value) VALUES ('owner', 'admin@h'), ('clean', '1')`,
	`INSERT INTO users (name, id, body) VALUES ('admin@h', 1, '{"name":"admin@h","status":"active"}')`,
	`INSERT INTO records (name, id, kind, body) VALUES ('svc@h', 2, 'agent', '{"name":"svc@h","kind":"agent","owner":"admin@h"}')`,
	`INSERT INTO queues (name, in_count, out_count, dropped, expired) VALUES ('svc@h', 5, 4, 0, 1)`,
	`INSERT INTO messages (queue, seq, body) VALUES ('svc@h', 0, '{"id":"m1","body":"kept"}')`,
	`INSERT INTO credentials (name, current, previous, issued) VALUES ('admin@h', 'hash', '', '')`,
	`PRAGMA user_version = 4`,
}

// A schema-4 database with data in it opens as schema 5, keeps every row,
// and then carries each queue's activity and the node's.
func TestSchemaFourMigratesWithItsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range schema4 {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	old.Close()
	s, err := Open(path, false)
	if err != nil {
		t.Fatalf("schema 4 did not open: %v", err)
	}
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != schema {
		t.Fatalf("user_version %d after migration: %v", version, err)
	}
	snap, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Owner != "admin@h" || !snap.Clean || len(snap.Users) != 1 || len(snap.Records) != 1 || snap.Activity != nil {
		t.Fatalf("meta, users or records lost: %+v", snap)
	}
	if len(snap.Queues) != 1 || snap.Queues[0].In != 5 || snap.Queues[0].Expired != 1 || len(snap.Queues[0].Messages) != 1 || len(snap.Queues[0].Activity) != 0 {
		t.Fatalf("queue after migration: %+v", snap.Queues)
	}
	if creds, err := s.Tokens().Load(); err != nil || len(creds) != 1 {
		t.Fatalf("credentials after migration: %+v %v", creds, err)
	}
	q := snap.Queues[0]
	q.Activity = []byte{1, 2, 3}
	if err := s.SaveQueues([]ports.Queue{q}, []byte{9, 8}, true); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	snap, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if string(snap.Queues[0].Activity) != "\x01\x02\x03" || string(snap.Activity) != "\x09\x08" {
		t.Fatalf("activity did not survive a reopen: queue %v node %v", snap.Queues[0].Activity, snap.Activity)
	}
	// nil leaves the node's day as it was saved.
	if err := s.SaveQueues(nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if snap, _ = s.Load(); string(snap.Activity) != "\x09\x08" {
		t.Fatalf("an empty save cleared the node's day: %v", snap.Activity)
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"svc@h": nil}, DropQueues: []string{"svc@h"}}); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := s.db.QueryRow(`SELECT count(*) FROM queues WHERE name = 'svc@h'`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("the removed record's queue row and its activity remain: %d %v", left, err)
	}
}
