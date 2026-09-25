// Package sqlite is the default durable store: registry records, users,
// groups, the daemon Owner, the local-account map, credentials and queue
// state, in one SQLite database behind the store ports
// (docs/constitution.md#persistence-and-loading).
//
// It takes SQLite's exclusive lock when it opens and keeps it for as long as
// it is open, so a second daemon pointed at the same file cannot serve. A
// missing database is created only when asked for; an unreadable, damaged or
// incompatible one is an error, never an empty node.
package sqlite

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"

	_ "modernc.org/sqlite"
)

// schema is the layout this daemon reads and writes. A database at an older
// version with a migration below is brought up to it at open, in one
// transaction; any other version is refused rather than guessed at.
const schema = 6

var tables = []string{
	`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE users (name TEXT PRIMARY KEY, id INTEGER NOT NULL UNIQUE, body TEXT NOT NULL)`,
	`CREATE TABLE records (name TEXT PRIMARY KEY, id INTEGER NOT NULL UNIQUE, kind TEXT NOT NULL, body TEXT NOT NULL)`,
	`CREATE TABLE accounts (account TEXT PRIMARY KEY, principal TEXT NOT NULL)`,
	`CREATE TABLE queues (name TEXT PRIMARY KEY, in_count INTEGER NOT NULL, out_count INTEGER NOT NULL, dropped INTEGER NOT NULL, expired INTEGER NOT NULL, activity BLOB NOT NULL DEFAULT x'')`,
	`CREATE TABLE messages (queue TEXT NOT NULL, seq INTEGER NOT NULL, body TEXT NOT NULL, PRIMARY KEY (queue, seq))`,
	`CREATE TABLE activity_days (day INTEGER NOT NULL, name TEXT NOT NULL, slots BLOB NOT NULL, PRIMARY KEY (day, name))`,
	`CREATE TABLE credentials (name TEXT PRIMARY KEY, current TEXT NOT NULL, previous TEXT NOT NULL, issued TEXT NOT NULL, used TEXT NOT NULL DEFAULT '', user_id INTEGER NOT NULL DEFAULT 0, agent_id INTEGER NOT NULL DEFAULT 0)`,
}

// migrations[v] takes a database from schema v to v+1.
var migrations = map[int][]string{
	// 0.8.12: a record's day of activity is saved with its queue, so it goes
	// with the queue's row (docs/05-discovery.md#activity-history).
	4: {`ALTER TABLE queues ADD COLUMN activity BLOB NOT NULL DEFAULT x''`},
	// 0.8.41: every name's traffic is kept per calendar day, beyond the ring's
	// last 24 hours (docs/05-discovery.md#activity-history).
	5: {`CREATE TABLE activity_days (day INTEGER NOT NULL, name TEXT NOT NULL, slots BLOB NOT NULL, PRIMARY KEY (day, name))`},
}

// ErrMissing is a database that is not there and was not asked to be made.
var ErrMissing = errors.New("no database")

// Store is one open database, held exclusively.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens the database at path, creating it only when create is set.
func Open(path string, create bool) (*Store, error) {
	fresh := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if !create {
			return nil, fmt.Errorf("%w at %s: initialize it with agent-bus-setup, or start the daemon with -create", ErrMissing, path)
		}
		// Made here, empty and private, so SQLite never creates it with a
		// wider mode; its journal takes the same mode as the database.
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		f.Close()
		fresh = true
	} else if err != nil {
		return nil, err
	}
	// locking_mode comes first: set before WAL is entered, SQLite keeps the
	// lock for the life of the connection and needs no shared-memory file.
	dsn := "file:" + path +
		"?_pragma=locking_mode(EXCLUSIVE)" +
		"&_pragma=busy_timeout(0)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(FULL)" +
		"&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection: the exclusive lock belongs to a connection, and a
	// second one in the same pool would be refused by the first.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err := s.claim(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.prepare(fresh); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// claim takes the exclusive lock with a write, which in EXCLUSIVE locking
// mode is held until Close.
func (s *Store) claim() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS lock_claim (x INTEGER); DROP TABLE lock_claim`); err != nil {
		if strings.Contains(err.Error(), "locked") || strings.Contains(err.Error(), "busy") {
			return fmt.Errorf("database %s is in use by another daemon: %w", s.path, err)
		}
		return fmt.Errorf("database %s: %w", s.path, err)
	}
	return nil
}

func (s *Store) prepare(fresh bool) error {
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("database %s: %w", s.path, err)
	}
	if fresh && version == 0 {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, t := range tables {
			if _, err := tx.Exec(t); err != nil {
				return fmt.Errorf("create schema: %w", err)
			}
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schema)); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		version = schema
	}
	if steps, known := migrations[version]; known && version < schema {
		if err := s.migrate(version, steps); err != nil {
			return fmt.Errorf("database %s: migrate schema %d: %w", s.path, version, err)
		}
		return s.prepare(false)
	}
	if version != schema {
		return fmt.Errorf("database %s has schema %d; this daemon reads %d", s.path, version, schema)
	}
	var check string
	if err := s.db.QueryRow(`PRAGMA quick_check`).Scan(&check); err != nil {
		return fmt.Errorf("database %s: %w", s.path, err)
	}
	if check != "ok" {
		return fmt.Errorf("database %s failed its integrity check: %s", s.path, check)
	}
	return nil
}

func (s *Store) migrate(from int, steps []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, step := range steps {
		if _, err := tx.Exec(step); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, from+1)); err != nil {
		return err
	}
	return tx.Commit()
}

// Close releases the database and its lock.
func (s *Store) Close() error { return s.db.Close() }

func meta(tx interface {
	QueryRow(string, ...any) *sql.Row
}, key string) (string, bool, error) {
	var v string
	err := tx.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func setMeta(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// Load reads the whole durable state.
func (s *Store) Load() (ports.Snapshot, error) {
	var snap ports.Snapshot
	tx, err := s.db.Begin()
	if err != nil {
		return snap, err
	}
	defer tx.Rollback()
	if owner, has, err := meta(tx, "owner"); err != nil {
		return snap, err
	} else if has {
		snap.OwnerEstablished, snap.Owner = true, owner
	}
	if _, has, err := meta(tx, "accounts_established"); err != nil {
		return snap, err
	} else {
		snap.AccountsEstablished = has
	}
	if v, _, err := meta(tx, "clean"); err != nil {
		return snap, err
	} else {
		snap.Clean = v == "1"
	}
	if v, has, err := meta(tx, "at"); err != nil {
		return snap, err
	} else if has {
		snap.At, _ = time.Parse(time.RFC3339Nano, v)
	}
	if v, has, err := meta(tx, "activity"); err != nil {
		return snap, err
	} else if has {
		if snap.Activity, err = base64.StdEncoding.DecodeString(v); err != nil {
			return snap, fmt.Errorf("meta activity: %w", err)
		}
	}
	rows, err := tx.Query(`SELECT account, principal FROM accounts ORDER BY account`)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var m protocol.AccountMapping
		if err := rows.Scan(&m.Account, &m.Principal); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Accounts = append(snap.Accounts, m)
	}
	rows.Close()
	if err := eachIDBody(tx, `SELECT name, id, body FROM users ORDER BY name`, func(name string, id uint32, body []byte) error {
		var u protocol.User
		if err := json.Unmarshal(body, &u); err != nil {
			return fmt.Errorf("user %s: %w", name, err)
		}
		u.ID = id
		snap.Users = append(snap.Users, u)
		return nil
	}); err != nil {
		return snap, err
	}
	if err := eachIDBody(tx, `SELECT name, id, body FROM records ORDER BY name`, func(name string, id uint32, body []byte) error {
		var r protocol.Record
		if err := json.Unmarshal(body, &r); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
		r.ID = id
		snap.Records = append(snap.Records, r)
		return nil
	}); err != nil {
		return snap, err
	}
	for key, into := range map[string]*uint32{"next_record_id": &snap.NextRecordID, "next_user_id": &snap.NextUserID} {
		v, has, err := meta(tx, key)
		if err != nil {
			return snap, err
		}
		if has {
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return snap, fmt.Errorf("meta %s: %w", key, err)
			}
			*into = uint32(n)
		}
	}
	queues := map[string]*ports.Queue{}
	var order []string
	rows, err = tx.Query(`SELECT name, in_count, out_count, dropped, expired, activity FROM queues ORDER BY name`)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		q := &ports.Queue{}
		if err := rows.Scan(&q.Name, &q.In, &q.Out, &q.Dropped, &q.Expired, &q.Activity); err != nil {
			rows.Close()
			return snap, err
		}
		queues[q.Name] = q
		order = append(order, q.Name)
	}
	rows.Close()
	rows, err = tx.Query(`SELECT queue, body FROM messages ORDER BY queue, seq`)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var name string
		var body []byte
		if err := rows.Scan(&name, &body); err != nil {
			rows.Close()
			return snap, err
		}
		q, known := queues[name]
		if !known {
			rows.Close()
			return snap, fmt.Errorf("messages stored for queue %s, which has no queue row", name)
		}
		var e protocol.Envelope
		if err := json.Unmarshal(body, &e); err != nil {
			rows.Close()
			return snap, fmt.Errorf("queue %s: %w", name, err)
		}
		q.Messages = append(q.Messages, e)
	}
	rows.Close()
	for _, name := range order {
		snap.Queues = append(snap.Queues, *queues[name])
	}
	return snap, tx.Commit()
}

func eachIDBody(tx *sql.Tx, query string, each func(string, uint32, []byte) error) error {
	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var id int64
		var body []byte
		if err := rows.Scan(&name, &id, &body); err != nil {
			return err
		}
		if id <= 0 || id > 1<<32-1 {
			return fmt.Errorf("%s has an id out of range: %d", name, id)
		}
		if err := each(name, uint32(id), body); err != nil {
			return err
		}
	}
	return rows.Err()
}

func eachBody(tx *sql.Tx, query string, each func(string, []byte) error) error {
	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var body []byte
		if err := rows.Scan(&name, &body); err != nil {
			return err
		}
		if err := each(name, body); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Commit writes one management change as one transaction.
func (s *Store) Commit(c ports.Change) error {
	if c.Empty() {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if c.Owner != nil {
		if err := setMeta(tx, "owner", *c.Owner); err != nil {
			return err
		}
	}
	if c.Accounts != nil {
		if _, err := tx.Exec(`DELETE FROM accounts`); err != nil {
			return err
		}
		accounts := make([]string, 0, len(c.Accounts))
		for account := range c.Accounts {
			accounts = append(accounts, account)
		}
		sort.Strings(accounts)
		for _, account := range accounts {
			if _, err := tx.Exec(`INSERT INTO accounts (account, principal) VALUES (?, ?)`, account, c.Accounts[account]); err != nil {
				return err
			}
		}
		if err := setMeta(tx, "accounts_established", "1"); err != nil {
			return err
		}
	}
	for name, u := range c.Users {
		if u == nil {
			if _, err := tx.Exec(`DELETE FROM users WHERE name = ?`, name); err != nil {
				return err
			}
			continue
		}
		body, err := json.Marshal(u)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO users (name, id, body) VALUES (?, ?, ?) ON CONFLICT(name) DO UPDATE SET id = excluded.id, body = excluded.body`, name, u.ID, body); err != nil {
			return err
		}
	}
	for name, r := range c.Records {
		if r == nil {
			if _, err := tx.Exec(`DELETE FROM records WHERE name = ?`, name); err != nil {
				return err
			}
			continue
		}
		body, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO records (name, id, kind, body) VALUES (?, ?, ?, ?) ON CONFLICT(name) DO UPDATE SET id = excluded.id, kind = excluded.kind, body = excluded.body`, name, r.ID, r.Kind, body); err != nil {
			return err
		}
	}
	if c.NextRecordID != nil {
		if err := setMeta(tx, "next_record_id", strconv.FormatUint(uint64(*c.NextRecordID), 10)); err != nil {
			return err
		}
	}
	if c.NextUserID != nil {
		if err := setMeta(tx, "next_user_id", strconv.FormatUint(uint64(*c.NextUserID), 10)); err != nil {
			return err
		}
	}
	for name, p := range c.Credentials {
		if p == nil {
			if _, err := tx.Exec(`DELETE FROM credentials WHERE name = ?`, name); err != nil {
				return err
			}
		} else if _, err := tx.Exec(`UPDATE credentials SET user_id = ?, agent_id = ? WHERE name = ?`, p.UserID, p.AgentID, name); err != nil {
			return err
		}
	}
	for _, name := range c.DropQueues {
		if _, err := tx.Exec(`DELETE FROM messages WHERE queue = ?`, name); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM queues WHERE name = ?`, name); err != nil {
			return err
		}
		// Its history goes with it: every stored reference to the name does.
		if _, err := tx.Exec(`DELETE FROM activity_days WHERE name = ?`, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SaveQueues replaces the given queues' state, and the node's activity, as
// one batch.
func (s *Store) SaveQueues(qs []ports.Queue, activity []byte, clean bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range qs {
		if _, err := tx.Exec(`DELETE FROM messages WHERE queue = ?`, q.Name); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO queues (name, in_count, out_count, dropped, expired, activity) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET in_count = excluded.in_count, out_count = excluded.out_count, dropped = excluded.dropped, expired = excluded.expired, activity = excluded.activity`,
			q.Name, q.In, q.Out, q.Dropped, q.Expired, blob(q.Activity)); err != nil {
			return err
		}
		for i, e := range q.Messages {
			body, err := json.Marshal(e)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO messages (queue, seq, body) VALUES (?, ?, ?)`, q.Name, i, body); err != nil {
				return err
			}
		}
	}
	c := "0"
	if clean {
		c = "1"
	}
	if err := setMeta(tx, "clean", c); err != nil {
		return err
	}
	if activity != nil {
		if err := setMeta(tx, "activity", base64.StdEncoding.EncodeToString(activity)); err != nil {
			return err
		}
	}
	if err := setMeta(tx, "at", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveActivityDays writes the given days whole, in one transaction.
func (s *Store) SaveActivityDays(days []ports.ActivityDay) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, d := range days {
		if len(d.Slots) == 0 {
			if _, err := tx.Exec(`DELETE FROM activity_days WHERE day = ? AND name = ?`, d.Date, d.Name); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(`INSERT INTO activity_days (day, name, slots) VALUES (?, ?, ?)
			ON CONFLICT(day, name) DO UPDATE SET slots = excluded.slots`, d.Date, d.Name, d.Slots); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ActivityDays reads every row from one date to another, both included, of
// the names given or of all of them.
func (s *Store) ActivityDays(from, to int, names []string) ([]ports.ActivityDay, error) {
	var out []ports.ActivityDay
	read := func(q string, args ...any) error {
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d ports.ActivityDay
			if err := rows.Scan(&d.Date, &d.Name, &d.Slots); err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	}
	if names == nil {
		return out, read(`SELECT day, name, slots FROM activity_days WHERE day BETWEEN ? AND ? ORDER BY day, name`, from, to)
	}
	// In batches, under SQLite's bound-parameter limit.
	for len(names) > 0 {
		n := min(len(names), 500)
		args := []any{from, to}
		for _, name := range names[:n] {
			args = append(args, name)
		}
		q := `SELECT day, name, slots FROM activity_days WHERE day BETWEEN ? AND ? AND name IN (?` + strings.Repeat(",?", n-1) + `)`
		if err := read(q, args...); err != nil {
			return nil, err
		}
		names = names[n:]
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// PruneActivity drops the rows older than before.
func (s *Store) PruneActivity(before int) error {
	_, err := s.db.Exec(`DELETE FROM activity_days WHERE day < ?`, before)
	return err
}

// Tokens is the credential half of the same database, behind the token port.
func (s *Store) Tokens() ports.TokenStore { return tokens{s} }

type tokens struct{ s *Store }

func (t tokens) Load() ([]ports.Credential, error) {
	rows, err := t.s.db.Query(`SELECT name, current, previous, issued, used, user_id, agent_id FROM credentials ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.Credential
	for rows.Next() {
		var c ports.Credential
		var issued, used string
		if err := rows.Scan(&c.Name, &c.Current, &c.Previous, &issued, &used, &c.UserID, &c.AgentID); err != nil {
			return nil, err
		}
		if c.Issued, err = parseTime(issued); err != nil {
			return nil, fmt.Errorf("credential %s: %w", c.Name, err)
		}
		if c.Used, err = parseTime(used); err != nil {
			return nil, fmt.Errorf("credential %s: %w", c.Name, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (t tokens) Put(c ports.Credential) error {
	_, err := t.s.db.Exec(`INSERT INTO credentials (name, current, previous, issued, used, user_id, agent_id) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET current = excluded.current, previous = excluded.previous, issued = excluded.issued,
		used = excluded.used, user_id = excluded.user_id, agent_id = excluded.agent_id`,
		c.Name, c.Current, c.Previous, formatTime(c.Issued), formatTime(c.Used), c.UserID, c.AgentID)
	return err
}

func (t tokens) Drop(name string) error {
	_, err := t.s.db.Exec(`DELETE FROM credentials WHERE name = ?`, name)
	return err
}

func (t tokens) Touch(used map[string]time.Time) error {
	if len(used) == 0 {
		return nil
	}
	tx, err := t.s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for name, at := range used {
		if _, err := tx.Exec(`UPDATE credentials SET used = ? WHERE name = ?`, formatTime(at), name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// blob is never NULL, which the column refuses.
func blob(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}
