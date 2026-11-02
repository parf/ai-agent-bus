package sqlite

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/parf/ai-agent-bus/internal/eventlog"
)

func eventIDKey(id uint32) string { return "event-high:" + strconv.FormatUint(uint64(id), 10) }
func eventHigh(raw string, has bool) (uint64, error) {
	if !has {
		return 0, nil
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n > eventlog.Safe || strconv.FormatUint(n, 10) != raw {
		return 0, fmt.Errorf("invalid event ID reservation")
	}
	return n, nil
}
func (s *Store) EventHighWater(id uint32) (uint64, error) {
	raw, has, err := meta(s.db, eventIDKey(id))
	if err != nil {
		return 0, err
	}
	return eventHigh(raw, has)
}
func (s *Store) ReserveEventIDs(id uint32, through uint64) error {
	if id == 0 || through == 0 || through > eventlog.Safe {
		return fmt.Errorf("invalid event ID reservation")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	raw, has, err := meta(tx, eventIDKey(id))
	if err != nil {
		return err
	}
	old, err := eventHigh(raw, has)
	if err != nil {
		return err
	}
	if through < old {
		return fmt.Errorf("event ID reservation cannot decrease")
	}
	if err = setMeta(tx, eventIDKey(id), strconv.FormatUint(through, 10)); err != nil {
		return err
	}
	return tx.Commit()
}

func validateEventIDs(tx interface {
	Query(string, ...any) (*sql.Rows, error)
}) error {
	rows, err := tx.Query(`SELECT key,value FROM meta WHERE key LIKE 'event-high:%'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			return err
		}
		raw := strings.TrimPrefix(key, "event-high:")
		id, er := strconv.ParseUint(raw, 10, 32)
		if er != nil || id == 0 || strconv.FormatUint(id, 10) != raw {
			return fmt.Errorf("invalid event record ID")
		}
		if _, err = eventHigh(value, true); err != nil {
			return err
		}
	}
	return rows.Err()
}
