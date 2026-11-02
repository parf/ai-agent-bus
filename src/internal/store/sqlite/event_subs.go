package sqlite

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// The subscriptions of agent logs: one row per subscription, the stored JSON
// opaque here, keyed by its id (unique across sources) and the source record's
// internal ID, which is never reused, so a new registration under an old name
// starts with none.

const eventSubsTable = `CREATE TABLE event_subs (id TEXT PRIMARY KEY, source_id INTEGER NOT NULL, body TEXT NOT NULL) WITHOUT ROWID`
const eventSubsIndex = `CREATE INDEX event_subs_source ON event_subs (source_id)`

func (s *Store) ListSubscriptions(sourceID uint32) (map[string][]byte, error) {
	rows, err := s.db.Query(`SELECT id, body FROM event_subs WHERE source_id = ?`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var id, body string
		if err = rows.Scan(&id, &body); err != nil {
			return nil, err
		}
		out[id] = []byte(body)
	}
	return out, rows.Err()
}

func (s *Store) AddSubscription(sourceID uint32, id string, value []byte) error {
	if sourceID == 0 || id == "" {
		return errors.New("invalid subscription")
	}
	_, err := s.db.Exec(`INSERT INTO event_subs (id, source_id, body) VALUES (?, ?, ?)`, id, sourceID, string(value))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ports.ErrSubscriptionExists
	}
	return err
}

func (s *Store) DeleteSubscription(sourceID uint32, id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM event_subs WHERE id = ? AND source_id = ?`, id, sourceID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) SetSubscriptionCounts(sourceID uint32, id string, sent, dropped int64) error {
	res, err := s.db.Exec(`UPDATE event_subs SET body = json_set(json_set(body, '$.sent', ?), '$.dropped', ?) WHERE id = ? AND source_id = ? AND json_valid(body)`, sent, dropped, id, sourceID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		err = sql.ErrNoRows
	}
	return err
}

func (s *Store) FindSubscription(id string) (uint32, bool, error) {
	var source uint32
	err := s.db.QueryRow(`SELECT source_id FROM event_subs WHERE id = ?`, id).Scan(&source)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return source, err == nil, err
}

func (s *Store) SubscriptionSources(creator string) ([]uint32, error) {
	rows, err := s.db.Query(`SELECT DISTINCT source_id FROM event_subs WHERE CASE WHEN json_valid(body) THEN json_extract(body, '$.created_by') END = ? ORDER BY source_id`, creator)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uint32
	for rows.Next() {
		var id uint32
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// dropEventSubs removes, for the record stored under name unless its ID is
// keep, the subscriptions of its log and the ones it authored: its ID is gone,
// so a name registered again must not inherit either. Caller holds tx.
func dropEventSubs(tx *sql.Tx, name string, keep uint32) error {
	if _, err := tx.Exec(`DELETE FROM event_subs WHERE source_id IN (SELECT id FROM records WHERE name = ? AND id <> ?)`, name, keep); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM event_subs WHERE CASE WHEN json_valid(body) THEN json_extract(body, '$.created_by') END = ? AND EXISTS (SELECT 1 FROM records WHERE name = ? AND id <> ?)`, name, name, keep)
	return err
}
