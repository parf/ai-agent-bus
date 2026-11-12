package sqlite

// Each record's schedules and their outcomes, keyed by the record's internal
// ID, which is never reused (Plans/R1.0-Release/cron-in-core.md#storage).

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/parf/ai-agent-bus/internal/ports"
)

var scheduleTables = []string{
	`CREATE TABLE schedules (id TEXT PRIMARY KEY, record_id INTEGER NOT NULL, once INTEGER NOT NULL, enabled INTEGER NOT NULL, next INTEGER NOT NULL, in_flight INTEGER NOT NULL, body TEXT NOT NULL) WITHOUT ROWID`,
	`CREATE INDEX schedules_due ON schedules (enabled, next)`,
	`CREATE INDEX schedules_record ON schedules (record_id)`,
	`CREATE TABLE schedule_outcomes (record_id INTEGER NOT NULL, schedule_id TEXT NOT NULL, at INTEGER NOT NULL, seq INTEGER NOT NULL, body TEXT NOT NULL, PRIMARY KEY (record_id, at, seq)) WITHOUT ROWID`,
}

// dropSchedules removes the schedules and outcomes of the record stored under
// name, unless its ID is keep. Caller holds tx.
func dropSchedules(tx *sql.Tx, name string, keep uint32) error {
	for _, t := range []string{"schedules", "schedule_outcomes"} {
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE record_id IN (SELECT id FROM records WHERE name = ? AND id <> ?)`, name, keep); err != nil {
			return err
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanSchedule(row interface{ Scan(...any) error }) (ports.Schedule, error) {
	var body string
	if err := row.Scan(&body); err != nil {
		return ports.Schedule{}, err
	}
	var s ports.Schedule
	err := json.Unmarshal([]byte(body), &s)
	return s, err
}

func (s *Store) schedules(query string, args ...any) ([]ports.Schedule, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.Schedule{}
	for rows.Next() {
		v, err := scanSchedule(rows)
		var bad *json.SyntaxError
		var typ *json.UnmarshalTypeError
		if errors.As(err, &bad) || errors.As(err, &typ) {
			continue // set aside: ScheduleBroken names it, nothing fires it
		}
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ScheduleBroken is the id of every stored schedule that does not decode.
func (s *Store) ScheduleBroken() ([]string, error) {
	rows, err := s.db.Query(`SELECT id, body FROM schedules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, body string
		if err := rows.Scan(&id, &body); err != nil {
			return nil, err
		}
		var v ports.Schedule
		if json.Unmarshal([]byte(body), &v) != nil || v.ID != id {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

// ScheduleGet reads one schedule.
func (s *Store) ScheduleGet(id string) (ports.Schedule, bool, error) {
	v, err := scanSchedule(s.db.QueryRow(`SELECT body FROM schedules WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ports.Schedule{}, false, nil
	}
	return v, err == nil, err
}

// ScheduleEdit reads, edits and writes one schedule in one transaction.
func (s *Store) ScheduleEdit(id string, record uint32, edit func(ports.Schedule, bool, ports.ScheduleCount) (ports.ScheduleChange, error)) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var held int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM records WHERE id = ?`, record).Scan(&held); err != nil {
		return err
	}
	if held == 0 {
		return ports.ErrKVNoRecord
	}
	old, err := scanSchedule(tx.QueryRow(`SELECT body FROM schedules WHERE id = ? AND record_id = ?`, id, record))
	found := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var count ports.ScheduleCount
	if err := tx.QueryRow(`SELECT COALESCE(SUM(once = 0), 0), COALESCE(SUM(once = 1), 0) FROM schedules WHERE record_id = ?`, record).Scan(&count.Recurring, &count.Once); err != nil {
		return err
	}
	c, err := edit(old, found, count)
	if err != nil {
		return err
	}
	switch {
	case c.Delete:
		if _, err := tx.Exec(`DELETE FROM schedules WHERE id = ?`, id); err != nil {
			return err
		}
	case c.Write:
		v := c.Value
		v.ID, v.Record = id, record
		body, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO schedules (id, record_id, once, enabled, next, in_flight, body) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET once = excluded.once, enabled = excluded.enabled, next = excluded.next, in_flight = excluded.in_flight, body = excluded.body`,
			id, record, boolInt(v.Once), boolInt(v.Enabled), v.Next, v.InFlight, body); err != nil {
			return err
		}
	}
	if o := c.Outcome; o != nil {
		o.Schedule, o.Record = id, record
		body, err := json.Marshal(o)
		if err != nil {
			return err
		}
		var seq int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM schedule_outcomes WHERE record_id = ? AND at = ?`, record, o.At).Scan(&seq); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO schedule_outcomes (record_id, schedule_id, at, seq, body) VALUES (?, ?, ?, ?, ?)`, record, id, o.At, seq, body); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM schedule_outcomes WHERE record_id = ? AND at < ?`, record, o.At-ports.ScheduleOutcomeAge); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM schedule_outcomes WHERE record_id = ? AND (at, seq) NOT IN (SELECT at, seq FROM schedule_outcomes WHERE record_id = ? ORDER BY at DESC, seq DESC LIMIT ?)`, record, record, ports.ScheduleOutcomesKept); err != nil {
			return err
		}
	}
	if !c.Write && !c.Delete && c.Outcome == nil {
		return nil
	}
	return tx.Commit()
}

// ScheduleList is every schedule record holds.
func (s *Store) ScheduleList(record uint32) ([]ports.Schedule, error) {
	return s.schedules(`SELECT body FROM schedules WHERE record_id = ? ORDER BY id`, record)
}

// ScheduleDue is the enabled schedules due at now, earliest first.
func (s *Store) ScheduleDue(now int64, limit int) ([]ports.Schedule, error) {
	return s.schedules(`SELECT body FROM schedules WHERE enabled = 1 AND next > 0 AND next <= ? ORDER BY next, id LIMIT ?`, now, limit)
}

// ScheduleNext is the earliest next slot of any enabled schedule.
func (s *Store) ScheduleNext() (int64, bool, error) {
	var next sql.NullInt64
	if err := s.db.QueryRow(`SELECT MIN(next) FROM schedules WHERE enabled = 1 AND next > 0`).Scan(&next); err != nil {
		return 0, false, err
	}
	return next.Int64, next.Valid, nil
}

// ScheduleInFlight is every schedule with a claimed, unrecorded firing.
func (s *Store) ScheduleInFlight() ([]ports.Schedule, error) {
	return s.schedules(`SELECT body FROM schedules WHERE in_flight > 0 ORDER BY id`)
}

// ScheduleCounts is how many schedules each record holding any has.
func (s *Store) ScheduleCounts() (map[uint32]ports.ScheduleCount, error) {
	rows, err := s.db.Query(`SELECT record_id, SUM(once = 0), SUM(once = 1) FROM schedules GROUP BY record_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uint32]ports.ScheduleCount{}
	for rows.Next() {
		var id uint32
		var c ports.ScheduleCount
		if err := rows.Scan(&id, &c.Recurring, &c.Once); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}

// ScheduleOutcomes is record's kept outcomes, newest first.
func (s *Store) ScheduleOutcomes(record uint32, schedule string) ([]ports.ScheduleOutcome, error) {
	q, args := `SELECT body FROM schedule_outcomes WHERE record_id = ? ORDER BY at DESC, seq DESC`, []any{record}
	if schedule != "" {
		q, args = `SELECT body FROM schedule_outcomes WHERE record_id = ? AND schedule_id = ? ORDER BY at DESC, seq DESC`, []any{record, schedule}
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.ScheduleOutcome{}
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		var o ports.ScheduleOutcome
		if err := json.Unmarshal([]byte(body), &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ScheduleOrphans lists record IDs holding schedules that name no stored record.
func (s *Store) ScheduleOrphans() ([]uint32, error) {
	rows, err := s.db.Query(`SELECT DISTINCT record_id FROM schedules WHERE record_id NOT IN (SELECT id FROM records) ORDER BY record_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uint32
	for rows.Next() {
		var id uint32
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// addSchedules brings a schema 11 database to 12: the schedule tables, in one
// transaction with the version.
func (s *Store) addSchedules() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range scheduleTables {
		if _, err := tx.Exec(t); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`PRAGMA user_version = 12`); err != nil {
		return err
	}
	return tx.Commit()
}
