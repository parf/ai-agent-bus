package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Durable agent-event alerts (Plans/R1.0-Release/agent-events/README.md#flood-and-persistence).
// One row per record says whether its log exists (active) and one row per
// retained alert holds the entry, both keyed by the record's internal ID, which
// is never reused: a new registration under an old name starts with nothing.
// Notices and warnings are never stored. The engine bounds what it asks to
// keep; the store keeps exactly the snapshot it was given, whole or not at all.

const eventLogsTable = `CREATE TABLE event_logs (record_id INTEGER PRIMARY KEY, active INTEGER NOT NULL) WITHOUT ROWID`
const eventAlertsTable = `CREATE TABLE event_alerts (record_id INTEGER NOT NULL, id INTEGER NOT NULL, text TEXT NOT NULL, at TEXT NOT NULL, last_at TEXT NOT NULL, count INTEGER NOT NULL, PRIMARY KEY (record_id, id)) WITHOUT ROWID`

// maxEventAlerts is the shared model's ceiling on one log's rings together.
const maxEventAlerts = 500

// LoadEventAlerts returns the record's alert snapshot and whether its log is
// active. A record never written is empty and inactive. Anything damaged is an
// error for this record only: nothing else reads these rows, so one record's
// bad row never stops another's log or the daemon's own load.
func (s *Store) LoadEventAlerts(recordID uint32) ([]ports.EventEntry, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var flag int64
	has := true
	if err = tx.QueryRow(`SELECT active FROM event_logs WHERE record_id = ?`, recordID).Scan(&flag); errors.Is(err, sql.ErrNoRows) {
		has = false
	} else if err != nil {
		return nil, false, fmt.Errorf("event log state: %w: %v", ports.ErrEventStateInvalid, err)
	}
	if has && flag != 0 && flag != 1 {
		return nil, false, fmt.Errorf("event log state: invalid active flag: %w", ports.ErrEventStateInvalid)
	}
	rows, err := tx.Query(`SELECT id, text, at, last_at, count FROM event_alerts WHERE record_id = ? ORDER BY id`, recordID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []ports.EventEntry
	for rows.Next() {
		var id, count int64
		var e ports.EventEntry
		if err = rows.Scan(&id, &e.Text, &e.At, &e.LastAt, &count); err != nil {
			return nil, false, fmt.Errorf("event alert row: %w: %v", ports.ErrEventStateInvalid, err)
		}
		if id <= 0 || count <= 0 {
			return nil, false, fmt.Errorf("event alert row: invalid id or count: %w", ports.ErrEventStateInvalid)
		}
		e.ID, e.Count, e.Level = uint64(id), uint64(count), "alert"
		out = append(out, e)
		if len(out) > maxEventAlerts {
			return nil, false, fmt.Errorf("event alert rows: more than %d: %w", maxEventAlerts, ports.ErrEventStateInvalid)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, false, fmt.Errorf("event alert rows: %w", err)
	}
	if !has {
		if len(out) > 0 {
			return nil, false, fmt.Errorf("event alert rows without a log state: %w", ports.ErrEventStateInvalid)
		}
		return nil, false, nil
	}
	return out, flag == 1, nil
}

// ReplaceEventAlerts stores the whole snapshot and the active flag in one
// transaction: the previous alerts are removed and the new ones written, or
// nothing changes.
func (s *Store) ReplaceEventAlerts(recordID uint32, alerts []ports.EventEntry, active bool) error {
	return s.ReplaceEventAlertBatch(context.Background(), []ports.EventAlertSnapshot{{RecordID: recordID, Alerts: alerts, Active: active}})
}

func (s *Store) ReplaceEventAlertBatch(ctx context.Context, batch []ports.EventAlertSnapshot) error {
	for _, v := range batch {
		if v.RecordID == 0 || len(v.Alerts) > maxEventAlerts {
			return fmt.Errorf("invalid event alert snapshot")
		}
		var last uint64
		for _, e := range v.Alerts {
			if e.Level != "alert" || e.ID == 0 || e.ID <= last || e.Count == 0 || e.ID > ports.EventMaxID || e.Count > ports.EventMaxID {
				return fmt.Errorf("invalid event alert snapshot")
			}
			last = e.ID
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, v := range batch {
		flag := 0
		if v.Active {
			flag = 1
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO event_logs (record_id, active) VALUES (?, ?) ON CONFLICT(record_id) DO UPDATE SET active = excluded.active`, v.RecordID, flag); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM event_alerts WHERE record_id = ?`, v.RecordID); err != nil {
			return err
		}
		for _, e := range v.Alerts {
			if _, err = tx.ExecContext(ctx, `INSERT INTO event_alerts (record_id, id, text, at, last_at, count) VALUES (?, ?, ?, ?, ?, ?)`,
				v.RecordID, e.ID, e.Text, e.At, e.LastAt, e.Count); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// dropEventAlerts removes the alerts and the id counter of the record stored under name, unless
// its ID is keep: a record's private log goes with it, and a name registered
// again under a new ID starts empty. Caller holds tx.
func dropEventAlerts(tx *sql.Tx, name string, keep uint32) error {
	for _, t := range []string{"event_alerts", "event_logs"} {
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE record_id IN (SELECT id FROM records WHERE name = ? AND id <> ?)`, name, keep); err != nil {
			return err
		}
	}
	// The record's id is never reused, so its reserved-id counter goes too.
	_, err := tx.Exec(`DELETE FROM meta WHERE key IN (SELECT 'event-high:' || id FROM records WHERE name = ? AND id <> ?)`, name, keep)
	return err
}
